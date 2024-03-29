package types

// CurrencyPair defines a currency exchange pair consisting of a base and a quote.
// We primarily utilize the base for broadcasting exchange rates and use the
// pair for querying for the ticker prices.
type CurrencyPair struct {
	Base  string
	Quote string
}

// String implements the Stringer interface and defines a ticker symbol for
// querying the exchange rate.
func (cp CurrencyPair) String() string {
	return cp.Base + cp.Quote
}
