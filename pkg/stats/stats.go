package stats

// Stats about the state of Lingo, used for autoscaling and exposed via each
// lingo instance.
type Stats struct {
	ActiveRequests map[string]int64 `json:"activeRequests"`
}
