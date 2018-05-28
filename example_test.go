package httpbin_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	httpbin "github.com/ahmetb/go-httpbin"
)

func ExampleGetMux_httptest() {
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

func ExampleGetMux_server() {
	log.Fatal(http.ListenAndServe(":8080", httpbin.GetMux()))
}
