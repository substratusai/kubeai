package stats

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/substratusai/lingo/pkg/queue"
)

type Handler struct {
	FIFO *queue.Manager
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	waitCounts := h.FIFO.TotalCounts()

	if err := json.NewEncoder(w).Encode(Stats{
		WaitCounts: waitCounts,
	}); err != nil {
		log.Printf("error writing response body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
