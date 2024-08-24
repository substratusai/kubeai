package openaiserver

import (
	"encoding/json"
	"net/http"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
)

func (h *Handler) getModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	list := &kubeaiv1.ModelList{}
	if err := h.K8sClient.List(r.Context(), list); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "failed to list models: %v", err)
		return
	}

	models := make([]Model, len(list.Items))
	for i, k8sModel := range list.Items {
		model := Model{}
		model.FromK8sModel(&k8sModel)
		models[i] = model
	}

	if err := json.NewEncoder(w).Encode(models); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "failed to encode response: %v", err)
		return
	}
}

// Model is a struct that represents a model object
// from the OpenAI API.
type Model struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`

	// Adiditional (non-OpenAI) fields

	Features []string `json:"features,omitempty"`
}

func (m *Model) FromK8sModel(model *kubeaiv1.Model) {
	m.ID = model.Name
	m.Created = model.CreationTimestamp.Unix()
	m.Object = "model"
	m.OwnedBy = model.Spec.Owner
	m.Features = model.Spec.Features
}
