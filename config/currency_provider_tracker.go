package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/tendermint/tendermint/libs/sync"
)

const (
	coinGeckoRestURL         = "https://api.coingecko.com/api/v3/coins"
	coinGeckoListEndpoint    = "list"
	coinGeckoTickersEndpoint = "tickers"
	trackingPeriod           = time.Hour * 24
	requestTimeout           = time.Second * 2
	minimumProvider          = 2
)

type (
	// CurrencyProviderTracker queries the CoinGecko API for all
	// the exchanges that support the currency pairs set in the price feeder config. It
	// will poll the APIs every 24 hours to log any new exchanges that were added for a
	// given currency.
	//
	// REF: https://www.coingecko.com/en/api/documentation
	CurrencyProviderTracker struct {
		logger              zerolog.Logger
		pairs               []CurrencyPair
		mutex               *sync.RWMutex
		coinIDSymbolMap     map[string]string   // ex: map["ATOM"] = "cosmos"
		currencyProviders   map[string][]string // map of price feeder currencies and what exchanges support them
		currencyProviderMin map[string]int      // map of price feeder currencies and min required providers for them
	}

	// List of assets on CoinGecko and their corresponding id and symbol.
	coinList struct {
		ID     string `json:"id"`     // ex: "cosmos"
		Symbol string `json:"symbol"` // ex: "ATOM"
	}

	// CoinGecko ticker shows market data for a given currency pair including what
	// exchanges they're on.
	coinTickerResponse struct {
		Tickers []coinTicker `json:"tickers"`
	}

	coinTicker struct {
		Base   string     `json:"base"`   // CurrencyPair.Base
		Target string     `json:"target"` // CurrencyPair.Quote
		Market coinMarket `json:"market"`
	}

	coinMarket struct {
		Name string `json:"name"` // ex: Binance
	}
)

func newCurrencyProviderTracker(
	ctx context.Context,
	logger zerolog.Logger,
	pairs ...CurrencyPair,
) (*CurrencyProviderTracker, error) {
	currencyProviderTracker := &CurrencyProviderTracker{
		logger:              logger,
		pairs:               pairs,
		mutex:               &sync.RWMutex{},
		coinIDSymbolMap:     map[string]string{},
		currencyProviders:   map[string][]string{},
		currencyProviderMin: map[string]int{},
	}

	if err := currencyProviderTracker.setCoinIDSymbolMap(); err != nil {
		return nil, err
	}

	if err := currencyProviderTracker.setCurrencyProviders(); err != nil {
		return nil, err
	}

	currencyProviderTracker.setCurrencyProviderMin()

	go currencyProviderTracker.trackCurrencyProviders(ctx)

	return currencyProviderTracker, nil
}

func (t *CurrencyProviderTracker) logCurrencyProviders() {
	for currency, providers := range t.currencyProviders {
		t.logger.Info().Msg(fmt.Sprintf("providers supporting %s: %v", currency, providers))
	}
}

// setCoinIDSymbolMap gets list of assets on CoinGecko to cross reference coin symbol to id.
func (t *CurrencyProviderTracker) setCoinIDSymbolMap() error {
	resp, err := http.Get(fmt.Sprintf("%s/%s", coinGeckoRestURL, coinGeckoListEndpoint))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var listResponse []coinList
	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return errors.Wrap(err, "failed to decode response body as JSON")
	}

	for _, coin := range listResponse {
		t.coinIDSymbolMap[coin.Symbol] = coin.ID
	}

	return nil
}

// setCurrencyProviders queries CoinGecko's tickers endpoint and the osmosis-api assetpairs
// endpoint to get all the exchanges that support each price feeder currency pair and store
// it in the CurrencyProviders map.
func (t *CurrencyProviderTracker) setCurrencyProviders() error {
	client := &http.Client{
		Timeout: requestTimeout,
	}

	for _, pair := range t.pairs {
		// check if CoinGecko API supports pair
		pairBaseID := t.coinIDSymbolMap[strings.ToLower(pair.Base)]

		coinGeckoResp, err := client.Get(fmt.Sprintf("%s/%s/%s/%s",
			coinGeckoRestURL,
			pairBaseID,
			coinGeckoTickersEndpoint,
			pair.Quote))
		if err != nil {
			return errors.Wrapf(err, "failed to query coin gecko api tickers endpoint for %s", pair.Base)
		}

		var tickerResponse coinTickerResponse
		if err = json.NewDecoder(coinGeckoResp.Body).Decode(&tickerResponse); err != nil {
			return errors.Wrap(err, "failed to decode response body as JSON")
		}

		// close the response body.
		coinGeckoResp.Body.Close()

		for _, ticker := range tickerResponse.Tickers {
			if ticker.Target == pair.Quote {
				t.currencyProviders[pair.Base] = append(t.currencyProviders[pair.Base], ticker.Market.Name)
			}
		}
	}

	return nil
}

// setCurrencyProviderMin will set the minimum amount of providers for each currency
// to the amount of exchanges that support them if it's less than 3. Otherwise it is
// set to 2 providers.
func (t *CurrencyProviderTracker) setCurrencyProviderMin() {
	for base, exchanges := range t.currencyProviders {
		if len(exchanges) < minimumProvider {
			t.currencyProviderMin[base] = len(exchanges)
		} else {
			t.currencyProviderMin[base] = minimumProvider
		}
	}
}

// trackCurrencyProviders resets CurrencyProviders map and logs out supported
// exchanges for each currency every 24 hours.
func (t *CurrencyProviderTracker) trackCurrencyProviders(ctx context.Context) {
	t.logCurrencyProviders()
	trackingTicker := time.NewTicker(trackingPeriod)
	defer trackingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-trackingTicker.C:
			if err := t.setCurrencyProviders(); err != nil {
				t.logger.Error().Err(err).Msg("Failed to set available providers for currencies")
			}

			t.logCurrencyProviders()
		}
	}
}

// return the minimum amount of providers for each currency.
func (t *CurrencyProviderTracker) GetMinCurrencyProvider() map[string]int {
	return t.currencyProviderMin
}
