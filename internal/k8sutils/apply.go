package k8sutils

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ServerSideApply(ctx context.Context, cl client.Client, obj client.Object, controllerName string) error {
	gvk, err := ObjectToGroupVersionKind(cl.Scheme(), obj)
	if err != nil {
		return fmt.Errorf("getting group version kind: %w", err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return cl.Patch(ctx, obj, client.Apply, client.FieldOwner(controllerName), client.ForceOwnership)
}
