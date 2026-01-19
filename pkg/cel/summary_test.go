package cel

import (
	"testing"
)

func TestClassifyState(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		// Success states
		{"Synced", CategorySynced},
		{"Observed", CategorySynced},
		{"Queried", CategorySynced},
		{"Completed", CategorySynced},
		// Failed state
		{"Failed", CategoryFailed},
		// Skipped state
		{"Skipped", CategorySkipped},
		// Pending states (explicit and default)
		{"Pending", CategoryPending},
		{"Creating", CategoryPending},
		{"Updating", CategoryPending},
		{"Unknown", CategoryPending},
		{"", CategoryPending},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := ClassifyState(tt.state)
			if got != tt.want {
				t.Errorf("ClassifyState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestIsSuccessState(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"Synced", true},
		{"Observed", true},
		{"Queried", true},
		{"Completed", true},
		{"Failed", false},
		{"Skipped", false},
		{"Pending", false},
		{"Unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := IsSuccessState(tt.state)
			if got != tt.want {
				t.Errorf("IsSuccessState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestSummaryCounter_Add(t *testing.T) {
	counter := &SummaryCounter{}

	// Add various states
	counter.Add("Synced")
	counter.Add("Synced")
	counter.Add("Observed")
	counter.Add("Queried")
	counter.Add("Completed")
	counter.Add("Failed")
	counter.Add("Skipped")
	counter.Add("Pending")
	counter.Add("Unknown")

	if counter.Total != 9 {
		t.Errorf("Total = %d, want 9", counter.Total)
	}
	if counter.Synced != 5 {
		t.Errorf("Synced = %d, want 5", counter.Synced)
	}
	if counter.Failed != 1 {
		t.Errorf("Failed = %d, want 1", counter.Failed)
	}
	if counter.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", counter.Skipped)
	}
	if counter.Pending != 2 {
		t.Errorf("Pending = %d, want 2", counter.Pending)
	}
}

func TestSummaryCounter_ToMap(t *testing.T) {
	counter := &SummaryCounter{
		Total:   10,
		Synced:  5,
		Failed:  2,
		Pending: 2,
		Skipped: 1,
	}

	got := counter.ToMap()

	if got["total"] != 10 {
		t.Errorf("total = %d, want 10", got["total"])
	}
	if got["synced"] != 5 {
		t.Errorf("synced = %d, want 5", got["synced"])
	}
	if got["failed"] != 2 {
		t.Errorf("failed = %d, want 2", got["failed"])
	}
	if got["pending"] != 2 {
		t.Errorf("pending = %d, want 2", got["pending"])
	}
	if got["skipped"] != 1 {
		t.Errorf("skipped = %d, want 1", got["skipped"])
	}
}

func TestSummaryCounter_ToMapWithoutSkipped(t *testing.T) {
	counter := &SummaryCounter{
		Total:   10,
		Synced:  5,
		Failed:  2,
		Pending: 2,
		Skipped: 1,
	}

	got := counter.ToMapWithoutSkipped()

	if _, hasSkipped := got["skipped"]; hasSkipped {
		t.Error("ToMapWithoutSkipped() should not include skipped key")
	}
	if got["total"] != 10 {
		t.Errorf("total = %d, want 10", got["total"])
	}
	if got["synced"] != 5 {
		t.Errorf("synced = %d, want 5", got["synced"])
	}
	if got["failed"] != 2 {
		t.Errorf("failed = %d, want 2", got["failed"])
	}
	if got["pending"] != 2 {
		t.Errorf("pending = %d, want 2", got["pending"])
	}
}

func TestCountStates(t *testing.T) {
	states := []string{
		"Synced", "Synced", "Observed",
		"Failed",
		"Skipped",
		"Pending", "Creating",
	}

	got := CountStates(states)

	if got["total"] != 7 {
		t.Errorf("total = %d, want 7", got["total"])
	}
	if got["synced"] != 3 {
		t.Errorf("synced = %d, want 3", got["synced"])
	}
	if got["failed"] != 1 {
		t.Errorf("failed = %d, want 1", got["failed"])
	}
	if got["skipped"] != 1 {
		t.Errorf("skipped = %d, want 1", got["skipped"])
	}
	if got["pending"] != 2 {
		t.Errorf("pending = %d, want 2", got["pending"])
	}
}

func TestCountStates_Empty(t *testing.T) {
	got := CountStates([]string{})

	if got["total"] != 0 {
		t.Errorf("total = %d, want 0", got["total"])
	}
	if got["synced"] != 0 {
		t.Errorf("synced = %d, want 0", got["synced"])
	}
	if got["failed"] != 0 {
		t.Errorf("failed = %d, want 0", got["failed"])
	}
	if got["pending"] != 0 {
		t.Errorf("pending = %d, want 0", got["pending"])
	}
	if got["skipped"] != 0 {
		t.Errorf("skipped = %d, want 0", got["skipped"])
	}
}
