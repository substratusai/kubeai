package config

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	corev1 "k8s.io/api/core/v1"
)

type System struct {
	SecretNames SecretNames `json:"secretNames" validate:"required"`

	ModelServers ModelServers `json:"modelServers" validate:"required"`

	ResourceProfiles map[string]ResourceProfile `json:"resourceProfiles"`

	Messaging Messaging `json:"messaging"`

	// MetricsAddr is the address the metric endpoint binds to. Default is ":8080".
	MetricsAddr string `json:"metricsAddr"`

	// HealthAddr is the address the health probe endpoint binds to. Default is ":8081".
	HealthAddress string `json:"healthAddress"`

	// AllowPodAddressOverride will allow the pod address to be overridden by the Model objects. This is useful for development purposes.
	AllowPodAddressOverride bool `json:"allowPodAddressOverride"`
}

func (s *System) DefaultAndValidate() error {
	if s.Messaging.ErrorMaxBackoff.Duration == 0 {
		s.Messaging.ErrorMaxBackoff.Duration = time.Minute
	}
	if s.MetricsAddr == "" {
		s.MetricsAddr = ":8080"
	}
	if s.HealthAddress == "" {
		s.HealthAddress = ":8081"
	}
	return validator.New(validator.WithRequiredStructEnabled()).Struct(s)
}

type SecretNames struct {
	Huggingface string `json:"huggingface" validate:"required"`
}

type Messaging struct {
	// ErrorMaxBackoff is the maximum backoff time that will be applied when
	// consecutive errors are encountered. Default is 1 minute.
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
}

type MessageStream struct {
	RequestsURL  string `json:"requestsURL"`
	ResponsesURL string `json:"responsesURL"`
	MaxHandlers  int    `json:"maxHandlers"`
}

type ModelServers struct {
	OLlama        ModelServer `json:"OLlama"`
	VLLM          ModelServer `json:"VLLM"`
	FasterWhisper ModelServer `json:"FasterWhisper"`
}

type ModelServer struct {
	Images map[string]string `json:"images"`
}
