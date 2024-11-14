package apiutils

import "strings"

const (
	// adapterSeparator is the separator used to split model and adapter names
	// in API requests.
	//
	// Alternatives considered:
	//
	// "-" (hyphen): This is a common separator in Kubernetes resource names.
	// "." (dot): This is a common separator in model versions "llama-3.2".
	// "/" (slash): This would be incompatible with specifying model names inbetween slashes in URL paths (i.e. "/some-api/models/<model-id>/details").
	// ":" (colon): This might cause problems when specifying model names before colons in URL paths (see example below).
	//
	// See example of a path used in the Gemini API (https://ai.google.dev/gemini-api/docs/text-generation?lang=rest):
	// "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=$GOOGLE_API_KEY"
	adapterSeparator = "_"
)

// SplitModelAdapter splits a requested model name into KubeAI
// Model.metadata.name and Model.spec.adapters[].id.
func SplitModelAdapter(s string) (model, adapter string) {
	parts := strings.SplitN(s, adapterSeparator, 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// MergeModelAdapter merges a model and adapter name into a single string.
func MergeModelAdapter(model, adapter string) string {
	if adapter == "" {
		return model
	}
	return model + adapterSeparator + adapter
}
