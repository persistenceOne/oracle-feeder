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
	}
)

func NewMockProvider() *MockProvider {
	return &MockProvider{
		baseURL: mockBaseURL,
		client: &http.Client{
			Timeout: defaultTimeout,
			// the mock provider is the only one which allows redirects
			// because it gets prices from a google spreadsheet, which redirects
		},
	}
}

// SubscribeCurrencyPairs performs a no-op since mock does not use websocket.
func (p MockProvider) SubscribeCurrencyPairs(...types.CurrencyPair) error {
	return fmt.Errorf("mock provider does not support subscriptions")
}

func (p MockProvider) GetTickerPrices(pairs ...types.CurrencyPair) (map[string]types.TickerPrice, error) {
	tickerPrices := make(map[string]types.TickerPrice, len(pairs))

	resp, err := p.client.Get(p.baseURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	csvReader := csv.NewReader(resp.Body)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	tickerMap := make(map[string]struct{})
	for _, cp := range pairs {
		tickerMap[strings.ToUpper(cp.String())] = struct{}{}
	}

	// Records are of the form [base, quote, price, volume] and we skip the first
	// record as that contains the header.
	for _, r := range records[1:] {
		ticker := strings.ToUpper(r[0] + r[1])
		if _, ok := tickerMap[ticker]; !ok {
			// skip records that are not requested
			continue
		}

		price, err := sdk.NewDecFromStr(r[2])
		if err != nil {
			return nil, fmt.Errorf("failed to read mock price (%s) for %s", r[2], ticker)
		}

		volume, err := sdk.NewDecFromStr(r[3])
		if err != nil {
			return nil, fmt.Errorf("failed to read mock volume (%s) for %s", r[3], ticker)
		}

		if _, ok := tickerPrices[ticker]; ok {
			return nil, fmt.Errorf("found duplicate ticker: %s", ticker)
		}

		tickerPrices[ticker] = types.TickerPrice{Price: price, Volume: volume}
	}

	for t := range tickerMap {
		if _, ok := tickerPrices[t]; !ok {
			return nil, fmt.Errorf(types.ErrMissingExchangeRate.Error(), t)
		}
	}

	return tickerPrices, nil
}

func (p MockProvider) GetCandlePrices(pairs ...types.CurrencyPair) (map[string][]types.CandlePrice, error) {
	price, err := p.GetTickerPrices(pairs...)
	if err != nil {
		return nil, err
	}
	candles := make(map[string][]types.CandlePrice)
	for pair, price := range price {
		candles[pair] = []types.CandlePrice{
			{
				Price:     price.Price,
				Volume:    price.Volume,
				TimeStamp: PastUnixTime(1 * time.Minute),
			},
		}
	}
	return candles, nil
}
