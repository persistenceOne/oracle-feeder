package keyring

import (
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cosmkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	multisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

func (s *KeyringTestSuite) TestErrCosmosKeyringCreationFailed() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithKeyFrom("persistence1t6dq82wyggtmu2cvegyat9et7uans46n9vfmj2"),
		WithKeyringBackend("kowabunga"),
	)

	requireT.ErrorIs(err, ErrCosmosKeyringCreationFailed)
}

func (s *KeyringTestSuite) TestErrCosmosKeyringImportFailed() {
	// No test here:
	//
	// failed to find scenario where this error could be returned
}

func (s *KeyringTestSuite) TestErrDeriveFailed() {
	// No test here:
	//
	// failed to find a scenario when it breaks, the original go-bip39
	// package that may return errors doesn't have test either.
}

func (s *KeyringTestSuite) TestErrFailedToApplyConfigOption() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithMnemonic(`???`),
	)

	requireT.ErrorIs(err, ErrFailedToApplyConfigOption)
}

func (s *KeyringTestSuite) TestErrFilepathIncorrect() {
	// No test here:
	//
	// fails only when Go syscall fails to get cwd (impossible to simulate on macOS)
}

func (s *KeyringTestSuite) TestErrHexFormatError() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithPrivKeyHex("nothex"),
	)

	requireT.ErrorIs(err, ErrHexFormatError)

	_, _, err = NewCosmosKeyring(
		WithMnemonic(testMnemonic),
		WithPrivKeyHex("nothex"),
	)

	requireT.ErrorIs(err, ErrHexFormatError)
}

func (s *KeyringTestSuite) TestErrIncompatibleOptionsProvided() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithMnemonic(testMnemonic),
		WithUseLedger(true),
	)

	requireT.ErrorIs(err, ErrIncompatibleOptionsProvided)

	_, _, err = NewCosmosKeyring(
		WithPrivKeyHex(testPrivKeyHex),
		WithUseLedger(true),
	)

	requireT.ErrorIs(err, ErrIncompatibleOptionsProvided)
}

func (s *KeyringTestSuite) TestErrInsufficientKeyDetails() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring()

	requireT.ErrorIs(err, ErrInsufficientKeyDetails)
}

func (s *KeyringTestSuite) TestErrKeyIncompatible() {
	requireT := s.Require()

	addr, kb, err := NewCosmosKeyring(
		WithPrivKeyHex(testPrivKeyHex),
	)
	requireT.NoError(err)

	testInfo, err := kb.KeyByAddress(addr)
	requireT.NoError(err)

	kbDir, err := os.MkdirTemp(os.TempDir(), "keyring-test-kbroot-*")
	requireT.NoError(err)
	s.T().Cleanup(func() {
		_ = os.RemoveAll(kbDir)
	})

	testKeyring, err := cosmkeyring.New(
		"keyring_test",
		cosmkeyring.BackendTest,
		kbDir,
		nil,
	)
	requireT.NoError(err)

	// This test requires a ledger-enabled build & device
	//
	// _, err = testKeyring.SaveLedgerKey(
	// 	"test_ledeger",
	// 	hd.Secp256k1,
	// 	"", 0, 0, 0,
	// )
	// requireT.NoError(err)

	_, err = testKeyring.SavePubKey("test_pubkey", testInfo.GetPubKey(), hd.Secp256k1Type)
	requireT.NoError(err)

	_, _, err = NewCosmosKeyring(
		WithKeyFrom("test_pubkey"),
		WithKeyringBackend(BackendTest),
		WithKeyringDir(kbDir),
		WithKeyringAppName("keyring_test"),
	)
	requireT.ErrorIs(err, ErrKeyIncompatible)

	multisigPubkey := multisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{
		testInfo.GetPubKey(),
	})

	_, err = testKeyring.SaveMultisig("test_multisig", multisigPubkey)
	requireT.NoError(err)

	_, _, err = NewCosmosKeyring(
		WithKeyFrom("test_multisig"),
		WithKeyringBackend(BackendTest),
		WithKeyringDir(kbDir),
		WithKeyringAppName("keyring_test"),
	)
	requireT.ErrorIs(err, ErrKeyIncompatible)
}

func (s *KeyringTestSuite) TestErrKeyInfoNotFound() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithKeyringBackend(BackendFile),
		WithKeyringDir("./testdata"),
		WithKeyFrom("kowabunga"),
		WithKeyPassphrase("test12345678"),
	)

	requireT.ErrorIs(err, ErrKeyInfoNotFound)
}

func (s *KeyringTestSuite) TestErrPrivkeyConflict() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithPrivKeyHex("0000000000000000000000000000000000000000000000000000000000000000"),
		WithMnemonic(testMnemonic), // different mnemonic
	)

	requireT.ErrorIs(err, ErrPrivkeyConflict)
}

func (s *KeyringTestSuite) TestErrUnexpectedAddress() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		WithPrivKeyHex("0000000000000000000000000000000000000000000000000000000000000000"),
		WithKeyFrom("persistence1t6dq82wyggtmu2cvegyat9et7uans46n9vfmj2"), // will not match privkey above
	)

	requireT.ErrorIs(err, ErrUnexpectedAddress)

	_, _, err = NewCosmosKeyring(
		WithMnemonic(testMnemonic),
		WithKeyFrom("persistence1pkkayn066msg6kn33wnl5srhdt3tnu2vv3k3tu"), // will not match mnemonic above
	)

	requireT.ErrorIs(err, ErrUnexpectedAddress)
}
