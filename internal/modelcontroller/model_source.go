package modelcontroller

import (
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type modelSource struct {
	*modelSourcePodAdditions
	url modelURL
}

func (r *ModelReconciler) parseModelSource(urlStr string) (modelSource, error) {
	u, err := parseModelURL(urlStr)
	if err != nil {
		return modelSource{}, err
	}
	src := modelSource{
		url: u,
	}

	switch {
	case u.scheme == "gs":
		src.modelSourcePodAdditions = r.authForGCS()
	case u.scheme == "oss":
		src.modelSourcePodAdditions = r.authForOSS()
	case u.scheme == "s3":
		src.modelSourcePodAdditions = r.authForS3()
	case u.scheme == "hf":
		src.modelSourcePodAdditions = r.authForHuggingfaceHub()
	case u.scheme == "pvc":
		src.modelSourcePodAdditions = r.pvcPodAdditions(u)
	default:
		src.modelSourcePodAdditions = &modelSourcePodAdditions{}
	}
	return src, nil
}

type modelSourcePodAdditions struct {
	env          []corev1.EnvVar
	volumes      []corev1.Volume
	volumeMounts []corev1.VolumeMount
}

func (c *modelSourcePodAdditions) append(other *modelSourcePodAdditions) {
	c.env = append(c.env, other.env...)
	c.volumes = append(c.volumes, other.volumes...)
	c.volumeMounts = append(c.volumeMounts, other.volumeMounts...)
}

func (c *modelSourcePodAdditions) applyToPodSpec(spec *corev1.PodSpec, containerIndex int) {
	spec.Containers[containerIndex].Env = append(spec.Containers[containerIndex].Env, c.env...)
	spec.Volumes = append(spec.Volumes, c.volumes...)
	spec.Containers[containerIndex].VolumeMounts = append(spec.Containers[containerIndex].VolumeMounts, c.volumeMounts...)
}

func (r *ModelReconciler) modelAuthCredentialsForAllSources() *modelSourcePodAdditions {
	c := &modelSourcePodAdditions{}
	c.append(r.authForHuggingfaceHub())
	c.append(r.authForGCS())
	c.append(r.authForOSS())
	c.append(r.authForS3())
	return c
}

func (r *ModelReconciler) authForS3() *modelSourcePodAdditions {
	return &modelSourcePodAdditions{
		env: []corev1.EnvVar{
			{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: r.SecretNames.AWS,
						},
						Key:      "accessKeyID",
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
		},
	}
}

func (r *ModelReconciler) authForHuggingfaceHub() *modelSourcePodAdditions {
	return &modelSourcePodAdditions{
		env: []corev1.EnvVar{
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
		},
	}
}

func (r *ModelReconciler) authForGCS() *modelSourcePodAdditions {
	const (
		credentialsDir      = "/secrets/gcp-credentials"
		credentialsFilename = "credentials.json"
		credentialsPath     = credentialsDir + "/" + credentialsFilename
		volumeName          = "gcp-credentials"
	)
	return &modelSourcePodAdditions{
		env: []corev1.EnvVar{
			{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: credentialsPath,
			},
		},
		volumes: []corev1.Volume{
			{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: r.SecretNames.GCP,
						Items: []corev1.KeyToPath{
							{
								Key:  "jsonKeyfile",
								Path: credentialsFilename,
							},
						},
						Optional: ptr.To(true),
					},
				},
			},
		},
		volumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeName,
				MountPath: credentialsDir,
			},
		},
	}
}

func (r *ModelReconciler) authForOSS() *modelSourcePodAdditions {
	return &modelSourcePodAdditions{
		env: []corev1.EnvVar{
			{
				Name: "OSS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: r.SecretNames.Alibaba,
						},
						Key:      "accessKeyID",
						Optional: ptr.To(true),
					},
				},
			},
			{
				Name: "OSS_ACCESS_KEY_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: r.SecretNames.Alibaba,
						},
						Key:      "accessKeySecret",
						Optional: ptr.To(true),
					},
				},
			},
		},
	}
}

func (r *ModelReconciler) pvcPodAdditions(url modelURL) *modelSourcePodAdditions {
	volumeName := "model"
	// Kubernetes does not support an subPath with a leading slash. SubPath needs to be
	// a relative path or empty string to mount the entire volume.
	path := strings.TrimLeft(url.path, "/")
	return &modelSourcePodAdditions{
		volumes: []corev1.Volume{
			{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: url.name,
						ReadOnly:  true,
					},
				},
			},
		},
		volumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeName,
				MountPath: "/model",
				SubPath:   path,
			},
		},
	}
}

var modelURLRegex = regexp.MustCompile(`^([a-z]+):\/\/(\S+)$`)

func parseModelURL(urlStr string) (modelURL, error) {
	matches := modelURLRegex.FindStringSubmatch(urlStr)
	if len(matches) != 3 {
		return modelURL{}, fmt.Errorf("invalid model URL: %s", urlStr)
	}
	scheme, ref := matches[1], matches[2]
	name, path, _ := strings.Cut(ref, "/")
	return modelURL{
		original: urlStr,
		scheme:   scheme,
		ref:      ref,
		name:     name,
		path:     path,
	}, nil
}

type modelURL struct {
	original string // e.g. "hf://username/model"
	scheme   string // e.g. "hf", "s3", "gs", "oss"
	ref      string // e.g. "username/model"
	name     string // e.g. username or bucket-name
	path     string // e.g. model or path/to/model
}
