package httpbin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

func writeJSON(w io.Writer, v interface{}) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return errors.Wrap(e.Encode(v), "failed to encode JSON")
}

func writeErrorJSON(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_ = writeJSON(w, errorResponse{errObj{err.Error()}}) // ignore error, can't do anything
}

func getHeaders(r *http.Request) map[string]string {
	hdr := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		hdr[k] = v[0]
	}
	return hdr
}

func getCookies(cs []*http.Cookie) map[string]string {
	m := make(map[string]string, len(cs))
	for _, v := range cs {
		m[v.Name] = v.Value
	}
	return m
}

func flattenValues(uv url.Values) map[string]interface{} {
	m := make(map[string]interface{}, len(uv))

	for k, v := range uv {
		if len(v) == 1 {
			m[k] = v[0]
		} else {
			m[k] = v
		}
	}
	return m
}
