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
	r.HandleFunc("/ip", IPHandler)
	r.HandleFunc("/user-agent", UserAgentHandler)
	r.HandleFunc("/headers", HeadersHandler)
	return r
}

// IPHandler returns Origin IP.
func IPHandler(w http.ResponseWriter, r *http.Request) {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		writeErrorJSON(w, errors.Wrapf(err, "cannot parse addr: %v", r.RemoteAddr))
	}

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
	hdr := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		hdr[k] = v[0]
	}
	if err := writeJSON(w, headersResponse{hdr}); err != nil {
		writeErrorJSON(w, errors.Wrap(err, "failed to write json"))
	}
}
