package k8sutils

import (
	"bytes"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// KubeAIManagesField finds the set of fields that KubeAI manages for a given object.
func KubeAIManagedFields(o client.Object) (fieldpath.Set, error) {
	managedFields := o.GetManagedFields()
	for _, mf := range managedFields {
		if mf.Manager == ManagerName {
			// Convert the managed fields to a set.
			var set fieldpath.Set
			if err := set.FromJSON(bytes.NewReader(mf.FieldsV1.Raw)); err != nil {
				return fieldpath.Set{}, fmt.Errorf("failed to parse managed fields: %w", err)
			}
			return set, nil
		}
	}

	return fieldpath.Set{}, nil
}

func ManagesField(set fieldpath.Set, path ...string) bool {
	return set.Has(strFieldPath(path...))
}

func strFieldPath(strPath ...string) fieldpath.Path {
	var fp fieldpath.Path
	for _, s := range strPath {
		fp = append(fp, fieldpath.PathElement{FieldName: &s})
	}
	return fp
}
