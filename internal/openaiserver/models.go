package openaiserver

import (
	"encoding/json"
	"net/http"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (h *Handler) getModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// List models based on the "feature" query parameter.
	// Example (default):  /v1/models
	// Example (single):   /v1/models?feature=TextEmbedding
	// Example (multiple): /v1/models?feature=TextGeneration&feature=TextEmbedding
	features := r.URL.Query()["feature"]
	if len(features) == 0 {
		// Default to listing text generation models.
		// Do this to play nicely with chat UIs like OpenWebUI.
		features = []string{kubeaiv1.ModelFeatureTextGeneration}
	}

	var listOpts []client.ListOption
	headerSelectors := r.Header.Values("X-Label-Selector")
	for _, sel := range headerSelectors {
		parsedSel, err := labels.Parse(sel)
		if err != nil {
			sendErrorResponse(w, http.StatusBadRequest, "failed to parse label selector: %v", err)
			return
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: parsedSel})
	}

	var k8sModels []kubeaiv1.Model
	k8sModelNames := map[string]struct{}{}
	for _, feature := range features {
		// NOTE: At time of writing an OR query is not supported with the
		// Kubernetes API server
		// so we just do multiple queries and merge the results.
		labelSelector := client.MatchingLabels{kubeaiv1.ModelFeatureLabelDomain + "/" + feature: "true"}
		list := &kubeaiv1.ModelList{}
		opts := append([]client.ListOption{labelSelector}, listOpts...)
		if err := h.K8sClient.List(r.Context(), list, opts...); err != nil {
			sendErrorResponse(w, http.StatusInternalServerError, "failed to list models: %v", err)
			return
		}
		for _, model := range list.Items {
			if _, ok := k8sModelNames[model.Name]; !ok {
				k8sModels = append(k8sModels, model)
				k8sModelNames[model.Name] = struct{}{}
			}
		}
	}

	models := make([]Model, len(k8sModels))
	for i, k8sModel := range k8sModels {
		model := Model{}
		model.FromK8sModel(&k8sModel)
		models[i] = model
	}

	// Wrapper struct to match the desired output format
	response := struct {
		Object string  `json:"object"`
		Data   []Model `json:"data"`
	}{
		Object: "list",
		Data:   models,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
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

	Features []kubeaiv1.ModelFeature `json:"features,omitempty"`
}

func (m *Model) FromK8sModel(model *kubeaiv1.Model) {
	m.ID = model.Name
	m.Created = model.CreationTimestamp.Unix()
	m.Object = "model"
	m.OwnedBy = model.Spec.Owner
	m.Features = model.Spec.Features
}
