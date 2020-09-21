package httpbin_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/ahmetb/go-httpbin"
	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/require"

	"golang.org/x/net/html/charset"
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

func noFollowGet(cl *http.Client, u string) (*http.Response, error) {
	return noFollow("GET", cl, u)
}

func noFollow(method string, cl *http.Client, u string) (*http.Response, error) {
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: method=%s url=%q error=%v", method, u, err)
	}
	resp, err := cl.Do(req)
	if err != nil {
		e, ok := err.(*url.Error)
		if ok && e.Err != errNoFollow {
			return nil, fmt.Errorf("failed to get: method=%s url=%q error=%v", method, u, err)
		}
	}
	return resp, nil
}

func get(t *testing.T, url string) []byte {
	return req(t, url, "GET", nil)
}

func post(t *testing.T, url string, body []byte) []byte {
	return req(t, url, "POST", body)
}

func req(t *testing.T, url, method string, body []byte) []byte {
	cl := &http.Client{}

	r, err := http.NewRequest(method, url, bytes.NewReader(body))
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

func TestHome(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	b := get(t, srv.URL)
	require.Regexp(t, "<!DOCTYPE html>", string(b))
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
func TestPost(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	data := `{[{"k1": "v1"}]}`

	b := post(t, srv.URL+"/post?k1=v1&k1=v2&k3=v3", []byte(data))
	v := struct {
		Args    map[string]interface{} `json:"args"`
		Headers map[string]string      `json:"headers"`
		Origin  string                 `json:"origin"`
		Data    string                 `json:"data"`
		JSON    interface{}            `json:"json"`
	}{}
	require.Nil(t, json.Unmarshal(b, &v))
	require.EqualValues(t, map[string]interface{}{
		"k1": []interface{}{"v1", "v2"},
		"k3": "v3",
	}, v.Args)

	require.EqualValues(t, data, v.Data)
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

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
		http.MethodTrace,
	}

	codes := []int{200, 201, 202, 203, 204, 205, 206, 207, 208, 226,
		300, 301, 302, 303, 304, 305, 307, 308, 400, 401, 402, 403, 404, 405, 406,
		407, 408, 409, 410, 411, 412, 413, 414, 415, 416, 417, 418, 422, 423, 424,
		426, 428, 429, 431, 451, 500, 501, 502, 503, 504, 505, 506, 507, 508, 510, 511}

	for _, method := range methods {
		for _, code := range codes {
			u := fmt.Sprintf("%s/status/%d", srv.URL, code)
			resp, err := noFollow(method, noRedirectClient(), u)
			require.NoErrorf(t, err, "%s %s", method, u)
			require.Equalf(t, code, resp.StatusCode, "invalid status from %s %s", method, u)
		}
	}
}

func TestStatus_invalidCodeWorks(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	code := 777
	u := fmt.Sprintf("%s/status/%d", srv.URL, code)
	resp, err := noFollowGet(noRedirectClient(), u)
	require.Nil(t, err, u)
	require.Equal(t, code, resp.StatusCode, u)
}

func TestStatus_3xxLocationHeader(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	codes := []int{301, 302, 303, 305, 307}

	for _, code := range codes {
		u := fmt.Sprintf("%s/status/%d", srv.URL, code)
		resp, err := noFollowGet(noRedirectClient(), u)
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
	require.EqualValues(t, map[string]string{"k1": "v1", "k2": "v2"}, v.Cookies)
}

func TestSetCookies(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.Nil(t, err)
	cj, err := cookiejar.New(nil)
	require.Nil(t, err)

	cl := noRedirectClient()
	cl.Jar = cj
	resp, err := noFollowGet(cl, srv.URL+"/cookies/set?k1=v1&k2=v2")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, "/cookies", resp.Header.Get("Location"))

	cs := cj.Cookies(u)
	m := make(map[string]string, len(cs))
	for _, v := range cs {
		m[v.Name] = v.Value
	}
	require.EqualValues(t, map[string]string{"k1": "v1", "k2": "v2"}, m)
}

func TestDeleteCookies(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.Nil(t, err)
	cj, err := cookiejar.New(nil)
	require.Nil(t, err)
	cj.SetCookies(u, []*http.Cookie{
		{Name: "k1", Value: "v1"},
		{Name: "k2", Value: "v2"},
		{Name: "k3", Value: "v3"},
	})
	cl := noRedirectClient()
	cl.Jar = cj
	resp, err := noFollowGet(cl, srv.URL+"/cookies/delete?k1&k2")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, "/cookies", resp.Header.Get("Location"))

	var cs []string
	for _, c := range cj.Cookies(u) {
		cs = append(cs, c.String())
	}
	if runtime.Version() >= "go1.8" {
		require.NotContains(t, cs, "k1=")
		require.NotContains(t, cs, "k2=")
		require.NotContains(t, cs, "k1=v1")
		require.NotContains(t, cs, "k2=v2")
		require.Contains(t, cs, "k3=v3")
		require.Equal(t, 1, len(cs))
	} else {
		require.Contains(t, cs, "k1=")
		require.Contains(t, cs, "k2=")
		require.Contains(t, cs, "k3=v3")
		require.Equal(t, 3, len(cs))
	}
}

func TestDrip_code(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/drip?numbytes=10&duration=0.1&code=500")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	b, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, bytes.Repeat([]byte{'*'}, 10), b)
}

func TestCache_ifModifiedSince(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/cache", nil)
	req.Header.Set("If-Modified-Since", "Sat, 29 Oct 1994 19:43:31 GMT")
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotModified, resp.StatusCode)
	require.EqualValues(t, 0, resp.ContentLength)
}

func TestCache_ifNoneMatch(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/cache", nil)
	req.Header.Set("If-None-Match", "some-etag")
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotModified, resp.StatusCode)
	require.EqualValues(t, 0, resp.ContentLength)
}

func TestCache_none(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/cache")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEqual(t, int64(0), resp.ContentLength)
}

func TestSetCache_none(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/cache/5")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "public, max-age=5", resp.Header.Get("Cache-Control"), "Cache-Control header")
	require.NotEqual(t, int64(0), resp.ContentLength)
}

func TestGZIP(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	client := new(http.Client)
	req, err := http.NewRequest("GET", srv.URL+"/gzip", nil)
	require.Nil(t, err)

	req.Header.Add("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()

	require.EqualValues(t, "gzip", resp.Header.Get("Content-Encoding"))
	require.EqualValues(t, "application/json", resp.Header.Get("Content-Type"))
	zr, err := gzip.NewReader(resp.Body)
	require.Nil(t, err)

	var v struct {
		Gzipped bool `json:"gzipped"`
	}
	require.Nil(t, json.NewDecoder(zr).Decode(&v))
	require.True(t, v.Gzipped)
}

func TestDeflate(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/deflate")
	require.Nil(t, err)
	defer resp.Body.Close()

	require.EqualValues(t, "deflate", resp.Header.Get("Content-Encoding"))

	var v struct {
		Deflated bool `json:"deflated"`
	}

	rr := flate.NewReader(resp.Body)
	defer rr.Close()
	require.Nil(t, json.NewDecoder(rr).Decode(&v))
	require.True(t, v.Deflated)
}

func TestBrotli(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	client := new(http.Client)
	req, err := http.NewRequest("GET", srv.URL+"/brotli", nil)
	require.Nil(t, err)

	req.Header.Add("Accept-Encoding", "br")
	resp, err := client.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()

	require.EqualValues(t, "br", resp.Header.Get("Content-Encoding"))
	require.EqualValues(t, "application/json", resp.Header.Get("Content-Type"))
	zr := brotli.NewReader(resp.Body)

	var v struct {
		Compressed bool `json:"compressed"`
	}
	require.Nil(t, json.NewDecoder(zr).Decode(&v))
	require.True(t, v.Compressed)
}

func TestRobotsTXT(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/robots.txt")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.EqualValues(t, "text/plain", resp.Header.Get("Content-Type"))
	b, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.EqualValues(t, "User-agent: *\nDisallow: /deny\n", string(b))
}

func TestDeny(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/deny")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.EqualValues(t, "text/plain", resp.Header.Get("Content-Type"))
}

func TestBasicAuthHandler_noAuth(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/basic-auth/foouser/foopass")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestBasicAuthHandler_badCreds(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/basic-auth/foouser/foopass", nil)
	req.SetBasicAuth("wronguser", "wrongpass")
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestBasicAuthHandler_correctCreds(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/basic-auth/foouser/foopass", nil)
	req.SetBasicAuth("foouser", "foopass")
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	type tt struct {
		Authenticated bool   `json:"authenticated"`
		User          string `json:"user"`
	}
	var v tt
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&v))
	require.Equal(t, tt{Authenticated: true, User: "foouser"}, v)
}

func TestHiddenBasicAuthHandler_noAuth(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hidden-basic-auth/foouser/foopass")
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHiddenBasicAuthHandler_badCreds(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/hidden-basic-auth/foouser/foopass", nil)
	req.SetBasicAuth("wronguser", "wrongpass")
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHiddenBasicAuthHandler_correctCreds(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/hidden-basic-auth/foouser/foopass", nil)
	req.SetBasicAuth("foouser", "foopass")
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	type tt struct {
		Authenticated bool   `json:"authenticated"`
		User          string `json:"user"`
	}
	var v tt
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&v))
	require.Equal(t, tt{Authenticated: true, User: "foouser"}, v)
}

func TestHTML(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/html")
	require.Nil(t, err)
	defer resp.Body.Close()

	doc, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	require.Contains(t, string(doc), "Moby-Dick")
}

func TestXML(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/xml")
	require.Nil(t, err)
	defer resp.Body.Close()

	type slide struct {
		Type  string `xml:"type,attr"`
		Title string `xml:"title"`
	}
	type val struct {
		XMLName xml.Name `xml:"slideshow"`
		Title   string   `xml:"title,attr"`
		Date    string   `xml:"date,attr"`
		Author  string   `xml:"author,attr"`
		Slides  []slide  `xml:"slide"`
	}
	var v val
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	require.Nil(t, decoder.Decode(&v))
	require.Equal(t, val{
		XMLName: xml.Name{Local: "slideshow"},
		Title:   "Sample Slide Show",
		Date:    "Date of publication",
		Author:  "Yours Truly",
		Slides: []slide{
			{Type: "all", Title: "Wake up to WonderWidgets!"},
			{Type: "all", Title: "Overview"},
		}}, v)
}

func TestJPEG(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/image/jpeg")
	require.Nil(t, err)
	defer resp.Body.Close()

	require.EqualValues(t, http.StatusOK, resp.StatusCode)
	require.EqualValues(t, "image/jpeg", resp.Header.Get("Content-Type"))
}

func TestGIF(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/image/gif")
	require.Nil(t, err)
	defer resp.Body.Close()

	require.EqualValues(t, http.StatusOK, resp.StatusCode)
	require.EqualValues(t, "image/gif", resp.Header.Get("Content-Type"))
}

func TestPNG(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/image/png")
	require.Nil(t, err)
	defer resp.Body.Close()

	require.EqualValues(t, http.StatusOK, resp.StatusCode)
	require.EqualValues(t, "image/png", resp.Header.Get("Content-Type"))
}
