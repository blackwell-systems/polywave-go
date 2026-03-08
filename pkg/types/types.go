// Package types defines the shared data types for the Scout-and-Wave protocol:
// protocol states, IMPL doc structure, agent specifications, wave definitions,
// and completion report formats. All other packages in this module import types
// rather than defining their own protocol structs.
package types

// State represents the state of the protocol state machine
type State int

const (
	ScoutPending    State = iota // Scout agent is analyzing the codebase (SCOUT_PENDING)
	ScoutValidating              // Scout has written the IMPL doc; E16 validation in progress (SCOUT_VALIDATING)
	NotSuitable                  // feature was rejected by the suitability gate; terminal
	Reviewed                     // IMPL doc has been reviewed and approved by a human
	ScaffoldPending              // Scaffold agent is creating shared type scaffold files
	WavePending                  // wave is ready to execute; agents not yet launched
	WaveExecuting                // wave agents are running concurrently
	WaveMerging                  // orchestrator is merging agent worktrees into HEAD
	WaveVerified                 // post-merge verification passed
	Blocked                      // agent failure or verification failure; requires resolution
	Complete                     // all waves merged and verified; terminal
)

// String returns the string representation of the State
func (s State) String() string {
	switch s {
	case ScoutPending:
		return "ScoutPending"
	case ScoutValidating:
		return "ScoutValidating"
	case NotSuitable:
		return "NotSuitable"
	case Reviewed:
		return "Reviewed"
	case ScaffoldPending:
		return "ScaffoldPending"
	case WavePending:
		return "WavePending"
	case WaveExecuting:
		return "WaveExecuting"
	case WaveMerging:
		return "WaveMerging"
	case WaveVerified:
		return "WaveVerified"
	case Blocked:
		return "Blocked"
	case Complete:
		return "Complete"
	default:
		return "Unknown"
	}
}

// CompletionStatus represents the completion status of an agent's work
type CompletionStatus string

const (
	StatusComplete CompletionStatus = "complete" // agent finished all assigned work
	StatusPartial  CompletionStatus = "partial"  // agent completed some work; wave goes to Blocked
	StatusBlocked  CompletionStatus = "blocked"  // agent could not proceed; wave goes to Blocked
)

// IMPLDoc is the parsed representation of an IMPL markdown document
type IMPLDoc struct {
	FeatureName   string
	Status        string // suitability verdict (e.g. "SUITABLE", "NOT SUITABLE")
	DocStatus     string // lifecycle status: "" (active) or "COMPLETE"
	CompletedAt   string // ISO date from <!-- SAW:COMPLETE YYYY-MM-DD --> tag, empty if active
	TestCommand   string            // post-merge verification command (e.g. "go test ./...")
	LintCommand   string            // check-mode lint command (e.g. "go vet ./...")
	Waves             []Wave
	FileOwnership     map[string]FileOwnershipInfo // file path -> ownership info
	FileOwnershipCol4 string                       // detected header name for 4th column (e.g. "Action", "Depends On")
	KnownIssues            []KnownIssue
	ScaffoldsDetail        []ScaffoldFile
	InterfaceContractsText string
	DependencyGraphText    string
	PostMergeChecklistText string
	PreMortem              *PreMortem   // parsed ## Pre-Mortem section; nil if absent
	StubReportText         string       // raw markdown content of ## Stub Report section; empty if absent
	QualityGates           *QualityGates // nil if absent; E21
}

// FileOwnershipInfo holds parsed data for one row of the file ownership table.
type FileOwnershipInfo struct {
	Agent     string // agent letter (e.g. "A")
	Wave      int    // wave number (0 if not specified)
	Action    string // "new", "modify", "delete", or "" if not specified
	DependsOn string // 4th column when header is "Depends On" (not "Action")
}

// Wave represents one wave of parallel agents
type Wave struct {
	Number int
	Agents []AgentSpec
}

// AgentSpec is the parsed agent prompt extracted from the IMPL doc
type AgentSpec struct {
	Letter     string
	Prompt     string
	FilesOwned []string
}

// CompletionReport is the structured YAML block each agent appends to the IMPL doc
type CompletionReport struct {
	Status               CompletionStatus
	Worktree             string
	Branch               string
	Commit               string
	FilesChanged         []string
	FilesCreated         []string
	InterfaceDeviations  []InterfaceDeviation
	OutOfScopeDeps       []string
	TestsAdded           []string
	Verification         string
	FailureType          FailureType `yaml:"failure_type,omitempty"` // E19
}

// InterfaceDeviation records a deviation from the spec contract
type InterfaceDeviation struct {
	Description              string
	DownstreamActionRequired bool
	Affects                  []string
}

// KnownIssue represents an identified issue in the Known Issues section
type KnownIssue struct {
	Description string
	Status      string
	Workaround  string
}

// ScaffoldFile represents a scaffold file entry from the detailed Scaffolds table
type ScaffoldFile struct {
	FilePath   string
	Contents   string
	ImportPath string
}

// PreMortemRow is one row of the ## Pre-Mortem risk table.
type PreMortemRow struct {
	Scenario   string
	Likelihood string
	Impact     string
	Mitigation string
}

// PreMortem holds the parsed ## Pre-Mortem section.
type PreMortem struct {
	OverallRisk string        // "low", "medium", or "high"
	Rows        []PreMortemRow
}

// ValidationError is one error returned by ValidateIMPLDoc (E16 Go validator).
type ValidationError struct {
	BlockType  string // e.g. "impl-file-ownership", "impl-dep-graph", "impl-wave-structure", "impl-completion-report"
	LineNumber int    // 1-based line number of the opening fence in the IMPL doc
	Message    string // human-readable description of the violation
}

// FailureType classifies why an agent reported partial or blocked status.
// E19
type FailureType string

const (
	FailureTypeTransient   FailureType = "transient"
	FailureTypeFixable     FailureType = "fixable"
	FailureTypeNeedsReplan FailureType = "needs_replan"
	FailureTypeEscalate    FailureType = "escalate"
	FailureTypeTimeout     FailureType = "timeout"
)

// QualityGate is one gate from the ## Quality Gates IMPL doc section (E21).
type QualityGate struct {
	Type        string
	Command     string
	Required    bool
	Description string
}

// QualityGates holds the parsed ## Quality Gates section.
// E21
type QualityGates struct {
	Level string
	Gates []QualityGate
}
