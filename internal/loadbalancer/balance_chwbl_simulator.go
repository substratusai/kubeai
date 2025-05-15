package loadbalancer

import (
	"fmt"
	v1 "github.com/substratusai/kubeai/api/k8s/v1"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// SimulateCHWBL simulates the distribution of requests using the CHWBL algorithm
// Parameters:
// - numNodes: total number of nodes
// - loadDistribution: 1.0 is totally random, 0.0 is all the same request key
// - loadFactor: common ranges from 0.8 to 2.0
// - numRequests: number of requests to simulate
func SimulateCHWBL(numNodes, numRequests, totalInFlights int, loadDistribution, loadFactor float64) SimulationResult {
	// Create a new group with the default replication factor
	g := newEndpointGroup(v1.LoadBalancing{
		PrefixHash: v1.PrefixHash{
			Replication: 10, // Default replication factor
		},
	})

	// Add the specified number of endpoints
	for i := 0; i < numNodes; i++ {
		name := fmt.Sprintf("endpoint-%d", i)
		address := fmt.Sprintf("http://endpoint-%05d:8080", i)

		// Add the endpoint to the group
		g.endpoints[name] = endpoint{
			address:  address,
			inFlight: &atomic.Int64{},
			adapters: map[string]struct{}{"default": {}},
		}

		// Add the endpoint to the hash ring
		g.chwblAddEndpoint(name)
	}

	// Generate keys with the specified randomness
	keys := generateKeys(numRequests, loadDistribution)

	// Track the distribution of requests
	endpointCounts := make(map[string]int)

	g.totalInFlight.Add(int64(totalInFlights))

	// Route requests using the CHWBL algorithm
	for _, key := range keys {
		ep, found := g.chwblGetAddr(key, loadFactor, "default")
		if !found {
			panic(fmt.Sprintf("endpoint not found for key %s", key))
		}
		ep.inFlight.Add(1)
		go func() {
			// tweak the sleep time
			time.Sleep(time.Duration(rand.Intn(int(10 * time.Millisecond))))
			ep.inFlight.Add(-1)
		}()
		// Extract the endpoint name from the address
		if parts := strings.Split(ep.address, "://"); len(parts) > 1 {
			if parts = strings.Split(parts[1], ":"); len(parts) > 0 {
				endpointCounts[parts[0]]++
			}
		}
	}

	return SimulationResult{
		EndpointCounts: endpointCounts,
		TotalRequests:  numRequests,
	}
}

// generateKeys generates a list of keys with the specified randomness
// loadDistribution: 1.0 is totally random, 0.0 is all the same request key
func generateKeys(numKeys int, loadDistribution float64) []string {
	if loadDistribution < 0 {
		loadDistribution = 0
	}
	if loadDistribution > 1 {
		loadDistribution = 1
	}

	keys := make([]string, numKeys)

	// If loadDistribution is 0, all keys are the same
	if loadDistribution == 0 {
		for i := range keys {
			keys[i] = "same-key"
		}
		return keys
	}
	rnd := rand.New(rand.NewSource(1)) // deterministic behaviour
	for i := range keys {
		if rnd.Float64() <= loadDistribution {
			keys[i] = fmt.Sprintf("key-%d", rnd.Intn(math.MaxInt64))
		} else {
			keys[i] = "fixed-key"
		}
	}

	return keys
}

// SimulationResult represents the result of a simulation run
type SimulationResult struct {
	EndpointCounts map[string]int
	TotalRequests  int
}

// String returns a formatted string representation of the simulation results
func (r SimulationResult) String() string {
	type endpointCount struct {
		Name  string
		Count int
	}

	counts := make([]endpointCount, 0, len(r.EndpointCounts))
	for name, count := range r.EndpointCounts {
		counts = append(counts, endpointCount{Name: name, Count: count})
	}

	// Sort by count in descending order
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Count > counts[j].Count
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total requests: %d\n\n", r.TotalRequests))
	sb.WriteString("Distribution of requests to endpoints:\n")
	sb.WriteString("Endpoint\tCount\tPercentage\n")
	sb.WriteString("--------\t-----\t----------\n")

	for _, c := range counts {
		percentage := float64(c.Count) / float64(r.TotalRequests) * 100
		sb.WriteString(fmt.Sprintf("%s\t%d\t%.2f%%\n", c.Name, c.Count, percentage))
	}

	return sb.String()
}
