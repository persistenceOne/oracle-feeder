package provider

import (
	"context"
	"encoding/json"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/persistenceOne/oracle-feeder/oracle/types"
)

//nolint:funlen // test
func TestBinanceProvider_GetTickerPrices(t *testing.T) {
	p, err := NewBinanceProvider(
		context.TODO(),
		zerolog.Nop(),
		Endpoint{},
		false,
		types.CurrencyPair{Base: "ATOM", Quote: "USD"},
	)
	require.NoError(t, err)

	t.Run("valid_request_single_ticker", func(t *testing.T) {
		lastPrice := "34.69000000"
		volume := "2396974.02000000"

		tickerMap := map[string]BinanceTicker{}
		tickerMap["ATOMUSD"] = BinanceTicker{
			Symbol:    "ATOMUSD",
			LastPrice: lastPrice,
			Volume:    volume,
		}

		p.tickers = tickerMap

		prices, err := p.GetTickerPrices(types.CurrencyPair{Base: "ATOM", Quote: "USD"})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		require.Equal(t, sdk.MustNewDecFromStr(lastPrice), prices["ATOMUSD"].Price)
		require.Equal(t, sdk.MustNewDecFromStr(volume), prices["ATOMUSD"].Volume)
	})

	t.Run("valid_request_multi_ticker", func(t *testing.T) {
		lastPriceAtom := "34.69000000"
		lastPriceOsmo := "41.35000000"
		volume := "2396974.02000000"

		tickerMap := map[string]BinanceTicker{}
		tickerMap["ATOMUSD"] = BinanceTicker{
			Symbol:    "ATOMUSD",
			LastPrice: lastPriceAtom,
			Volume:    volume,
		}

		tickerMap["OSMOUSD"] = BinanceTicker{
			Symbol:    "OSMOUSD",
			LastPrice: lastPriceOsmo,
			Volume:    volume,
		}

		p.tickers = tickerMap
		prices, err := p.GetTickerPrices(
			types.CurrencyPair{Base: "ATOM", Quote: "USD"},
			types.CurrencyPair{Base: "OSMO", Quote: "USD"},
		)
		require.NoError(t, err)
		require.Len(t, prices, 2)
		require.Equal(t, sdk.MustNewDecFromStr(lastPriceAtom), prices["ATOMUSD"].Price)
		require.Equal(t, sdk.MustNewDecFromStr(volume), prices["ATOMUSD"].Volume)
		require.Equal(t, sdk.MustNewDecFromStr(lastPriceOsmo), prices["OSMOUSD"].Price)
		require.Equal(t, sdk.MustNewDecFromStr(volume), prices["OSMOUSD"].Volume)
	})

	t.Run("invalid_request_invalid_ticker", func(t *testing.T) {
		prices, err := p.GetTickerPrices(types.CurrencyPair{Base: "FOO", Quote: "BAR"})
		require.EqualError(t, err, "binance failed to get ticker price for FOOBAR")
		require.Nil(t, prices)
	})
}

func TestBinanceCurrencyPairToBinancePair(t *testing.T) {
	cp := types.CurrencyPair{Base: "ATOM", Quote: "USD"}
	binanceSymbol := currencyPairToBinanceTickerPair(cp)
	require.Equal(t, binanceSymbol, "atomusd@ticker")
}

func TestBinanceProvider_getSubscriptionMsgs(t *testing.T) {
	provider := &BinanceProvider{
		subscribedPairs: map[string]types.CurrencyPair{},
	}
	cps := []types.CurrencyPair{
		{Base: "ATOM", Quote: "USD"},
	}

	subMsgs := provider.getSubscriptionMsgs(cps...)

	msg, _ := json.Marshal(subMsgs[0])
	require.Equal(t, "{\"method\":\"SUBSCRIBE\",\"params\":[\"atomusd@ticker\"],\"id\":1}", string(msg))

	msg, _ = json.Marshal(subMsgs[1])
	require.Equal(t, "{\"method\":\"SUBSCRIBE\",\"params\":[\"atomusd@kline_1m\"],\"id\":1}", string(msg))
}
