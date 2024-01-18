package main

import (
	"io"
	"net/http"
	"os"
)

func main() {
	http.ListenAndServe(":8082", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(os.Stdout, r.Body)
		w.Write([]byte("success\n"))
	}))
}
