package queue

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	queueFixture := func(inflight, total int64) *Queue {
		q := &Queue{}
		q.inProgressCount.Add(inflight)
		q.totalCount.Add(total)
		return q
	}
	specs := map[string]struct {
		setup      func() *Manager
		expMetrics string
	}{
		"single queues": {
			setup: func() *Manager {
				return &Manager{
					queues: map[string]*Queue{"my_model": queueFixture(1, 2)},
				}
			},
			expMetrics: `
# HELP requests_queue Number of request queued
# TYPE requests_queue gauge
requests_queue{deployment="my_model"} 2
# HELP requests_queue_inflight Number of requests in flight
# TYPE requests_queue_inflight gauge
requests_queue_inflight{deployment="my_model"} 1
`,
		},
		"multiple queues": {
			setup: func() *Manager {
				return &Manager{
					queues: map[string]*Queue{
						"my_model":       queueFixture(1, 2),
						"my_other_model": queueFixture(3, 4),
					},
				}
			},
			expMetrics: `
# HELP requests_queue Number of request queued
# TYPE requests_queue gauge
requests_queue{deployment="my_model"} 2
requests_queue{deployment="my_other_model"} 4
# HELP requests_queue_inflight Number of requests in flight
# TYPE requests_queue_inflight gauge
requests_queue_inflight{deployment="my_model"} 1
requests_queue_inflight{deployment="my_other_model"} 3
`,
		},
		"empty queues": {
			setup: func() *Manager {
				return &Manager{queues: map[string]*Queue{}}
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
	q := &Queue{}
	q.inProgressCount.Add(1)
	q.totalCount.Add(2)
	manager := &Manager{queues: map[string]*Queue{"my_model": q}}

	NewMetricsCollector(manager).MustRegister(registry)

	problems, err := testutil.GatherAndLint(registry)
	require.NoError(t, err)
	require.Empty(t, problems)
}
