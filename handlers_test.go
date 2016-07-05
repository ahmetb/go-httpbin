package httpbin_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ahmetalpbalkan/go-httpbin"
	"github.com/stretchr/testify/require"
)

func testServer() *httptest.Server {
	mux := httpbin.GetMux()
	return httptest.NewServer(mux)
}

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("do not follow redirect")
		}}
}

func get(t *testing.T, url string) []byte {
	return req(t, url, "GET")
}

func post(t *testing.T, url string) []byte {
	return req(t, url, "POST")
}

func req(t *testing.T, url, method string) []byte {
	cl := &http.Client{}

	r, err := http.NewRequest(method, url, nil)
	require.Nil(t, err, "cannot create request")

	resp, err := cl.Do(r)
	require.Nil(t, err, "request failed")

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
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

func TestGet(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	b := get(t, srv.URL+"/get?k1=v1&k1=v2&k3=v3")
	v := struct {
		Args    map[string]interface{} `json:"args"`
		Headers map[string]string      `json:"headers"`
		Origin  string                 `json:"origin"`
	}{}
	require.Nil(t, json.Unmarshal(b, &v))
	require.NotEmpty(t, v.Args, "args empty")
	require.EqualValues(t, map[string]interface{}{
		"k1": []interface{}{"v1", "v2"},
		"k3": "v3",
	}, v.Args)
	require.NotEmpty(t, v.Headers)
	require.NotEmpty(t, v.Origin)
}

func TestRedirect(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	compareLocHeader := func(path, expected string) {
		resp, err := noRedirectClient().Get(srv.URL + path)
		require.IsType(t, err, &url.Error{}, path)
		require.Equal(t, http.StatusFound, resp.StatusCode, path)
		require.Equal(t, expected, resp.Header.Get("Location"), path)
	}

	compareLocHeader("/redirect/0", "/get")
	compareLocHeader("/redirect/1", "/get")
	compareLocHeader("/redirect/2", "/redirect/1")
	compareLocHeader("/redirect/100", "/redirect/99")
}
