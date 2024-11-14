package apiutils

import "strings"

func SplitModelAdapter(s string) (model, adapter string) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func MergeModelAdapter(model, adapter string) string {
	if adapter == "" {
		return model
	}
	return model + "." + adapter
}
