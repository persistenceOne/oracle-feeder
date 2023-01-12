package client

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	tmrpcclient "github.com/tendermint/tendermint/rpc/client"
	tmctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	errParseEventDataNewBlockHeader = errors.New("error parsing EventDataNewBlockHeader")
	queryEventNewBlockHeader        = tmtypes.QueryForEvent(tmtypes.EventNewBlockHeader)
)

// ChainHeight is used to cache the chain height of the
// current node which is being updated each time the
// node sends an event of EventNewBlockHeader.
// It starts a goroutine to subscribe to blockchain new block event and update the cached height.
type ChainHeight struct {
	Logger zerolog.Logger

	mtx               sync.RWMutex
	errGetChainHeight error
	lastChainHeight   int64
}

// newChainHeight returns a new ChainHeight struct that
// starts a new goroutine subscribed to EventNewBlockHeader.
func newChainHeight(
	ctx context.Context,
	rpcClient tmrpcclient.Client,
	logger zerolog.Logger,
	initialHeight int64,
) (*ChainHeight, error) {
	if initialHeight < 1 {
		return nil, fmt.Errorf("expected positive initial block height")
	}

	if !rpcClient.IsRunning() {
		if err := rpcClient.Start(); err != nil {
			return nil, err
		}
	}

	newBlockHeaderSubscription, err := rpcClient.Subscribe(
		ctx, tmtypes.EventNewBlockHeader, queryEventNewBlockHeader.String())
	if err != nil {
		return nil, err
	}

	chainHeight := &ChainHeight{
		Logger:            logger.With().Str("oracle_client", "chain_height").Logger(),
		errGetChainHeight: nil,
		lastChainHeight:   initialHeight,
	}

	go chainHeight.subscribe(ctx, rpcClient, newBlockHeaderSubscription)

	return chainHeight, nil
}

// updateChainHeight receives the data to be updated thread safe.
func (ch *ChainHeight) updateChainHeight(blockHeight int64, err error) {
	ch.mtx.Lock()
	defer ch.mtx.Unlock()

	ch.lastChainHeight = blockHeight
	ch.errGetChainHeight = err
}

// subscribe listens to new blocks being made
// and updates the chain height.
func (ch *ChainHeight) subscribe(
	ctx context.Context,
	eventsClient tmrpcclient.EventsClient,
	newBlockHeaderSubscription <-chan tmctypes.ResultEvent,
) {
	// Unsubscribe from new block header events and log an error if it fails
	defer func() {
		err := eventsClient.Unsubscribe(ctx, tmtypes.EventNewBlockHeader, queryEventNewBlockHeader.String())
		if err != nil {
			ch.Logger.Err(err)
			ch.updateChainHeight(ch.lastChainHeight, err)
		}
		ch.Logger.Info().Msg("closing the ChainHeight new block header subscription")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case resultEvent := <-newBlockHeaderSubscription:
			eventDataNewBlockHeader, ok := resultEvent.Data.(tmtypes.EventDataNewBlockHeader)
			if !ok {
				ch.Logger.Err(errParseEventDataNewBlockHeader)
				ch.updateChainHeight(ch.lastChainHeight, errParseEventDataNewBlockHeader)
				continue
			}
			ch.updateChainHeight(eventDataNewBlockHeader.Header.Height, nil)
		}
	}
}

// GetChainHeight returns the last chain height available.
func (ch *ChainHeight) GetChainHeight() (int64, error) {
	ch.mtx.RLock()
	defer ch.mtx.RUnlock()

	return ch.lastChainHeight, ch.errGetChainHeight
}
