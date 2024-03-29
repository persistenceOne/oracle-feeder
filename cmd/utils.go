package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"

	"github.com/persistenceOne/persistenceCore/v8/app"
)

const (
	logLevelJSON = "json"
	logLevelText = "text"

	envPriceFeederPass = "ORACLE_FEEDER_KEY_PASSPHRASE" // #nosec G101
)

// setConfig params at the package state.
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

// trapSignal will listen for any OS signal and invoke Done on the main
// WaitGroup allowing the main process to gracefully exit.
func trapSignal(cancel context.CancelFunc, logger zerolog.Logger) {
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGTERM)
	signal.Notify(sigCh, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		logger.Info().Str("signal", sig.String()).Msg("received signal; shutting down...")
		cancel()
	}()
}
