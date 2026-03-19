// Wire into ValidateSchema in schema_validation.go by adding: errs = append(errs, ValidateReactions(m)...)
package protocol

import "fmt"

// validReactionActions is the set of allowed action values for a ReactionEntry.
var validReactionActions = map[string]bool{
	"retry":           true,
	"send-fix-prompt": true,
	"pause":           true,
	"auto-scout":      true,
}

// ValidateReactions validates the reactions block in an IMPL manifest.
// Called by ValidateSchema. Returns SV01 errors for invalid action values
// or negative max_attempts. Returns nil if reactions is absent.
func ValidateReactions(m *IMPLManifest) []ValidationError {
	if m.Reactions == nil {
		return nil
	}
	var errs []ValidationError
	entries := map[string]*ReactionEntry{
		"transient":    m.Reactions.Transient,
		"timeout":      m.Reactions.Timeout,
		"fixable":      m.Reactions.Fixable,
		"needs_replan": m.Reactions.NeedsReplan,
		"escalate":     m.Reactions.Escalate,
	}
	for name, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Action == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("reactions.%s.action is required", name),
				Field:   fmt.Sprintf("reactions.%s.action", name),
			})
		} else if !validReactionActions[entry.Action] {
			errs = append(errs, ValidationError{
				Code:    SV01InvalidEnum,
				Message: fmt.Sprintf("reactions.%s.action %q is not valid; must be one of: retry, send-fix-prompt, pause, auto-scout", name, entry.Action),
				Field:   fmt.Sprintf("reactions.%s.action", name),
			})
		}
		if entry.MaxAttempts < 0 {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("reactions.%s.max_attempts must be >= 0, got %d", name, entry.MaxAttempts),
				Field:   fmt.Sprintf("reactions.%s.max_attempts", name),
			})
		}
	}
	return errs
}
