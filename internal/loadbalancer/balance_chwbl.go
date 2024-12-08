package loadbalancer

import (
	"fmt"
	"sort"

	"github.com/cespare/xxhash"
)

func (g *group) chwblGetAddr(key string, loadFactor float64) (endpoint, bool) {
	if len(g.chwblHashes) == 0 {
		return endpoint{}, false
	}

	h := g.chwblHash(key)
	_, idx := g.chwblSearch(h)

	i := idx
	// Avoid an infinite loop by checking if we've checked all the endpoints.
	for n := 0; n < len(g.chwblSortedHashes); n++ {
		name := g.chwblHashes[g.chwblSortedHashes[i]]
		ep, ok := g.endpoints[name]
		if !ok {
			continue
		}
		if chwblLoadOK(ep.inFlight.Load(), g.totalInFlight.Load(), len(g.endpoints), loadFactor) {
			return ep, true
		}
		i++
		if i >= len(g.chwblSortedHashes) {
			// wrap around
			i = 0
		}
	}

	return endpoint{}, false
}

func (g *group) chwblAddEndpoint(name string) {
	for i := 0; i < g.chwblReplication; i++ {
		h := g.chwblHashEndpointReplica(name, i)
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
		h := g.chwblHashEndpointReplica(name, i)
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
		if g.chwblSortedHashes[middle] == val {
			idx = middle
			break
		} else if g.chwblSortedHashes[middle] < val {
			left = middle + 1
		} else if g.chwblSortedHashes[middle] > val {
			right = middle - 1
		}
	}
	if idx != -1 {
		g.chwblSortedHashes = append(g.chwblSortedHashes[:idx], g.chwblSortedHashes[idx+1:]...)
	}
}

func (g *group) chwblHash(s string) uint64 {
	return xxhash.Sum64([]byte(s))
}

func (g *group) chwblHashEndpointReplica(name string, replica int) uint64 {
	return g.chwblHash(fmt.Sprintf("%s%d", name, replica))
}

func chwblLoadOK(load, totalLoad int64, n int, loadFactor float64) bool {
	if totalLoad == 0 {
		return true
	}

	avgLoad := float64(totalLoad+1) / float64(n)
	threshold := avgLoad * loadFactor

	return float64(load)+1 <= threshold
}
