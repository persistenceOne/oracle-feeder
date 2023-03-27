package keyring

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	cosmcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cosmkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
)

var (
	defaultKeyringKeyName = "default"
	emptyCosmosAddress    = sdk.AccAddress{}
)

// NewCosmosKeyring creates a new keyring from a variety of options. See ConfigOpt and related options.
func NewCosmosKeyring(opts ...ConfigOpt) (sdk.AccAddress, cosmkeyring.Keyring, error) {
	config := &cosmosKeyringConfig{}
	for optIdx, optFn := range opts {
		if err := optFn(config); err != nil {
			err = errors.Wrapf(ErrFailedToApplyConfigOption, "option #%d: %s", optIdx+1, err.Error())
			return emptyCosmosAddress, nil, err
		}
	}

	switch {
	case len(config.Mnemonic) > 0:
		if config.UseLedger {
			err := errors.Wrap(ErrIncompatibleOptionsProvided, "cannot combine ledger and mnemonic options")
			return emptyCosmosAddress, nil, err
		}

		return fromMnemonic(config, config.Mnemonic)

	case len(config.PrivKeyHex) > 0:
		if config.UseLedger {
			err := errors.Wrap(ErrIncompatibleOptionsProvided, "cannot combine ledger and privkey options")
			return emptyCosmosAddress, nil, err
		}

		return fromPrivkeyHex(config, config.PrivKeyHex)

	case len(config.KeyFrom) > 0:
		var fromIsAddress bool

		addressFrom, err := sdk.AccAddressFromBech32(config.KeyFrom)
		if err == nil {
			fromIsAddress = true
		}

		return fromCosmosKeyring(config, addressFrom, fromIsAddress)

	default:
		return emptyCosmosAddress, nil, errors.WithStack(ErrInsufficientKeyDetails)
	}
}

func fromPrivkeyHex(
	config *cosmosKeyringConfig,
	privkeyHex string,
) (sdk.AccAddress, cosmkeyring.Keyring, error) {
	pkBytes, err := hexToBytes(privkeyHex)
	if err != nil {
		err = errors.Wrapf(ErrHexFormatError, "failed to decode cosmos account privkey: %s", err.Error())
		return emptyCosmosAddress, nil, err
	}

	cosmosAccPk := hd.Secp256k1.Generate()(pkBytes)
	addressFromPk := sdk.AccAddress(cosmosAccPk.PubKey().Address().Bytes())

	var keyName string

	// check that if cosmos 'From' specified separately, it must match the provided privkey
	if len(config.KeyFrom) > 0 {
		addressFrom, err := sdk.AccAddressFromBech32(config.KeyFrom)
		if err == nil {
			if !bytes.Equal(addressFrom.Bytes(), addressFromPk.Bytes()) {
				err = errors.Wrapf(
					ErrUnexpectedAddress,
					"expected account address %s but got %s from the private key",
					addressFrom.String(), addressFromPk.String(),
				)

				return emptyCosmosAddress, nil, err
			}
		} else {
			// use it as a name then
			keyName = config.KeyFrom
		}
	}

	if len(keyName) == 0 {
		keyName = defaultKeyringKeyName
	}

	// wrap a PK into a Keyring
	kb, err := newFromPrivKey(keyName, cosmosAccPk)
	if err != nil {
		err = errors.WithStack(err)
	}

	return addressFromPk, kb, err
}

func fromMnemonic(
	config *cosmosKeyringConfig,
	mnemonic string,
) (sdk.AccAddress, cosmkeyring.Keyring, error) {
	cfg := sdk.GetConfig()

	pkBytes, err := hd.Secp256k1.Derive()(
		mnemonic,
		cosmkeyring.DefaultBIP39Passphrase,
		cfg.GetFullBIP44Path(),
	)
	if err != nil {
		err = errors.Wrapf(ErrDeriveFailed, "failed to derive secp256k1 private key: %s", err.Error())
		return emptyCosmosAddress, nil, err
	}

	cosmosAccPk := hd.Secp256k1.Generate()(pkBytes)
	addressFromPk := sdk.AccAddress(cosmosAccPk.PubKey().Address().Bytes())

	var keyName string

	// check that if cosmos 'From' specified separately, it must match the derived privkey
	if len(config.KeyFrom) > 0 {
		addressFrom, err := sdk.AccAddressFromBech32(config.KeyFrom)
		if err == nil {
			if !bytes.Equal(addressFrom.Bytes(), addressFromPk.Bytes()) {
				err = errors.Wrapf(
					ErrUnexpectedAddress,
					"expected account address %s but got %s from the mnemonic at /0",
					addressFrom.String(), addressFromPk.String(),
				)

				return emptyCosmosAddress, nil, err
			}
		} else {
			// use it as a name then
			keyName = config.KeyFrom
		}
	}

	// check that if 'PrivKeyHex' specified separately, it must match the derived privkey too
	if len(config.PrivKeyHex) > 0 {
		if err := checkPrivkeyHexMatchesMnemonic(config.PrivKeyHex, pkBytes); err != nil {
			return emptyCosmosAddress, nil, err
		}
	}

	if len(keyName) == 0 {
		keyName = defaultKeyringKeyName
	}

	// wrap a PK into a Keyring
	kb, err := newFromPrivKey(keyName, cosmosAccPk)
	if err != nil {
		err = errors.WithStack(err)
	}

	return addressFromPk, kb, err
}

func checkPrivkeyHexMatchesMnemonic(pkHex string, mnemonicDerivedPkBytes []byte) error {
	pkBytesFromHex, err := hexToBytes(pkHex)
	if err != nil {
		err = errors.Wrapf(ErrHexFormatError, "failed to decode cosmos account privkey: %s", err.Error())
		return err
	}

	if !bytes.Equal(mnemonicDerivedPkBytes, pkBytesFromHex) {
		err := errors.Wrap(
			ErrPrivkeyConflict,
			"both mnemonic and privkey hex options provided, but privkey doesn't match mnemonic",
		)
		return err
	}

	return nil
}

func fromCosmosKeyring(
	config *cosmosKeyringConfig,
	fromAddress sdk.AccAddress,
	fromIsAddress bool,
) (sdk.AccAddress, cosmkeyring.Keyring, error) {
	var passReader io.Reader = os.Stdin
	if len(config.KeyPassphrase) > 0 {
		passReader = newPassReader(config.KeyPassphrase)
	}

	var (
		absoluteKeyringDir string
		err                error
	)
	if filepath.IsAbs(config.KeyringDir) {
		absoluteKeyringDir = config.KeyringDir
	} else {
		absoluteKeyringDir, err = filepath.Abs(config.KeyringDir)
		if err != nil {
			err = errors.Wrapf(ErrFilepathIncorrect, "failed to get abs path for keyring dir: %s", err.Error())
			return emptyCosmosAddress, nil, err
		}
	}

	kb, err := cosmkeyring.New(
		config.KeyringAppName,
		string(config.KeyringBackend),
		absoluteKeyringDir,
		passReader,
	)
	if err != nil {
		err = errors.Wrapf(ErrCosmosKeyringCreationFailed, "failed to init cosmos keyring: %s", err.Error())
		return emptyCosmosAddress, nil, err
	}

	var keyInfo cosmkeyring.Info
	if fromIsAddress {
		if keyInfo, err = kb.KeyByAddress(fromAddress); err != nil {
			err = errors.Wrapf(
				ErrKeyInfoNotFound,
				"couldn't find an entry for the key %s in keybase: %s",
				fromAddress.String(), err.Error(),
			)
			return emptyCosmosAddress, nil, err
		}
	} else {
		if keyInfo, err = kb.Key(config.KeyFrom); err != nil {
			err = errors.Wrapf(
				ErrKeyInfoNotFound,
				"could not find an entry for the key '%s' in keybase: %s",
				config.KeyFrom, err.Error(),
			)
			return emptyCosmosAddress, nil, err
		}
	}

	if err := checkKeyInfo(config, keyInfo); err != nil {
		return emptyCosmosAddress, nil, err
	}

	return keyInfo.GetAddress(), kb, nil
}

func checkKeyInfo(
	config *cosmosKeyringConfig,
	keyInfo cosmkeyring.Info,
) error {
	switch keyType := keyInfo.GetType(); keyType {
	case cosmkeyring.TypeLocal:
		// kb has a key and it's totally usable
		return nil

	case cosmkeyring.TypeLedger:
		// the kb stores references to ledger keys, so we must explicitly
		// check that. kb doesn't know how to scan HD keys - they must be added manually before
		if config.UseLedger {
			return nil
		}
		err := errors.Wrapf(
			ErrKeyIncompatible,
			"'%s' key is a ledger reference, enable ledger option",
			keyInfo.GetName(),
		)
		return err

	case cosmkeyring.TypeOffline:
		err := errors.Wrapf(
			ErrKeyIncompatible,
			"'%s' key is an offline key, not supported yet",
			keyInfo.GetName(),
		)
		return err

	case cosmkeyring.TypeMulti:
		err := errors.Wrapf(
			ErrKeyIncompatible,
			"'%s' key is an multisig key, not supported yet",
			keyInfo.GetName(),
		)
		return err

	default:
		err := errors.Wrapf(
			ErrKeyIncompatible,
			"'%s' key  has unsupported type: %s",
			keyInfo.GetName(), keyType,
		)
		return err
	}
}

func newPassReader(pass string) io.Reader {
	return &passReader{
		pass: pass,
		buf:  new(bytes.Buffer),
	}
}

type passReader struct {
	pass string
	buf  *bytes.Buffer
}

var _ io.Reader = &passReader{}

func (r *passReader) Read(p []byte) (n int, err error) {
	n, err = r.buf.Read(p)
	if err == io.EOF || n == 0 {
		r.buf.WriteString(r.pass + "\n")

		n, err = r.buf.Read(p)
	}

	return n, err
}

// newFromPrivKey creates a temporary in-mem keyring for a PrivKey.
// Allows to init Context when the key has been provided in plaintext and parsed.
func newFromPrivKey(name string, privKey cryptotypes.PrivKey) (cosmkeyring.Keyring, error) {
	kb := cosmkeyring.NewInMemory()
	tmpPhrase := randPhrase(64)
	armored := cosmcrypto.EncryptArmorPrivKey(privKey, tmpPhrase, privKey.Type())
	err := kb.ImportPrivKey(name, armored, tmpPhrase)
	if err != nil {
		err = errors.Wrapf(ErrCosmosKeyringImportFailed, "failed to import privkey: %s", err.Error())
		return nil, err
	}

	return kb, nil
}

func hexToBytes(str string) ([]byte, error) {
	data, err := hex.DecodeString(strings.TrimPrefix(str, "0x"))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func randPhrase(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return string(buf)
}
