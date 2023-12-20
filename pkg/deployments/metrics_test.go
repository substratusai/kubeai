package deployments

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	specs := map[string]struct {
		setup      func() *Manager
		expMetrics string
	}{
		"single scaler": {
			setup: func() *Manager {
				return &Manager{
					scalers: map[string]*scaler{
						"my_model": {minScale: 1, currentScale: 2, maxScale: 3},
					},
				}
			},
			expMetrics: `
# HELP current_model_backends Number of model backends currently deployed
# TYPE current_model_backends gauge
current_model_backends{model="my_model"} 2
# HELP max_model_backends Min number of model backends to deploy
# TYPE max_model_backends gauge
max_model_backends{model="my_model"} 3
# HELP min_model_backends Max number of model backends to deploy
# TYPE min_model_backends gauge
min_model_backends{model="my_model"} 1
`,
		},
		"multiple scalers": {
			setup: func() *Manager {
				return &Manager{
					scalers: map[string]*scaler{
						"my_model":       {minScale: 1, currentScale: 2, maxScale: 3},
						"my_other_model": {minScale: 4, currentScale: 5, maxScale: 6},
					},
				}
			},
			expMetrics: `
# HELP current_model_backends Number of model backends currently deployed
# TYPE current_model_backends gauge
current_model_backends{model="my_model"} 2
current_model_backends{model="my_other_model"} 5
# HELP max_model_backends Min number of model backends to deploy
# TYPE max_model_backends gauge
max_model_backends{model="my_model"} 3
max_model_backends{model="my_other_model"} 6
# HELP min_model_backends Max number of model backends to deploy
# TYPE min_model_backends gauge
min_model_backends{model="my_model"} 1
min_model_backends{model="my_other_model"} 4
`,
		},
		"empty scalers": {
			setup: func() *Manager {
				return &Manager{
					scalers: map[string]*scaler{},
				}
			},
			expMetrics: ``,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			gotErr := testutil.CollectAndCompare(NewMetricsCollector(spec.setup()), strings.NewReader(spec.expMetrics))
			require.NoError(t, gotErr)
		})
	}
}

func TestMetricsViaLinter(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	manager := &Manager{
		scalers: map[string]*scaler{"my_model": {minScale: 1, currentScale: 2, maxScale: 3}},
	}
	NewMetricsCollector(manager).MustRegister(registry)

	problems, err := testutil.GatherAndLint(registry)
	require.NoError(t, err)
	require.Empty(t, problems)
}
