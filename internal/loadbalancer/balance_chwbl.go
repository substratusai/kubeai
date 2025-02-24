package loadbalancer

import (
	"context"
	"fmt"
	"sort"

	"github.com/cespare/xxhash"
	"github.com/substratusai/kubeai/internal/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (g *group) chwblGetAddr(key string, loadFactor float64, adapter string) (endpoint, bool) {
	if len(g.chwblHashes) == 0 {
		return endpoint{}, false
	}

	h := chwblHash(key)
	hash0, i0 := g.chwblSearch(h)
	//log.Printf("hash: %v, i0: %v", h, i0)

	{
		name0 := g.chwblHashes[hash0]
		metrics.InferenceRequestsHashLookupInitial.Add(context.Background(), 1, metric.WithAttributeSet(attribute.NewSet(
			metrics.AttrEndpoint.String(name0),
		)))
	}

	// The default endpoint is the endpoint that is able to serve the request (has the adapter)
	// but might not meet the load requirement after all other endpoints have been checked.
	var defaultEndpoint *endpoint

	i := i0
	// Avoid an infinite loop by checking if we've checked all the endpoints.
	var defaultEndpointName string
	for n := 0; n < len(g.chwblSortedHashes); n++ {
		name := g.chwblHashes[g.chwblSortedHashes[i]]
		ep, ok := g.endpoints[name]
		if !ok {
			panic(fmt.Sprintf("endpoints corrupted, %q should be in map", name))
		}

		var adapterMatches bool
		if adapter == "" {
			adapterMatches = true
		} else {
			_, adapterMatches = ep.adapters[adapter]
		}

		if adapterMatches {
			if defaultEndpoint == nil {
				// Save the first endpoint that has the adapter in case no
				// endpoint is found with acceptable load.
				defaultEndpoint = &ep
				defaultEndpointName = name
			}
			if chwblLoadOK(ep.inFlight.Load(), g.totalInFlight.Load(), len(g.endpoints), loadFactor) {
				metrics.InferenceRequestsHashLookupIterations.Record(context.Background(), int64(n+1))
				metrics.InferenceRequestsHashLookupFinal.Add(context.Background(), 1, metric.WithAttributeSet(attribute.NewSet(
					metrics.AttrEndpoint.String(name),
				)))
				return ep, true
			}
		}

		i++
		if i >= len(g.chwblSortedHashes) {
			// wrap around
			i = 0
		}
	}

	if defaultEndpoint != nil {
		metrics.InferenceRequestsHashLookupIterations.Record(context.Background(), int64(len(g.chwblSortedHashes)))
		metrics.InferenceRequestsHashLookupFinal.Add(context.Background(), 1, metric.WithAttributeSet(attribute.NewSet(
			metrics.AttrEndpoint.String(defaultEndpointName),
		)))
		metrics.InferenceRequestsHashLookupDefault.Add(context.Background(), 1, metric.WithAttributeSet(attribute.NewSet(
			metrics.AttrEndpoint.String(defaultEndpointName),
		)))
		return *defaultEndpoint, true
	}
	return endpoint{}, false
}

func (g *group) chwblAddEndpoint(name string) {
	for i := 0; i < g.chwblReplication; i++ {
		h := chwblHashEndpointReplica(name, i)
		g.chwblHashes[h] = name
		g.chwblSortedHashes = append(g.chwblSortedHashes, h)
	}

	// sort hashes in ascending order
	sort.Slice(g.chwblSortedHashes, func(i int, j int) bool {
		return g.chwblSortedHashes[i] < g.chwblSortedHashes[j]
	})
}

func (g *group) chwblRemoveEndpoint(name string) {
	for i := 0; i < g.chwblReplication; i++ {
		h := chwblHashEndpointReplica(name, i)
		delete(g.chwblHashes, h)
		g.chwblDeleteSortedHash(h)
	}
}

// search returns the hash values and its index.
func (g *group) chwblSearch(key uint64) (uint64, int) {
	idx := sort.Search(len(g.chwblSortedHashes), func(i int) bool {
		return g.chwblSortedHashes[i] >= key
	})

	if idx >= len(g.chwblSortedHashes) {
		idx = 0
	}
	return g.chwblSortedHashes[idx], idx
}

func (g *group) chwblDeleteSortedHash(val uint64) {
	idx := -1
	left := 0
	right := len(g.chwblSortedHashes) - 1
	for left <= right {
		middle := (left + right) / 2
		current := g.chwblSortedHashes[middle]
		if current == val {
			idx = middle
			break
		} else if current < val {
			left = middle + 1
		} else if current > val {
			right = middle - 1
		}
	}
	if idx != -1 {
		g.chwblSortedHashes = append(g.chwblSortedHashes[:idx], g.chwblSortedHashes[idx+1:]...)
	}
}

func chwblHash(s string) uint64 {
	return xxhash.Sum64([]byte(s))
}

func chwblHashEndpointReplica(name string, replica int) uint64 {
	return chwblHash(chwblEndpointReplicaHashInput(name, replica))
}

func chwblEndpointReplicaHashInput(name string, replica int) string {
	return fmt.Sprintf("%s%d", name, replica)
}

func chwblLoadOK(load, totalLoad int64, n int, loadFactor float64) bool {
	if totalLoad == 0 {
		return true
	}

	// The "+1"s are to simulate the load of the new request.
	avgLoad := float64(totalLoad+1) / float64(n)
	threshold := avgLoad * loadFactor
	ok := float64(load)+1 <= threshold
	return ok
}
