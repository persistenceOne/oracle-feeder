package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/persistenceOne/persistenceCore/v6/app"

	"github.com/cosmos/cosmos-sdk/client/input"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
)

const (
	logLevelJSON = "json"
	logLevelText = "text"

	envPriceFeederPass = "PRICE_FEEDER_KEY_PASS"
)

// setConfig params at the package state
func setConfig() {
	cfg := sdk.GetConfig()

	cfg.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(app.Bech32PrefixValAddr, app.Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(app.Bech32PrefixConsAddr, app.Bech32PrefixConsPub)
	cfg.SetCoinType(app.CoinType)
	cfg.SetPurpose(app.Purpose)

	cfg.Seal()
}

func setUpLogger(logLevel string, logFormat string) (zerolog.Logger, error) {
	logLvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return zerolog.Logger{}, err
	}

	var logWriter io.Writer
	if logFormat == "text" {
		logWriter = zerolog.ConsoleWriter{Out: os.Stderr}
	} else {
		logWriter = os.Stderr
	}

	switch logFormat {
	case logLevelJSON:
		logWriter = os.Stderr
	case logLevelText:
		logWriter = zerolog.ConsoleWriter{Out: os.Stderr}

	default:
		return zerolog.Logger{}, fmt.Errorf("invalid logging format: %s", logFormat)
	}

	return zerolog.New(logWriter).Level(logLvl).With().Timestamp().Logger(), nil
}

func getKeyringPassword() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	pass := os.Getenv(envPriceFeederPass)
	if pass == "" {
		return input.GetString("Enter keyring password", reader)
	}

	return pass, nil
}

// trapSignal will listen for any OS signal and invoke Done on the main
// WaitGroup allowing the main process to gracefully exit.
func trapSignal(cancel context.CancelFunc, logger zerolog.Logger) {
	sigCh := make(chan os.Signal)

	signal.Notify(sigCh, syscall.SIGTERM)
	signal.Notify(sigCh, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		logger.Info().Str("signal", sig.String()).Msg("received signal; shutting down...")
		cancel()
	}()
}
