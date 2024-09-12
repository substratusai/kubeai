# API Reference

## Packages
- [kubeai.org/v1](#kubeaiorgv1)


## kubeai.org/v1

Package v1 contains API Schema definitions for the kubeai v1 API group

### Resource Types
- [Model](#model)



#### Model



Model is the Schema for the models API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `kubeai.org/v1` | | |
| `kind` _string_ | `Model` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ModelSpec](#modelspec)_ |  |  |  |
| `status` _[ModelStatus](#modelstatus)_ |  |  |  |


#### ModelFeature

_Underlying type:_ _string_



_Validation:_
- Enum: [TextGeneration TextEmbedding SpeechToText]

_Appears in:_
- [ModelSpec](#modelspec)



#### ModelSpec



ModelSpec defines the desired state of Model



_Appears in:_
- [Model](#model)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `owner` _string_ |  |  |  |
| `url` _string_ |  |  |  |
| `features` _[ModelFeature](#modelfeature) array_ |  |  | Enum: [TextGeneration TextEmbedding SpeechToText] <br /> |
| `engine` _string_ |  |  | Enum: [OLlama VLLM FasterWhisper] <br /> |
| `replicas` _integer_ |  |  |  |
| `minReplicas` _integer_ |  |  |  |
| `maxReplicas` _integer_ |  |  |  |
| `resourceProfile` _string_ | ResourceProfile maps to specific pre-configured resources. |  |  |
| `image` _string_ | Image to be used for the server process.<br />Will be set from the ResourceProfile if not specified. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#resourcerequirements-v1-core)_ | Resources to be allocated to the server process.<br />Will be set from the ResourceProfile if not specified. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector to be added to the server process.<br />Will be set from the ResourceProfile if not specified. |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#affinity-v1-core)_ | Affinity to be added to the server process.<br />Will be set from the ResourceProfile if not specified. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#toleration-v1-core) array_ | Tolerations to be added to the server process.<br />Will be set from the ResourceProfile if not specified. |  |  |
| `args` _string array_ | Args to be added to the server process. |  |  |
| `env` _object (keys:string, values:string)_ | Env variables to be added to the server process. |  |  |


#### ModelStatus



ModelStatus defines the observed state of Model



_Appears in:_
- [Model](#model)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _[ModelStatusReplicas](#modelstatusreplicas)_ |  |  |  |


#### ModelStatusReplicas







_Appears in:_
- [ModelStatus](#modelstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `all` _integer_ |  |  |  |
| `ready` _integer_ |  |  |  |


