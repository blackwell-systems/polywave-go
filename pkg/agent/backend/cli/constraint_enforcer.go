package cli

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/tools"
)

// ToolUseEvent records a single Write or Edit tool invocation observed in the CLI
// backend stream. FilePath is empty for tools that don't target a file path
// (e.g. Bash, Read, Glob, Grep).
type ToolUseEvent struct {
	ToolName string
	FilePath string // extracted from Write/Edit input; empty for Bash/Read/Glob/Grep
}

// ConstraintViolation describes a single I1/I2/I6 constraint violation detected
// during post-hoc enforcement.
type ConstraintViolation struct {
	ToolName  string
	FilePath  string
	Invariant string // "I1", "I2", or "I6"
	Message   string
}

// CheckConstraints inspects the accumulated tool-use events recorded during a CLI
// backend run and reports any violations of I1/I2/I6 constraints.
// Returns a slice of violation descriptions; empty slice means no violations.
// This is a post-hoc pass: it cannot prevent writes, only detect them.
// Returns nil when constraints is nil.
func CheckConstraints(events []ToolUseEvent, constraints *tools.Constraints) []ConstraintViolation {
	if constraints == nil {
		return nil
	}

	var violations []ConstraintViolation

	for _, ev := range events {
		// Only inspect Write and Edit events.
		if ev.ToolName != "Write" && ev.ToolName != "Edit" {
			continue
		}
		// Skip events with no file path.
		if ev.FilePath == "" {
			continue
		}

		// I1: File ownership enforcement.
		// If OwnedFiles is non-empty and the path is not in the owned set, emit a violation.
		if len(constraints.OwnedFiles) > 0 && !constraints.OwnedFiles[ev.FilePath] {
			violations = append(violations, ConstraintViolation{
				ToolName:  ev.ToolName,
				FilePath:  ev.FilePath,
				Invariant: "I1",
				Message: fmt.Sprintf("agent %q wrote %q which is not in its owned file set",
					constraints.AgentID, ev.FilePath),
			})
		}

		// I2: Interface freeze enforcement.
		// If the path is in FrozenPaths and FreezeTime is set, emit a violation.
		if constraints.FrozenPaths[ev.FilePath] && constraints.FreezeTime != nil {
			violations = append(violations, ConstraintViolation{
				ToolName:  ev.ToolName,
				FilePath:  ev.FilePath,
				Invariant: "I2",
				Message: fmt.Sprintf("agent %q wrote frozen path %q (frozen since %s)",
					constraints.AgentID, ev.FilePath,
					constraints.FreezeTime.Format("2006-01-02T15:04:05Z07:00")),
			})
		}

		// I6: Role-based path restriction.
		// If AllowedPathPrefixes is non-empty and no prefix matches, emit a violation.
		if len(constraints.AllowedPathPrefixes) > 0 {
			allowed := false
			for _, prefix := range constraints.AllowedPathPrefixes {
				if strings.HasPrefix(ev.FilePath, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				violations = append(violations, ConstraintViolation{
					ToolName:  ev.ToolName,
					FilePath:  ev.FilePath,
					Invariant: "I6",
					Message: fmt.Sprintf("agent %q (role %q) wrote %q which does not match allowed prefixes %v",
						constraints.AgentID, constraints.AgentRole,
						ev.FilePath, constraints.AllowedPathPrefixes),
				})
			}
		}
	}

	return violations
}
