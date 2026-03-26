package protocol

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ValidateActionEnums checks that all file_ownership action fields contain valid enum values.
// Valid action values: "new", "modify", "delete".
// Empty/omitted action is also valid (backward compatibility).
func ValidateActionEnums(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	validActions := map[string]bool{
		"new":    true,
		"modify": true,
		"delete": true,
	}

	for i, fo := range m.FileOwnership {
		// Empty action is valid (backward compatibility — action field is optional)
		if fo.Action == "" {
			continue
		}

		if !validActions[fo.Action] {
			errs = append(errs, result.SAWError{
				Code:     result.CodeInvalidActionEnum,
				Message:  fmt.Sprintf("file_ownership[%d].action has invalid value %q — must be new, modify, or delete", i, fo.Action),
				Severity: "error",
				Field:    fmt.Sprintf("file_ownership[%d].action", i),
			})
		}
	}

	return errs
}
