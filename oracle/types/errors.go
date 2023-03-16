package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const ModuleName = "price-feeder"

//nolint:gomnd //const
var (
	ErrMissingExchangeRate = sdkerrors.Register(ModuleName, 1, "missing exchange rate for %s")
	ErrWebsocketDial       = sdkerrors.Register(ModuleName, 2, "error connecting to %s websocket: %w")
	ErrWebsocketClose      = sdkerrors.Register(ModuleName, 3, "error closing %s websocket: %w")
	ErrWebsocketSend       = sdkerrors.Register(ModuleName, 4, "error sending to %s websocket: %w")
	ErrWebsocketRead       = sdkerrors.Register(ModuleName, 5, "error reading from %s websocket: %w")
	ErrTickerNotFound      = sdkerrors.Register(ModuleName, 6, "%s failed to get ticker price for %s")
	ErrCandleNotFound      = sdkerrors.Register(ModuleName, 7, "%s failed to get candle price for %s")
)
