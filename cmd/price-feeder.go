package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/persistenceOne/oracle-feeder/oracle/provider"
	v1 "github.com/persistenceOne/oracle-feeder/router/v1"

	"github.com/persistenceOne/oracle-feeder/config"
	"github.com/persistenceOne/oracle-feeder/oracle"
	"github.com/persistenceOne/oracle-feeder/oracle/client"
)

const (
	flagLogLevel  = "log-level"
	flagLogFormat = "log-format"
)

var rootCmd = &cobra.Command{
	Use:   "price-feeder [config-file]",
	Args:  cobra.ExactArgs(1),
	Short: "price-feeder is a side-car process for providing on-chain oracle with price data",
	Long: `A side-car process that validators must run in order to provide
on-chain price oracle with price information. The price-feeder performs
two primary functions. First, it is responsible for obtaining price information
from various reliable data sources, e.g. exchanges, and exposing this data via
an API. Secondly, the price-feeder consumes this data and periodically submits
vote and prevote messages following the oracle voting procedure.`,
	RunE: priceFeederCmdHandler,
}

func init() {
	setConfig()
	rootCmd.PersistentFlags().String(flagLogLevel, zerolog.InfoLevel.String(), "logging level")
	rootCmd.PersistentFlags().String(flagLogFormat, logLevelText, "logging format; must be either json or text")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

//nolint:funlen //No need to split this function
func priceFeederCmdHandler(cmd *cobra.Command, args []string) error {
	logLvlStr, err := cmd.Flags().GetString(flagLogLevel)
	if err != nil {
		return err
	}

	logFormatStr, err := cmd.Flags().GetString(flagLogFormat)
	if err != nil {
		return err
	}

	logger, err := setUpLogger(logLvlStr, strings.ToLower(logFormatStr))
	if err != nil {
		return fmt.Errorf("failed to set up logger: %w", err)
	}

	cfg, err := config.ParseConfig(args[0])
	if err != nil {
		return err
	}

	err = config.CheckProviderMinimum(cmd.Context(), logger, cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	// listen for and trap any OS signal to gracefully shutdown and exit
	trapSignal(cancel, logger)

	timeout, err := time.ParseDuration(cfg.RPC.RPCTimeout)
	if err != nil {
		return fmt.Errorf("failed to parse RPC timeout: %w", err)
	}

	// Gather pass via env variable || std input
	keyringPass, err := getKeyringPassword()
	if err != nil {
		return err
	}

	oracleClient, err := client.NewOracleClient(
		ctx,
		logger,
		cfg.Account.ChainID,
		cfg.Keyring.Backend,
		cfg.Keyring.Dir,
		keyringPass,
		cfg.RPC.TMRPCEndpoint,
		timeout,
		cfg.Account.Address,
		cfg.Account.Validator,
		cfg.RPC.GRPCEndpoint,
		cfg.GasAdjustment,
		cfg.Fees,
	)
	if err != nil {
		return err
	}

	providerTimeout, err := time.ParseDuration(cfg.ProviderTimeout)
	if err != nil {
		return fmt.Errorf("failed to parse provider timeout: %w", err)
	}

	deviations := make(map[string]sdk.Dec, len(cfg.Deviations))
	for _, deviation := range cfg.Deviations {
		threshold, err := sdk.NewDecFromStr(deviation.Threshold)
		if err != nil {
			return err
		}
		deviations[deviation.Base] = threshold
	}

	endpoints := make(map[provider.Name]provider.Endpoint, len(cfg.ProviderEndpoints))
	for _, endpoint := range cfg.ProviderEndpoints {
		endpoints[endpoint.Name] = endpoint
	}

	oracle := oracle.New(
		logger,
		oracleClient,
		cfg.CurrencyPairs,
		providerTimeout,
		deviations,
		endpoints,
	)

	g.Go(func() error {
		// start the process that observes and publishes exchange prices
		return startPriceFeeder(ctx, logger, cfg, oracle)
	})
	g.Go(func() error {
		// start the process that calculates oracle prices and votes
		return startOracle(ctx, logger, oracle)
	})

	// Block main process until all spawned goroutines have gracefully exited and
	// signal has been captured in the main process or if an error occurs.
	return g.Wait()
}

// This function is a Go language function that starts a HTTP server that serves as a price feeder.
// It takes in several parameters including a context, a logger, a config, an oracle, and metrics.
// It creates a new router for the server using the mux package, creates a new version 1
// router using the v1 package, and registers the routes for this router on the mux router.
// Then it parses the write and read timeouts from the config and sets them on the http server.
// It also creates a channel called srvErrCh that will be used to communicate any errors that occur
// while starting the server.
// It starts the server in a goroutine and listens for done events from the context and shutdown the
// server gracefully when done is received. It also listen for errors from srvErrCh and if any errors
// occurs it will log them and return it.
func startPriceFeeder(
	ctx context.Context,
	logger zerolog.Logger,
	cfg config.Config,
	oracle *oracle.Oracle,
) error {
	rtr := mux.NewRouter()
	v1Router := v1.New(logger, cfg, oracle)
	v1Router.RegisterRoutes(rtr, v1.APIPathPrefix)

	writeTimeout, err := time.ParseDuration(cfg.Server.WriteTimeout)
	if err != nil {
		return err
	}

	readTimeout, err := time.ParseDuration(cfg.Server.ReadTimeout)
	if err != nil {
		return err
	}

	srvErrCh := make(chan error, 1)
	srv := &http.Server{
		Handler:      rtr,
		Addr:         cfg.Server.ListenAddr,
		WriteTimeout: writeTimeout,
		ReadTimeout:  readTimeout,
	}

	go func() {
		logger.Info().Str("listen_addr", cfg.Server.ListenAddr).Msg("starting price-feeder server...")
		srvErrCh <- srv.ListenAndServe()
	}()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second) //nolint:gomnd //const
			// no need to make const
			defer cancel()

			logger.Info().Str("listen_addr", cfg.Server.ListenAddr).Msg("shutting down price-feeder server...")
			if err := srv.Shutdown(shutdownCtx); err != nil {
				logger.Error().Err(err).Msg("failed to gracefully shutdown price-feeder server")
				return err
			}

			return nil

		case err := <-srvErrCh:
			logger.Error().Err(err).Msg("failed to start price-feeder server")
			return err
		}
	}
}

// This function is a Go language function that starts an oracle for a price feeder.
// It takes in three parameters including a context, a logger, and an oracle.
// It creates a channel called srvErrCh that will be used to communicate
// any errors that occur while starting the oracle.
// It starts the oracle in a goroutine and listens for done events from
// the context and stop the oracle when done is received.
// It also listen for errors from srvErrCh, logs the error and stop
// the oracle and return error if any.
// An oracle, in general, is a program or service that can provide external data or
// information to a smart contract on a blockchain network. In this specific implementation,
// it appears that the oracle is being used to provide price data to the smart contract,
// which the price feeder server will serve to clients making HTTP requests to it.
func startOracle(ctx context.Context, logger zerolog.Logger, oracle *oracle.Oracle) error {
	srvErrCh := make(chan error, 1)

	go func() {
		logger.Info().Msg("starting price-feeder oracle...")
		srvErrCh <- oracle.Start(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("shutting down price-feeder oracle...")
			return nil

		case err := <-srvErrCh:
			logger.Err(err).Msg("error starting the price-feeder oracle")
			oracle.Stop()
			return err
		}
	}
}
