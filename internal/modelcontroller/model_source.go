package modelcontroller

import (
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type modelSource struct {
	*modelAuthCredentials
	url *url.URL
}

// ref returns a short reference to the model source.
// If url is "hf://username/model", then ref is "username/model".
func (s modelSource) ref() string {
	return s.url.Host + s.url.Path
}

func (r *ModelReconciler) parseModelSource(urlStr string) (modelSource, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return modelSource{}, err
	}
	src := modelSource{
		url: u,
	}

	switch {
	case u.Scheme == "gs":
		src.modelAuthCredentials = r.authForGCS()
	case u.Scheme == "oss":
		src.modelAuthCredentials = r.authForOSS()
	case u.Scheme == "s3":
		src.modelAuthCredentials = r.authForS3()
	case u.Scheme == "hf":
		src.modelAuthCredentials = r.authForHuggingfaceHub()
	case u.Scheme == "ollama":
		src.modelAuthCredentials = &modelAuthCredentials{}
	}
	return src, nil
}

type modelAuthCredentials struct {
	env          []corev1.EnvVar
	volumes      []corev1.Volume
	volumeMounts []corev1.VolumeMount
}

func (c *modelAuthCredentials) append(other *modelAuthCredentials) {
	c.env = append(c.env, other.env...)
	c.volumes = append(c.volumes, other.volumes...)
	c.volumeMounts = append(c.volumeMounts, other.volumeMounts...)
}

func (c *modelAuthCredentials) applyToPodSpec(spec *corev1.PodSpec, containerIndex int) {
	spec.Containers[containerIndex].Env = append(spec.Containers[containerIndex].Env, c.env...)
	spec.Volumes = append(spec.Volumes, c.volumes...)
	spec.Containers[containerIndex].VolumeMounts = append(spec.Containers[containerIndex].VolumeMounts, c.volumeMounts...)
}

func (r *ModelReconciler) modelAuthCredentialsForAllSources() *modelAuthCredentials {
	c := &modelAuthCredentials{}
	c.append(r.authForHuggingfaceHub())
	c.append(r.authForGCS())
	c.append(r.authForOSS())
	c.append(r.authForS3())
	return c
}

func (r *ModelReconciler) authForS3() *modelAuthCredentials {
	return &modelAuthCredentials{
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

func (r *ModelReconciler) authForHuggingfaceHub() *modelAuthCredentials {
	return &modelAuthCredentials{
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

func (r *ModelReconciler) authForGCS() *modelAuthCredentials {
	const (
		credentialsDir      = "/secrets/gcp-credentials"
		credentialsFilename = "credentials.json"
		credentialsPath     = credentialsDir + "/" + credentialsFilename
		volumeName          = "gcp-credentials"
	)
	return &modelAuthCredentials{
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

func (r *ModelReconciler) authForOSS() *modelAuthCredentials {
	return &modelAuthCredentials{
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
