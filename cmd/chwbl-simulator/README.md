# CHWBL Load Balancer Simulator

This simulator tests the distribution of requests in the Consistent Hashing with Bounded Loads (CHWBL) algorithm used by the load balancer.

## Overview

The CHWBL algorithm is a load balancing strategy that combines consistent hashing with bounded loads. It provides a good balance between:
- **Consistency**: Similar requests are routed to the same endpoint
- **Load distribution**: Prevents any single endpoint from being overloaded

This simulator helps you understand how requests are distributed across endpoints with different configurations.

## Usage

To run the simulator:

```bash
go run cmd/chwbl-simulator/main.go [flags]
```

### Flags

- `--nodes`: Total number of nodes (default: 300)
- `--requests`: Number of requests to simulate (default: 10000)
- `--distribution`: Load distribution (1.0 = totally random, 0.0 = all the same request key) (default: 0.75)
- `--loadFactor`: Load factor (common ranges from 0.8 to 2.0) (default: 2.0)

### Examples

1. Basic usage with default parameters:
   ```bash
   go run cmd/chwbl-simulator/main.go
   ```

2. Simulate 10 nodes with 5000 requests:
   ```bash
   go run cmd/chwbl-simulator/main.go --nodes=10 --requests=5000
   ```

3. Test with all requests having the same key:
   ```bash
   go run cmd/chwbl-simulator/main.go --distribution=0.0
   ```

4. Test with a higher load factor:
   ```bash
   go run cmd/chwbl-simulator/main.go --loadFactor=1.5
   ```

## Understanding the Results

The simulator outputs:
1. The total number of requests
2. The distribution of requests to endpoints, including:
   - The number of requests routed to each endpoint
   - The percentage of total requests for each endpoint

The results are sorted by the number of requests in descending order.

## Parameters Explained

### Load Distribution

- **1.0 (totally random)**: Each request has a unique key, resulting in maximum distribution across endpoints
- **0.0 (all the same key)**: All requests have the same key, resulting in all requests going to the same endpoint
- **Values between 0.0 and 1.0**: A mix of random and fixed keys, with the proportion determined by the value

### Load Factor

The load factor controls how much an endpoint's load can exceed the average load:
- **1.0**: An endpoint's load cannot exceed the average load
- **2.0**: An endpoint's load can be up to twice the average load
- **Common range**: 0.8 to 2.0

A higher load factor allows more requests to be routed to the same endpoint, improving consistency at the cost of even distribution.