package config

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/go-playground/validator/v10"
	v1 "github.com/substratusai/kubeai/api/v1"
	corev1 "k8s.io/api/core/v1"
)

type System struct {
	SecretNames SecretNames `json:"secretNames" validate:"required"`

	ModelServers ModelServers `json:"modelServers" validate:"required"`

	ResourceProfiles map[string]ResourceProfile `json:"resourceProfiles" validate:"required"`

	Messaging Messaging `json:"messaging"`

	// MetricsAddr is the address the metric endpoint binds to.
	// Defaults to ":8080"
	MetricsAddr string `json:"metricsAddr" validate:"required"`

	// HealthAddr is the address the health probe endpoint binds to.
	// Defaults to ":8081"
	HealthAddress string `json:"healthAddress" validate:"required"`

	// AllowPodAddressOverride will allow the pod address to be overridden by the Model objects. This is useful for development purposes.
	AllowPodAddressOverride bool `json:"allowPodAddressOverride"`

	Autoscaling Autoscaling `json:"autoscaling" validate:"required"`
}

func (s *System) DefaultAndValidate() error {
	if s.MetricsAddr == "" {
		s.MetricsAddr = ":8080"
	}
	if s.HealthAddress == "" {
		s.HealthAddress = ":8081"
	}

	for i := range s.Messaging.Streams {
		if s.Messaging.Streams[i].MaxHandlers == 0 {
			s.Messaging.Streams[i].MaxHandlers = 1
		}
	}

	if _, ok := s.Autoscaling.Profiles["default"]; !ok {
		return errors.New("default autoscaling profile is required")
	}

	if s.Autoscaling.Interval.Duration == 0 {
		s.Autoscaling.Interval.Duration = 10 * time.Second
	}
	if s.Autoscaling.TimeWindow.Duration == 0 {
		s.Autoscaling.TimeWindow.Duration = 10 * time.Minute
	}

	for name, profile := range s.Autoscaling.Profiles {
		// NOTE: Keep in sync with CRD defaults in api pkg.
		// Those defaults are only applied by the API Server when a user
		// sets the .spec.autoscaling field directly (outside of using
		// an autoscaling profile).
		if profile.MaxReplicas == 0 {
			profile.MaxReplicas = 3
		}
		if profile.TargetRequests == 0 {
			profile.TargetRequests = 100
		}

		s.Autoscaling.Profiles[name] = profile
	}

	return validator.New(validator.WithRequiredStructEnabled()).Struct(s)
}

type Autoscaling struct {
	// Interval is the time between each autoscaling check.
	// Defaults to 10 seconds.
	Interval Duration `json:"interval" validate:"required"`
	// TimeWindow that the autoscaling algorithm will consider when
	// calculating the average number of requests.
	// Defaults to 10 minutes.
	TimeWindow Duration `json:"timeWindow" validate:"required"`
	// Profiles is a map of autoscaling profiles that can be used to configure
	// autoscaling on a model-by-model basis.
	// A "default" profile is required.
	Profiles map[string]v1.ModelAutoscaling `json:"profiles" validate:"required"`
}

// RequiredConsecutiveScaleDowns returns the number of consecutive scale down
// operations required before the deployment is scaled down. This is calculated
// by dividing the ScaleDownDelay by the Interval.
func (a *Autoscaling) RequiredConsecutiveScaleDowns(profile v1.ModelAutoscaling) int {
	return int(math.Ceil(float64(profile.ScaleDownDelay.Duration) / float64(a.Interval.Duration)))
}

// AverageWindowCount returns the number of intervals that will be considered when
// calculating the average value.
func (a *Autoscaling) AverageWindowCount() int {
	return int(math.Ceil(float64(a.TimeWindow.Duration) / float64(a.Interval.Duration)))
}

type SecretNames struct {
	Huggingface string `json:"huggingface" validate:"required"`
}

type Messaging struct {
	// ErrorMaxBackoff is the maximum backoff time that will be applied when
	// consecutive errors are encountered.
	ErrorMaxBackoff Duration        `json:"errorMaxBackoff"`
	Streams         []MessageStream `json:"streams"`
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

type ResourceProfile struct {
	ImageName    string              `json:"imageName"`
	Requests     corev1.ResourceList `json:"requests,omitempty"`
	Limits       corev1.ResourceList `json:"limits,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
}

type MessageStream struct {
	RequestsURL  string `json:"requestsURL"`
	ResponsesURL string `json:"responsesURL"`
	// MaxHandlers is the maximum number of handlers that will be started for this stream.
	// Must be greater than 0. Defaults to 1.
	MaxHandlers int `json:"maxHandlers" validate:"min=1"`
}

type ModelServers struct {
	OLlama        ModelServer `json:"OLlama"`
	VLLM          ModelServer `json:"VLLM"`
	FasterWhisper ModelServer `json:"FasterWhisper"`
}

type ModelServer struct {
	Images map[string]string `json:"images"`
}
