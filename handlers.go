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
