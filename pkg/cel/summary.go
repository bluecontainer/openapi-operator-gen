package cel

// Summary category constants for consistent usage across the codebase.
const (
	CategorySynced  = "synced"
	CategoryFailed  = "failed"
	CategoryPending = "pending"
	CategorySkipped = "skipped"
)

// SuccessStates defines resource states that count as "synced" in summaries.
// This includes:
//   - "Synced": CRUD resources successfully synchronized
//   - "Observed": CRUD resources observed (read-only mode)
//   - "Queried": Query CRDs successfully queried
//   - "Completed": Action CRDs successfully completed
var SuccessStates = map[string]bool{
	"Synced":    true,
	"Observed":  true,
	"Queried":   true,
	"Completed": true,
}

// ClassifyState returns the summary category for a resource state.
// Returns one of: "synced", "failed", "skipped", or "pending" (default).
func ClassifyState(state string) string {
	if SuccessStates[state] {
		return CategorySynced
	}
	switch state {
	case "Failed":
		return CategoryFailed
	case "Skipped":
		return CategorySkipped
	default:
		return CategoryPending
	}
}

// SummaryCounter tracks resource state counts for summary calculation.
type SummaryCounter struct {
	Total   int64
	Synced  int64
	Failed  int64
	Pending int64
	Skipped int64
}

// Add increments the appropriate counter based on the resource state.
func (s *SummaryCounter) Add(state string) {
	s.Total++
	switch ClassifyState(state) {
	case CategorySynced:
		s.Synced++
	case CategoryFailed:
		s.Failed++
	case CategorySkipped:
		s.Skipped++
	default:
		s.Pending++
	}
}

// ToMap converts the counter to a map suitable for CEL evaluation.
func (s *SummaryCounter) ToMap() map[string]int64 {
	return map[string]int64{
		"total":   s.Total,
		"synced":  s.Synced,
		"failed":  s.Failed,
		"pending": s.Pending,
		"skipped": s.Skipped,
	}
}

// ToMapWithoutSkipped converts the counter to a map without the skipped field.
// Use this for Aggregate CRDs which don't support skipped resources.
func (s *SummaryCounter) ToMapWithoutSkipped() map[string]int64 {
	return map[string]int64{
		"total":   s.Total,
		"synced":  s.Synced,
		"failed":  s.Failed,
		"pending": s.Pending,
	}
}

// CountStates aggregates a slice of states into summary counts.
func CountStates(states []string) map[string]int64 {
	var counter SummaryCounter
	for _, state := range states {
		counter.Add(state)
	}
	return counter.ToMap()
}

// IsSuccessState returns true if the state is considered a success state.
func IsSuccessState(state string) bool {
	return SuccessStates[state]
}
