package types

const ModuleName = "price-feeder"

//nolint:gomnd //const
var (
	ErrMissingExchangeRate = "missing exchange rate for %s"
	ErrWebsocketDial       = "error connecting to %s websocket: %w"
	ErrWebsocketClose      = "error closing %s websocket: %w"
	ErrWebsocketSend       = "error sending to %s websocket: %w"
	ErrWebsocketRead       = "error reading from %s websocket: %w"
	ErrTickerNotFound      = "%s failed to get ticker price for %s"
	ErrCandleNotFound      = "%s failed to get candle price for %s"
)
