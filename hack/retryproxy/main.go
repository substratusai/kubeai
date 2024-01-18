package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		r.Body.Close()

		// go run ./hack/failserver
		first := newReverseProxy("http://localhost:8081")

		first.ModifyResponse = func(r *http.Response) error {
			if r.StatusCode == http.StatusServiceUnavailable {
				// Returning an error will trigger the ErrorHandler.
				return errRetry
			}
			return nil
		}

		first.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if err == errRetry {
				log.Println("retrying")

				// Simulate calling the next backend.
				// go run ./hack/successserver
				next := newReverseProxy("http://localhost:8082")
				next.ServeHTTP(w, newReq(r, body))
				return
			}
		}

		log.Println("serving")
		first.ServeHTTP(w, newReq(r, body))
	}))
}

var errRetry = errors.New("retry")

func newReq(r *http.Request, body []byte) *http.Request {
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	return clone
}

func newReverseProxy(addr string) *httputil.ReverseProxy {
	u, err := url.Parse(addr)
	if err != nil {
		panic(err)
	}
	return httputil.NewSingleHostReverseProxy(u)
}
