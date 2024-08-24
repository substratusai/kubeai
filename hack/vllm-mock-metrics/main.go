package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	// Serve metrics.txt at /metrics
	metrics, err := os.ReadFile("metrics.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("starting")
	log.Fatal(http.ListenAndServe(":8888", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("serving")
		w.Write(metrics)
	})))
}
