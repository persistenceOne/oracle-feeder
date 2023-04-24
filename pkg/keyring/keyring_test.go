package keyring

import (
	"encoding/hex"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cosmcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cosmkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/persistenceOne/persistence-sdk/v2/simapp"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	Bech32MainPrefix = "persistence"
	CoinType         = 118
)

type KeyringTestSuite struct {
	suite.Suite

	cdc codec.Codec
}

func TestKeyringTestSuite(t *testing.T) {
	suite.Run(t, new(KeyringTestSuite))
}

func (s *KeyringTestSuite) SetupTest() {
	setCoinTypeAndPrefix(uint32(CoinType), Bech32MainPrefix)
	s.cdc = simapp.MakeTestEncodingConfig().Marshaler
}

func (s *KeyringTestSuite) TestKeyFromPrivkey() {
	requireT := s.Require()

	accAddr, kb, err := NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex(testPrivKeyHex),
		WithKeyFrom(testAccAddressBech), // must match the privkey above
	)
	requireT.NoError(err)
	requireT.Equal(testAccAddressBech, accAddr.String())

	record, err := kb.KeyByAddress(accAddr)
	requireT.NoError(err)
	requireT.Equal(cosmkeyring.TypeLocal, record.GetType())
	requireT.Equal(expectedPubKeyType, record.PubKey.TypeUrl)
	recordPubKey, err := record.GetPubKey()
	requireT.NoError(err)

	logPrivKey(s.T(), kb, accAddr)

	res, pubkey, err := kb.SignByAddress(accAddr, []byte("test"))
	requireT.NoError(err)
	requireT.EqualValues(recordPubKey, pubkey)
	requireT.Equal(testSig, res)
}

func (s *KeyringTestSuite) TestKeyFromMnemonic() {
	requireT := s.Require()

	accAddr, kb, err := NewCosmosKeyring(
		s.cdc,
		WithMnemonic(testMnemonic),
		WithPrivKeyHex(testPrivKeyHex),  // must match mnemonic above
		WithKeyFrom(testAccAddressBech), // must match mnemonic above
	)
	requireT.NoError(err)
	requireT.Equal(testAccAddressBech, accAddr.String())

	record, err := kb.KeyByAddress(accAddr)
	requireT.NoError(err)
	requireT.Equal(cosmkeyring.TypeLocal, record.GetType())
	requireT.Equal(expectedPubKeyType, record.PubKey.TypeUrl)
	recordPubKey, err := record.GetPubKey()
	requireT.NoError(err)

	logPrivKey(s.T(), kb, accAddr)

	res, pubkey, err := kb.SignByAddress(accAddr, []byte("test"))
	requireT.NoError(err)
	requireT.Equal(recordPubKey, pubkey)
	requireT.Equal(testSig, res)
}

func (s *KeyringTestSuite) TestKeyringFile() {
	requireT := s.Require()

	accAddr, _, err := NewCosmosKeyring(
		s.cdc,
		WithKeyringBackend(BackendFile),
		WithKeyringDir("./testdata"),
		WithKeyFrom("test"),
		WithKeyPassphrase("test12345678"),
	)
	requireT.NoError(err)
	requireT.Equal(testAccAddressBech, accAddr.String())

	accAddr, kb, err := NewCosmosKeyring(
		s.cdc,
		WithKeyringBackend(BackendFile),
		WithKeyringDir("./testdata"),
		WithKeyFrom(testAccAddressBech),
		WithKeyPassphrase("test12345678"),
	)
	requireT.NoError(err)
	requireT.Equal(testAccAddressBech, accAddr.String())

	record, err := kb.KeyByAddress(accAddr)
	requireT.NoError(err)
	requireT.Equal(cosmkeyring.TypeLocal, record.GetType())
	requireT.Equal(expectedPubKeyType, record.PubKey.TypeUrl)
	requireT.Equal("test", record.Name)
	recordPubKey, err := record.GetPubKey()
	requireT.NoError(err)

	logPrivKey(s.T(), kb, accAddr)

	res, pubkey, err := kb.SignByAddress(accAddr, []byte("test"))
	requireT.NoError(err)
	requireT.Equal(recordPubKey, pubkey)
	requireT.Equal(testSig, res)
}

func (s *KeyringTestSuite) TestKeyringOsWithAppName() {
	if testing.Short() {
		s.T().Skip("skipping testing in short mode")
		return
	}

	requireT := require.New(s.T())

	osKeyring, err := cosmkeyring.New("keyring_test", cosmkeyring.BackendOS, "", nil, s.cdc)
	requireT.NoError(err)

	var accRecord *cosmkeyring.Record
	if accRecord, err = osKeyring.Key("test"); err != nil {
		accRecord, err = osKeyring.NewAccount(
			"test",
			testMnemonic,
			cosmkeyring.DefaultBIP39Passphrase,
			sdk.GetConfig().GetFullBIP44Path(),
			hd.Secp256k1,
		)

		requireT.NoError(err)

		accAddr, err := accRecord.GetAddress()
		requireT.NoError(err)
		requireT.Equal(testAccAddressBech, accAddr.String())
	}

	s.T().Cleanup(func() {
		// cleanup
		addr, err := accRecord.GetAddress()
		if err == nil {
			_ = osKeyring.DeleteByAddress(addr)
		}
	})

	accAddr, kb, err := NewCosmosKeyring(
		s.cdc,
		WithKeyringBackend(BackendOS),
		WithKeyFrom("test"),
		WithKeyringAppName("keyring_test"),
	)
	requireT.NoError(err)
	requireT.Equal(testAccAddressBech, accAddr.String())

	record, err := kb.KeyByAddress(accAddr)
	requireT.NoError(err)
	requireT.Equal(cosmkeyring.TypeLocal, record.GetType())
	requireT.Equal(expectedPubKeyType, record.PubKey.TypeUrl)
	recordPubKey, err := record.GetPubKey()
	requireT.NoError(err)

	requireT.Equal("test", record.Name)

	res, pubkey, err := kb.SignByAddress(accAddr, []byte("test"))
	requireT.NoError(err)
	requireT.Equal(recordPubKey, pubkey)
	requireT.Equal(testSig, res)
}

func (s *KeyringTestSuite) TestUseFromAsName() {
	requireT := s.Require()

	_, _, err := NewCosmosKeyring(
		s.cdc,
		WithPrivKeyHex(testPrivKeyHex),
		WithKeyFrom("kowabunga"),
	)

	requireT.NoError(err)

	_, _, err = NewCosmosKeyring(
		s.cdc,
		WithMnemonic(testMnemonic),
		WithKeyFrom("kowabunga"),
	)

	requireT.NoError(err)
}

const expectedPubKeyType = "/cosmos.crypto.secp256k1.PubKey"

const testAccAddressBech = "persistence1t6dq82wyggtmu2cvegyat9et7uans46n9vfmj2"

//nolint:lll // mnemonic fixture
const testMnemonic = `toddler gossip soap crop property true off record horn route enable raise produce wheat mango social output ritual pond powder test biology address romance`

var testPrivKeyHex = "2bcc4aa9d2374a80169fa7568ea221133a96288bd13a499abfa110dd0f0c55bd"

var testSig = []byte{
	0xcf, 0xbe, 0x24, 0xb7, 0xd9, 0xbb, 0xaf, 0x60,
	0x62, 0x2e, 0xc0, 0x0a, 0x1f, 0x13, 0x85, 0x34,
	0x4e, 0xce, 0x52, 0x84, 0x4b, 0xbe, 0x88, 0x98,
	0x45, 0x6c, 0xaa, 0x57, 0x8d, 0x13, 0x3e, 0x2f,
	0x72, 0x50, 0xa9, 0x43, 0x4b, 0x9c, 0x18, 0xaa,
	0x18, 0x85, 0x67, 0xeb, 0x9c, 0x7a, 0x5f, 0x43,
	0x55, 0x93, 0x3c, 0xba, 0xd6, 0x0d, 0x22, 0x0f,
	0xe1, 0xb0, 0x24, 0x13, 0x4d, 0x98, 0xe2, 0x04,
}

func logPrivKey(t *testing.T, kb cosmkeyring.Keyring, accAddr sdk.AccAddress) {
	armor, _ := kb.ExportPrivKeyArmorByAddress(accAddr, "")
	privKey, _, _ := cosmcrypto.UnarmorDecryptPrivKey(armor, "")
	t.Log("[PRIV]", hex.EncodeToString(privKey.Bytes()))
}

// setCoinTypeAndPrefix sets the chain coin type and account bech32 prefixes in global config for the current process.
func setCoinTypeAndPrefix(coinType uint32, accountAddressPrefix string) {
	var (
		Bech32PrefixAccAddr  = accountAddressPrefix
		Bech32PrefixAccPub   = accountAddressPrefix + sdk.PrefixPublic
		Bech32PrefixValAddr  = accountAddressPrefix + sdk.PrefixValidator + sdk.PrefixOperator
		Bech32PrefixValPub   = accountAddressPrefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic
		Bech32PrefixConsAddr = accountAddressPrefix + sdk.PrefixValidator + sdk.PrefixConsensus
		Bech32PrefixConsPub  = accountAddressPrefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic
	)

	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
	config.SetCoinType(coinType)
	config.SetPurpose(44)
}
