package provider

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/persistenceOne/oracle-feeder/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Google Sheets document containing mock exchange rates.
	//
	// Ref: https://docs.google.com/spreadsheets/d/1MiLpTLaivgKAxbbTPbzECqHzc6RIVIhPRoEj8mDmOiw/edit#gid=0
	//nolint:lll // ignore long line length due to URL
	mockBaseURL = "https://docs.google.com/spreadsheets/d/e/2PACX-1vQwfo4t2r3CGoVtyzvVfk_th_t8Domm_su1VKYJJ14Qxs63qbj6gFYpFtJF_RDXydijQk5KZh7-cmft/pub?output=csv"
)

var _ Provider = (*MockProvider)(nil)

type (
	// MockProvider defines a mocked exchange rate provider using a published
	// Google sheets document to fetch mocked/fake exchange rates.
	MockProvider struct {
		baseURL string
		client  *http.Client
		prices  map[string]*tickerPrices
	}

	tickerPrices struct {
		items []types.TickerPrice
		// index is the index of the price to that was last used for the ticker.
		index int
		// storing this price to make sure GetTickerPrice and GetCandlePrice return the same price at a time.
		lastTickerPrice types.TickerPrice
	}
)

func NewMockProvider(baseURL string, client *http.Client) (*MockProvider, error) {
	if baseURL == "" {
		baseURL = mockBaseURL
	}

	if client == nil {
		client = &http.Client{
			Timeout: defaultTimeout,
			// the mock provider is the only one which allows redirects
			// because it gets prices from a google spreadsheet, which redirects
		}
	}

	m := &MockProvider{
		baseURL: baseURL,
		client:  client,
	}

	if err := m.loadPrices(); err != nil {
		return nil, fmt.Errorf("failed to load prices: %w", err)
	}
	return m, nil
}

func (p *MockProvider) loadPrices() error {
	p.prices = make(map[string]*tickerPrices)
	resp, err := p.client.Get(p.baseURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	records, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		return err
	}

	for i, r := range records[1:] {
		ticker := strings.ToUpper(r[0] + r[1])
		if _, ok := p.prices[ticker]; !ok {
			p.prices[ticker] = &tickerPrices{}
		}

		prices := p.prices[ticker].items
		price, err := sdk.NewDecFromStr(r[2])
		if err != nil {
			return fmt.Errorf("failed to read mock price (%s) for %s %d", r[2], ticker, i)
		}

		volume, err := sdk.NewDecFromStr(r[3])
		if err != nil {
			return fmt.Errorf("failed to read mock volume (%s) for %s", r[3], ticker)
		}

		tickerPrice := types.TickerPrice{
			Price:  price,
			Volume: volume,
		}
		prices = append(prices, tickerPrice)
		p.prices[ticker] = &tickerPrices{items: prices, index: 0}
	}
	return nil
}

// GetTickerPrices returns the mocked ticker price for the given symbol.
func (p *MockProvider) GetTickerPrices(pairs ...types.CurrencyPair) (map[string]types.TickerPrice, error) {
	prices := make(map[string]types.TickerPrice, len(pairs))
	for _, pair := range pairs {
		ticker := strings.ToUpper(pair.String())
		tickerPrices, ok := p.prices[ticker]
		if !ok {
			return nil, fmt.Errorf("no ticker price for %s", ticker)
		}
		if tickerPrices.index >= len(tickerPrices.items) {
			tickerPrices.index = 0
		}
		price := tickerPrices.items[tickerPrices.index]
		tickerPrices.index++
		tp := types.TickerPrice{Price: price.Price, Volume: price.Volume}
		prices[ticker] = tp

		// storing this price to make sure GetTickerPrice and GetCandlePrice return the same price at a time.
		tickerPrices.lastTickerPrice = tp
	}
	return prices, nil
}

func (p *MockProvider) GetCandlePrices(pairs ...types.CurrencyPair) (map[string][]types.CandlePrice, error) {
	candles := make(map[string][]types.CandlePrice, len(pairs))
	for _, pair := range pairs {
		ticker := strings.ToUpper(pair.String())
		tickerPrices := p.prices[ticker]
		if tickerPrices == nil {
			return nil, fmt.Errorf("no candle price for: %s", ticker)
		}

		// Returning the same price as the GetTickerPrices.
		price := tickerPrices.lastTickerPrice
		candles[ticker] = []types.CandlePrice{{
			Price:     price.Price,
			Volume:    price.Volume,
			TimeStamp: PastUnixTime(1 * time.Minute),
		}}
	}
	return candles, nil
}

// SubscribeCurrencyPairs performs a no-op since mock does not use websocket.
func (p MockProvider) SubscribeCurrencyPairs(...types.CurrencyPair) error {
	return fmt.Errorf("mock provider does not support subscriptions")
}
