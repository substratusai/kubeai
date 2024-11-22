package modelcontroller

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
)

func (r *ModelReconciler) execPod(ctx context.Context, pod *corev1.Pod, container string, command []string) error {
	execReq := r.PodRESTClient.
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(r.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(r.RESTConfig, "POST", execReq.URL())
	if err != nil {
		return fmt.Errorf("creating remote command executor: %w", err)
	}

	if err := exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	}); err != nil {
		return fmt.Errorf("streaming: %w", err)
	}

	return nil
}

func (r *ModelReconciler) updatePodRemoveLabel(ctx context.Context, pod *corev1.Pod, key string) error {
	if pod.Labels == nil {
		return nil
	}
	delete(pod.Labels, key)
	if err := r.Client.Update(ctx, pod); err != nil {
		return fmt.Errorf("update pod labels: %w", err)
	}
	return nil
}

func (r *ModelReconciler) updatePodAddLabel(ctx context.Context, pod *corev1.Pod, key, value string) error {
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[key] = value
	if err := r.Client.Update(ctx, pod); err != nil {
		return fmt.Errorf("update pod labels: %w", err)
	}
	return nil
}

func (r *ModelReconciler) envAuthForSource(s modelSource) []corev1.EnvVar {
	switch s.typ {
	case modelSourceTypeS3:
		return r.envAuthForS3()
	case modelSourceTypeHuggingface:
		return r.envAuthForHuggingface()
	default:
		return []corev1.EnvVar{}
	}
}

func (r *ModelReconciler) envAuthAll() []corev1.EnvVar {
	return append(r.envAuthForS3(), r.envAuthForHuggingface()...)
}

func (r *ModelReconciler) envAuthForS3() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.SecretNames.AWS,
					},
					Key:      "accessKeyId",
					Optional: ptr.To(true),
				},
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.SecretNames.AWS,
					},
					Key:      "secretAccessKey",
					Optional: ptr.To(true),
				},
			},
		},
	}
}

func (r *ModelReconciler) envAuthForHuggingface() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "HF_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.SecretNames.Huggingface,
					},
					Key:      "token",
					Optional: ptr.To(true),
				},
			},
		},
	}
}
