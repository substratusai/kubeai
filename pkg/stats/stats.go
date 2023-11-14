package stats

type Stats struct {
	WaitCounts map[string]int64 `json:"waitCounts"`
}
