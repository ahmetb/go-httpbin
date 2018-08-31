// Package httpbin providers HTTP handlers for httpbin.org endpoints and a
// multiplexer to directly hook it up to any http.Server or httptest.Server.
package httpbin

import (
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

var (
	// BinaryChunkSize is buffer length used for stuff like generating
	// large blobs.
	BinaryChunkSize = 64 * 1024

	// DelayMax is the maximum execution time for /delay endpoint.
	DelayMax = 10 * time.Second

	// StreamInterval is the default interval between writing objects to the stream.
	StreamInterval = 1 * time.Second
)

// GetMux returns the mux with handlers for httpbin endpoints registered.
func GetMux() *mux.Router {

	r := mux.NewRouter()
	r.HandleFunc(`/`, HomeHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/ip`, IPHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/user-agent`, UserAgentHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/headers`, HeadersHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/get`, GetHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/post`, PostHandler).Methods(http.MethodPost)
	r.HandleFunc(`/redirect/{n:[\d]+}`, RedirectHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/absolute-redirect/{n:[\d]+}`, AbsoluteRedirectHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/redirect-to`, RedirectToHandler).Methods(http.MethodGet, http.MethodHead).Queries("url", "{url:.+}")
	r.HandleFunc(`/status/{code:[\d]+}`, StatusHandler)
	r.HandleFunc(`/bytes/{n:[\d]+}`, BytesHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/delay/{n:\d+(?:\.\d+)?}`, DelayHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/stream/{n:[\d]+}`, StreamHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/drip`, DripHandler).Methods(http.MethodGet, http.MethodHead).Queries(
		"numbytes", `{numbytes:\d+}`,
		"duration", `{duration:\d+(?:\.\d+)?}`)
	r.HandleFunc(`/cookies`, CookiesHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/cookies/set`, SetCookiesHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/cookies/delete`, DeleteCookiesHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/cache`, CacheHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/cache/{n:[\d]+}`, SetCacheHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/gzip`, GZIPHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/brotli`, BrotliHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/deflate`, DeflateHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/html`, HTMLHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/xml`, XMLHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/robots.txt`, RobotsTXTHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/deny`, DenyHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/basic-auth/{u}/{p}`, BasicAuthHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/hidden-basic-auth/{u}/{p}`, HiddenBasicAuthHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/image/gif`, GIFHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/image/png`, PNGHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(`/image/jpeg`, JPEGHandler).Methods(http.MethodGet, http.MethodHead)
	return r
}

// HomeHandler serves static HTML content for the index page.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<!DOCTYPE html>
	<html lang="en">
	<head>
		<title>go-httpbin</title>
	<head>
	<body>
		<h1>go-httpbin</h1>
		<p>
			<a href="https://github.com/ahmetb/go-httpbin">
				Read documentation &rarr;
			</a>
	</body>
	</html>`)
}

// IPHandler returns Origin IP.
func IPHandler(w http.ResponseWriter, r *http.Request) {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	if err := writeJSON(w, ipResponse{h}); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json")) // TODO handle this error in writeJSON(w,v)
	}
}

// UserAgentHandler returns user agent.
func UserAgentHandler(w http.ResponseWriter, r *http.Request) {
	if err := writeJSON(w, userAgentResponse{r.UserAgent()}); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// HeadersHandler returns user agent.
func HeadersHandler(w http.ResponseWriter, r *http.Request) {
	if err := writeJSON(w, headersResponse{getHeaders(r)}); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// GetHandler returns user agent.
func GetHandler(w http.ResponseWriter, r *http.Request) {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)

	v := getResponse{
		headersResponse: headersResponse{getHeaders(r)},
		ipResponse:      ipResponse{h},
		Args:            flattenValues(r.URL.Query()),
	}

	if err := writeJSON(w, v); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// PostHandler accept a post and echo its data back
func PostHandler(w http.ResponseWriter, r *http.Request) {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)

	data, err := parseData(r)
	if err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to read body"))
		return
	}

	var jsonPayload interface{}
	if strings.Contains(r.Header.Get("Content-Type"), "json") {
		err := json.Unmarshal(data, &jsonPayload)
		if err != nil {
			writeErrorJSON(w, errors.Wrap(err, "failed to read body"))
			return
		}
	}

	v := postResponse{
		headersResponse: headersResponse{getHeaders(r)},
		ipResponse:      ipResponse{h},
		Args:            flattenValues(r.URL.Query()),
		Data:            string(data),
		JSON:            jsonPayload,
	}

	if err := writeJSON(w, v); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// RedirectHandler returns a 302 Found response if n=1 pointing
// to /get, otherwise to /redirect/(n-1)
func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	n := mux.Vars(r)["n"]
	i, _ := strconv.Atoi(n) // shouldn't fail due to route pattern

	var loc string
	if i <= 1 {
		loc = "/get"
	} else {
		loc = fmt.Sprintf("/redirect/%d", i-1)
	}
	w.Header().Set("Location", loc)
	w.WriteHeader(http.StatusFound)
}

// AbsoluteRedirectHandler returns a 302 Found response if n=1 pointing
// to /host/get, otherwise to /host/absolute-redirect/(n-1)
func AbsoluteRedirectHandler(w http.ResponseWriter, r *http.Request) {
	n := mux.Vars(r)["n"]
	i, _ := strconv.Atoi(n) // shouldn't fail due to route pattern

	var loc string
	if i <= 1 {
		loc = "/get"
	} else {
		loc = fmt.Sprintf("/absolute-redirect/%d", i-1)
	}

	w.Header().Set("Location", "http://"+r.Host+loc)
	w.WriteHeader(http.StatusFound)
}

// RedirectToHandler returns a 302 Found response pointing to
// the url query parameter
func RedirectToHandler(w http.ResponseWriter, r *http.Request) {
	u := mux.Vars(r)["url"]
	w.Header().Set("Location", u)
	w.WriteHeader(http.StatusFound)
}

// StatusHandler returns a proper response for provided status code
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	code, _ := strconv.Atoi(mux.Vars(r)["code"])

	statusWritten := false
	switch code {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusUseProxy,
		http.StatusTemporaryRedirect:
		w.Header().Set("Location", "/redirect/1")
	case http.StatusUnauthorized: // 401
		w.Header().Set("WWW-Authenticate", `Basic realm="Fake Realm"`)
	case http.StatusPaymentRequired: // 402
		w.WriteHeader(code)
		statusWritten = true
		io.WriteString(w, "Fuck you, pay me!")
		w.Header().Set("x-more-info", "http://vimeo.com/22053820")
	case http.StatusNotAcceptable: // 406
		w.WriteHeader(code)
		statusWritten = true
		io.WriteString(w, `{"message": "Client did not request a supported media type.", "accept": ["image/webp", "image/svg+xml", "image/jpeg", "image/png", "image/*"]}`)
	case http.StatusTeapot:
		w.WriteHeader(code)
		statusWritten = true
		w.Header().Set("x-more-info", "http://tools.ietf.org/html/rfc2324")
		io.WriteString(w, `
    -=[ teapot ]=-

       _...._
     .'  _ _ '.
    | ."  ^  ". _,
    \_;'"---"'|//
      |       ;/
      \_     _/
        '"""'
`)
	}
	if !statusWritten {
		w.WriteHeader(code)
	}
}

// BytesHandler returns n random bytes of binary data and accepts an
// optional 'seed' integer query parameter.
func BytesHandler(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(mux.Vars(r)["n"]) // shouldn't fail due to route pattern

	seedStr := r.URL.Query().Get("seed")
	if seedStr == "" {
		seedStr = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	seed, _ := strconv.ParseInt(seedStr, 10, 64) // shouldn't fail due to route pattern
	rnd := rand.New(rand.NewSource(seed))
	buf := make([]byte, BinaryChunkSize)
	for n > 0 {
		rnd.Read(buf) // will never return err
		if n >= len(buf) {
			n -= len(buf)
			w.Write(buf)
		} else {
			// last chunk
			w.Write(buf[:n])
			break
		}
	}
}

// DelayHandler delays responding for min(n, 10) seconds and responds
// with /get endpoint
func DelayHandler(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.ParseFloat(mux.Vars(r)["n"], 64) // shouldn't fail due to route pattern

	// allow only millisecond precision
	duration := time.Millisecond * time.Duration(n*float64(time.Second/time.Millisecond))
	if duration > DelayMax {
		duration = DelayMax
	}
	time.Sleep(duration)
	GetHandler(w, r)
}

// StreamHandler writes a json object to a new line every second.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(mux.Vars(r)["n"]) // shouldn't fail due to route pattern
	nl := []byte{'\n'}
	// allow only millisecond precision
	for i := 0; i < n; i++ {
		time.Sleep(StreamInterval)
		b, _ := json.Marshal(struct {
			N    int       `json:"n"`
			Time time.Time `json:"time"`
		}{i, time.Now().UTC()})
		w.Write(b)
		w.Write(nl)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// CookiesHandler returns the cookies provided in the request.
func CookiesHandler(w http.ResponseWriter, r *http.Request) {
	if err := writeJSON(w, cookiesResponse{getCookies(r.Cookies())}); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// SetCookiesHandler sets the query key/value pairs as cookies
// in the response and returns a 302 redirect to /cookies.
func SetCookiesHandler(w http.ResponseWriter, r *http.Request) {
	for k := range r.URL.Query() {
		v := r.URL.Query().Get(k)
		http.SetCookie(w, &http.Cookie{
			Name:  k,
			Value: v,
			Path:  "/",
		})
	}
	w.Header().Set("Location", "/cookies")
	w.WriteHeader(http.StatusFound)
}

// DeleteCookiesHandler deletes cookies with provided query value keys
// in the response by settings a Unix epoch expiration date and returns
// a 302 redirect to /cookies.
func DeleteCookiesHandler(w http.ResponseWriter, r *http.Request) {
	for k := range r.URL.Query() {
		http.SetCookie(w, &http.Cookie{
			Name:    k,
			Value:   "",
			Path:    "/",
			Expires: time.Unix(0, 0),
			MaxAge:  0,
		})
	}
	w.Header().Set("Location", "/cookies")
	w.WriteHeader(http.StatusFound)
}

// DripHandler drips data over a duration after an optional initial delay,
// then optionally returns with the given status code.
func DripHandler(w http.ResponseWriter, r *http.Request) {
	var retCode int

	retCodeStr := r.URL.Query().Get("code")
	delayStr := r.URL.Query().Get("delay")
	durationSec, _ := strconv.ParseFloat(mux.Vars(r)["duration"], 32) // shouldn't fail due to route pattern
	numBytes, _ := strconv.Atoi(mux.Vars(r)["numbytes"])              // shouldn't fail due to route pattern

	if retCodeStr != "" { // optional: status code
		var err error
		retCode, err = strconv.Atoi(r.URL.Query().Get("code"))
		if err != nil {
			writeErrorJSON(w, errors.New("failed to parse 'code'"))
			return
		}
		w.WriteHeader(retCode)
	}

	if delayStr != "" { // optional: initial delay
		delaySec, err := strconv.ParseFloat(r.URL.Query().Get("delay"), 64)
		if err != nil {
			writeErrorJSON(w, errors.New("failed to parse 'delay'"))
			return
		}
		delayMs := (time.Second / time.Millisecond) * time.Duration(delaySec)
		time.Sleep(delayMs * time.Millisecond)
	}

	t := time.Second * time.Duration(durationSec) / time.Duration(numBytes)
	for i := 0; i < numBytes; i++ {
		w.Write([]byte{'*'})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(t)
	}
}

// CacheHandler returns 200 with the response of /get unless an If-Modified-Since
//or If-None-Match header is provided, when it returns a 304.
func CacheHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("If-Modified-Since") != "" || r.Header.Get("If-None-Match") != "" {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	GetHandler(w, r)
}

// SetCacheHandler sets a Cache-Control header for n seconds and returns with
// the /get response.
func SetCacheHandler(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(mux.Vars(r)["n"]) // shouldn't fail due to route pattern
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", n))
	GetHandler(w, r)
}

// GZIPHandler returns a GZIP-encoded response
func GZIPHandler(w http.ResponseWriter, r *http.Request) {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)

	v := gzipResponse{
		headersResponse: headersResponse{getHeaders(r)},
		ipResponse:      ipResponse{h},
		Gzipped:         true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Content-Encoding", "gzip")
	ww := gzip.NewWriter(w)
	defer ww.Close() // flush
	if err := writeJSON(ww, v); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// DeflateHandler returns a DEFLATE-encoded response.
func DeflateHandler(w http.ResponseWriter, r *http.Request) {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)

	v := deflateResponse{
		headersResponse: headersResponse{getHeaders(r)},
		ipResponse:      ipResponse{h},
		Deflated:        true,
	}

	w.Header().Set("Content-Encoding", "deflate")
	ww, _ := flate.NewWriter(w, flate.BestCompression)
	defer ww.Close() // flush
	if err := writeJSON(ww, v); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// BrotliHandler returns a Brotli-encoded response
func BrotliHandler(w http.ResponseWriter, r *http.Request) {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)

	v := brotliResponse{
		headersResponse: headersResponse{getHeaders(r)},
		ipResponse:      ipResponse{h},
		Compressed:      true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Content-Encoding", "br")
	ww := brotli.NewWriter(w)
	defer ww.Close() // flush
	if err := writeJSON(ww, v); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}

// RobotsTXTHandler returns a robots.txt response.
func RobotsTXTHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "User-agent: *\nDisallow: /deny\n")
}

// DenyHandler returns a plain-text response.
func DenyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, `
          .-''''''-.
        .' _      _ '.
       /   O      O   \
      :                :
      |                |
      :       __       :
       \  .-"'  '"-.  /
        '.          .'
          '-......-'
     YOU SHOULDN'T BE HERE
`)
}

// BasicAuthHandler challenges with given username and password.
func BasicAuthHandler(w http.ResponseWriter, r *http.Request) {
	basicAuthHandler(w, r, http.StatusUnauthorized)
}

// HiddenBasicAuthHandler challenges with given username and password and
// returns 404 if authentication fails.
func HiddenBasicAuthHandler(w http.ResponseWriter, r *http.Request) {
	basicAuthHandler(w, r, http.StatusNotFound)
}

func basicAuthHandler(w http.ResponseWriter, r *http.Request, status int) {
	user := mux.Vars(r)["u"]
	pass := mux.Vars(r)["p"]

	inUser, inPass, ok := r.BasicAuth()
	if !ok || inUser != user || inPass != pass {
		w.WriteHeader(status)
	} else {
		v := basicAuthResponse{
			Authenticated: true,
			User:          user,
		}
		if err := writeJSON(w, v); err != nil {
			writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
		}
	}
}

// HTMLHandler returns some HTML response.
func HTMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, htmlData)
}

// XMLHandler returns some XML response.
func XMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	fmt.Fprint(w, xmlData)
}

type circle struct {
	X, Y, R float64
}

func (c *circle) Brightness(x, y float64) uint8 {
	var dx, dy float64 = c.X - x, c.Y - y
	d := math.Sqrt(dx*dx+dy*dy) / c.R
	if d > 1 {
		return 0
	}
	return 255
}

// GIFHandler returns an animated GIF image.
// Source: http://tech.nitoyon.com/en/blog/2016/01/07/go-animated-gif-gen/
func GIFHandler(rw http.ResponseWriter, r *http.Request) {
	var w, h int = 240, 240
	var hw, hh float64 = float64(w / 2), float64(h / 2)
	circles := []*circle{{}, {}, {}}

	var palette = []color.Color{
		color.RGBA{0x00, 0x00, 0x00, 0xff},
		color.RGBA{0x00, 0x00, 0xff, 0xff},
		color.RGBA{0x00, 0xff, 0x00, 0xff},
		color.RGBA{0x00, 0xff, 0xff, 0xff},
		color.RGBA{0xff, 0x00, 0x00, 0xff},
		color.RGBA{0xff, 0x00, 0xff, 0xff},
		color.RGBA{0xff, 0xff, 0x00, 0xff},
		color.RGBA{0xff, 0xff, 0xff, 0xff},
	}

	var images []*image.Paletted
	var delays []int
	steps := 20
	for step := 0; step < steps; step++ {
		img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
		images = append(images, img)
		delays = append(delays, 0)

		θ := 2.0 * math.Pi / float64(steps) * float64(step)
		for i, circle := range circles {
			θ0 := 2 * math.Pi / 3 * float64(i)
			circle.X = hw - 40*math.Sin(θ0) - 20*math.Sin(θ0+θ)
			circle.Y = hh - 40*math.Cos(θ0) - 20*math.Cos(θ0+θ)
			circle.R = 50
		}

		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				img.Set(x, y, color.RGBA{
					circles[0].Brightness(float64(x), float64(y)),
					circles[1].Brightness(float64(x), float64(y)),
					circles[2].Brightness(float64(x), float64(y)),
					255,
				})
			}
		}
	}

	gif.EncodeAll(rw, &gif.GIF{
		Image: images,
		Delay: delays,
	})
}

// JPEGHandler returns a JPEG image.
func JPEGHandler(w http.ResponseWriter, r *http.Request) {
	jpeg.Encode(w, getImg(), nil)
}

// PNGHandler returns a PNG image.
func PNGHandler(w http.ResponseWriter, r *http.Request) {
	png.Encode(w, getImg())
}

func getImg() image.Image {
	const n = 512
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}
	sq := func(i int) int { return i * i }

	for x := 0; x <= n; x++ {
		for y := 0; y <= n; y++ {
			if x == n/2 && y == n/2 {
				continue
			}
			d := math.Sqrt(float64(sq(abs(x-n/2)) + sq(abs(y-n/2))))
			if d > n/2 {
				continue
			}

			sin := float64(y-n/2) / d
			deg := math.Asin(sin)/math.Pi*359.0 + 180
			sec := int(deg) / 60

			var fix, mod *uint8
			var inc bool

			c := color.RGBA{0, 0, 0, 0xFF}
			switch sec {
			case 0:
				fix, mod = &c.R, &c.G
				inc = true
			case 1:
				fix, mod = &c.G, &c.R
				inc = false
			case 2:
				fix, mod = &c.G, &c.B
				inc = true
			case 3:
				fix, mod = &c.B, &c.G
				inc = false
			case 4:
				fix, mod = &c.B, &c.R
				inc = true
			case 5:
				fix, mod = &c.R, &c.B
				inc = false
			default:
				panic(fmt.Sprintf("deg=%f sec=%d", deg, sec))
			}

			v := uint8((int(deg) % 60) * 255.0 / 60.0)
			*fix = 255
			if inc {
				*mod = v
			} else {
				*mod = 255 - v
			}
			img.Set(x, y, c)

		}
	}
	return img
}

func parseData(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
