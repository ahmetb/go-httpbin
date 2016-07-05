package httpbin

import (
	"fmt"
	"net/http"
	"strconv"

	"net"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// GetMux returns the mux with handlers for httpbin endpoints registered.
func GetMux() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/ip", IPHandler).Methods("GET")
	r.HandleFunc("/user-agent", UserAgentHandler).Methods("GET")
	r.HandleFunc("/headers", HeadersHandler).Methods("GET")
	r.HandleFunc("/get", GetHandler).Methods("GET")
	r.HandleFunc("/redirect/{n:[0-9]+}", RedirectHandler).Methods("GET")
	r.HandleFunc("/absolute-redirect/{n:[0-9]+}", AbsoluteRedirectHandler).Methods("GET")
	r.HandleFunc("/redirect-to", RedirectToHandler).Methods("GET").Queries("url", "{url:.+}")
	return r
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
// to //host/get, otherwise to /host/absolute-redirect/(n-1)
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
