package endpoints

import (
	"fmt"
	"sort"

	"github.com/cespare/xxhash"
)

type Hasher interface {
	Sum64([]byte) uint64
}

type chwblConfig struct {
	LoadFactor  float64
	Replication int
}

func newCHWBL(cfg chwblConfig) *chwbl {
	if cfg.LoadFactor == 0 {
		cfg.LoadFactor = 1.25
	}
	if cfg.Replication == 0 {
		cfg.Replication = 100
	}
	return &chwbl{
		cfg: cfg,

		hashes:       map[uint64]string{},
		sortedHashes: []uint64{},
	}
}

type chwbl struct {
	cfg chwblConfig

	// hashToEndpoint is a map of hash to endpoint
	hashes map[uint64]string
	// sortedHashes is a sorted list of hashed node-replicas
	sortedHashes []uint64
}

func (r *chwbl) addEndpoint(name string) {
	for i := 0; i < r.cfg.Replication; i++ {
		h := r.hash(fmt.Sprintf("%s%d", name, i))
		r.hashes[h] = name
		r.sortedHashes = append(r.sortedHashes, h)
	}

	// sort hashes in ascending order
	sort.Slice(r.sortedHashes, func(i int, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})
}

func (r *chwbl) getAddr(key string) (endpoint, bool) {
	if len(r.hashes) == 0 {
		return "", false
	}

	h := r.hash(key)
	_, idx := r.search(h)

	i := idx
	// Avoid an infinite loop by checking if we've checked all the nodes.
	for n := 0; n < len(r.hashes); n++ {
		node := r.hashes[r.sortedHashes[i]]
		if r.loadOK(node) {
			return node, true
		}
		i++
		if i >= len(r.hashes) {
			// wrap around
			i = 0
		}
	}

	return "", false
}

// search returns the hash values and its index.
func (r *chwbl) search(key uint64) (uint64, int) {
	idx := sort.Search(len(r.sortedHashes), func(i int) bool {
		return r.sortedHashes[i] >= key
	})

	if idx >= len(r.sortedHashes) {
		idx = 0
	}
	return r.sortedHashes[idx], idx
}

func (r *chwbl) UpdateLoad(node string, load int64) {
	if _, ok := r.loads[node]; !ok {
		return
	}

	r.totalLoad -= r.loads[node]
	r.loads[node] = load
	r.totalLoad += load

	if r.loads[node] < 0 {
		panic(fmt.Sprintf("load for node %q should not be negative", node))
	}
	if r.totalLoad < 0 {
		panic(fmt.Sprintf("total load should not be negative after updating load for node %q", node))
	}
}

// Decrements the load of node by 1
func (r *chwbl) Done(node string) {
	if _, ok := r.loads[node]; !ok {
		return
	}
	r.loads[node]--
	r.totalLoad--

	if r.loads[node] < 0 {
		panic(fmt.Sprintf("load for node %q should not be negative", node))
	}
	if r.totalLoad < 0 {
		panic(fmt.Sprintf("total load should not be negative after decrementing node %q", node))
	}
}

// Deletes node from the ring
func (r *chwbl) Remove(node string) bool {
	for i := 0; i < r.cfg.Replication; i++ {
		h := r.hash(fmt.Sprintf("%s%d", node, i))
		delete(r.hashes, h)
		r.deleteSortedHash(h)
	}
	r.totalLoad -= r.loads[node]
	delete(r.loads, node)
	return true
}

func (r *chwbl) loadOK(node string) bool {
	if r.totalLoad == 0 {
		return true
	}

	load, ok := r.loads[node]
	if !ok {
		panic(fmt.Sprintf("given node %q not in loads", node))
	}

	avgLoad := float64(r.totalLoad+1) / float64(len(r.loads))
	threshold := avgLoad * r.cfg.LoadFactor

	return float64(load)+1 <= threshold
}

func (r *chwbl) deleteSortedHash(val uint64) {
	idx := -1
	left := 0
	right := len(r.sortedHashes) - 1
	for left <= right {
		middle := (left + right) / 2
		if r.sortedHashes[middle] == val {
			idx = middle
			break
		} else if r.sortedHashes[middle] < val {
			left = middle + 1
		} else if r.sortedHashes[middle] > val {
			right = middle - 1
		}
	}
	if idx != -1 {
		r.sortedHashes = append(r.sortedHashes[:idx], r.sortedHashes[idx+1:]...)
	}
}

func (r *chwbl) hash(s string) uint64 {
	return xxhash.Sum64([]byte(s))
}
