package v1

const (
	PodModelLabel = "model"

	// ModelPodIPAnnotation is the annotation key used to specify an IP
	// to use for the model Pod instead of the IP address in the status of the Pod.
	// Use in conjunction with --allow-pod-address-override for development purposes.
	ModelPodIPAnnotation   = "model-pod-ip"
	ModelPodPortAnnotation = "model-pod-port"
)
