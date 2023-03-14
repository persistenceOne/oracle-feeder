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

func TestSuccessFilterCandleDeviations(t *testing.T) {
	providerCandles := make(provider.AggregatedProviderCandles, 4)
	pair := types.CurrencyPair{
		Base:  "ATOM",
		Quote: "USD",
	}

	atomPrice := sdk.MustNewDecFromStr("29.93")
	atomVolume := sdk.MustNewDecFromStr("1994674.34000000")

	atomCandlePrice := []types.CandlePrice{
		{
			Price:     atomPrice,
			Volume:    atomVolume,
			TimeStamp: provider.PastUnixTime(1 * time.Minute),
		},
	}

	providerCandles[provider.Binance] = map[string][]types.CandlePrice{
		pair.Base: atomCandlePrice,
	}
	providerCandles[provider.Kraken] = map[string][]types.CandlePrice{
		pair.Base: atomCandlePrice,
	}
	providerCandles[provider.Osmosis] = map[string][]types.CandlePrice{
		pair.Base: {
			{
				Price:     sdk.MustNewDecFromStr("27.1"),
				Volume:    atomVolume,
				TimeStamp: provider.PastUnixTime(1 * time.Minute),
			},
		},
	}

	pricesFiltered, err := filterCandleDeviations(
		zerolog.Nop(),
		providerCandles,
		make(map[string]sdk.Dec),
	)

	_, ok := pricesFiltered[provider.Osmosis]
	require.NoError(t, err, "It should successfully filter out the provider using candles")
	require.False(t, ok, "The filtered candle deviation price at coinbase should be empty")

	customDeviations := make(map[string]sdk.Dec, 1)
	customDeviations[pair.Base] = sdk.NewDec(2)

	pricesFilteredCustom, err := filterCandleDeviations(
		zerolog.Nop(),
		providerCandles,
		customDeviations,
	)

	_, ok = pricesFilteredCustom[provider.Osmosis]
	require.NoError(t, err, "It should successfully not filter out coinbase")
	require.True(t, ok, "The filtered candle deviation price of coinbase should remain")
}

func TestSuccessFilterTickerDeviations(t *testing.T) {
	providerTickers := make(provider.AggregatedProviderPrices, 4)
	pair := types.CurrencyPair{
		Base:  "ATOM",
		Quote: "USD",
	}

	atomPrice := sdk.MustNewDecFromStr("29.93")
	atomVolume := sdk.MustNewDecFromStr("1994674.34000000")

	atomTickerPrice := types.TickerPrice{
		Price:  atomPrice,
		Volume: atomVolume,
	}

	providerTickers[provider.Binance] = map[string]types.TickerPrice{
		pair.Base: atomTickerPrice,
	}
	providerTickers[provider.Kraken] = map[string]types.TickerPrice{
		pair.Base: atomTickerPrice,
	}
	providerTickers[provider.Osmosis] = map[string]types.TickerPrice{
		pair.Base: {
			Price:  sdk.MustNewDecFromStr("27.1"),
			Volume: atomVolume,
		},
	}

	pricesFiltered, err := FilterTickerDeviations(
		zerolog.Nop(),
		providerTickers,
		make(map[string]sdk.Dec),
	)

	_, ok := pricesFiltered[provider.Osmosis]
	require.NoError(t, err, "It should successfully filter out the provider using tickers")
	require.False(t, ok, "The filtered ticker deviation price at coinbase should be empty")

	customDeviations := make(map[string]sdk.Dec, 1)
	customDeviations[pair.Base] = sdk.NewDec(2)

	pricesFilteredCustom, err := FilterTickerDeviations(
		zerolog.Nop(),
		providerTickers,
		customDeviations,
	)

	_, ok = pricesFilteredCustom[provider.Osmosis]
	require.NoError(t, err, "It should successfully not filter out coinbase")
	require.True(t, ok, "The filtered candle deviation price of coinbase should remain")
}
