package provider

import (
	"context"
	"encoding/json"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/persistence/oracle-feeder/oracle/types"
)

func TestKrakenProvider_GetTickerPrices(t *testing.T) {
	p, err := NewKrakenProvider(
		context.TODO(),
		zerolog.Nop(),
		Endpoint{},
		types.CurrencyPair{Base: "ATOM", Quote: "USD"},
	)
	require.NoError(t, err)

	t.Run("valid_request_single_ticker", func(t *testing.T) {
		lastPrice := sdk.MustNewDecFromStr("34.69000000")
		volume := sdk.MustNewDecFromStr("2396974.02000000")

		tickerMap := map[string]types.TickerPrice{}
		tickerMap["ATOMUSD"] = types.TickerPrice{
			Price:  lastPrice,
			Volume: volume,
		}

		p.tickers = tickerMap

		prices, err := p.GetTickerPrices(types.CurrencyPair{Base: "ATOM", Quote: "USD"})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		require.Equal(t, lastPrice, prices["ATOMUSD"].Price)
		require.Equal(t, volume, prices["ATOMUSD"].Volume)
	})

	t.Run("valid_request_multi_ticker", func(t *testing.T) {
		lastPriceAtom := sdk.MustNewDecFromStr("34.69000000")
		lastPriceOsmosis := sdk.MustNewDecFromStr("41.35000000")
		volume := sdk.MustNewDecFromStr("2396974.02000000")

		tickerMap := map[string]types.TickerPrice{}
		tickerMap["ATOMUSD"] = types.TickerPrice{
			Price:  lastPriceAtom,
			Volume: volume,
		}

		tickerMap["OSMOUSD"] = types.TickerPrice{
			Price:  lastPriceOsmosis,
			Volume: volume,
		}

		p.tickers = tickerMap
		prices, err := p.GetTickerPrices(
			types.CurrencyPair{Base: "ATOM", Quote: "USD"},
			types.CurrencyPair{Base: "OSMO", Quote: "USD"},
		)
		require.NoError(t, err)
		require.Len(t, prices, 2)
		require.Equal(t, lastPriceAtom, prices["ATOMUSD"].Price)
		require.Equal(t, volume, prices["ATOMUSD"].Volume)
		require.Equal(t, lastPriceOsmosis, prices["OSMOUSD"].Price)
		require.Equal(t, volume, prices["OSMOUSD"].Volume)
	})

	t.Run("invalid_request_invalid_ticker", func(t *testing.T) {
		prices, err := p.GetTickerPrices(types.CurrencyPair{Base: "FOO", Quote: "BAR"})
		require.EqualError(t, err, "kraken failed to get ticker price for FOOBAR")
		require.Nil(t, prices)
	})
}

func TestKrakenPairToCurrencyPairSymbol(t *testing.T) {
	cp := types.CurrencyPair{Base: "ATOM", Quote: "USD"}
	currencyPairSymbol := krakenPairToCurrencyPairSymbol("ATOM/USD")
	require.Equal(t, cp.String(), currencyPairSymbol)
}

func TestKrakenCurrencyPairToKrakenPair(t *testing.T) {
	cp := types.CurrencyPair{Base: "ATOM", Quote: "USD"}
	krakenSymbol := currencyPairToKrakenPair(cp)
	require.Equal(t, krakenSymbol, "ATOM/USD")
}

func TestNormalizeKrakenOSMOPair(t *testing.T) {
	osmoSymbol := normalizeKrakenBTCPair("OSMO/USD")
	require.Equal(t, osmoSymbol, "OSMO/USD")

	atomSymbol := normalizeKrakenBTCPair("ATOM/USD")
	require.Equal(t, atomSymbol, "ATOM/USD")
}

func TestKrakenProvider_getSubscriptionMsgs(t *testing.T) {
	provider := &KrakenProvider{
		subscribedPairs: map[string]types.CurrencyPair{},
	}
	cps := []types.CurrencyPair{
		{Base: "ATOM", Quote: "USD"},
	}
	subMsgs := provider.getSubscriptionMsgs(cps...)

	msg, _ := json.Marshal(subMsgs[0])
	require.Equal(t, "{\"event\":\"subscribe\",\"pair\":[\"ATOM/USD\"],\"subscription\":{\"name\":\"ticker\"}}", string(msg))

	msg, _ = json.Marshal(subMsgs[1])
	require.Equal(t, "{\"event\":\"subscribe\",\"pair\":[\"ATOM/USD\"],\"subscription\":{\"name\":\"ohlc\"}}", string(msg))
}
