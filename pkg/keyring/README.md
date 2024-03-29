# Cosmos SDK / PersistenceCore Keyring Helper

## Usage

```
NewCosmosKeyring(opts ...ConfigOpt) (sdk.AccAddress, keyring.Keyring, error)
```

**Options are:**

* `WithKeyringDir` option sets keyring path in the filesystem, useful when keyring backend is `file`.
* `WithKeyringAppName` option sets keyring application name (used by Cosmos to separate keyrings)
* `WithKeyringBackend` sets the keyring backend. Expected values: `test`, `file`, `os`.
* `WithKeyFrom` sets the key name to use for signing. Must exist in the provided keyring.
* `WithKeyPassphrase` sets the passphrase for keyring files. The package will fallback to `os.Stdin` if this option was not provided, but passphrase is required.
* `WithPrivKeyHex` allows to specify a private key as plaintext hex. Insecure option, use for testing only. The package will create a virtual keyring holding that key, to meet all the interfaces.
* `WithMnemonic` allows to specify a mnemonic pharse as plaintext hex. Insecure option, use for testing only. The package will create a virtual keyring to derive the keys and meet all the interfaces.
* `WithUseLedger` sets the option to use hardware wallet, if available on the system.

## Testing 

```bash
go test
```

## Generating a Test Fixture

```bash
> cd testdata

> persistenceCore keys --keyring-dir `pwd` --keyring-backend file add test
```

Passphrase should be `test12345678` for this fixture to work.
