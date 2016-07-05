package httpbin_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"io/ioutil"

	"github.com/ahmetalpbalkan/go-httpbin"
)

func ExampleGetMux() {
	srv := httptest.NewServer(httpbin.GetMux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/bytes/65536")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// read from an actual HTTP server hosted locally
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Retrieved %d bytes.\n", len(b))
	// Output: Retrieved 65536 bytes.
}
