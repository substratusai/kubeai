package main

import (
	"flag"
	"fmt"
	"github.com/substratusai/kubeai/internal/loadbalancer"
	"github.com/substratusai/kubeai/internal/metrics"
	"go.opentelemetry.io/otel"
	"os"
	"time"
)

func main() {
	// Define command-line flags
	numNodes := flag.Int("nodes", 300, "Total number of nodes")
	numRequests := flag.Int("requests", 15_000, "Number of requests to simulate")
	loadDistribution := flag.Float64("distribution", 0.75, "Load distribution (1.0 = totally random, 0.0 = all the same request key)")
	loadFactor := flag.Float64("loadFactor", 2., "Load factor (common ranges from 0.8 to 2.0)")
	totalInFlights := flag.Int("totalInFlights", 100., "total number of parallel requests to the ring")

	// Parse command-line flags
	flag.Parse()

	// Validate input parameters
	if *numNodes <= 0 {
		fmt.Println("Error: Number of nodes must be greater than 0")
		os.Exit(1)
	}
	if *numRequests <= 0 {
		fmt.Println("Error: Number of requests must be greater than 0")
		os.Exit(1)
	}
	if *totalInFlights <= 0 {
		fmt.Println("Error: Number of total in flight requests must be greater than 0")
		os.Exit(1)
	}

	if *loadDistribution < 0 || *loadDistribution > 1 {
		fmt.Println("Error: Load distribution must be between 0.0 and 1.0")
		os.Exit(1)
	}
	if *loadFactor <= 0 {
		fmt.Println("Error: Load factor must be greater than 0")
		os.Exit(1)
	}
	if initErr := metrics.Init(otel.Meter(metrics.MeterName)); initErr != nil {
		fmt.Println("Error: Failed to initialize metrics")
		os.Exit(1)
	}

	fmt.Println("CHWBL Load Balancer Simulator")
	fmt.Println("============================")
	fmt.Printf("Number of nodes: %d\n", *numNodes)
	fmt.Printf("Number of requests: %d\n", *numRequests)
	fmt.Printf("Load distribution: %.2f (1.0 = totally random, 0.0 = all the same request key)\n", *loadDistribution)
	fmt.Printf("Load factor: %.2f\n", *loadFactor)
	fmt.Println()

	start := time.Now()
	result := loadbalancer.SimulateCHWBL(*numNodes, *numRequests, *totalInFlights, *loadDistribution, *loadFactor)
	fmt.Println(result.String())
	fmt.Printf("Total time: %s", time.Since(start))
}
