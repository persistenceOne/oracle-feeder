# price-feeder

The `price-feeder` tool is responsible for performing the following:

1. Fetching and aggregating exchange rate price data from various providers, e.g.
   Binance and Coinbase, based on operator configuration. These exchange rates
   are exposed via an API and are used to feed into the main oracle process.
2. Taking aggregated exchange rate price data and submitting those exchange rates
   on-chain to persistence-sdk `x/oracle`. 

### Steps to run price-feeder locally:
1. `make install` (builds and installs price-feeder binary)
2. run: `price-feeder -h` for more info.
3. Set the price feeder keyring password. It can be done in either of the following ways:
    * Set environment variable for the password `PRICE_FEEDER_KEY_PASS=test`.
    * Enter the password once we run the price-feeder.
4. run: `price-feeder price-feeder.example.toml` to start the price-feeder

