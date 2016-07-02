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
	return r
}

// IPHandler returns Origin IP.
func IPHandler(w http.ResponseWriter, r *http.Request) {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		writeErrorJSON(w, errors.Wrapf(err, "cannot parse addr: %v", r.RemoteAddr))
	}

	v := ipResponse{Origin: h}
	if err := writeJSON(w, v); err != nil {
		// TODO handle
	}
}
