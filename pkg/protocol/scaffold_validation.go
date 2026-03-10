package protocol

import "strings"

// ScaffoldStatus represents the validation result for a single scaffold file.
type ScaffoldStatus struct {
	FilePath string `json:"file_path"`
	Status   string `json:"status"` // "committed" | "pending" | "FAILED"
	Valid    bool   `json:"valid"`
	Message  string `json:"message,omitempty"`
}

// ValidateScaffolds checks that all scaffold files in a manifest have been committed.
// Returns one ScaffoldStatus per scaffold file. All must be Valid for wave execution to proceed.
func ValidateScaffolds(m *IMPLManifest) []ScaffoldStatus {
	if m == nil {
		return []ScaffoldStatus{}
	}

	results := make([]ScaffoldStatus, 0, len(m.Scaffolds))

	for _, scaffold := range m.Scaffolds {
		status := ScaffoldStatus{
			FilePath: scaffold.FilePath,
			Status:   scaffold.Status,
		}

		switch {
		case strings.HasPrefix(scaffold.Status, "committed"):
			status.Valid = true
			status.Message = ""
		case scaffold.Status == "pending":
			status.Valid = false
			status.Message = "scaffold not yet committed"
		case strings.HasPrefix(scaffold.Status, "FAILED"):
			status.Valid = false
			status.Message = scaffold.Status
		case scaffold.Status == "":
			status.Valid = false
			status.Message = "no status set"
		default:
			// Any other status is treated as invalid
			status.Valid = false
			status.Message = "unknown status: " + scaffold.Status
		}

		results = append(results, status)
	}

	return results
}

// AllScaffoldsCommitted is a convenience function that returns true if all scaffolds
// have status "committed" (or if there are no scaffolds).
func AllScaffoldsCommitted(m *IMPLManifest) bool {
	statuses := ValidateScaffolds(m)

	// Empty manifest or no scaffolds is valid
	if len(statuses) == 0 {
		return true
	}

	for _, status := range statuses {
		if !status.Valid {
			return false
		}
	}

	return true
}
