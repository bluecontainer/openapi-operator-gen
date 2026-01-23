package cel

import (
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/common/types"
)

func TestNowFunctions(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	t.Run("now returns RFC3339 string", func(t *testing.T) {
		result := Evaluate(env, "now()", vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		str, ok := result.Value.(string)
		if !ok {
			t.Fatalf("Expected string, got %T", result.Value)
		}

		// Verify it's a valid RFC3339 time
		_, err := time.Parse(time.RFC3339, str)
		if err != nil {
			t.Errorf("now() did not return valid RFC3339: %v", err)
		}
	})

	t.Run("nowUnix returns timestamp", func(t *testing.T) {
		result := Evaluate(env, "nowUnix()", vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		ts, ok := result.Value.(int64)
		if !ok {
			t.Fatalf("Expected int64, got %T", result.Value)
		}

		// Should be within a few seconds of now
		now := time.Now().Unix()
		if ts < now-10 || ts > now+10 {
			t.Errorf("nowUnix() = %d, expected near %d", ts, now)
		}
	})

	t.Run("nowUnixMilli returns milliseconds", func(t *testing.T) {
		result := Evaluate(env, "nowUnixMilli()", vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		ts, ok := result.Value.(int64)
		if !ok {
			t.Fatalf("Expected int64, got %T", result.Value)
		}

		// Should be within a few seconds of now (in milliseconds)
		now := time.Now().UnixMilli()
		if ts < now-10000 || ts > now+10000 {
			t.Errorf("nowUnixMilli() = %d, expected near %d", ts, now)
		}
	})
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       string
	}{
		{
			name:       "format as RFC3339",
			expression: `formatTimeRFC3339(1706000000)`,
			want:       "2024-01-23T08:53:20Z",
		},
		{
			name:       "format with custom layout",
			expression: `formatTime(1706000000, "2006-01-02")`,
			want:       "2024-01-23",
		},
		{
			name:       "format with time only",
			expression: `formatTime(1706000000, "15:04:05")`,
			want:       "08:53:20",
		},
	}

	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Fatalf("Evaluate() error = %v", result.Error)
			}

			got, ok := result.Value.(string)
			if !ok {
				t.Fatalf("Expected string, got %T", result.Value)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       int64
	}{
		{
			name:       "parse RFC3339",
			expression: `parseTimeRFC3339("2024-01-23T08:53:20Z")`,
			want:       1706000000,
		},
		{
			name:       "parse with custom layout",
			expression: `parseTime("2024-01-23", "2006-01-02")`,
			want:       1705968000, // 2024-01-23 00:00:00 UTC
		},
		{
			name:       "parse RFC3339 with timezone",
			expression: `parseTimeRFC3339("2024-01-23T10:53:20+02:00")`,
			want:       1706000000, // Same instant as 08:53:20Z
		},
	}

	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Fatalf("Evaluate() error = %v", result.Error)
			}

			got, ok := result.Value.(int64)
			if !ok {
				t.Fatalf("Expected int64, got %T", result.Value)
			}

			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAddDuration(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       string
	}{
		{
			name:       "add 1 hour",
			expression: `addDuration("2024-01-23T08:00:00Z", "1h")`,
			want:       "2024-01-23T09:00:00Z",
		},
		{
			name:       "add 30 minutes",
			expression: `addDuration("2024-01-23T08:00:00Z", "30m")`,
			want:       "2024-01-23T08:30:00Z",
		},
		{
			name:       "subtract 1 hour",
			expression: `addDuration("2024-01-23T08:00:00Z", "-1h")`,
			want:       "2024-01-23T07:00:00Z",
		},
		{
			name:       "add 24 hours",
			expression: `addDuration("2024-01-23T08:00:00Z", "24h")`,
			want:       "2024-01-24T08:00:00Z",
		},
		{
			name:       "add complex duration",
			expression: `addDuration("2024-01-23T08:00:00Z", "1h30m45s")`,
			want:       "2024-01-23T09:30:45Z",
		},
	}

	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Fatalf("Evaluate() error = %v", result.Error)
			}

			got, ok := result.Value.(string)
			if !ok {
				t.Fatalf("Expected string, got %T", result.Value)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimeSinceAndUntil(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	t.Run("timeSince past time", func(t *testing.T) {
		// Time 1 hour ago
		pastTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
		expression := `timeSince("` + pastTime + `")`

		result := Evaluate(env, expression, vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		secs, ok := result.Value.(int64)
		if !ok {
			t.Fatalf("Expected int64, got %T", result.Value)
		}

		// Should be approximately 3600 seconds (1 hour)
		if secs < 3590 || secs > 3610 {
			t.Errorf("timeSince() = %d, expected near 3600", secs)
		}
	})

	t.Run("timeUntil future time", func(t *testing.T) {
		// Time 1 hour from now
		futureTime := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
		expression := `timeUntil("` + futureTime + `")`

		result := Evaluate(env, expression, vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		secs, ok := result.Value.(int64)
		if !ok {
			t.Fatalf("Expected int64, got %T", result.Value)
		}

		// Should be approximately 3600 seconds (1 hour)
		if secs < 3590 || secs > 3610 {
			t.Errorf("timeUntil() = %d, expected near 3600", secs)
		}
	})
}

func TestDurationSeconds(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       int64
	}{
		{
			name:       "1 hour",
			expression: `durationSeconds("1h")`,
			want:       3600,
		},
		{
			name:       "30 minutes",
			expression: `durationSeconds("30m")`,
			want:       1800,
		},
		{
			name:       "90 seconds",
			expression: `durationSeconds("90s")`,
			want:       90,
		},
		{
			name:       "complex duration",
			expression: `durationSeconds("1h30m")`,
			want:       5400,
		},
		{
			name:       "24 hours",
			expression: `durationSeconds("24h")`,
			want:       86400,
		},
	}

	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(env, tt.expression, vars)
			if result.Error != nil {
				t.Fatalf("Evaluate() error = %v", result.Error)
			}

			got, ok := result.Value.(int64)
			if !ok {
				t.Fatalf("Expected int64, got %T", result.Value)
			}

			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDateTimeFunctions_InvalidInput(t *testing.T) {
	t.Run("formatTime with non-int", func(t *testing.T) {
		result := FormatTime(types.String("not-int"), types.String("2006-01-02"))
		if !types.IsError(result) {
			t.Error("Expected error for non-int input")
		}
	})

	t.Run("formatTime with non-string layout", func(t *testing.T) {
		result := FormatTime(types.Int(1000), types.Int(123))
		if !types.IsError(result) {
			t.Error("Expected error for non-string layout")
		}
	})

	t.Run("parseTimeRFC3339 with invalid time", func(t *testing.T) {
		result := ParseTimeRFC3339(types.String("not-a-time"))
		if !types.IsError(result) {
			t.Error("Expected error for invalid time string")
		}
	})

	t.Run("addDuration with invalid duration", func(t *testing.T) {
		result := AddDuration(types.String("2024-01-23T08:00:00Z"), types.String("invalid"))
		if !types.IsError(result) {
			t.Error("Expected error for invalid duration")
		}
	})

	t.Run("durationSeconds with invalid string", func(t *testing.T) {
		result := ParseDuration(types.String("invalid"))
		if !types.IsError(result) {
			t.Error("Expected error for invalid duration string")
		}
	})
}

func TestDateTimeFunctions_UseCases(t *testing.T) {
	env, err := NewEnvironment([]string{})
	if err != nil {
		t.Fatalf("NewEnvironment() error = %v", err)
	}

	vars := map[string]any{
		"resources": []map[string]any{},
		"summary":   map[string]int64{"total": 0},
	}

	t.Run("check if time is older than 1 hour", func(t *testing.T) {
		// Time 2 hours ago
		oldTime := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
		expression := `timeSince("` + oldTime + `") > durationSeconds("1h")`

		result := Evaluate(env, expression, vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		got, ok := result.Value.(bool)
		if !ok {
			t.Fatalf("Expected bool, got %T", result.Value)
		}

		if !got {
			t.Error("Expected true for 2 hours ago > 1 hour")
		}
	})

	t.Run("calculate expiry time", func(t *testing.T) {
		// Add 24 hours to current time
		expression := `addDuration(now(), "24h")`

		result := Evaluate(env, expression, vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		got, ok := result.Value.(string)
		if !ok {
			t.Fatalf("Expected string, got %T", result.Value)
		}

		// Parse and verify it's ~24 hours from now
		parsed, err := time.Parse(time.RFC3339, got)
		if err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		expected := time.Now().Add(24 * time.Hour)
		diff := parsed.Sub(expected)
		if diff < -10*time.Second || diff > 10*time.Second {
			t.Errorf("Expected time ~24 hours from now, got %v", got)
		}
	})

	t.Run("format current time for logging", func(t *testing.T) {
		expression := `formatTime(nowUnix(), "2006-01-02 15:04:05")`

		result := Evaluate(env, expression, vars)
		if result.Error != nil {
			t.Fatalf("Evaluate() error = %v", result.Error)
		}

		got, ok := result.Value.(string)
		if !ok {
			t.Fatalf("Expected string, got %T", result.Value)
		}

		// Should contain the current date
		currentDate := time.Now().UTC().Format("2006-01-02")
		if !strings.Contains(got, currentDate) {
			t.Errorf("Expected formatted time to contain %s, got %s", currentDate, got)
		}
	})
}

func TestDateTimeFunctions_Count(t *testing.T) {
	funcs := DateTimeFunctions()

	// Should have 11 functions:
	// now, nowUnix, nowUnixMilli, formatTime, formatTimeRFC3339,
	// parseTime, parseTimeRFC3339, addDuration, timeSince, timeUntil, durationSeconds
	expectedCount := 11
	if len(funcs) != expectedCount {
		t.Errorf("DateTimeFunctions() returned %d options, want %d", len(funcs), expectedCount)
	}
}
