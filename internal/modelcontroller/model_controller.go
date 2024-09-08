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

package modelcontroller

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeaiv1 "github.com/substratusai/kubeai/api/v1"
	"github.com/substratusai/kubeai/internal/config"
	utils "github.com/substratusai/kubeai/internal/k8sutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const modelReconcilerName = "kubeai-model-controller"

// ModelReconciler reconciles a Model object
type ModelReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Namespace               string
	AllowPodAddressOverride bool
	HuggingfaceSecretName   string
	ResourceProfiles        map[string]config.ResourceProfile
	ModelServers            config.ModelServers
}

// +kubebuilder:rbac:groups=kubeai.org,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeai.org,resources=models/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeai.org,resources=models/scale,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeai.org,resources=models/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=pods/finalizers,verbs=update

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	model := &kubeaiv1.Model{}
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var shouldUpdate bool
	if model.Spec.Replicas == nil {
		shouldUpdate = true
		model.Spec.Replicas = ptr.To(model.Spec.MinReplicas)
	}
	{
		changed, err := r.applyResourceProfile(model)
		if err != nil {
			log.Error(err, "applying resource profile")
			// No use in retrying here, return nil error.
			return ctrl.Result{}, nil
		}
		if changed {
			log.Info("applied resource profile")
			shouldUpdate = true
		}
	}
	// Apply self labels based on features so that we can easily filter models.
	if changed := r.applySelfLabels(model); changed {
		shouldUpdate = true
	}
	if shouldUpdate {
		if err := r.Update(ctx, model); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating model: %w", err)
		}
	}

	allPods := &corev1.PodList{}
	if err := r.List(ctx, allPods, client.InNamespace(model.Namespace), client.MatchingLabels{
		kubeaiv1.PodModelLabel: model.Name,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing all node pools: %w", err)
	}

	scaleActual := int32(len(allPods.Items))
	scaleDesired := *model.Spec.Replicas
	scaleDiff := scaleActual - scaleDesired
	scaleDiffAbs := int32(math.Abs(float64(scaleDiff)))

	// TODO: Take into account Pods that are in a deletion state.

	var podForModel func(*kubeaiv1.Model, int32) *corev1.Pod
	switch model.Spec.Engine {
	case kubeaiv1.OLlamaEngine:
		podForModel = r.oLlamaPodForModel
	case kubeaiv1.FasterWhisperEngine:
		podForModel = r.fasterWhisperPodForModel
	case kubeaiv1.InfinityEngine:
		podForModel = r.infinityPodForModel
	default:
		podForModel = r.vLLMPodForModel
	}

	switch {
	case scaleDiff == 0:
		// At correct scale.
		log.Info("Pod count matches", "actualReplicas", scaleActual, "desiredReplicas", scaleDesired)
	case scaleDiff < 0:
		// Create Pods.
		log.Info("Need to add pods", "scaleDiff", scaleDiff)

		var toCreate []*corev1.Pod
		for i := int32(0); i < scaleDiffAbs; i++ {
			toCreate = append(toCreate, podForModel(model, scaleActual+i))
		}

		for _, pod := range toCreate {
			if err := ctrl.SetControllerReference(model, pod, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
			}
			if err := r.Client.Create(ctx, pod); err != nil {
				return ctrl.Result{}, fmt.Errorf("creating pod: %w", err)
			}
		}
	case scaleDiff > 0:
		// Delete Pods.
		log.Info("Need to delete pods", "replicaDiff", scaleDiff)

		toDeleteCount := scaleDiffAbs
		for _, pod := range allPods.Items {
			if toDeleteCount == 0 {
				break
			}
			if err := r.Client.Delete(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("deleting pod: %w", err)
			}
			toDeleteCount--
		}
	}

	// Summarize all nodes.
	var readyPods int32
	for _, pod := range allPods.Items {
		if utils.PodIsReady(&pod) {
			readyPods++
		}
	}

	if statusReplicas := (kubeaiv1.ModelStatusReplicas{
		All:   int32(len(allPods.Items)),
		Ready: readyPods,
	}); statusReplicas != model.Status.Replicas {
		model.Status.Replicas = statusReplicas
		if err := r.Status().Update(ctx, model); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeaiv1.Model{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

/*
func (r *ModelReconciler) apply(ctx context.Context, model *kubeaiv1.Model, obj client.Object) error {
	if err := ctrlutil.SetControllerReference(model, obj, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference: %w", err)
	}
	return utils.ServerSideApply(ctx, r.Client, obj, modelReconcilerName)
}
*/

func (r *ModelReconciler) vLLMPodForModel(m *kubeaiv1.Model, index int32) *corev1.Pod {
	lbs := labelsForModel(m)
	ann := r.annotationsForModel(m)
	if _, ok := ann[kubeaiv1.ModelPodPortAnnotation]; !ok {
		// Set port to 8000 (vLLM) if not overwritten.
		ann[kubeaiv1.ModelPodPortAnnotation] = "8000"
	}

	args := []string{
		"--model=" + strings.TrimPrefix(m.Spec.URL, "hf://"),
		"--served-model-name=" + m.Name,
		// NOTE: The following flag is a workaround for a known issue with VLLM where metrics wont show.
		// https://github.com/vllm-project/vllm/issues/7188
		"--disable-frontend-multiprocessing",
	}
	args = append(args, m.Spec.Args...)

	env := []corev1.EnvVar{
		{
			// TODO: Conditionally set this token based on whether
			// huggingface is the model source.
			Name: "HF_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.HuggingfaceSecretName,
					},
					Key: "token",
				},
			},
		},
	}
	var envKeys []string
	for key := range m.Spec.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		env = append(env, corev1.EnvVar{
			Name:  key,
			Value: m.Spec.Env[key],
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("model-%s-%d", m.Name, index),
			Namespace:   m.Namespace,
			Labels:      lbs,
			Annotations: ann,
		},
		Spec: corev1.PodSpec{
			NodeSelector: m.Spec.NodeSelector,
			Containers: []corev1.Container{
				{
					Name:      "server",
					Image:     m.Spec.Image,
					Args:      args,
					Env:       env,
					Resources: *m.Spec.Resources,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8000,
							Protocol:      corev1.ProtocolTCP,
							Name:          "http",
						},
					},
					StartupProbe: &corev1.Probe{
						// TODO: Decrease the default and make it configurable.
						// Give the model 3 hours to start up.
						FailureThreshold: 5400,
						PeriodSeconds:    2,
						TimeoutSeconds:   2,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						FailureThreshold: 3,
						PeriodSeconds:    10,
						TimeoutSeconds:   2,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						FailureThreshold: 3,
						PeriodSeconds:    30,
						TimeoutSeconds:   3,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dshm",
							MountPath: "/dev/shm",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "dshm",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
							// TODO: Set size limit
						},
					},
				},
			},
		},
	}

	return pod
}

func (r *ModelReconciler) oLlamaPodForModel(m *kubeaiv1.Model, index int32) *corev1.Pod {
	lbs := labelsForModel(m)
	ann := r.annotationsForModel(m)

	if _, ok := ann[kubeaiv1.ModelPodPortAnnotation]; !ok {
		// Set port to 8000 (vLLM) if not overwritten.
		ann[kubeaiv1.ModelPodPortAnnotation] = "8000"
	}

	env := []corev1.EnvVar{
		{
			Name:  "OLLAMA_HOST",
			Value: "0.0.0.0:8000",
		},
		{
			// Ollama server typically operates in a 1:N server-to-model mode so it
			// swaps models in and out of memory. In our case we are deploying 1:1
			// model-to-server-pod so we want to always keep the model in memory.
			Name: "OLLAMA_KEEP_ALIVE",
			// Ollama treates 0 as "no keep alive" so we need to set a large value.
			Value: "999999h",
		},
	}
	var envKeys []string
	for key := range m.Spec.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		env = append(env, corev1.EnvVar{
			Name:  key,
			Value: m.Spec.Env[key],
		})
	}

	ollamaModelRef := strings.TrimPrefix(m.Spec.URL, "ollama://")

	featuresMap := map[kubeaiv1.ModelFeature]struct{}{}
	for _, f := range m.Spec.Features {
		featuresMap[f] = struct{}{}
	}

	// Pull model and copy to rename it to Model.metadata.name.
	// See Ollama issue for rename/copy workaround: https://github.com/ollama/ollama/issues/5914
	// NOTE: The cp command should just create a pointer to the old model, not copy data
	// (see https://github.com/ollama/ollama/issues/5914#issuecomment-2248168474).
	// Use `ollama run` to send a single prompt to ollama to load the model into memory
	// before the Pod becomes Ready. (by default it will load on the first prompt request).
	startupProbeScript := fmt.Sprintf("/bin/ollama pull %s && /bin/ollama cp %s %s",
		ollamaModelRef, ollamaModelRef, m.Name)
	if _, ok := featuresMap[kubeaiv1.ModelFeatureTextGeneration]; ok {
		// NOTE: Embedding text models do not support "ollama run":
		//
		// ollama run nomic-embed-text hey
		// Error: "nomic-embed-text" does not support generate
		//
		startupProbeScript += fmt.Sprintf(" && /bin/ollama run %s hi", m.Name)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("model-%s-%d", m.Name, index),
			Namespace:   m.Namespace,
			Labels:      lbs,
			Annotations: ann,
		},
		Spec: corev1.PodSpec{
			NodeSelector: m.Spec.NodeSelector,
			Containers: []corev1.Container{
				{
					Name:      "server",
					Image:     m.Spec.Image,
					Args:      m.Spec.Args,
					Env:       env,
					Resources: *m.Spec.Resources,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8000,
							Protocol:      corev1.ProtocolTCP,
							Name:          "http",
						},
					},
					// Use a startup probe to pull the model because ollama server needs
					// to be running already (`ollama pull` issues a HTTP request to the server).
					// Example log from ollama server when a model is pulled:
					// [GIN] 2024/08/20 - 15:12:28 | 200 |  981.561436ms |       127.0.0.1 | POST     "/api/pull"
					StartupProbe: &corev1.Probe{
						InitialDelaySeconds: 1,
						PeriodSeconds:       3,
						FailureThreshold:    10,
						// Give the model pull 180 minutes to complete.
						TimeoutSeconds: 60 * 180,
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{
									"bash", "-c",
									startupProbeScript,
								},
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						FailureThreshold: 3,
						// Will be delayed by the startup probe, so no need to delay here.
						InitialDelaySeconds: 0,
						PeriodSeconds:       10,
						TimeoutSeconds:      2,
						SuccessThreshold:    1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromString("http"),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						FailureThreshold:    3,
						InitialDelaySeconds: 900,
						TimeoutSeconds:      3,
						PeriodSeconds:       30,
						SuccessThreshold:    1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromString("http"),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dshm",
							MountPath: "/dev/shm",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "dshm",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
							// TODO: Set size limit
						},
					},
				},
			},
		},
	}

	return pod

}

func (r *ModelReconciler) fasterWhisperPodForModel(m *kubeaiv1.Model, index int32) *corev1.Pod {
	lbs := labelsForModel(m)
	ann := r.annotationsForModel(m)
	if _, ok := ann[kubeaiv1.ModelPodPortAnnotation]; !ok {
		ann[kubeaiv1.ModelPodPortAnnotation] = "8000"
	}

	args := []string{}
	args = append(args, m.Spec.Args...)

	env := []corev1.EnvVar{
		{
			Name:  "WHISPER__MODEL",
			Value: strings.TrimPrefix(m.Spec.URL, "hf://"),
		},
		{
			Name:  "ENABLE_UI",
			Value: "false",
		},
		{
			// TODO: Conditionally set this token based on whether
			// huggingface is the model source.
			Name: "HF_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.HuggingfaceSecretName,
					},
					Key:      "token",
					Optional: ptr.To(true),
				},
			},
		},
	}
	var envKeys []string
	for key := range m.Spec.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		env = append(env, corev1.EnvVar{
			Name:  key,
			Value: m.Spec.Env[key],
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("model-%s-%d", m.Name, index),
			Namespace:   m.Namespace,
			Labels:      lbs,
			Annotations: ann,
		},
		Spec: corev1.PodSpec{
			NodeSelector: m.Spec.NodeSelector,
			Containers: []corev1.Container{
				{
					Name:      "server",
					Image:     m.Spec.Image,
					Args:      args,
					Env:       env,
					Resources: *m.Spec.Resources,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8000,
							Protocol:      corev1.ProtocolTCP,
							Name:          "http",
						},
					},
					StartupProbe: &corev1.Probe{
						// Give the model 30 minutes to start up.
						FailureThreshold: 900,
						PeriodSeconds:    2,
						TimeoutSeconds:   2,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						FailureThreshold: 3,
						PeriodSeconds:    10,
						TimeoutSeconds:   2,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						FailureThreshold: 3,
						PeriodSeconds:    30,
						TimeoutSeconds:   3,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dshm",
							MountPath: "/dev/shm",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "dshm",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
			},
		},
	}

	return pod
}

func (r *ModelReconciler) infinityPodForModel(m *kubeaiv1.Model, index int32) *corev1.Pod {
	lbs := labelsForModel(m)
	ann := r.annotationsForModel(m)

	if _, ok := ann[kubeaiv1.ModelPodPortAnnotation]; !ok {
		// Set port to 8000 (vLLM) if not overwritten.
		ann[kubeaiv1.ModelPodPortAnnotation] = "8000"
	}

	env := []corev1.EnvVar{
		{
			Name:  "INFINITY_MODEL_ID",
			// TODO: infinity supports multiple models, separate by comma.
			Value: strings.TrimPrefix(m.Spec.URL, "hf://"),
		},
		{
			Name:  "INFINITY_ENGINE",
			// TODO: switch between optimum backend (cpu), nvidia/amd (torch), inf2 (inferentia) based on what is available.
			Value: "torch",
		},
		{
			Name:  "INFINITY_PORT",
			Value: "8000",
		},
		{
			// TODO: Conditionally set this token based on whether
			// huggingface is the model source.
			Name: "HF_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.HuggingfaceSecretName,
					},
					Key:      "token",
					Optional: ptr.To(true),
				},
			},
		},
	}
	var envKeys []string
	for key := range m.Spec.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		env = append(env, corev1.EnvVar{
			Name:  key,
			Value: m.Spec.Env[key],
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("model-%s-%d", m.Name, index),
			Namespace:   m.Namespace,
			Labels:      lbs,
			Annotations: ann,
		},
		Spec: corev1.PodSpec{
			NodeSelector: m.Spec.NodeSelector,
			Containers: []corev1.Container{
				{
					Name:      "server",
					Image:     m.Spec.Image,
					Args:      args,
					Env:       env,
					Resources: *m.Spec.Resources,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8000,
							Protocol:      corev1.ProtocolTCP,
							Name:          "http",
						},
					},
					StartupProbe: &corev1.Probe{
						// TODO: Decrease the default and make it configurable.
						// Give the model 20 minutes to start up.
						FailureThreshold: 600,
						PeriodSeconds:    2,
						TimeoutSeconds:   2,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						FailureThreshold: 3,
						PeriodSeconds:    10,
						TimeoutSeconds:   2,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						FailureThreshold: 3,
						PeriodSeconds:    30,
						TimeoutSeconds:   3,
						SuccessThreshold: 1,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromString("http"),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dshm",
							MountPath: "/dev/shm",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "dshm",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
							// TODO: Set size limit
						},
					},
				},
			},
		},
	}

	return pod
}

func labelsForModel(m *kubeaiv1.Model) map[string]string {
	return map[string]string{"app": "model", "model": m.Name}
}

func (r *ModelReconciler) annotationsForModel(m *kubeaiv1.Model) map[string]string {
	ann := map[string]string{}

	if modelAnn := m.GetAnnotations(); modelAnn != nil {
		var keys []string
		if r.AllowPodAddressOverride {
			keys = append(keys,
				kubeaiv1.ModelPodIPAnnotation,
				kubeaiv1.ModelPodPortAnnotation,
			)
		}
		// Copy over relevant model annotations.
		for _, key := range keys {
			if val, ok := modelAnn[key]; ok {
				ann[key] = val
			}
		}
	}

	return ann
}

func (r *ModelReconciler) applyResourceProfile(model *kubeaiv1.Model) (bool, error) {
	split := strings.Split(model.Spec.ResourceProfile, ":")
	if len(split) != 2 {
		return false, fmt.Errorf("invalid resource profile: %q, should match <name>:<multiple>, example: nvidia-gpu-l4:2", model.Spec.ResourceProfile)
	}
	name := split[0]
	multiple, err := strconv.Atoi(split[1])
	if err != nil {
		return false, fmt.Errorf("invalid multiple in resource profile multiple: %q: %w", split[1], err)
	}

	profile, ok := r.ResourceProfiles[name]
	if !ok {
		return false, fmt.Errorf("resource profile not found: %q", name)
	}

	requests := make(corev1.ResourceList)
	for key, quantity := range profile.Requests {
		q := quantity.DeepCopy()
		q.Mul(int64(multiple))
		requests[key] = q
	}

	limits := make(corev1.ResourceList)
	for key, quantity := range profile.Limits {
		q := quantity.DeepCopy()
		q.Mul(int64(multiple))
		limits[key] = q
	}

	var changed bool

	resources := corev1.ResourceRequirements{
		Requests: requests,
		Limits:   limits,
	}
	if model.Spec.Resources == nil || !resourcesEqual(model.Spec.Resources.Requests, requests) || !resourcesEqual(model.Spec.Resources.Limits, limits) {
		model.Spec.Resources = &resources
		changed = true
	}

	nodeSelector := profile.NodeSelector
	if !selectorsEqual(nodeSelector, model.Spec.NodeSelector) {
		model.Spec.NodeSelector = nodeSelector
		changed = true
	}

	image, err := r.lookupServerImage(model, profile)
	if err != nil {
		return false, fmt.Errorf("looking up server image: %w", err)
	}
	if model.Spec.Image == "" || model.Spec.Image != image {
		model.Spec.Image = image
		changed = true
	}

	return changed, nil
}

func (r *ModelReconciler) lookupServerImage(model *kubeaiv1.Model, profile config.ResourceProfile) (string, error) {
	var serverImgs map[string]string
	switch model.Spec.Engine {
	case kubeaiv1.OLlamaEngine:
		serverImgs = r.ModelServers.OLlama.Images
	case kubeaiv1.FasterWhisperEngine:
		serverImgs = r.ModelServers.FasterWhisper.Images
	case kubeaiv1.InfinityEngine:
		serverImgs = r.ModelServers.Infinity.Images
	default:
		serverImgs = r.ModelServers.VLLM.Images
	}

	// If no image name is provided for a profile, use the default image name.
	const defaultImageName = "default"
	imageName := defaultImageName
	if profile.ImageName != "" {
		imageName = profile.ImageName
	}

	if img, ok := serverImgs[imageName]; ok {
		return img, nil
	}

	// If the specific profile image name does not exist, use the default image name.
	if img, ok := serverImgs[defaultImageName]; ok {
		return img, nil
	} else {
		return "", fmt.Errorf("missing default server image")
	}
}

func (r *ModelReconciler) applySelfLabels(model *kubeaiv1.Model) bool {
	modelFeaturesMap := make(map[kubeaiv1.ModelFeature]struct{}, len(model.Spec.Features))
	for _, f := range model.Spec.Features {
		modelFeaturesMap[f] = struct{}{}
	}

	if model.GetLabels() == nil {
		model.SetLabels(map[string]string{})
	}

	var changed bool

	// Delete non-matching feature labels.
	for key := range model.GetLabels() {
		if strings.HasPrefix(key, kubeaiv1.ModelFeatureLabelDomain) {
			feat := strings.TrimPrefix(key, kubeaiv1.ModelFeatureLabelDomain+"/")
			if _, ok := modelFeaturesMap[kubeaiv1.ModelFeature(feat)]; !ok {
				delete(model.GetLabels(), key)
				changed = true
			}
		}
	}

	// Add missing feature labels.
	for feat := range modelFeaturesMap {
		labelKey := fmt.Sprintf("%s/%s", kubeaiv1.ModelFeatureLabelDomain, feat)
		if _, ok := model.GetLabels()[labelKey]; !ok {
			model.GetLabels()[labelKey] = "true"
			changed = true
		}
	}

	return changed
}

func resourcesEqual(a, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for key, quantity := range a {
		if q, ok := b[key]; !ok || !q.Equal(quantity) {
			return false
		}
	}
	return true
}

func selectorsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, val := range a {
		if v, ok := b[key]; !ok || v != val {
			return false
		}
	}
	return true
}
