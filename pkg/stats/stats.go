package stats

// Stats about the state of Lingo, used for autoscaling and exposed via each
// lingo instance.
type Stats struct {
	// ActiveRequests maps deployment names to the number of active requests
	ActiveRequests map[string]int64 `json:"activeRequests"`
}
