package main

import (
	"net/http"
	"sync"
)

func main() {
	// HTTP server that fails once and then succeeds for a given request path
	var mtx sync.RWMutex
	paths := map[string]bool{}

	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtx.RLock()
		shouldSucceed := paths[r.URL.Path]
		mtx.RUnlock()

		defer func() {
			mtx.Lock()
			paths[r.URL.Path] = true
			mtx.Unlock()
		}()

		if !shouldSucceed {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("failure\n"))
			return
		}

		w.Write([]byte("success\n"))
	}))
}
