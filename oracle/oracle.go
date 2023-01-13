package oracle

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/persistenceOne/oracle-feeder/config"
	"github.com/persistenceOne/oracle-feeder/oracle/provider"
	"github.com/persistenceOne/oracle-feeder/oracle/types"
	pfsync "github.com/persistenceOne/oracle-feeder/pkg/sync"

	oracletypes "github.com/persistenceOne/persistence-sdk/x/oracle/types"

	"github.com/persistenceOne/oracle-feeder/oracle/client"
)

var (
	errExpectedPositiveBlockHeight = errors.New("expected positive block height")
	errNoPriceAvailable            = errors.New("price is not available")
)

// We define tickerTimeout as the minimum timeout between each oracle loop. We
// define this value empirically based on enough time to collect exchange rates,
// and broadcast pre-vote and vote transactions such that they're committed in a
// block during each voting period.
const (
	tickerTimeout = 5 * time.Second
)

// PreviousPrevote defines a structure for defining the previous prevote
// submitted on-chain.
type PreviousPrevote struct {
	ExchangeRates     string
	Salt              string
	SubmitBlockHeight int64
}

// Oracle implements the core component responsible for fetching exchange rates
// for a given set of currency pairs and determining the correct exchange rates
// to submit to the on-chain price oracle adhering the oracle specification.
type Oracle struct {
	logger zerolog.Logger
	closer *pfsync.Closer

	providerTimeout    time.Duration
	providerPairs      map[provider.Name][]types.CurrencyPair
	previousPrevote    *PreviousPrevote
	previousVotePeriod float64
	priceProviders     map[provider.Name]provider.Provider
	client             client.OracleClient
	deviations         map[string]sdk.Dec
	endpoints          map[provider.Name]provider.Endpoint
	paramCache         ParamCache

	pricesMutex     sync.RWMutex
	lastPriceSyncTS time.Time
	prices          map[string]sdk.Dec

	tvwapsByProvider PricesWithMutex
	vwapsByProvider  PricesWithMutex
}

func New(
	logger zerolog.Logger,
	oc client.OracleClient,
	currencyPairs []config.CurrencyPair,
	providerTimeout time.Duration,
	deviations map[string]sdk.Dec,
	endpoints map[provider.Name]provider.Endpoint,
) *Oracle {
	providerPairs := make(map[provider.Name][]types.CurrencyPair)

	for _, pair := range currencyPairs {
		for _, provider := range pair.Providers {
			providerPairs[provider] = append(providerPairs[provider], types.CurrencyPair{
				Base:  pair.Base,
				Quote: pair.Quote,
			})
		}
	}
	return &Oracle{
		logger:          logger.With().Str("module", "oracle").Logger(),
		closer:          pfsync.NewCloser(),
		client:          oc,
		providerPairs:   providerPairs,
		priceProviders:  make(map[provider.Name]provider.Provider),
		previousPrevote: nil,
		providerTimeout: providerTimeout,
		deviations:      deviations,
		paramCache:      ParamCache{},
		endpoints:       endpoints,
	}
}

/*
This function is a method of a struct called Oracle in Go language.
The function starts an infinite loop that repeatedly performs an "oracle tick"
and sleeps for a period of time defined by the tickerTimeout variable.

Each tick of the loop performs the following operations:

 - It checks if the context is done and if so, it closes the closer object and exit the loop.

 - It logs a message at the debug level, indicating that the oracle tick has begun.

 - It stores the current time in the startTime variable.

 - It calls another function called "executeTick" and pass the context. If this function
returns an error, it increments a counter for failures and logs the error message returned.

 - It sets the value of the lastPriceSyncTS variable to the current time.

 - It sleeps for a period of time defined by the tickerTimeout variable.

It is likely that this function is designed to run continuously in the background and periodically
update some sort of price data which is being used by the smart contract. The executeTick function
which is being called inside the loop could be doing the price fetching and updating job.
*/

// Start starts the oracle process in a blocking fashion.
func (o *Oracle) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			o.closer.Close()

		default:
			o.logger.Debug().Msg("starting oracle tick")

			if err := o.executeTick(ctx); err != nil {
				o.logger.Err(err).Msg("oracle tick failed")
			}

			o.lastPriceSyncTS = time.Now()

			o.logger.Debug().Msg("New tick")
			time.Sleep(tickerTimeout)
		}
	}
}

// Stop stops the oracle process and waits for it to gracefully exit.
func (o *Oracle) Stop() {
	o.closer.Close()
	<-o.closer.Done()
}

// GetLastPriceSyncTimestamp returns the latest timestamp at which prices where
// fetched from the oracle's set of exchange rate providers.
func (o *Oracle) GetLastPriceSyncTimestamp() time.Time {
	o.pricesMutex.RLock()
	defer o.pricesMutex.RUnlock()

	return o.lastPriceSyncTS
}

// GetPrices returns a copy of the current prices fetched from the oracle's
// set of exchange rate providers.
func (o *Oracle) GetPrices() map[string]sdk.Dec {
	o.pricesMutex.RLock()
	defer o.pricesMutex.RUnlock()

	// Creates a new array for the prices in the oracle
	prices := make(map[string]sdk.Dec, len(o.prices))
	for k, v := range o.prices {
		// Fills in the prices with each value in the oracle
		prices[k] = v
	}

	return prices
}

// GetTVWAPPrices returns a copy of the tvwapsByProvider map
func (o *Oracle) GetTVWAPPrices() PricesByProvider {
	return o.tvwapsByProvider.GetPricesClone()
}

// GetVWAPPrices returns the vwapsByProvider map using a read lock
func (o *Oracle) GetVWAPPrices() PricesByProvider {
	return o.vwapsByProvider.GetPricesClone()
}

/*
This function is also a method of a struct called Oracle in Go language, this function
performs the following operations:

 - It creates an error group, a mutex, and a couple of maps (providerPrices and providerCandles)
to store the price information from different providers.
 - It also creates a requiredRates map to keep track of the required rates.
 - It then iterates through a map (o.providerPairs) that maps provider names to
currency pairs and for each provider, it calls a function (getOrSetProvider) and pass
the context, it also pass the provider name and gets a provider object.
 - Then for each currency pair it adds the base currency to the requiredRates map.
 - It launches a goroutine using the error group and using the provider object it fetches
prices and candles from the provider.
 - It then flattens and collect prices based on the base currency per provider
 - Finally, it computes the price using GetComputedPrices, then updates the oracle's
prices property by acquiring the lock and it finally returns an error if any.
The function is likely designed to fetch price information from different providers and use that information to compute the prices of different currency pairs. The getOrSetProvider function is likely used to create a provider object for a given provider name, if it does not already exist.
The GetComputedPrices likely to do some logic to calculate price based on the provided information by multiple providers, and the providerPairs, and deviations are likely used to configure which currency pairs and deviation thresholds to use when calculating prices.

*/

// setPrices retrieve all the prices from our set of providers as determined
// in the config, average them out, and update the oracle's current exchange
// rates.
func (o *Oracle) setPrices(ctx context.Context) error {
	g := errgroup.Group{}
	mtx := sync.Mutex{}
	providerPrices := provider.AggregatedProviderPrices{}
	providerCandles := provider.AggregatedProviderCandles{}
	requiredRates := map[string]struct{}{}

	for providerName, currencyPairs := range o.providerPairs {
		pn := providerName
		priceProvider, err := o.getOrSetProvider(ctx, pn)
		if err != nil {
			return err
		}

		for _, pair := range currencyPairs {
			requiredRates[pair.Base] = struct{}{}
		}

		var cp = currencyPairs
		g.Go(func() error {
			prices, err := priceProvider.GetTickerPrices(cp...)
			if err != nil {
				return err
			}

			candles, err := priceProvider.GetCandlePrices(cp...)
			if err != nil {
				return err
			}

			// flatten and collect prices based on the base currency per provider
			//
			// e.g.: {Kraken: {"ATOM": <price, volume>, ...}}
			mtx.Lock()
			for _, pair := range cp {
				success := SetProviderTickerPricesAndCandles(pn, providerPrices, providerCandles, prices, candles, pair)
				if !success {
					mtx.Unlock()
					return fmt.Errorf("failed to find any exchange rates in provider responses")
				}
			}
			mtx.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		o.logger.Err(err).Msg("failed to get ticker prices from provider")
	}

	computedPrices, err := o.GetComputedPrices(
		providerCandles,
		providerPrices,
		o.providerPairs,
		o.deviations,
	)
	if err != nil {
		return err
	}

	for base := range requiredRates {
		if _, ok := computedPrices[base]; !ok {
			o.logger.Warn().Str("asset", base).Msg("unable to report price for expected asset")
		}
	}

	o.pricesMutex.Lock()
	o.prices = computedPrices
	o.pricesMutex.Unlock()
	return nil
}

/*
This function takes in several parameters: providerCandles, providerPrices, providerPairs, and deviations.
It performs the following operations:

- it converts any non-USD denominated candles into USD using ConvertCandlesToUSD function and
returns an error if any.
- it filters out any erroneous candles using filterCandleDeviations function and returns an error if any.
- it computes TVWAP by provider using computeTvwapsByProvider function and sets the values to
o.tvwapsByProvider.
- it computes TVWAP using ComputeTVWAP function.
- If TVWAP candles are not available or were filtered out, the function uses most recent prices
and VWAP instead:
- it converts tickers to USD using ConvertTickersToUSD function.
- it filters tickers deviations using FilterTickerDeviations function.
- it computes VWAP by provider using computeVwapsByProvider function and sets the values to o.vwapsByProvider.
- it computes VWAP using ComputeVWAP function and return the prices and error if any.
This function appears to be responsible for taking in data from multiple providers, performing
some computations and calculations on that data (such as converting to USD and filtering out outliers),
and then returning the final computed prices.
The tvwapsByProvider, vwapsByProvider, prices properties of the struct are likely used to keep track
of the computed prices, so they can be accessed by other parts of the program.
It could be used to calculate the overall prices using multiple providers, and use TVWAP or VWAP to
calculate price, it also consider the deviation rate to filter out inaccurate data.
*/

// GetComputedPrices gets the candle and ticker prices and computes it.
// It returns candles' TVWAP if possible, if not possible (not available
// or due to some staleness) it will use the most recent ticker prices
// and the VWAP formula instead.
func (o *Oracle) GetComputedPrices(
	providerCandles provider.AggregatedProviderCandles,
	providerPrices provider.AggregatedProviderPrices,
	providerPairs map[provider.Name][]types.CurrencyPair,
	deviations map[string]sdk.Dec,
) (prices map[string]sdk.Dec, err error) {
	// convert any non-USD denominated candles into USD
	convertedCandles, err := ConvertCandlesToUSD(
		o.logger,
		providerCandles,
		providerPairs,
		deviations,
	)
	if err != nil {
		return nil, err
	}

	// filter out any erroneous candles
	filteredCandles, err := filterCandleDeviations(
		o.logger,
		convertedCandles,
		deviations,
	)
	if err != nil {
		return nil, err
	}

	computedPrices, _ := computeTvwapsByProvider(filteredCandles)
	o.tvwapsByProvider.SetPrices(computedPrices)

	// attempt to use candles for TVWAP calculations
	tvwapPrices, err := ComputeTVWAP(filteredCandles)
	if err != nil {
		return nil, err
	}

	// If TVWAP candles are not available or were filtered out due to staleness,
	// use most recent prices & VWAP instead.
	if len(tvwapPrices) == 0 {
		convertedTickers, err := ConvertTickersToUSD(
			o.logger,
			providerPrices,
			providerPairs,
			deviations,
		)
		if err != nil {
			return nil, err
		}

		filteredProviderPrices, err := FilterTickerDeviations(
			o.logger,
			convertedTickers,
			deviations,
		)
		if err != nil {
			return nil, err
		}

		o.vwapsByProvider.SetPrices(computeVwapsByProvider(filteredProviderPrices))

		vwapPrices := ComputeVWAP(filteredProviderPrices)

		return vwapPrices, nil
	}

	return tvwapPrices, nil
}

func (o *Oracle) getOrSetProvider(ctx context.Context, providerName provider.Name) (provider.Provider, error) {
	var (
		priceProvider provider.Provider
		ok            bool
	)

	priceProvider, ok = o.priceProviders[providerName]
	if !ok {
		newProvider, err := NewProvider(
			ctx,
			providerName,
			o.logger,
			o.endpoints[providerName],
			o.providerPairs[providerName]...,
		)
		if err != nil {
			return nil, err
		}
		priceProvider = newProvider

		o.priceProviders[providerName] = priceProvider
	}

	return priceProvider, nil
}

func NewProvider(
	ctx context.Context,
	providerName provider.Name,
	logger zerolog.Logger,
	endpoint provider.Endpoint,
	providerPairs ...types.CurrencyPair,
) (provider.Provider, error) {
	switch providerName {
	case provider.Binance:
		return provider.NewBinanceProvider(ctx, logger, endpoint, false, providerPairs...)

	case provider.BinanceUS:
		return provider.NewBinanceProvider(ctx, logger, endpoint, true, providerPairs...)

	case provider.Kraken:
		return provider.NewKrakenProvider(ctx, logger, endpoint, providerPairs...)

	case provider.Osmosis:
		return provider.NewOsmosisProvider(endpoint), nil
	}

	return nil, fmt.Errorf("provider %s not found", providerName)
}

func (o *Oracle) checkAcceptList(params oracletypes.Params) {
	for _, denom := range params.AcceptList {
		symbol := strings.ToUpper(denom.SymbolDenom)
		if _, ok := o.prices[symbol]; !ok {
			o.logger.Warn().Str("denom", symbol).Msg("price missing for required denom")
		}
	}
}

func (o *Oracle) executeTick(ctx context.Context) error {
	o.logger.Debug().Msg("executing oracle tick")

	blockHeight, err := o.client.ChainHeight.GetChainHeight()
	if err != nil {
		return err
	}

	if blockHeight < 1 {
		return errExpectedPositiveBlockHeight
	}

	oracleParams, err := o.getParamCache(ctx, blockHeight)
	if err != nil {
		return err
	}

	if err := o.setPrices(ctx); err != nil {
		return err
	}

	// Get oracle vote period, next block height, current vote period, and index
	// in the vote period.
	oracleVotePeriod := int64(oracleParams.VotePeriod)
	nextBlockHeight := blockHeight + 1
	currentVotePeriod := math.Floor(float64(nextBlockHeight) / float64(oracleVotePeriod))
	indexInVotePeriod := nextBlockHeight % oracleVotePeriod

	ok := o.checkVotingPeriod(currentVotePeriod, oracleVotePeriod, indexInVotePeriod)
	if !ok {
		// either we are past the voting period or skipping this voting period
		return nil
	}

	salt, err := generateSalt(32)
	if err != nil {
		return err
	}

	valAddr, err := sdk.ValAddressFromBech32(o.client.ValidatorAddrString)
	if err != nil {
		return err
	}

	exchangeRatesStr, err := generateExchangeRatesString(o.prices)
	if err != nil {
		return fmt.Errorf("failed to generate exchange rate string %w", err)
	}

	hash := oracletypes.GetAggregateVoteHash(salt, exchangeRatesStr, valAddr)
	preVoteMsg := &oracletypes.MsgAggregateExchangeRatePrevote{
		Hash:      hash.String(), // hash of prices from the oracle
		Feeder:    o.client.OracleAddrString,
		Validator: valAddr.String(),
	}

	if o.previousPrevote == nil {
		// This timeout could be as small as oracleVotePeriod-indexInVotePeriod,
		// but we give it some extra time just in case.
		//
		// Ref : https://github.com/terra-money/oracle-feeder/blob/baef2a4a02f57a2ffeaa207932b2e03d7fb0fb25/feeder/src/vote.ts#L222
		o.logger.Info().
			Str("hash", hash.String()).
			Str("validator", preVoteMsg.Validator).
			Str("feeder", preVoteMsg.Feeder).
			Msg("broadcasting pre-vote")
		if err := o.client.BroadcastTx(nextBlockHeight, oracleVotePeriod*2, preVoteMsg); err != nil {
			return err
		}

		currentHeight, err := o.client.ChainHeight.GetChainHeight()
		if err != nil {
			return err
		}

		o.previousVotePeriod = math.Floor(float64(currentHeight) / float64(oracleVotePeriod))
		o.previousPrevote = &PreviousPrevote{
			Salt:              salt,
			ExchangeRates:     exchangeRatesStr,
			SubmitBlockHeight: currentHeight,
		}
	} else {
		// otherwise, we're in the next voting period and thus we vote
		voteMsg := &oracletypes.MsgAggregateExchangeRateVote{
			Salt:          o.previousPrevote.Salt,
			ExchangeRates: o.previousPrevote.ExchangeRates,
			Feeder:        o.client.OracleAddrString,
			Validator:     valAddr.String(),
		}

		o.logger.Info().
			Str("exchange_rates", voteMsg.ExchangeRates).
			Str("validator", voteMsg.Validator).
			Str("feeder", voteMsg.Feeder).
			Msg("broadcasting vote")
		if err := o.client.BroadcastTx(
			nextBlockHeight,
			oracleVotePeriod-indexInVotePeriod,
			voteMsg,
		); err != nil {
			return err
		}

		o.previousPrevote = nil
		o.previousVotePeriod = 0
	}

	return nil
}

// SetProviderTickerPricesAndCandles flattens and collects prices for
// candles and tickers based on the base currency per provider.
// Returns true if at least one of price or candle exists.
func SetProviderTickerPricesAndCandles(
	providerName provider.Name,
	providerPrices provider.AggregatedProviderPrices,
	providerCandles provider.AggregatedProviderCandles,
	prices map[string]types.TickerPrice,
	candles map[string][]types.CandlePrice,
	pair types.CurrencyPair,
) (success bool) {
	if _, ok := providerPrices[providerName]; !ok {
		providerPrices[providerName] = make(map[string]types.TickerPrice)
	}
	if _, ok := providerCandles[providerName]; !ok {
		providerCandles[providerName] = make(map[string][]types.CandlePrice)
	}

	tp, pricesOk := prices[pair.String()]
	cp, candlesOk := candles[pair.String()]

	if pricesOk {
		providerPrices[providerName][pair.Base] = tp
	}
	if candlesOk {
		providerCandles[providerName][pair.Base] = cp
	}

	return pricesOk || candlesOk
}

// getParamCache returns the last updated parameters of the x/oracle module
// if the current ParamCache is outdated, we will query it again.
func (o *Oracle) getParamCache(ctx context.Context, currentBlockHeigh int64) (oracletypes.Params, error) {
	if !o.paramCache.IsOutdated(currentBlockHeigh) {
		return *o.paramCache.params, nil
	}

	params, err := o.getParams(ctx)
	if err != nil {
		return oracletypes.Params{}, err
	}

	o.checkAcceptList(params)
	o.paramCache.Update(currentBlockHeigh, params)
	return params, nil
}

// getParams returns the current on-chain parameters of the x/oracle module.
func (o *Oracle) getParams(ctx context.Context) (oracletypes.Params, error) {
	grpcConn, err := grpc.Dial(
		o.client.GRPCEndpoint,
		// the Cosmos SDK doesn't support any transport security mechanism
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialerFunc),
	)
	if err != nil {
		return oracletypes.Params{}, fmt.Errorf("failed to dial Cosmos gRPC service: %w", err)
	}

	defer grpcConn.Close()
	queryClient := oracletypes.NewQueryClient(grpcConn)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	queryResponse, err := queryClient.Params(ctx, &oracletypes.QueryParamsRequest{})
	if err != nil {
		return oracletypes.Params{}, fmt.Errorf("failed to get x/oracle params: %w", err)
	}

	return queryResponse.Params, nil
}

func (o *Oracle) checkVotingPeriod(currentVotePeriod float64, oracleVotePeriod, indexInVotePeriod int64) bool {
	// Skip until new voting period. Specifically, skip when:
	// index [0, oracleVotePeriod - 1] > oracleVotePeriod - 2 OR index is 0
	if (o.previousVotePeriod != 0 && currentVotePeriod == o.previousVotePeriod) ||
		oracleVotePeriod-indexInVotePeriod < 2 {
		o.logger.Info().
			Int64("vote_period", oracleVotePeriod).
			Float64("previous_vote_period", o.previousVotePeriod).
			Float64("current_vote_period", currentVotePeriod).
			Msg("skipping until next voting period")

		return false
	}

	// If we're past the voting period we needed to hit, reset and submit another
	// prevote.
	if o.previousVotePeriod != 0 && currentVotePeriod-o.previousVotePeriod != 1 {
		o.logger.Info().
			Int64("vote_period", oracleVotePeriod).
			Float64("previous_vote_period", o.previousVotePeriod).
			Float64("current_vote_period", currentVotePeriod).
			Msg("missing vote during voting period")

		o.previousVotePeriod = 0
		o.previousPrevote = nil
		return false
	}

	return true
}

// generateSalt generates a random salt, size length/2,  as a HEX encoded string.
func generateSalt(length int) (string, error) {
	if length == 0 {
		return "", fmt.Errorf("failed to generate salt: zero length")
	}

	salt := make([]byte, length)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	// Encode the salt as a hex string for storage
	return fmt.Sprintf("%x", salt), nil
}

// generateExchangeRatesString generates a canonical string representation of
// the aggregated exchange rates.
func generateExchangeRatesString(prices map[string]sdk.Dec) (string, error) {
	if len(prices) == 0 {
		return "", errNoPriceAvailable
	}

	exchangeRates := make([]string, len(prices))
	i := 0

	// aggregate exchange rates as "<base>:<price>"
	for base, avgPrice := range prices {
		exchangeRates[i] = fmt.Sprintf("%s:%s", base, avgPrice.String())
		i++
	}

	sort.Strings(exchangeRates)

	return strings.Join(exchangeRates, ","), nil
}
