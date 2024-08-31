package config

import (
	"encoding/json"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
)

type System struct {
	SecretNames struct {
		Huggingface string `json:"huggingface"`
	} `json:"secretNames"`

	ModelServers ModelServers `json:"modelServers"`

	ResourceProfiles map[string]ResourceProfile `json:"resourceProfiles"`

	Messaging struct {
		ErrorMaxBackoff Duration        `json:"errorMaxBackoff"`
		Streams         []MessageStream `json:"streams"`
	} `json:"messaging"`
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
	OLlama ModelServer `json:"OLlama"`
	VLLM   ModelServer `json:"VLLM"`
}

type ModelServer struct {
	Images map[string]string `json:"images"`
}
