package oracle

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"

	"github.com/persistence/oracle-feeder/oracle/provider"
	"github.com/persistence/oracle-feeder/oracle/types"
)

// defaultDeviationThreshold defines how many 𝜎 a provider can be away
// from the mean without being considered faulty. This can be overridden
// in the config.
var defaultDeviationThreshold = sdk.MustNewDecFromStr("1.0")

// FilterTickerDeviations finds the standard deviations of the prices of
// all assets, and filters out any providers that are not within 2𝜎 of the mean.
func FilterTickerDeviations(
	logger zerolog.Logger,
	prices provider.AggregatedProviderPrices,
	deviationThresholds map[string]sdk.Dec,
) (provider.AggregatedProviderPrices, error) {
	priceMap := make(map[provider.Name]map[string]sdk.Dec)
	for providerName, priceTickers := range prices {
		priceMap[providerName] = make(map[string]sdk.Dec)
		for base, tp := range priceTickers {
			priceMap[providerName][base] = tp.Price
		}
	}

	deviations, means, err := ComputeStandardDeviationsAndMeans(priceMap)
	if err != nil {
		return nil, err
	}

	// We accept any prices that are within (2 * T)𝜎, or for which we couldn't get 𝜎.
	// T is defined as the deviation threshold, either set by the config
	// or defaulted to 1.
	filteredPrices := make(provider.AggregatedProviderPrices)
	for providerName, priceTickers := range prices {
		for base, tp := range priceTickers {
			t := defaultDeviationThreshold
			if _, ok := deviationThresholds[base]; ok {
				t = deviationThresholds[base]
			}

			if d, ok := deviations[base]; !ok || isBetween(tp.Price, means[base], d.Mul(t)) {
				if _, ok := filteredPrices[providerName]; !ok {
					filteredPrices[providerName] = make(map[string]types.TickerPrice)
				}

				filteredPrices[providerName][base] = tp
			} else {
				provider.TelemetryFailure(providerName, provider.MessageTypeTicker)
				logger.Warn().
					Str("base", base).
					Str("provider", string(providerName)).
					Str("price", tp.Price.String()).
					Msg("provider deviating from other prices")
			}
		}
	}

	return filteredPrices, nil
}

// filterCandleDeviations finds the standard deviations of the tvwaps of
// all assets, and filters out any providers that are not within 2𝜎 of the mean.
func filterCandleDeviations(
	logger zerolog.Logger,
	candles provider.AggregatedProviderCandles,
	deviationThresholds map[string]sdk.Dec,
) (provider.AggregatedProviderCandles, error) {
	var (
		filteredCandles = make(provider.AggregatedProviderCandles)
		tvwaps          = make(map[provider.Name]map[string]sdk.Dec)
	)

	for providerName, priceCandles := range candles {
		candlePrices := make(provider.AggregatedProviderCandles)

		for base, cp := range priceCandles {
			candlePrices[providerName] = map[string][]types.CandlePrice{
				base: cp,
			}
		}

		tvwap, err := ComputeTVWAP(candlePrices)
		if err != nil {
			return nil, err
		}

		for base, asset := range tvwap {
			if _, ok := tvwaps[providerName]; !ok {
				tvwaps[providerName] = make(map[string]sdk.Dec)
			}

			tvwaps[providerName][base] = asset
		}
	}

	deviations, means, err := ComputeStandardDeviationsAndMeans(tvwaps)
	if err != nil {
		return nil, err
	}

	// We accept any prices that are within (2 * T)𝜎, or for which we couldn't get 𝜎.
	// T is defined as the deviation threshold, either set by the config
	// or defaulted to 1.
	for providerName, priceMap := range tvwaps {
		for base, price := range priceMap {
			t := defaultDeviationThreshold
			if _, ok := deviationThresholds[base]; ok {
				t = deviationThresholds[base]
			}

			if d, ok := deviations[base]; !ok || isBetween(price, means[base], d.Mul(t)) {
				if _, ok := filteredCandles[providerName]; !ok {
					filteredCandles[providerName] = make(map[string][]types.CandlePrice)
				}

				filteredCandles[providerName][base] = candles[providerName][base]
			} else {
				provider.TelemetryFailure(providerName, provider.MessageTypeCandle)
				logger.Warn().
					Str("base", base).
					Str("provider", string(providerName)).
					Str("price", price.String()).
					Msg("provider deviating from other candles")
			}
		}
	}

	return filteredCandles, nil
}

func isBetween(p, mean, margin sdk.Dec) bool {
	return p.GTE(mean.Sub(margin)) &&
		p.LTE(mean.Add(margin))
}
