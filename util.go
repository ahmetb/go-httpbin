package httpbin

import "io"
import "encoding/json"
import "github.com/pkg/errors"
import "net/http"

func writeJSON(w io.Writer, v interface{}) error {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return errors.Wrap(err, "failed to encode JSON")
	}
	return nil
}

func writeErrorJSON(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_ = writeJSON(w, errorResponse{errObj{err.Error()}}) // ignore error, can't do anything
}
