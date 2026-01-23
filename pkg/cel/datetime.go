package cel

import (
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// DateTimeFunctions returns the CEL function declarations for date/time operations.
// These should be added to the CEL environment options.
//
// Available functions:
//   - now() -> string: Returns current UTC time in RFC3339 format
//   - nowUnix() -> int: Returns current Unix timestamp (seconds since epoch)
//   - nowUnixMilli() -> int: Returns current Unix timestamp in milliseconds
//   - formatTime(unixSeconds, layout) -> string: Formats Unix timestamp with Go layout
//   - formatTimeRFC3339(unixSeconds) -> string: Formats Unix timestamp as RFC3339
//   - parseTime(timeStr, layout) -> int: Parses time string to Unix timestamp
//   - parseTimeRFC3339(timeStr) -> int: Parses RFC3339 time string to Unix timestamp
//   - addDuration(timeStr, duration) -> string: Adds duration to RFC3339 time string
//   - timeSince(timeStr) -> int: Returns seconds since the given RFC3339 time
//   - timeUntil(timeStr) -> int: Returns seconds until the given RFC3339 time
//   - durationSeconds(durationStr) -> int: Parses duration string to seconds
func DateTimeFunctions() []cel.EnvOption {
	return []cel.EnvOption{
		// now() -> string: Returns current UTC time in RFC3339 format
		cel.Function("now",
			cel.Overload("now_string",
				[]*cel.Type{},
				cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.String(time.Now().UTC().Format(time.RFC3339))
				}),
			),
		),

		// nowUnix() -> int: Returns current Unix timestamp (seconds)
		cel.Function("nowUnix",
			cel.Overload("nowUnix_int",
				[]*cel.Type{},
				cel.IntType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.Int(time.Now().Unix())
				}),
			),
		),

		// nowUnixMilli() -> int: Returns current Unix timestamp in milliseconds
		cel.Function("nowUnixMilli",
			cel.Overload("nowUnixMilli_int",
				[]*cel.Type{},
				cel.IntType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.Int(time.Now().UnixMilli())
				}),
			),
		),

		// formatTime(unixSeconds, layout) -> string: Formats Unix timestamp with Go layout
		cel.Function("formatTime",
			cel.Overload("formatTime_int_string",
				[]*cel.Type{cel.IntType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(FormatTime),
			),
		),

		// formatTimeRFC3339(unixSeconds) -> string: Formats Unix timestamp as RFC3339
		cel.Function("formatTimeRFC3339",
			cel.Overload("formatTimeRFC3339_int",
				[]*cel.Type{cel.IntType},
				cel.StringType,
				cel.UnaryBinding(FormatTimeRFC3339),
			),
		),

		// parseTime(timeStr, layout) -> int: Parses time string to Unix timestamp
		cel.Function("parseTime",
			cel.Overload("parseTime_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.IntType,
				cel.BinaryBinding(ParseTime),
			),
		),

		// parseTimeRFC3339(timeStr) -> int: Parses RFC3339 time string to Unix timestamp
		cel.Function("parseTimeRFC3339",
			cel.Overload("parseTimeRFC3339_string",
				[]*cel.Type{cel.StringType},
				cel.IntType,
				cel.UnaryBinding(ParseTimeRFC3339),
			),
		),

		// addDuration(timeStr, duration) -> string: Adds duration to RFC3339 time
		// Duration format: "1h", "30m", "24h", "-1h30m", etc.
		cel.Function("addDuration",
			cel.Overload("addDuration_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(AddDuration),
			),
		),

		// timeSince(timeStr) -> int: Returns seconds since the given RFC3339 time
		cel.Function("timeSince",
			cel.Overload("timeSince_string",
				[]*cel.Type{cel.StringType},
				cel.IntType,
				cel.UnaryBinding(TimeSince),
			),
		),

		// timeUntil(timeStr) -> int: Returns seconds until the given RFC3339 time
		cel.Function("timeUntil",
			cel.Overload("timeUntil_string",
				[]*cel.Type{cel.StringType},
				cel.IntType,
				cel.UnaryBinding(TimeUntil),
			),
		),

		// durationSeconds(durationStr) -> int: Parses duration string to seconds
		// Duration format: "1h", "30m", "1h30m", "90s", etc.
		cel.Function("durationSeconds",
			cel.Overload("durationSeconds_string",
				[]*cel.Type{cel.StringType},
				cel.IntType,
				cel.UnaryBinding(ParseDuration),
			),
		),
	}
}

// FormatTime formats a Unix timestamp using the given Go time layout.
func FormatTime(unixSeconds, layout ref.Val) ref.Val {
	secs, ok := unixSeconds.Value().(int64)
	if !ok {
		return types.NewErr("formatTime() requires an int for unixSeconds")
	}
	layoutStr, ok := layout.Value().(string)
	if !ok {
		return types.NewErr("formatTime() requires a string for layout")
	}

	t := time.Unix(secs, 0).UTC()
	return types.String(t.Format(layoutStr))
}

// FormatTimeRFC3339 formats a Unix timestamp as RFC3339.
func FormatTimeRFC3339(unixSeconds ref.Val) ref.Val {
	secs, ok := unixSeconds.Value().(int64)
	if !ok {
		return types.NewErr("formatTimeRFC3339() requires an int")
	}

	t := time.Unix(secs, 0).UTC()
	return types.String(t.Format(time.RFC3339))
}

// ParseTime parses a time string using the given Go time layout and returns Unix timestamp.
func ParseTime(timeStr, layout ref.Val) ref.Val {
	str, ok := timeStr.Value().(string)
	if !ok {
		return types.NewErr("parseTime() requires a string for timeStr")
	}
	layoutStr, ok := layout.Value().(string)
	if !ok {
		return types.NewErr("parseTime() requires a string for layout")
	}

	t, err := time.Parse(layoutStr, str)
	if err != nil {
		return types.NewErr("parseTime() failed to parse time: %v", err)
	}

	return types.Int(t.Unix())
}

// ParseTimeRFC3339 parses an RFC3339 time string and returns Unix timestamp.
func ParseTimeRFC3339(timeStr ref.Val) ref.Val {
	str, ok := timeStr.Value().(string)
	if !ok {
		return types.NewErr("parseTimeRFC3339() requires a string")
	}

	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		// Try RFC3339Nano as fallback
		t, err = time.Parse(time.RFC3339Nano, str)
		if err != nil {
			return types.NewErr("parseTimeRFC3339() failed to parse time: %v", err)
		}
	}

	return types.Int(t.Unix())
}

// AddDuration adds a duration to an RFC3339 time string and returns the result as RFC3339.
// Duration format: "1h", "30m", "24h", "-1h30m", "90s", etc.
func AddDuration(timeStr, durationStr ref.Val) ref.Val {
	str, ok := timeStr.Value().(string)
	if !ok {
		return types.NewErr("addDuration() requires a string for timeStr")
	}
	durStr, ok := durationStr.Value().(string)
	if !ok {
		return types.NewErr("addDuration() requires a string for duration")
	}

	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, str)
		if err != nil {
			return types.NewErr("addDuration() failed to parse time: %v", err)
		}
	}

	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return types.NewErr("addDuration() failed to parse duration: %v", err)
	}

	result := t.Add(dur).UTC()
	return types.String(result.Format(time.RFC3339))
}

// TimeSince returns the number of seconds since the given RFC3339 time.
func TimeSince(timeStr ref.Val) ref.Val {
	str, ok := timeStr.Value().(string)
	if !ok {
		return types.NewErr("timeSince() requires a string")
	}

	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, str)
		if err != nil {
			return types.NewErr("timeSince() failed to parse time: %v", err)
		}
	}

	return types.Int(int64(time.Since(t).Seconds()))
}

// TimeUntil returns the number of seconds until the given RFC3339 time.
func TimeUntil(timeStr ref.Val) ref.Val {
	str, ok := timeStr.Value().(string)
	if !ok {
		return types.NewErr("timeUntil() requires a string")
	}

	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, str)
		if err != nil {
			return types.NewErr("timeUntil() failed to parse time: %v", err)
		}
	}

	return types.Int(int64(time.Until(t).Seconds()))
}

// ParseDuration parses a duration string (e.g., "1h30m", "90s") and returns seconds.
func ParseDuration(durationStr ref.Val) ref.Val {
	str, ok := durationStr.Value().(string)
	if !ok {
		return types.NewErr("duration() requires a string")
	}

	dur, err := time.ParseDuration(str)
	if err != nil {
		return types.NewErr("duration() failed to parse duration: %v", err)
	}

	return types.Int(int64(dur.Seconds()))
}
