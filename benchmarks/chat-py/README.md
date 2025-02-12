# Benchmarking Text Generation

This script was adopted from the vLLM code base. The main differences are:
- Load the whole conversation as prompts.
- Limit the amount of max conversations and re-use the same conversation if needed.

This allows us to verify whether prefix aware load balancing provides a performance
boost under heavy production traffic with ongoing chat conversations.

## Running

Adjust the parameters in the `job.yaml` file and run the job using the following command:
```
kubectl apply -f job.yaml
```

