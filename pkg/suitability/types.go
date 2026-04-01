package suitability

// SuitabilityResult is the top-level output schema for pre-implementation status scanning
type SuitabilityResult struct {
	PreImplementation PreImplStatus `json:"pre_implementation"`
}

// PreImplStatus contains aggregated metrics and per-item status
type PreImplStatus struct {
	TotalItems       int          `json:"total_items"`
	Done             int          `json:"done"`
	Partial          int          `json:"partial"`
	Todo             int          `json:"todo"`
	TimeSavedMinutes int          `json:"time_saved_minutes"`
	ItemStatus       []ItemStatus `json:"item_status"`
}

// ItemStatus represents the classification status of a single requirement
type ItemStatus struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"` // "DONE" | "PARTIAL" | "TODO"
	File         string   `json:"file,omitempty"`
	TestCoverage string   `json:"test_coverage,omitempty"`
	Completeness float64  `json:"completeness"`
	Missing      []string `json:"missing,omitempty"`
}

// Requirement represents a single item from an audit or requirements document.
// It contains an ID (e.g., "F1", "SEC-01"), a description of the requirement,
// and a list of files expected to implement it.
type Requirement struct {
	ID          string   // "F1", "SEC-01", etc.
	Description string
	Files       []string // Files expected to implement this requirement
}
