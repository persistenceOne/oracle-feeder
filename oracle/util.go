package oracle

import (
	"fmt"
	"sort"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/persistence/oracle-feeder/oracle/provider"
)

var (
	minimumTimeWeight   = sdk.MustNewDecFromStr("0.2000")
	minimumCandleVolume = sdk.MustNewDecFromStr("0.0001")
)

const (
	// tvwapCandlePeriod represents the time period we use for tvwap in minutes
	tvwapCandlePeriod = 5 * time.Minute
)

// compute VWAP for each base by dividing the Σ {P * V} by Σ {V}
func vwap(weightedPrices, volumeSum map[string]sdk.Dec) map[string]sdk.Dec {
	vwaps := make(map[string]sdk.Dec)

	for base, p := range weightedPrices {
		if volumeSum[base].Equal(sdk.ZeroDec()) {
			continue
		}

		vwaps[base] = p.Quo(volumeSum[base])
	}

	return vwaps
}

// ComputeVWAP computes the volume weighted average price for all price points
// for each ticker/exchange pair. The provided prices argument reflects a mapping
// of provider => {<base> => <TickerPrice>, ...}.
//
// Ref: https://en.wikipedia.org/wiki/Volume-weighted_average_price
func ComputeVWAP(prices provider.AggregatedProviderPrices) map[string]sdk.Dec {
	var (
		weightedPrices = make(map[string]sdk.Dec)
		volumeSum      = make(map[string]sdk.Dec)
	)

	for _, providerPrices := range prices {
		for base, tp := range providerPrices {
			if _, ok := weightedPrices[base]; !ok {
				weightedPrices[base] = sdk.ZeroDec()
			}
			if _, ok := volumeSum[base]; !ok {
				volumeSum[base] = sdk.ZeroDec()
			}

			// weightedPrices[base] = Σ {P * V} for all TickerPrice
			weightedPrices[base] = weightedPrices[base].Add(tp.Price.Mul(tp.Volume))

			// track total volume for each base
			volumeSum[base] = volumeSum[base].Add(tp.Volume)
		}
	}

	return vwap(weightedPrices, volumeSum)
}

// ComputeTVWAP computes the time volume weighted average price for all points
// for each exchange pair. Filters out any candles that did not occur within
// timePeriod. The provided prices argument reflects a mapping of
// provider => {<base> => <TickerPrice>, ...}.
//
// Ref : https://en.wikipedia.org/wiki/Time-weighted_average_price
func ComputeTVWAP(prices provider.AggregatedProviderCandles) (map[string]sdk.Dec, error) {
	var (
		weightedPrices = make(map[string]sdk.Dec)
		volumeSum      = make(map[string]sdk.Dec)
		now            = provider.PastUnixTime(0)
		timePeriod     = provider.PastUnixTime(tvwapCandlePeriod)
	)

	for _, providerPrices := range prices {
		for base := range providerPrices {
			cp := providerPrices[base]
			if len(cp) == 0 {
				continue
			}

			if _, ok := weightedPrices[base]; !ok {
				weightedPrices[base] = sdk.ZeroDec()
			}
			if _, ok := volumeSum[base]; !ok {
				volumeSum[base] = sdk.ZeroDec()
			}

			// Sort by timestamp old -> new
			sort.SliceStable(cp, func(i, j int) bool {
				return cp[i].TimeStamp < cp[j].TimeStamp
			})

			period := sdk.NewDec(now - cp[0].TimeStamp)
			if period.Equal(sdk.ZeroDec()) {
				return nil, fmt.Errorf("unable to divide by zero")
			}
			// weightUnit = (1 - minimumTimeWeight) / period
			weightUnit := sdk.OneDec().Sub(minimumTimeWeight).Quo(period)

			// get weighted prices, and sum of volumes
			for _, candle := range cp {
				// we only want candles within the last timePeriod
				if timePeriod < candle.TimeStamp {
					// timeDiff = now - candle.TimeStamp
					timeDiff := sdk.NewDec(now - candle.TimeStamp)
					// set minimum candle volume for low-trading assets
					if candle.Volume.Equal(sdk.ZeroDec()) {
						candle.Volume = minimumCandleVolume
					}

					// volume = candle.Volume * (weightUnit * (period - timeDiff) + minimumTimeWeight)
					volume := candle.Volume.Mul(
						weightUnit.Mul(period.Sub(timeDiff).Add(minimumTimeWeight)),
					)
					volumeSum[base] = volumeSum[base].Add(volume)
					weightedPrices[base] = weightedPrices[base].Add(candle.Price.Mul(volume))
				}
			}

		}
	}

	return vwap(weightedPrices, volumeSum), nil
}

// ComputeStandardDeviationsAndMeans returns maps of the standard deviations and means of assets.
// Will skip calculating for an asset if there are less than 3 prices.
func ComputeStandardDeviationsAndMeans(prices map[provider.Name]map[string]sdk.Dec) (map[string]sdk.Dec, map[string]sdk.Dec, error) {
	stdDevs := make(map[string]sdk.Dec)
	means := make(map[string]sdk.Dec)
	priceSlice := make(map[string][]sdk.Dec)
	priceSums := make(map[string]sdk.Dec)

	for _, assetPrices := range prices {
		for asset, price := range assetPrices {
			if _, ok := priceSums[asset]; !ok {
				priceSums[asset] = sdk.ZeroDec()
			}
			if _, ok := priceSlice[asset]; !ok {
				priceSlice[asset] = []sdk.Dec{}
			}

			priceSums[asset] = priceSums[asset].Add(price)
			priceSlice[asset] = append(priceSlice[asset], price)
		}
	}

	for asset, sum := range priceSums {
		if len(priceSlice[asset]) < 3 {
			continue
		}

		if _, ok := stdDevs[asset]; !ok {
			stdDevs[asset] = sdk.ZeroDec()
		}

		if _, ok := means[asset]; !ok {
			means[asset] = sdk.ZeroDec()
		}

		numPrices := int64(len(priceSlice[asset]))
		mean := sum.QuoInt64(numPrices)
		means[asset] = mean

		varianceSum := sdk.ZeroDec()
		for _, p := range priceSlice[asset] {
			varianceSum = varianceSum.Add(p.Sub(mean).Mul(p.Sub(mean)))
		}

		variance := varianceSum.QuoInt64(numPrices)
		standardDeviation, err := variance.ApproxSqrt()
		if err != nil {
			return make(map[string]sdk.Dec), make(map[string]sdk.Dec), err
		}

		stdDevs[asset] = standardDeviation
	}
	return stdDevs, means, nil
}

// computeTvwapsByProvider computes the tvwap prices from candles for each provider separately and returns them
// in a map separated by provider name
func computeTvwapsByProvider(prices provider.AggregatedProviderCandles) (map[provider.Name]map[string]sdk.Dec, error) {
	tvwaps := make(map[provider.Name]map[string]sdk.Dec)
	var err error

	for providerName, candles := range prices {
		singleProviderCandles := provider.AggregatedProviderCandles{"providerName": candles}
		tvwaps[providerName], err = ComputeTVWAP(singleProviderCandles)
		if err != nil {
			return nil, err
		}
	}
	return tvwaps, nil
}

// computeVwapsByProvider computes the vwap prices from tickers for each provider separately and returns them
// in a map separated by provider name
func computeVwapsByProvider(prices provider.AggregatedProviderPrices) map[provider.Name]map[string]sdk.Dec {
	vwaps := make(map[provider.Name]map[string]sdk.Dec)

	for providerName, tickers := range prices {
		singleProviderCandles := provider.AggregatedProviderPrices{"providerName": tickers}
		vwaps[providerName] = ComputeVWAP(singleProviderCandles)
	}
	return vwaps
}
