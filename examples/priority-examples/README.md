# Model Priority Examples

This directory contains examples of how to use Kubernetes Pod Priority and Preemption with KubeAI models.

## Understanding Pod Priority and Preemption

Kubernetes [Pod Priority and Preemption](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) is a feature that allows you to assign different priority levels to pods. When resources are constrained, Kubernetes will prioritize scheduling high-priority pods over low-priority pods, and may even evict low-priority pods to make room for high-priority pods.

This is particularly useful in GPU-constrained environments where you want to ensure that critical AI services always have access to resources, even if it means temporarily removing less important tasks.

## Files in this Directory

- `priority-classes.yaml`: Defines the PriorityClass resources needed to implement pod priority
- `critical-service-model.yaml`: An example of a high-priority model that should preempt other models when resources are limited
- `background-research-model.yaml`: An example of a low-priority model that can be preempted when resources are needed for higher priority models

## Usage

1. First, apply the priority classes to your cluster:

```bash
kubectl apply -f priority-classes.yaml
```

2. Deploy your models with appropriate priority classes:

```bash
kubectl apply -f critical-service-model.yaml
kubectl apply -f background-research-model.yaml
```

## Behavior

When the cluster is under resource pressure:

1. The `critical-service-model` will be scheduled preferentially
2. If necessary, the `background-research-model` may be evicted to make room for the `critical-service-model`

This ensures that your most important models remain available even during resource constraints.

## Notes on Pod Disruption

When a low-priority pod is preempted:

1. The pod receives a SIGTERM signal
2. After the grace period (30s by default), if the pod hasn't terminated, it receives a SIGKILL
3. The pod is removed from the cluster
4. KubeAI's controller will attempt to reschedule the pod when resources become available again

For more details on configuring priority and preemption, see the [Kubernetes documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/). 