package openaiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/substratusai/kubeai/internal/modelproxy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	ModelProxy *modelproxy.Handler
	K8sClient  client.Client
	mux        *http.ServeMux
}

func NewHandler(k8sClient client.Client, modelProxy *modelproxy.Handler) *Handler {
	h := &Handler{
		K8sClient: k8sClient,
		mux:       http.NewServeMux(),
	}

	h.mux.Handle("/openai/v1/chat/completions", http.StripPrefix("/openai", modelProxy))
	h.mux.Handle("/openai/v1/completions", http.StripPrefix("/openai", modelProxy))
	h.mux.Handle("/openai/v1/embeddings", http.StripPrefix("/openai", modelProxy))
	h.mux.Handle("/openai/v1/audio/transcriptions", http.StripPrefix("/openai", modelProxy))
	h.mux.HandleFunc("/openai/v1/models", h.getModels)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func sendErrorResponse(w http.ResponseWriter, status int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("sending error response: %v: %v", status, msg)

	w.WriteHeader(status)

	if status >= 500 {
		// Don't leak internal error messages to the client.
		msg = http.StatusText(status)
	}

	if err := json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{
		Error: msg,
	}); err != nil {
		log.Printf("error encoding error response: %v", err)
	}
}
