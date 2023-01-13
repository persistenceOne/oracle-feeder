package oracle

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/persistenceOne/oracle-feeder/config"
	"github.com/persistenceOne/oracle-feeder/oracle/client"
	"github.com/persistenceOne/oracle-feeder/oracle/provider"
	"github.com/persistenceOne/oracle-feeder/oracle/types"
)

type OracleTestSuite struct {
	suite.Suite

	oracle *Oracle
}

// SetupSuite executes once before the suite's tests are executed.
func (ots *OracleTestSuite) SetupSuite() {
	ots.oracle = New(
		zerolog.Nop(),
		client.OracleClient{},
		[]config.CurrencyPair{
			{
				Base:      "ATOM",
				Quote:     "USD",
				Providers: []provider.Name{provider.Binance},
			},
			{
				Base:      "OSMO",
				Quote:     "USD",
				Providers: []provider.Name{provider.Kraken},
			},
		},
		time.Millisecond*100,
		make(map[string]sdk.Dec),
		make(map[provider.Name]provider.Endpoint),
	)
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OracleTestSuite))
}

func (ots *OracleTestSuite) TestStop() {
	ots.Eventually(
		func() bool {
			ots.oracle.Stop()
			return true
		},
		5*time.Second,
		time.Second,
	)
}

func (ots *OracleTestSuite) TestGetLastPriceSyncTimestamp() {
	// when no tick() has been invoked, assume zero value
	ots.Require().Equal(time.Time{}, ots.oracle.GetLastPriceSyncTimestamp())
}

func TestGenerateSalt(t *testing.T) {
	salt, err := generateSalt(0)
	require.Error(t, err)
	require.Empty(t, salt)

	salt, err = generateSalt(32)
	require.NoError(t, err)
	require.NotEmpty(t, salt)
}

func TestGenerateExchangeRatesString(t *testing.T) {
	testCases := map[string]struct {
		input    map[string]sdk.Dec
		expected string
		err      error
	}{
		"empty input": {
			input:    make(map[string]sdk.Dec),
			expected: "",
			err:      errNoPriceAvailable,
		},
		"single denom": {
			input: map[string]sdk.Dec{
				"ATOM": sdk.MustNewDecFromStr("3.72"),
			},
			expected: "ATOM:3.720000000000000000",
		},
		"multi denom": {
			input: map[string]sdk.Dec{
				"AXLUSDC": sdk.MustNewDecFromStr("3.72"),
				"ATOM":    sdk.MustNewDecFromStr("40.13"),
				"OSMO":    sdk.MustNewDecFromStr("8.69"),
			},
			expected: "ATOM:40.130000000000000000,AXLUSDC:3.720000000000000000,OSMO:8.690000000000000000",
		},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			out, err := generateExchangeRatesString(tc.input)
			require.Equal(t, err, tc.err)
			require.Equal(t, tc.expected, out)
		})
	}
}

func TestSuccessSetProviderTickerPricesAndCandles(t *testing.T) {
	providerPrices := make(provider.AggregatedProviderPrices, 1)
	providerCandles := make(provider.AggregatedProviderCandles, 1)
	pair := types.CurrencyPair{
		Base:  "ATOM",
		Quote: "USD",
	}

	atomPrice := sdk.MustNewDecFromStr("29.93")
	atomVolume := sdk.MustNewDecFromStr("894123.00")

	prices := make(map[string]types.TickerPrice, 1)
	prices[pair.String()] = types.TickerPrice{
		Price:  atomPrice,
		Volume: atomVolume,
	}

	candles := make(map[string][]types.CandlePrice, 1)
	candles[pair.String()] = []types.CandlePrice{
		{
			Price:     atomPrice,
			Volume:    atomVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		},
	}

	success := SetProviderTickerPricesAndCandles(
		provider.BinanceUS,
		providerPrices,
		providerCandles,
		prices,
		candles,
		pair,
	)

	require.True(t, success, "It should successfully set the prices")
	require.Equal(t, atomPrice, providerPrices[provider.BinanceUS][pair.Base].Price)
	require.Equal(t, atomPrice, providerCandles[provider.BinanceUS][pair.Base][0].Price)
}

func TestFailedSetProviderTickerPricesAndCandles(t *testing.T) {
	success := SetProviderTickerPricesAndCandles(
		provider.Kraken,
		make(provider.AggregatedProviderPrices, 1),
		make(provider.AggregatedProviderCandles, 1),
		make(map[string]types.TickerPrice, 1),
		make(map[string][]types.CandlePrice, 1),
		types.CurrencyPair{
			Base:  "ATOM",
			Quote: "USD",
		},
	)

	require.False(t, success, "It should failed to set the prices, prices and candle are empty")
}

func (ots *OracleTestSuite) TestSuccessGetComputedPricesCandles() {
	providerCandles := make(provider.AggregatedProviderCandles, 1)
	pair := types.CurrencyPair{
		Base:  "ATOM",
		Quote: "USD",
	}

	atomPrice := sdk.MustNewDecFromStr("29.93")
	atomVolume := sdk.MustNewDecFromStr("894123.00")

	candles := make(map[string][]types.CandlePrice, 1)
	candles[pair.Base] = []types.CandlePrice{
		{
			Price:     atomPrice,
			Volume:    atomVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		},
	}
	providerCandles[provider.Binance] = candles

	providerPair := map[provider.Name][]types.CurrencyPair{
		provider.Binance: {pair},
	}

	prices, err := ots.oracle.GetComputedPrices(
		providerCandles,
		make(provider.AggregatedProviderPrices, 1),
		providerPair,
		make(map[string]sdk.Dec),
	)

	require.NoError(ots.T(), err, "It should successfully get computed candle prices")
	require.Equal(ots.T(), prices[pair.Base], atomPrice)
}

func (ots *OracleTestSuite) TestSuccessGetComputedPricesTickers() {
	providerPrices := make(provider.AggregatedProviderPrices, 1)
	pair := types.CurrencyPair{
		Base:  "ATOM",
		Quote: "USD",
	}

	atomPrice := sdk.MustNewDecFromStr("29.93")
	atomVolume := sdk.MustNewDecFromStr("894123.00")

	tickerPrices := make(map[string]types.TickerPrice, 1)
	tickerPrices[pair.Base] = types.TickerPrice{
		Price:  atomPrice,
		Volume: atomVolume,
	}
	providerPrices[provider.Binance] = tickerPrices

	providerPair := map[provider.Name][]types.CurrencyPair{
		provider.Binance: {pair},
	}

	prices, err := ots.oracle.GetComputedPrices(
		make(provider.AggregatedProviderCandles, 1),
		providerPrices,
		providerPair,
		make(map[string]sdk.Dec),
	)

	require.NoError(ots.T(), err, "It should successfully get computed ticker prices")
	require.Equal(ots.T(), prices[pair.Base], atomPrice)
}
