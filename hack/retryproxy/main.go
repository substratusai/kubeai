package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	var maxRetries = 1

	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("serving")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		r.Body.Close()

		fmt.Println("body:", string(body))

		newProxy(body, 0, maxRetries).ServeHTTP(w, newRequest(r, body))
	}))

}

var errRetry = errors.New("retry")

func newProxy(body []byte, attempt, maxRetries int) http.Handler {
	// go run ./hack/failserver
	u, err := url.Parse(getEndpoint(attempt))
	if err != nil {
		panic(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)

	proxy.ModifyResponse = func(r *http.Response) error {
		if r.StatusCode == http.StatusServiceUnavailable {
			// Returning an error will trigger the ErrorHandler.
			return errRetry
		}
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if err != nil && attempt < maxRetries {
			log.Println("retrying")

			// Simulate calling the next backend.
			// go run ./hack/successserver
			newProxy(body, attempt+1, maxRetries).ServeHTTP(w, newRequest(r, body))
			return
		}

		log.Printf("http: proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}

func getEndpoint(attempt int) string {
	switch attempt {
	case 0:
		return "http://localhost:8081"
	default:
		return "http://localhost:8082"
	}
}

func newRequest(r *http.Request, body []byte) *http.Request {
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	return clone
}
