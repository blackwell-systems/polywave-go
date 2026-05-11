package deps

import "github.com/blackwell-systems/polywave-go/pkg/result"

// LockFileParser interface for multi-language lock file parsing
type LockFileParser interface {
	// Parse reads a lock file and returns package metadata
	Parse(filePath string) result.Result[[]PackageInfo]

	// Detect checks if this parser can handle the given file
	Detect(filePath string) bool
}

// PackageInfo represents a package from a lock file
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"` // registry URL or repo URL
}

// ConflictReport contains the results of dependency conflict detection
type ConflictReport struct {
	MissingDeps      []MissingDep      `json:"missing_deps"`
	VersionConflicts []VersionConflict `json:"version_conflicts"`
	Recommendations  []string          `json:"recommendations"`
}

// MissingDep represents a dependency that is imported but not in lock files
type MissingDep struct {
	Agent        string `json:"agent"`
	Package      string `json:"package"`
	RequiredBy   string `json:"required_by"`   // file that imports it
	AvailableVer string `json:"available_version"` // empty if not found in any lock file
}

// VersionConflict represents a package with conflicting version requirements
type VersionConflict struct {
	Agents           []string `json:"agents"`
	Package          string   `json:"package"`
	Versions         []string `json:"versions"`
	ResolutionNeeded bool     `json:"resolution_needed"`
}
