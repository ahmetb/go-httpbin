package httpbin_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ahmetalpbalkan/go-httpbin"
	"github.com/stretchr/testify/require"
)

var (
	errNoFollow = errors.New("do not follow redirect")
)

func testServer() *httptest.Server {
	mux := httpbin.GetMux()
	return httptest.NewServer(mux)
}

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errNoFollow
		}}
}

func noFollowGet(u string) (*http.Response, error) {
	resp, err := noRedirectClient().Get(u)
	if err != nil {
		e, ok := err.(*url.Error)
		if ok && e.Err != errNoFollow {
			return nil, fmt.Errorf("failed to get: url=%q error=%v", u, err)
		}
	}
	return resp, nil
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

func assertLocationHeader(t *testing.T, u, expected string) {
	resp, err := noRedirectClient().Get(u)
	require.IsType(t, &url.Error{}, err, u)
	require.Equal(t, err.(*url.Error).Err, errNoFollow, u)
	require.Equal(t, http.StatusFound, resp.StatusCode, u)
	require.Equal(t, expected, resp.Header.Get("Location"), u)
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

	assertLocationHeader(t, srv.URL+"/redirect/0", "/get")
	assertLocationHeader(t, srv.URL+"/redirect/1", "/get")
	assertLocationHeader(t, srv.URL+"/redirect/2", "/redirect/1")
	assertLocationHeader(t, srv.URL+"/redirect/100", "/redirect/99")
}

func TestAbsoluteRedirect(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	assertLocationHeader(t, srv.URL+"/absolute-redirect/0", srv.URL+"/get")
	assertLocationHeader(t, srv.URL+"/absolute-redirect/1", srv.URL+"/get")
	assertLocationHeader(t, srv.URL+"/absolute-redirect/2", srv.URL+"/absolute-redirect/1")
	assertLocationHeader(t, srv.URL+"/absolute-redirect/100", srv.URL+"/absolute-redirect/99")
}

func TestRedirectTo(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	assertLocationHeader(t, srv.URL+"/redirect-to?url=ip", "ip")
	assertLocationHeader(t, srv.URL+"/redirect-to?url=/ip", "/ip")
	assertLocationHeader(t, srv.URL+"/redirect-to?url=http%3A%2F%2Fexample.com%2F", "http://example.com/")
}

func TestStatus_assertValidCodes(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	codes := []int{200, 201, 202, 203, 204, 205, 206, 207, 208, 226,
		300, 301, 302, 303, 304, 305, 307, 308, 400, 401, 402, 403, 404, 405, 406,
		407, 408, 409, 410, 411, 412, 413, 414, 415, 416, 417, 418, 422, 423, 424,
		426, 428, 429, 431, 451, 500, 501, 502, 503, 504, 505, 506, 507, 508, 510, 511}

	for _, code := range codes {
		u := fmt.Sprintf("%s/status/%d", srv.URL, code)
		resp, err := noFollowGet(u)
		require.Nil(t, err, u)
		require.Equal(t, code, resp.StatusCode, u)
	}
}

func TestStatus_invalidCodeWorks(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	code := 777
	u := fmt.Sprintf("%s/status/%d", srv.URL, code)
	resp, err := noFollowGet(u)
	require.Nil(t, err, u)
	require.Equal(t, code, resp.StatusCode, u)
}

func TestStatus_3xxLocationHeader(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	codes := []int{301, 302, 303, 305, 307}

	for _, code := range codes {
		u := fmt.Sprintf("%s/status/%d", srv.URL, code)
		resp, err := noFollowGet(u)
		require.Nil(t, err, u)
		require.NotEmpty(t, resp.Header.Get("Location"), "code=%d", code)
	}
}

func TestBytes_size(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	sizes := []int{
		0, // empty
		1, // 1 byte
		httpbin.BinaryChunkSize - 1, // off by one case
		httpbin.BinaryChunkSize,     // off by one case
		httpbin.BinaryChunkSize + 1, // off by one case
		1 * 1024 * 1024,             // 1 MB
		100 * 1024 * 1024,           // 100 MB
	}
	for _, size := range sizes {
		b := get(t, srv.URL+fmt.Sprintf("/bytes/%d", size))
		require.Equal(t, size, len(b), "wrong Content-Length for %d", size)
	}
}

func TestBytes_noSeed(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	u := srv.URL + "/bytes/1024"
	b1 := get(t, u)
	b2 := get(t, u)
	require.NotEqual(t, b1, b2, "generated the same bytes in multiple runs")
}

func TestBytes_seed(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	u := srv.URL + "/bytes/1024?seed=1"
	b1 := get(t, u)
	b2 := get(t, u)
	require.Equal(t, b1, b2, "generated different bytes for the same seed")
}

func TestDelay_supportsFloat(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	n := 0.5
	s := time.Now()
	_ = get(t, srv.URL+fmt.Sprintf("/delay/%v", n))
	e := time.Since(s).Seconds()
	require.InEpsilon(t, e, n, 0.2, "delay=%v elapsed=%vs", n, e)
}

func TestDelay_limited(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	orig := httpbin.DelayMax
	defer func() { httpbin.DelayMax = orig }()

	httpbin.DelayMax = 300 * time.Millisecond

	s := time.Now()
	_ = get(t, srv.URL+"/delay/20")
	e := time.Since(s).Seconds()
	require.InEpsilon(t, e, 0.3, 0.1, "max=%v elapsed=%vs", httpbin.DelayMax, e)
}

func TestStream(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	orig := httpbin.StreamInterval
	new := time.Millisecond * 100
	httpbin.StreamInterval = new
	defer func() { httpbin.StreamInterval = orig }()

	total := 10
	resp, err := http.Get(srv.URL + fmt.Sprintf("/stream/%d", total))
	require.Nil(t, err, "request failed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	dec := json.NewDecoder(resp.Body)

	var lastMsg time.Time
	var m struct {
		N    int       `json:"n"`
		Time time.Time `json:"time"`
	}
	n := 0
	for {
		err := dec.Decode(&m)
		if err == io.EOF {
			break
		}
		require.Nil(t, err, "cannot decode msg")
		t.Logf("msg {n=%v, time=%v} -- recvd at %v", m.N, m.Time, time.Now().UTC())
		if lastMsg.IsZero() {
			lastMsg = time.Now()
		} else {
			elapsedMs := time.Since(lastMsg).Seconds() * 1000
			require.InDelta(t, int(new/time.Millisecond), int(elapsedMs), 20, "time since last msg=%dms", elapsedMs)
			lastMsg = time.Now()
		}
		n++
	}
	require.Equal(t, total, n, "some messages not received")
}

func TestCookies(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.Nil(t, err)
	cj, err := cookiejar.New(nil)
	require.Nil(t, err)
	cj.SetCookies(u, []*http.Cookie{
		{Name: "k1", Value: "v1"},
		{Name: "k2", Value: "v2"},
	})
	cl := &http.Client{Jar: cj}
	resp, err := cl.Get(srv.URL + "/cookies")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	var v struct {
		Cookies map[string]string `json:"cookies"`
	}
	require.Nil(t, json.Unmarshal(b, &v))
	require.EqualValues(t, v.Cookies, map[string]string{"k1": "v1", "k2": "v2"})
}
