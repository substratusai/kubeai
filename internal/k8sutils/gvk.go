package k8sutils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ObjectToGroupKind(s *runtime.Scheme, o client.Object) (schema.GroupKind, error) {
	gvks, _, err := s.ObjectKinds(o)
	if err != nil {
		return schema.GroupKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupKind{}, fmt.Errorf("no group kind for object")
	}
	return schema.GroupKind{
		Group: gvks[0].Group,
		Kind:  gvks[0].Kind,
	}, nil
}

func ObjectToGroupVersionKind(s *runtime.Scheme, o client.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := s.ObjectKinds(o)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("no group version kind for object")
	}
	return schema.GroupVersionKind{
		Group:   gvks[0].Group,
		Version: gvks[0].Version,
		Kind:    gvks[0].Kind,
	}, nil
}
