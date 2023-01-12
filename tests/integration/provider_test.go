package integration

import (
	"context"
	"os"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/persistence/oracle-feeder/oracle"
	"github.com/persistence/oracle-feeder/oracle/provider"
	"github.com/persistence/oracle-feeder/oracle/types"
)

type IntegrationTestSuite struct {
	suite.Suite

	logger zerolog.Logger
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.logger = getLogger()
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestWebsocketProviders() {
	if testing.Short() {
		s.T().Skip("skipping integration test in short mode")
	}

	testCases := ProviderAndCurrencyPairsFixture
	for _, testCase := range testCases {
		tc := testCase
		s.T().Run(string(tc.provider), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			pvd, _ := oracle.NewProvider(ctx, tc.provider, getLogger(), provider.Endpoint{}, tc.currencyPairs...)
			time.Sleep(30 * time.Second) // wait for provider to connect and receive some prices
			checkForPrices(t, pvd, tc.currencyPairs)
			cancel()
		})
	}
}

func (s *IntegrationTestSuite) TestSubscribeCurrencyPairs() {
	if testing.Short() {
		s.T().Skip("skipping integration test in short mode")
	}

	testCases := ProviderAndCurrencyPairsFixture
	for _, testCase := range testCases {
		tc := testCase
		s.T().Run(string(tc.provider), func(t *testing.T) {
			currencyPairs := tc.currencyPairs
			ctx, cancel := context.WithCancel(context.Background())
			pvd, _ := oracle.NewProvider(ctx, tc.provider, getLogger(), provider.Endpoint{}, tc.currencyPairs...)
			time.Sleep(5 * time.Second)

			err := pvd.SubscribeCurrencyPairs()
			require.NoError(t, err)

			time.Sleep(25 * time.Second) // wait for provider to connect and receive some prices

			checkForPrices(s.T(), pvd, currencyPairs)
			cancel()
		})
	}
}

func checkForPrices(t *testing.T, pvd provider.Provider, currencyPairs []types.CurrencyPair) {
	tickerPrices, err := pvd.GetTickerPrices(currencyPairs...)
	require.NoError(t, err)
	require.NotEqual(t, len(tickerPrices), 0)

	candlePrices, err := pvd.GetCandlePrices(currencyPairs...)
	require.NoError(t, err)

	for _, cp := range currencyPairs {
		currencyPairKey := cp.String()

		// verify ticker price for currency pair is above zero
		require.True(t, tickerPrices[currencyPairKey].Price.GT(sdk.NewDec(0)))

		require.NotEqual(t, len(candlePrices[currencyPairKey]), 0)

		// verify candle price for currency pair is above zero
		require.True(t, candlePrices[currencyPairKey][0].Price.GT(sdk.NewDec(0)))
	}
}

func getLogger() zerolog.Logger {
	logWriter := zerolog.ConsoleWriter{Out: os.Stderr}
	logLvl := zerolog.DebugLevel
	zerolog.SetGlobalLevel(logLvl)
	return zerolog.New(logWriter).Level(logLvl).With().Timestamp().Logger()
}
