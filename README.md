# go-httpbin

[![Read GoDoc](https://godoc.org/github.com/ahmetalpbalkan/go-httpbin?status.svg)](https://godoc.org/github.com/ahmetalpbalkan/go-httpbin)
[![Build Status](https://travis-ci.org/ahmetalpbalkan/go-httpbin.svg?branch=master)](https://travis-ci.org/ahmetalpbalkan/go-httpbin)

A Go handler that lets you test your HTTP client, retry logic, streaming behavior, timeouts etc.
with the endpoints of [httpbin.org][ht] locally in a [`net/http/httptest.Server`][hts].

This way, you can write tests without relying on an external dependency like [httpbin.org][ht].

## Endpoints

- `/ip` Returns Origin IP.
- `/user-agent` Returns user-agent.
- `/headers` Returns headers.
- `/get` Returns GET data.
- `/status/:code` Returns given HTTP Status code.
- `/redirect/:n` 302 Redirects _n_ times.
- `/absolute-redirect/:n` 302 Absolute redirects _n_ times.
- `/redirect-to?url=foo` 302 Redirects to the _foo_ URL.
- `/stream/:n` Streams _n_ lines of JSON objects
- `/delay/:n` Delays responding for _min(n, 10)_ seconds.
- `/bytes/:n` Generates _n_ random bytes of binary data, accepts optional _seed_ integer parameter.
- `/cookies` Returns the cookies.
- `/cookies/set?name=value` Sets one or more simple cookies.
- `/cookies/delete?name` Deletes one or more simple cookies.
- `/drip?numbytes=n&duration=s&delay=s&code=code` Drips data over a duration after
  an optional initial _delay_, then optionally returns with the given status _code_.

## How to use

Standing up a Go server running httpbin endpoints is just 1 line:

```go
package main

import (
    "log"
    "net/http"
    "github.com/ahmetalpbalkan/go-httpbin"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", httpbin.GetMux()))
}
```

Let's say you do not want a server running all the time because you just want to
test your HTTP logic after all. Integrating `httpbin` to your tests is very simple:

```go
package test

import (
    "testing"
    "net/http"
    "net/http/httptest"

    "github.com/ahmetalpbalkan/go-httpbin"
)

func TestDownload(t *testing.T)
    srv := httptest.NewServer(httpbin.GetMux())
    defer srv.Close()

    resp, err := http.Get(srv.URL + "/bytes/65536")
    if err != nil {
        t.Fatal(err)
    }
    // read from an actual HTTP server hosted locally
    // test whatever you are going to test...
}
```

## TODO

If you would like to contribute, I am hoping to implement the following

- [ ] `/stream-bytes` endpoint
- [ ] 100% go test coverage

# License

```
Copyright 2016 Ahmet Alp Balkan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

# Authors

- Ahmet Alp Balkan ([@ahmetalpbalkan][tw])

[ht]: https://httpbin.org/
[hts]: https://godoc.org/net/http/httptest#Server
[tw]: https://twitter.com/ahmetalpbalkan
