package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/persistenceOne/oracle-feeder/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

//nolint:funlen //No need to split this function
func TestMockProvider_GetTickerPrices(t *testing.T) {
	mp := NewMockProvider()

	t.Run("valid_request_single_ticker", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.Equal(t, "/", req.URL.String())
			resp := `Base,Quote,Price,Volume
OSMO,USD,3.04,1827884.77
ATOM,USD,21.84,1827884.77
`
			_, err := rw.Write([]byte(resp))
			require.NoError(t, err)
		}))
		defer server.Close()

		mp.client = server.Client()
		mp.baseURL = server.URL

		prices, err := mp.GetTickerPrices(types.CurrencyPair{Base: "OSMO", Quote: "USD"})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		require.Equal(t, sdk.MustNewDecFromStr("3.04"), prices["OSMOUSD"].Price)
		require.Equal(t, sdk.MustNewDecFromStr("1827884.77"), prices["OSMOUSD"].Volume)
	})

	t.Run("valid_request_multi_ticker", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.Equal(t, "/", req.URL.String())
			resp := `Base,Quote,Price,Volume
OSMO,USD,3.04,1827884.77
ATOM,USD,21.84,1827884.77
`
			_, err := rw.Write([]byte(resp))
			require.NoError(t, err)
		}))
		defer server.Close()

		mp.client = server.Client()
		mp.baseURL = server.URL

		prices, err := mp.GetTickerPrices(
			types.CurrencyPair{Base: "OSMO", Quote: "USD"},
			types.CurrencyPair{Base: "ATOM", Quote: "USD"},
		)
		require.NoError(t, err)
		require.Len(t, prices, 2)
		require.Equal(t, sdk.MustNewDecFromStr("3.04"), prices["OSMOUSD"].Price)
		require.Equal(t, sdk.MustNewDecFromStr("1827884.77"), prices["OSMOUSD"].Volume)
		require.Equal(t, sdk.MustNewDecFromStr("21.84"), prices["ATOMUSD"].Price)
		require.Equal(t, sdk.MustNewDecFromStr("1827884.77"), prices["ATOMUSD"].Volume)
	})

	t.Run("invalid_request_bad_response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.Equal(t, "/", req.URL.String())
			_, err := rw.Write([]byte(`FOO`))
			require.NoError(t, err)
		}))
		defer server.Close()

		mp.client = server.Client()
		mp.baseURL = server.URL

		prices, err := mp.GetTickerPrices(types.CurrencyPair{Base: "OSMO", Quote: "USD"})
		require.Error(t, err)
		require.Nil(t, prices)
	})
}
