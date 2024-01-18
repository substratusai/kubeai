package main

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	// go run ./hack/failserver
	u1, err := url.Parse("http://localhost:8081")
	if err != nil {
		panic(err)
	}
	p1 := httputil.NewSingleHostReverseProxy(u1)

	// go run ./hack/successserver
	u2, err := url.Parse("http://localhost:8082")
	if err != nil {
		panic(err)
	}
	p2 := httputil.NewSingleHostReverseProxy(u2)

	p1.ModifyResponse = func(r *http.Response) error {
		if r.StatusCode == http.StatusServiceUnavailable {
			// Returning an error will trigger the ErrorHandler.
			return errRetry
		}
		return nil
	}

	p1.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if err == errRetry {
			log.Println("retrying")
			// Simulate calling the next backend.
			p2.ServeHTTP(w, r)
			return
		}
	}

	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p1.ServeHTTP(w, r)
	}))
}

var errRetry = errors.New("retry")
