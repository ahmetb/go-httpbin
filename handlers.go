package httpbin

import (
	"net/http"

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
