package command

import (
	"fmt"
	"os"

	"github.com/substratusai/kubeai/internal/config"
	"sigs.k8s.io/yaml"
)

func LoadConfigFile(path string) (config.System, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return config.System{}, err
	}
	var cfg config.System
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return config.System{}, err
	}

	if err := cfg.DefaultAndValidate(); err != nil {
		return config.System{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}
