package keyring

import (
	"os"

	cosmkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	multisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

func (s *KeyringTestSuite) TestErrCosmosKeyringCreationFailed() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithKeyFrom(testAccAddressBech),
		WithKeyringBackend("kowabunga"),
	)

	requireT.ErrorIs(err, ErrCosmosKeyringCreationFailed)
}

func (s *KeyringTestSuite) TestErrFailedToApplyConfigOption() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithMnemonic(`???`),
	)

	requireT.ErrorIs(err, ErrFailedToApplyConfigOption)
}

func (s *KeyringTestSuite) TestErrHexFormatError() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex("nothex"),
	)

	requireT.ErrorIs(err, ErrHexFormatError)

	_, _, err = NewCosmosKeyring(
		s.cdc,
		WithMnemonic(testMnemonic),
		WithPrivKeyHex("nothex"),
	)

	requireT.ErrorIs(err, ErrHexFormatError)
}

func (s *KeyringTestSuite) TestErrIncompatibleOptionsProvided() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithMnemonic(testMnemonic),
		WithUseLedger(true),
	)

	requireT.ErrorIs(err, ErrIncompatibleOptionsProvided)

	_, _, err = NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex(testPrivKeyHex),
		WithUseLedger(true),
	)

	requireT.ErrorIs(err, ErrIncompatibleOptionsProvided)
}

func (s *KeyringTestSuite) TestErrInsufficientKeyDetails() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(s.cdc)

	requireT.ErrorIs(err, ErrInsufficientKeyDetails)
}

func (s *KeyringTestSuite) TestErrKeyIncompatible() {
	requireT := s.Require()

	addr, kb, err := NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex(testPrivKeyHex),
	)
	requireT.NoError(err)

	testRecord, err := kb.KeyByAddress(addr)
	requireT.NoError(err)
	testRecordPubKey, err := testRecord.GetPubKey()
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
		s.cdc,
	)
	requireT.NoError(err)

	_, err = testKeyring.SaveOfflineKey("test_pubkey", testRecordPubKey)
	requireT.NoError(err)

	_, _, err = NewCosmosKeyring(
		s.cdc,
		WithKeyFrom("test_pubkey"),
		WithKeyringBackend(BackendTest),
		WithKeyringDir(kbDir),
		WithKeyringAppName("keyring_test"),
	)
	requireT.ErrorIs(err, ErrKeyIncompatible)

	multisigPubkey := multisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{
		testRecordPubKey,
	})

	_, err = testKeyring.SaveMultisig("test_multisig", multisigPubkey)
	requireT.NoError(err)

	_, _, err = NewCosmosKeyring(
		s.cdc,
		WithKeyFrom("test_multisig"),
		WithKeyringBackend(BackendTest),
		WithKeyringDir(kbDir),
		WithKeyringAppName("keyring_test"),
	)
	requireT.ErrorIs(err, ErrKeyIncompatible)
}

func (s *KeyringTestSuite) TestErrKeyRecordNotFound() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithKeyringBackend(BackendFile),
		WithKeyringDir("./testdata"),
		WithKeyFrom("kowabunga"),
		WithKeyPassphrase("test12345678"),
	)

	requireT.ErrorIs(err, ErrKeyRecordNotFound)
}

func (s *KeyringTestSuite) TestErrPrivkeyConflict() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex("0000000000000000000000000000000000000000000000000000000000000000"),
		WithMnemonic(testMnemonic), // different mnemonic
	)

	requireT.ErrorIs(err, ErrPrivkeyConflict)
}

func (s *KeyringTestSuite) TestErrUnexpectedAddress() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex("0000000000000000000000000000000000000000000000000000000000000000"),
		WithKeyFrom(testAccAddressBech), // will not match privkey above
	)

	requireT.ErrorIs(err, ErrUnexpectedAddress)

	_, _, err = NewCosmosKeyring(
		s.cdc,
		WithMnemonic(testMnemonic),
		WithKeyFrom("persistence1pkkayn066msg6kn33wnl5srhdt3tnu2vv3k3tu"), // will not match mnemonic above
	)

	requireT.ErrorIs(err, ErrUnexpectedAddress)
}
