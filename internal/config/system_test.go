package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/substratusai/kubeai/internal/config"
)

func TestAutoscalingConfig(t *testing.T) {
	cases := []struct {
		name                                  string
		cfg                                   config.Autoscaling
		expectedRequiredConsecutiveScaleDowns int
		expectedAverageWindowCount            int
	}{
		{
			name: "default",
			cfg: config.Autoscaling{
				Interval:       config.Duration{Duration: 10 * time.Second},
				TimeWindow:     config.Duration{Duration: 3 * time.Minute},
				ScaleDownDelay: config.Duration{Duration: 3 * time.Minute},
			},
			// (3mins) * (60sec/min) / (10secInterval) = 18
			expectedRequiredConsecutiveScaleDowns: 18,
			// (3mins) * (60sec/min) / (10secInterval) = 18
			expectedAverageWindowCount: 18,
		},
		{
			name: "even",
			cfg: config.Autoscaling{
				Interval:       config.Duration{Duration: 1 * time.Second},
				TimeWindow:     config.Duration{Duration: 10 * time.Second},
				ScaleDownDelay: config.Duration{Duration: 10 * time.Second},
			},
			expectedRequiredConsecutiveScaleDowns: 10,
			expectedAverageWindowCount:            10,
		},
		{
			name: "with-remainder",
			cfg: config.Autoscaling{
				Interval:       config.Duration{Duration: 2 * time.Second},
				TimeWindow:     config.Duration{Duration: 5 * time.Second},
				ScaleDownDelay: config.Duration{Duration: 3 * time.Second},
			},
			expectedRequiredConsecutiveScaleDowns: 2,
			expectedAverageWindowCount:            3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expectedRequiredConsecutiveScaleDowns, c.cfg.RequiredConsecutiveScaleDowns())
		})
	}
}
