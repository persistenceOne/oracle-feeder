package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	rpcclient "github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/rs/zerolog"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	tmjsonclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"

	"github.com/persistenceOne/persistence-sdk/simapp"
	simparams "github.com/persistenceOne/persistence-sdk/simapp/params"
)

const (
	wsEndPoint    = "/websocket"
	jsonFormat    = "json"
	oracleAppName = "oracle"
)

type (
	// OracleClient defines a structure that interfaces with the persistence node.
	OracleClient struct {
		Logger              zerolog.Logger
		ChainID             string
		KeyringBackend      string
		KeyringDir          string
		KeyringPass         string
		TMRPC               string
		RPCTimeout          time.Duration
		OracleAddr          sdk.AccAddress
		OracleAddrString    string
		ValidatorAddr       sdk.ValAddress
		ValidatorAddrString string
		Encoding            simparams.EncodingConfig
		GasPrices           string
		GasAdjustment       float64
		GRPCEndpoint        string
		KeyringPassphrase   string
		ChainHeight         *ChainHeight
		Fees                string
	}

	passReader struct {
		pass string
		buf  *bytes.Buffer
	}
)

func NewOracleClient(
	ctx context.Context,
	logger zerolog.Logger,
	chainID string,
	keyringBackend string,
	keyringDir string,
	keyringPass string,
	tmRPC string,
	rpcTimeout time.Duration,
	oracleAddrString string,
	validatorAddrString string,
	grpcEndpoint string,
	gasAdjustment float64,
	fees string,
) (OracleClient, error) {
	oracleAddr, err := sdk.AccAddressFromBech32(oracleAddrString)
	if err != nil {
		return OracleClient{}, err
	}

	oracleClient := OracleClient{
		Logger:              logger.With().Str("module", "oracle_client").Logger(),
		ChainID:             chainID,
		KeyringBackend:      keyringBackend,
		KeyringDir:          keyringDir,
		KeyringPass:         keyringPass,
		TMRPC:               tmRPC,
		RPCTimeout:          rpcTimeout,
		OracleAddr:          oracleAddr,
		OracleAddrString:    oracleAddrString,
		ValidatorAddr:       sdk.ValAddress(validatorAddrString),
		ValidatorAddrString: validatorAddrString,
		Encoding:            simapp.MakeTestEncodingConfig(),
		GasAdjustment:       gasAdjustment,
		GRPCEndpoint:        grpcEndpoint,
		Fees:                fees,
	}

	clientCtx, err := oracleClient.createClientContext()
	if err != nil {
		return OracleClient{}, err
	}

	blockHeight, err := rpcclient.GetChainHeight(clientCtx)
	if err != nil {
		return OracleClient{}, err
	}

	chainHeight, err := newChainHeight(
		ctx,
		clientCtx.Client,
		oracleClient.Logger,
		blockHeight,
	)
	if err != nil {
		return OracleClient{}, err
	}
	oracleClient.ChainHeight = chainHeight

	return oracleClient, nil
}

func newPassReader(pass string) io.Reader {
	return &passReader{
		pass: pass,
		buf:  new(bytes.Buffer),
	}
}

func (r *passReader) Read(p []byte) (n int, err error) {
	n, err = r.buf.Read(p)
	if err == io.EOF || n == 0 {
		r.buf.WriteString(r.pass + "\n")

		n, err = r.buf.Read(p)
	}

	return n, err
}

// BroadcastTx attempts to broadcast a signed transaction. If it fails, a few re-attempts
// will be made until the transaction succeeds or ultimately times out or fails.
// Ref: https://github.com/terra-money/oracle-feeder/blob/baef2a4a02f57a2ffeaa207932b2e03d7fb0fb25/feeder/src/vote.ts#L230
func (oc OracleClient) BroadcastTx(nextBlockHeight, timeoutHeight int64, msgs ...sdk.Msg) error {
	maxBlockHeight := nextBlockHeight + timeoutHeight
	lastCheckHeight := nextBlockHeight - 1

	clientCtx, err := oc.createClientContext()
	if err != nil {
		return err
	}

	factory, err := oc.createTxFactory()
	if err != nil {
		return err
	}

	// re-try voting until timeout
	for {
		latestBlockHeight, err := oc.ChainHeight.GetChainHeight()
		if err != nil {
			return err
		}

		if latestBlockHeight <= lastCheckHeight {
			time.Sleep(time.Second * 1) // sleep before retrying
			continue
		}

		// set last check height to latest block height
		lastCheckHeight = latestBlockHeight

		if lastCheckHeight >= maxBlockHeight {
			return errors.New("broadcasting tx timed out")
		}

		resp, err := broadcastTx(clientCtx, factory, msgs...)
		if resp != nil && resp.Code != 0 {
			err = fmt.Errorf("invalid response code from tx: %d", resp.Code)
		}
		if err != nil {
			var (
				code uint32
				hash string
			)
			if resp != nil {
				code = resp.Code
				hash = resp.TxHash
			}

			oc.Logger.Debug().
				Err(err).
				Int64("max_height", maxBlockHeight).
				Int64("last_check_height", lastCheckHeight).
				Str("tx_hash", hash).
				Uint32("tx_code", code).
				Msg("failed to broadcast tx; retrying...")

			time.Sleep(time.Second * 1)
			continue
		}

		oc.Logger.Info().
			Uint32("tx_code", resp.Code).
			Str("tx_hash", resp.TxHash).
			Int64("tx_height", resp.Height).
			Msg("successfully broadcasted tx")

		return nil
	}
}

// createClientContext creates an SDK client Context instance used for transaction
// generation, signing and broadcasting.
func (oc OracleClient) createClientContext() (client.Context, error) {
	var keyringInput io.Reader
	if len(oc.KeyringPass) > 0 {
		keyringInput = newPassReader(oc.KeyringPass)
	} else {
		keyringInput = os.Stdin
	}

	kr, err := keyring.New(oracleAppName, oc.KeyringBackend, oc.KeyringDir, keyringInput)
	if err != nil {
		return client.Context{}, err
	}

	httpClient, err := tmjsonclient.DefaultHTTPClient(oc.TMRPC)
	if err != nil {
		return client.Context{}, err
	}

	httpClient.Timeout = oc.RPCTimeout

	tmRPC, err := rpchttp.NewWithClient(oc.TMRPC, wsEndPoint, httpClient)
	if err != nil {
		return client.Context{}, err
	}

	keyInfo, err := kr.KeyByAddress(oc.OracleAddr)
	if err != nil {
		return client.Context{}, err
	}
	clientCtx := client.Context{
		ChainID:           oc.ChainID,
		InterfaceRegistry: oc.Encoding.InterfaceRegistry,
		Output:            os.Stderr,
		BroadcastMode:     flags.BroadcastSync,
		TxConfig:          oc.Encoding.TxConfig,
		AccountRetriever:  authtypes.AccountRetriever{},
		Codec:             oc.Encoding.Marshaler,
		LegacyAmino:       oc.Encoding.Amino,
		Input:             os.Stdin,
		NodeURI:           oc.TMRPC,
		Client:            tmRPC,
		Keyring:           kr,
		FromAddress:       oc.OracleAddr,
		FromName:          keyInfo.GetName(),
		From:              keyInfo.GetName(),
		OutputFormat:      jsonFormat,
		UseLedger:         false,
		Simulate:          false,
		GenerateOnly:      false,
		Offline:           false,
		SkipConfirm:       true,
	}

	return clientCtx, nil
}

// createTxFactory creates an SDK Factory instance used for transaction
// generation, signing and broadcasting.
func (oc OracleClient) createTxFactory() (tx.Factory, error) {
	clientCtx, err := oc.createClientContext()
	if err != nil {
		return tx.Factory{}, err
	}

	txFactory := tx.Factory{}.
		WithAccountRetriever(clientCtx.AccountRetriever).
		WithChainID(oc.ChainID).
		WithTxConfig(clientCtx.TxConfig).
		WithGasAdjustment(oc.GasAdjustment).
		WithGasPrices(oc.GasPrices).
		WithKeybase(clientCtx.Keyring).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT).
		WithSimulateAndExecute(true).
		WithFees(oc.Fees) // TODO: discuss this

	return txFactory, nil
}
