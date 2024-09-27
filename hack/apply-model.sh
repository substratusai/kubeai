model='opt-125m-cpu'
helm template ./charts/models --set "catalog.$model.enabled=true" --set "catalog.$model.minReplicas=1" | kubectl apply -f -