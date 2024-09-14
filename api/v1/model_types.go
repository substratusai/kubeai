/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	Owner string `json:"owner"`
	URL   string `json:"url"`

	Features []ModelFeature `json:"features"`

	// +kubebuilder:validation:Enum=OLlama;VLLM;FasterWhisper
	Engine string `json:"engine"`

	Replicas *int32 `json:"replicas,omitempty"`

	// ResourceProfile maps to specific pre-configured resources.
	ResourceProfile string `json:"resourceProfile,omitempty"`

	// Image to be used for the server process.
	// Will be set from the ResourceProfile if not specified.
	Image string `json:"image,omitempty"`

	// Resources to be allocated to the server process.
	// Will be set from the ResourceProfile if not specified.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector to be added to the server process.
	// Will be set from the ResourceProfile if not specified.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity to be added to the server process.
	// Will be set from the ResourceProfile if not specified.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations to be added to the server process.
	// Will be set from the ResourceProfile if not specified.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Args to be added to the server process.
	Args []string `json:"args,omitempty"`

	// Env variables to be added to the server process.
	Env map[string]string `json:"env,omitempty"`

	// AutoscalingProfile to be used for autoscaling.
	AutoscalingProfile string `json:"autoscalingProfile,omitempty"`

	// Autoscaling configuration.
	// Will be set from the AutoscalingProfile if not specified.
	Autoscaling *ModelAutoscaling `json:"autoscaling,omitempty"`
}

type ModelAutoscaling struct {
	// Disabled is used to disable autoscaling.
	Disabled bool `json:"disabled,omitempty"`
	// TargetRequests is the target number of active requests per Pod.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=100
	TargetRequests int32 `json:"targetRequests"`
	// ScaleDownDelay is the minimum time before a deployment is scaled down after
	// the autoscaling algorithm determines that it should be scaled down.
	ScaleDownDelay metav1.Duration `json:"scaleDownDelay"`
	// MinReplicas is the minimum number of Pod replicas that the model can scale down to.
	// Note: 0 is a valid value.
	// +kubebuilder:validation:Minimum=0
	MinReplicas int32 `json:"minReplicas"`
	// MinReplicas is the minimum number of Pod replicas that the model can scale up to.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	MaxReplicas int32 `json:"maxReplicas"`
}

// +kubebuilder:validation:Enum=TextGeneration;TextEmbedding;SpeechToText
type ModelFeature string

const (
	ModelFeatureTextGeneration = "TextGeneration"
	ModelFeatureTextEmbedding  = "TextEmbedding"
	// TODO (samos123): Add validation that Speech to Text only supports Faster Whisper.
	ModelFeatureSpeechToText = "SpeechToText"
)

const (
	OLlamaEngine        = "OLlama"
	VLLMEngine          = "VLLM"
	FasterWhisperEngine = "FasterWhisper"
)

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	Replicas ModelStatusReplicas `json:"replicas,omitempty"`
}

type ModelStatusReplicas struct {
	All   int32 `json:"all"`
	Ready int32 `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas.all

// Model is the Schema for the models API
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec,omitempty"`
	Status ModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
