package k8sutils

import "sigs.k8s.io/controller-runtime/pkg/client"

const ManagerName = "kubeai-manager"

func DefaultUpdateOptions() *client.UpdateOptions {
	return &client.UpdateOptions{
		FieldManager: ManagerName,
	}
}

func DefaultSubResourceUpdateOptions() *client.UpdateOptions {
	return &client.UpdateOptions{
		FieldManager: ManagerName,
	}
}

func DefaultCreateOptions() *client.CreateOptions {
	return &client.CreateOptions{
		FieldManager: ManagerName,
	}
}

func DefaultPatchOptions() *client.PatchOptions {
	return &client.PatchOptions{
		FieldManager: ManagerName,
	}
}
