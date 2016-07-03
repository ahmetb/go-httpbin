package httpbin_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ahmetalpbalkan/go-httpbin"
	"github.com/stretchr/testify/require"
)

func testServer() *httptest.Server {
	mux := httpbin.GetMux()
	return httptest.NewServer(mux)
}

func get(t *testing.T, url string) []byte {
	r, err := http.Get(url)
	require.Nil(t, err, "request failed")
	defer r.Body.Close()
	require.Equal(t, http.StatusOK, r.StatusCode)

	b, err := ioutil.ReadAll(r.Body)
	require.Nil(t, err, "failed to read response body")
	return b
}

func TestIP(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	b := get(t, srv.URL+"/ip")
	v := struct {
		Origin string `json:"origin"`
	}{}
	require.Nil(t, json.Unmarshal(b, &v))
	require.Equal(t, "127.0.0.1", v.Origin)
}

func TestUserAgent(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	b := get(t, srv.URL+"/user-agent")
	v := struct {
		UA string `json:"user-agent"`
	}{}
	require.Nil(t, json.Unmarshal(b, &v))
	require.NotEmpty(t, v.UA)
}

func TestHeaders(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	b := get(t, srv.URL+"/headers")
	v := struct {
		Headers map[string]string `json:"headers"`
	}{}
	require.Nil(t, json.Unmarshal(b, &v))
	require.NotEmpty(t, v.Headers["User-Agent"]) // provided by default Go HTTP client
}
