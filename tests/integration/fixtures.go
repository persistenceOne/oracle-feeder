package integration

import (
	"github.com/persistence/oracle-feeder/oracle/provider"
	"github.com/persistence/oracle-feeder/oracle/types"
)

var ProviderAndCurrencyPairsFixture = []struct {
	provider      provider.Name
	currencyPairs []types.CurrencyPair
}{
	{
		provider:      provider.BinanceUS,
		currencyPairs: []types.CurrencyPair{{Base: "ATOM", Quote: "USD"}},
	},
	{
		provider:      provider.Kraken,
		currencyPairs: []types.CurrencyPair{{Base: "ATOM", Quote: "USD"}},
	},
	{
		provider:      provider.Osmosis,
		currencyPairs: []types.CurrencyPair{{Base: "ATOM", Quote: "USD"}},
	},
}
