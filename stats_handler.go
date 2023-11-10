package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type StatsHandler struct {
	FIFO *FIFOQueueManager
}

type Stats struct {
	WaitCounts map[string]int64 `json:"waitCounts"`
}

func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	waitCounts := h.FIFO.WaitCounts()

	if err := json.NewEncoder(w).Encode(Stats{
		WaitCounts: waitCounts,
	}); err != nil {
		log.Printf("error writing response body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
