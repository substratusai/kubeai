package main

import "net/http"

func main() {
	http.ListenAndServe(":8082", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("success\n"))
	}))
}
