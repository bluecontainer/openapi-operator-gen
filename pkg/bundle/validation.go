package bundle

// IsValidResourceID checks if a string is a valid bundle resource ID.
// Valid IDs must start with a lowercase letter and contain only
// lowercase letters, numbers, and hyphens.
func IsValidResourceID(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must start with lowercase letter
	if s[0] < 'a' || s[0] > 'z' {
		return false
	}
	// Must only contain lowercase letters, numbers, and hyphens
	for i := 1; i < len(s); i++ {
		if !IsIdentChar(s[i]) {
			return false
		}
	}
	return true
}

// IsIdentChar returns true if c is a valid identifier character (a-z, 0-9, -).
func IsIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}
