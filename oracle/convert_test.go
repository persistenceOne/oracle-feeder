package oracle

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/persistenceOne/oracle-feeder/oracle/provider"
	"github.com/persistenceOne/oracle-feeder/oracle/types"
)

var (
	atomPrice  = sdk.MustNewDecFromStr("29.93")
	atomVolume = sdk.MustNewDecFromStr("894123.00")
	osmoPrice  = sdk.MustNewDecFromStr("0.98")
	osmoVolume = sdk.MustNewDecFromStr("894123.00")

	atomPair = types.CurrencyPair{
		Base:  "ATOM",
		Quote: "OSMO",
	}
	osmoPair = types.CurrencyPair{
		Base:  "OSMO",
		Quote: "USD",
	}
)

func TestGetUSDBasedProviders(t *testing.T) {
	providerPairs := make(map[provider.Name][]types.CurrencyPair, 3)
	providerPairs[provider.Osmosis] = []types.CurrencyPair{
		{
			Base:  "AXLUSDC",
			Quote: "USD",
		},
	}
	providerPairs[provider.Kraken] = []types.CurrencyPair{
		{
			Base:  "AXLUSDC",
			Quote: "USD",
		},
	}
	providerPairs[provider.Binance] = []types.CurrencyPair{
		{
			Base:  "OSMO",
			Quote: "USD",
		},
	}

	pairs, err := getUSDBasedProviders("AXLUSDC", providerPairs)
	require.NoError(t, err)
	expectedPairs := map[provider.Name]struct{}{
		provider.Osmosis: {},
		provider.Kraken:  {},
	}
	require.Equal(t, pairs, expectedPairs)

	pairs, err = getUSDBasedProviders("OSMO", providerPairs)
	require.NoError(t, err)
	expectedPairs = map[provider.Name]struct{}{
		provider.Binance: {},
	}
	require.Equal(t, pairs, expectedPairs)

	_, err = getUSDBasedProviders("BAR", providerPairs)
	require.Error(t, err)
}

func TestConvertCandlesToUSD(t *testing.T) {
	providerCandles := make(provider.AggregatedProviderCandles, 2)

	binanceCandles := map[string][]types.CandlePrice{
		"ATOM": {{
			Price:     atomPrice,
			Volume:    atomVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		}},
	}
	providerCandles[provider.Binance] = binanceCandles

	krakenCandles := map[string][]types.CandlePrice{
		"OSMO": {{
			Price:     osmoPrice,
			Volume:    osmoVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		}},
	}
	providerCandles[provider.Kraken] = krakenCandles

	providerPairs := map[provider.Name][]types.CurrencyPair{
		provider.Binance: {atomPair},
		provider.Kraken:  {osmoPair},
	}

	convertedCandles, err := ConvertCandlesToUSD(
		zerolog.Nop(),
		providerCandles,
		providerPairs,
		make(map[string]sdk.Dec),
	)
	require.NoError(t, err)

	require.Equal(
		t,
		atomPrice.Mul(osmoPrice),
		convertedCandles[provider.Binance]["ATOM"][0].Price,
	)
}

func TestConvertCandlesToUSDFiltering(t *testing.T) {
	providerCandles := make(provider.AggregatedProviderCandles, 2)

	binanceCandles := map[string][]types.CandlePrice{
		"ATOM": {{
			Price:     atomPrice,
			Volume:    atomVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		}},
	}
	providerCandles[provider.Binance] = binanceCandles

	krakenCandles := map[string][]types.CandlePrice{
		"OSMO": {{
			Price:     osmoPrice,
			Volume:    osmoVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		}},
	}
	providerCandles[provider.Kraken] = krakenCandles

	osmosisCandles := map[string][]types.CandlePrice{
		"OSMO": {{
			Price:     osmoPrice,
			Volume:    osmoVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		}},
	}
	providerCandles[provider.Osmosis] = osmosisCandles

	providerPairs := map[provider.Name][]types.CurrencyPair{
		provider.Binance: {atomPair},
		provider.Kraken:  {osmoPair},
		provider.Osmosis: {osmoPair},
	}

	convertedCandles, err := ConvertCandlesToUSD(
		zerolog.Nop(),
		providerCandles,
		providerPairs,
		make(map[string]sdk.Dec),
	)
	require.NoError(t, err)

	require.Equal(
		t,
		atomPrice.Mul(osmoPrice),
		convertedCandles[provider.Binance]["ATOM"][0].Price,
	)
}

func TestConvertTickersToUSD(t *testing.T) {
	providerPrices := make(provider.AggregatedProviderPrices, 2)

	binanceTickers := map[string]types.TickerPrice{
		"ATOM": {
			Price:  atomPrice,
			Volume: atomVolume,
		},
	}
	providerPrices[provider.Binance] = binanceTickers

	krakenTicker := map[string]types.TickerPrice{
		"OSMO": {
			Price:  osmoPrice,
			Volume: osmoVolume,
		},
	}
	providerPrices[provider.Kraken] = krakenTicker

	providerPairs := map[provider.Name][]types.CurrencyPair{
		provider.Binance: {atomPair},
		provider.Kraken:  {osmoPair},
	}

	convertedTickers, err := ConvertTickersToUSD(
		zerolog.Nop(),
		providerPrices,
		providerPairs,
		make(map[string]sdk.Dec),
	)
	require.NoError(t, err)

	require.Equal(
		t,
		atomPrice.Mul(osmoPrice),
		convertedTickers[provider.Binance]["ATOM"].Price,
	)
}

func TestConvertTickersToUSDFiltering(t *testing.T) {
	providerPrices := make(provider.AggregatedProviderPrices, 2)

	binanceTickers := map[string]types.TickerPrice{
		"ATOM": {
			Price:  atomPrice,
			Volume: atomVolume,
		},
	}
	providerPrices[provider.Binance] = binanceTickers

	krakenTicker := map[string]types.TickerPrice{
		"OSMO": {
			Price:  osmoPrice,
			Volume: osmoVolume,
		},
	}
	providerPrices[provider.Kraken] = krakenTicker

	osmosisTicker := map[string]types.TickerPrice{
		"OSMO": krakenTicker["OSMO"],
	}
	providerPrices[provider.Osmosis] = osmosisTicker

	providerPairs := map[provider.Name][]types.CurrencyPair{
		provider.Binance: {atomPair},
		provider.Kraken:  {osmoPair},
		provider.Osmosis: {osmoPair},
	}

	covertedDeviation, err := ConvertTickersToUSD(
		zerolog.Nop(),
		providerPrices,
		providerPairs,
		make(map[string]sdk.Dec),
	)
	require.NoError(t, err)

	require.Equal(
		t,
		atomPrice.Mul(osmoPrice),
		covertedDeviation[provider.Binance]["ATOM"].Price,
	)
}
