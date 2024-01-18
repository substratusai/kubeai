package main

import (
	"io"
	"net/http"
	"os"
)

func main() {
	http.ListenAndServe(":8081", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(os.Stdout, r.Body)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("unavailable\n"))
	}))
}
