package keyring

import "github.com/pkg/errors"

var (
	ErrCosmosKeyringCreationFailed = errors.New("cosmos keyring creation failed")
	ErrCosmosKeyringImportFailed   = errors.New("cosmos keyring unable to import key")
	ErrDeriveFailed                = errors.New("key derivation failed")
	ErrFailedToApplyConfigOption   = errors.New("failed to apply config option")
	ErrFilepathIncorrect           = errors.New("incorrect filepath")
	ErrHexFormatError              = errors.New("hex format error")
	ErrIncompatibleOptionsProvided = errors.New("incompatible keyring options provided")
	ErrInsufficientKeyDetails      = errors.New("insufficient cosmos key details provided")
	ErrKeyIncompatible             = errors.New("provided key is incompatible with requested config")
	ErrKeyInfoNotFound             = errors.New("key info not found")
	ErrPrivkeyConflict             = errors.New("privkey conflict")
	ErrUnexpectedAddress           = errors.New("unexpected address")
)
