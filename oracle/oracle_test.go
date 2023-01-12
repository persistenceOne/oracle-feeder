package oracle

import (
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/persistence/oracle-feeder/config"
	"github.com/persistence/oracle-feeder/oracle/client"
	"github.com/persistence/oracle-feeder/oracle/provider"
	"github.com/persistence/oracle-feeder/oracle/types"
)

type mockProvider struct {
	prices map[string]types.TickerPrice
}

func (m mockProvider) GetTickerPrices(_ ...types.CurrencyPair) (map[string]types.TickerPrice, error) {
	return m.prices, nil
}

func (m mockProvider) GetCandlePrices(_ ...types.CurrencyPair) (map[string][]types.CandlePrice, error) {
	candles := make(map[string][]types.CandlePrice)
	for pair, price := range m.prices {
		candles[pair] = []types.CandlePrice{
			{
				Price:     price.Price,
				TimeStamp: provider.PastUnixTime(1 * time.Minute),
				Volume:    price.Volume,
			},
		}
	}
	return candles, nil
}

func (m mockProvider) SubscribeCurrencyPairs(_ ...types.CurrencyPair) error {
	return nil
}

type failingProvider struct {
	prices map[string]types.TickerPrice
}

func (m failingProvider) GetTickerPrices(_ ...types.CurrencyPair) (map[string]types.TickerPrice, error) {
	return nil, fmt.Errorf("unable to get ticker prices")
}

func (m failingProvider) GetCandlePrices(_ ...types.CurrencyPair) (map[string][]types.CandlePrice, error) {
	return nil, fmt.Errorf("unable to get candle prices")
}

func (m failingProvider) SubscribeCurrencyPairs(_ ...types.CurrencyPair) error {
	return nil
}

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
			expected: "ATOM:40.130000000000000000,OSMO:8.690000000000000000,AXLUSDC:3.720000000000000000",
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
