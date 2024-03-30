package subscriber

import "context"

type DeploymentManager interface {
	ResolveDeployment(model string) (string, bool)
	AtLeastOne(model string)
}

type EndpointManager interface {
	AwaitHostAddress(ctx context.Context, service, portName string) (string, error)
}

type QueueManager interface {
	EnqueueAndWait(ctx context.Context, deploymentName, id string) func()
}
