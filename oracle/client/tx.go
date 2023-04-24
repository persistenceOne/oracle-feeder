package client

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

const awaitBlock = 1

// broadcastTx attempts to generate, sign and broadcast a transaction with the
// given set of messages. It will also simulate gas requirements if necessary.
// It will return an error upon failure.
//
// Note, broadcastTx is copied from the SDK except it removes a few unnecessary
// things like prompting for confirmation and printing the response. Instead,
// we return the TxResponse.
func broadcastTx(
	ctx context.Context,
	clientCtx client.Context,
	txf tx.Factory, msgs ...sdk.Msg,
) (*sdk.TxResponse, error) {
	txf, err := prepareFactory(clientCtx, txf)
	if err != nil {
		return nil, err
	}

	_, adjusted, err := tx.CalculateGas(clientCtx, txf, msgs...)
	if err != nil {
		return nil, err
	}

	txf = txf.WithGas(adjusted)

	unsignedTx, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	unsignedTx.SetFeeGranter(clientCtx.GetFeeGranterAddress())

	if err = tx.Sign(txf, clientCtx.GetFromName(), unsignedTx, true); err != nil {
		return nil, err
	}

	txBytes, err := clientCtx.TxConfig.TxEncoder()(unsignedTx.GetTx())
	if err != nil {
		return nil, err
	}

	resp, err := clientCtx.BroadcastTx(txBytes)
	if err := handleBroadcastResult(resp, err); err != nil {
		return nil, err
	}

	res, err := waitForTx(ctx, clientCtx.Client, resp.TxHash)
	if err != nil {
		return nil, err
	}

	return sdk.NewResponseResultTx(res, nil, ""), nil
}

// prepareFactory ensures the account defined by ctx.GetFromAddress() exists and
// if the account number and/or the account sequence number are zero (not set),
// they will be queried for and set on the provided Factory. A new Factory with
// the updated fields will be returned.
func prepareFactory(clientCtx client.Context, txf tx.Factory) (tx.Factory, error) {
	from := clientCtx.GetFromAddress()

	if err := txf.AccountRetriever().EnsureExists(clientCtx, from); err != nil {
		return txf, err
	}

	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		num, seq, err := txf.AccountRetriever().GetAccountNumberSequence(clientCtx, from)
		if err != nil {
			return txf, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	return txf, nil
}

// handleBroadcastResult handles the result of broadcast messages result and checks if an error occurred.
func handleBroadcastResult(resp *sdk.TxResponse, err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.New("make sure that your account has enough balance")
		}

		return err
	}

	if resp.Code != 0 {
		return errors.Errorf("error code: '%d' msg: '%s'", resp.Code, resp.RawLog)
	}
	return nil
}

// WaitForTx requests the tx from hash, if not found, waits for next block and
// tries again. Returns an error if ctx is canceled.
func waitForTx(ctx context.Context, client rpcclient.Client, hash string) (*ctypes.ResultTx, error) {
	bz, err := hex.DecodeString(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to decode tx hash '%s'", hash)
	}

	for {
		resp, err := client.Tx(ctx, bz, false)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				// Tx not found, wait for next block and try again
				err := waitForNextBlock(ctx, client)
				if err != nil {
					return nil, errors.Wrap(err, "waiting for next block")
				}
				continue
			}
			return nil, errors.Wrapf(err, "fetching tx '%s'", hash)
		}

		// Tx found
		return resp, nil
	}
}

func waitForNextBlock(ctx context.Context, c rpcclient.Client) error {
	start, err := latestBlockHeight(ctx, c)
	if err != nil {
		return err
	}

	return waitForBlockHeight(ctx, c, start+awaitBlock)
}

// waitForBlockHeight waits until block height h is committed, or returns an
// error if ctx is canceled.
func waitForBlockHeight(ctx context.Context, c rpcclient.Client, h int64) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		latestHeight, err := latestBlockHeight(ctx, c)
		if err != nil {
			return err
		}
		if latestHeight >= h {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "timeout exceeded waiting for block")
		case <-ticker.C:
		}
	}
}

// latestBlockHeight returns the latest block height of the app.
func latestBlockHeight(ctx context.Context, c rpcclient.Client) (int64, error) {
	resp, err := c.Status(ctx)
	if err != nil {
		return 0, err
	}

	return resp.SyncInfo.LatestBlockHeight, nil
}
