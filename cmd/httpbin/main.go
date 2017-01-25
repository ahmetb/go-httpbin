package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/ahmetalpbalkan/go-httpbin"
)

var (
	host = flag.String("host", ":8080", "<host:port>")
)

func main() {
	flag.Parse()

	log.Printf("httpbin listening on %s", *host)
	log.Fatal(http.ListenAndServe(*host, httpbin.GetMux()))
}
