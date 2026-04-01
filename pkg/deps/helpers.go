package deps

import "strings"

// unquoteTOML removes quotes from TOML string values.
// Handles both double quotes ("value") and single quotes ('value').
// Returns the unquoted string, or the original string if no quotes are found.
func unquoteTOML(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
