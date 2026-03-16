package protocol

import (
	"errors"
	"time"
)

// ErrReportNotFound is returned by ParseCompletionReport when the requested
// agent's completion report section does not exist in the IMPL doc.
var ErrReportNotFound = errors.New("completion report not found")

// IMPLManifest is the structured representation of a SAW IMPL document.
// It contains all metadata, wave definitions, agent tasks, and completion reports.
type IMPLManifest struct {
	Title                 string                         `yaml:"title" json:"title"`
	FeatureSlug           string                         `yaml:"feature_slug" json:"feature_slug"`
	Verdict               string                         `yaml:"verdict" json:"verdict"` // "SUITABLE" | "NOT_SUITABLE" | "SUITABLE_WITH_CAVEATS"
	SuitabilityAssessment string                         `yaml:"suitability_assessment,omitempty" json:"suitability_assessment,omitempty"`
	TestCommand           string                         `yaml:"test_command" json:"test_command"`
	LintCommand           string                         `yaml:"lint_command" json:"lint_command"`
	FileOwnership         []FileOwnership                `yaml:"file_ownership" json:"file_ownership"`
	InterfaceContracts    []InterfaceContract            `yaml:"interface_contracts" json:"interface_contracts"`
	Waves                 []Wave                         `yaml:"waves" json:"waves"`
	QualityGates          *QualityGates                  `yaml:"quality_gates,omitempty" json:"quality_gates,omitempty"`
	PostMergeChecklist    *PostMergeChecklist            `yaml:"post_merge_checklist,omitempty" json:"post_merge_checklist,omitempty"`
	Scaffolds             []ScaffoldFile                 `yaml:"scaffolds,omitempty" json:"scaffolds,omitempty"`
	CompletionReports     map[string]CompletionReport    `yaml:"completion_reports,omitempty" json:"completion_reports,omitempty"`
	StubReports           map[string]*ScanStubsResult    `yaml:"stub_reports,omitempty" json:"stub_reports,omitempty"`
	PreMortem             *PreMortem                     `yaml:"pre_mortem,omitempty" json:"pre_mortem,omitempty"`
	KnownIssues           []KnownIssue                   `yaml:"known_issues,omitempty" json:"known_issues,omitempty"`
	State                 ProtocolState                  `yaml:"state,omitempty" json:"state,omitempty"`
	MergeState            MergeState                     `yaml:"merge_state,omitempty" json:"merge_state,omitempty"`
	// Freeze enforcement fields (E2/I2-02)
	WorktreesCreatedAt  *time.Time `yaml:"worktrees_created_at,omitempty" json:"worktrees_created_at,omitempty"`
	FrozenContractsHash string     `yaml:"frozen_contracts_hash,omitempty" json:"frozen_contracts_hash,omitempty"`
	FrozenScaffoldsHash string     `yaml:"frozen_scaffolds_hash,omitempty" json:"frozen_scaffolds_hash,omitempty"`
	CompletionDate      string     `yaml:"completion_date,omitempty"        json:"completion_date,omitempty"`
}

// FileOwnership tracks which agent owns which file in which wave.
// It includes dependency information and cross-repo tracking.
type FileOwnership struct {
	File      string   `yaml:"file" json:"file"`
	Agent     string   `yaml:"agent" json:"agent"`
	Wave      int      `yaml:"wave" json:"wave"`
	Action    string   `yaml:"action,omitempty" json:"action,omitempty"` // "new" | "modify" | "delete"
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Repo      string   `yaml:"repo,omitempty" json:"repo,omitempty"` // For cross-repo waves
}

// Wave represents a concurrent execution phase with multiple agents.
// Agents in the same wave run in parallel; waves execute sequentially.
type Wave struct {
	Number           int      `yaml:"number" json:"number"`
	Agents           []Agent  `yaml:"agents" json:"agents"`
	AgentLaunchOrder []string `yaml:"agent_launch_order,omitempty" json:"agent_launch_order,omitempty"`
	BaseCommit       string   `yaml:"base_commit,omitempty" json:"base_commit,omitempty"` // Recorded when worktrees created for post-merge verification
}

// Agent represents a single concurrent task within a wave.
// Each agent has a unique ID, task description, owned files, and dependencies on other agents.
type Agent struct {
	ID           string   `yaml:"id" json:"id"`
	Task         string   `yaml:"task" json:"task"`
	Files        []string `yaml:"files" json:"files"`
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Model        string   `yaml:"model,omitempty" json:"model,omitempty"`
}

// CompletionReport records the outcome of an agent's work.
// It tracks status, files changed, interface deviations, and test coverage.
type CompletionReport struct {
	Status              string                `yaml:"status" json:"status"` // "complete" | "partial" | "blocked"
	Worktree            string                `yaml:"worktree,omitempty" json:"worktree,omitempty"`
	Branch              string                `yaml:"branch,omitempty" json:"branch,omitempty"`
	Commit              string                `yaml:"commit,omitempty" json:"commit,omitempty"`
	FilesChanged        []string              `yaml:"files_changed,omitempty" json:"files_changed,omitempty"`
	FilesCreated        []string              `yaml:"files_created,omitempty" json:"files_created,omitempty"`
	InterfaceDeviations []InterfaceDeviation  `yaml:"interface_deviations,omitempty" json:"interface_deviations,omitempty"`
	OutOfScopeDeps      []string              `yaml:"out_of_scope_deps,omitempty" json:"out_of_scope_deps,omitempty"`
	TestsAdded          []string              `yaml:"tests_added,omitempty" json:"tests_added,omitempty"`
	Verification        string                `yaml:"verification,omitempty" json:"verification,omitempty"`
	FailureType         string                `yaml:"failure_type,omitempty" json:"failure_type,omitempty"`
	Notes               string                `yaml:"notes,omitempty" json:"notes,omitempty"`
	Repo                string                `yaml:"repo,omitempty" json:"repo,omitempty"`
}

// InterfaceDeviation records a deviation from the planned interface contract.
// It tracks which downstream agents are affected by the deviation.
type InterfaceDeviation struct {
	Description              string   `yaml:"description" json:"description"`
	DownstreamActionRequired bool     `yaml:"downstream_action_required" json:"downstream_action_required"`
	Affects                  []string `yaml:"affects,omitempty" json:"affects,omitempty"`
}

// InterfaceContract defines an expected interface or API between agents or systems.
// It is used to coordinate expected behavior across parallel waves.
type InterfaceContract struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Definition  string `yaml:"definition" json:"definition"`
	Location    string `yaml:"location" json:"location"`
}

// QualityGates defines the set of checks that must pass before a wave is considered complete.
// Gates can be build, lint, or test commands.
type QualityGates struct {
	Level string        `yaml:"level" json:"level"` // "quick" | "standard" | "full"
	Gates []QualityGate `yaml:"gates" json:"gates"`
}

// QualityGate represents a single quality check (build, lint, test, etc.).
// Gates marked as Required must pass; others are advisory.
type QualityGate struct {
	Type        string `yaml:"type" json:"type"` // "build" | "lint" | "test"
	Command     string `yaml:"command" json:"command"`
	Required    bool   `yaml:"required" json:"required"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Repo        string `yaml:"repo,omitempty" json:"repo,omitempty"` // if set, gate only runs in this repo
}

// ScaffoldFile represents a type scaffold file that is created before wave execution.
// Scaffolds allow parallel agents to reference shared types without race conditions.
type ScaffoldFile struct {
	FilePath   string `yaml:"file_path" json:"file_path"`
	Contents   string `yaml:"contents,omitempty" json:"contents,omitempty"`
	ImportPath string `yaml:"import_path,omitempty" json:"import_path,omitempty"`
	Status     string `yaml:"status,omitempty" json:"status,omitempty"` // "pending" | "committed"
	Commit     string `yaml:"commit,omitempty" json:"commit,omitempty"`
}

// PreMortem records potential failure modes and their mitigations.
// It captures medium/high-risk scenarios identified during the scout phase.
type PreMortem struct {
	OverallRisk string         `yaml:"overall_risk" json:"overall_risk"` // "low" | "medium" | "high"
	Rows        []PreMortemRow `yaml:"rows" json:"rows"`
}

// PreMortemRow represents a single failure scenario and its mitigation strategy.
type PreMortemRow struct {
	Scenario   string `yaml:"scenario" json:"scenario"`
	Likelihood string `yaml:"likelihood" json:"likelihood"`
	Impact     string `yaml:"impact" json:"impact"`
	Mitigation string `yaml:"mitigation" json:"mitigation"`
}

// KnownIssue records an issue discovered during scout phase with status and workaround.
type KnownIssue struct {
	Title       string `yaml:"title,omitempty" json:"title,omitempty"`
	Description string `yaml:"description" json:"description"`
	Status      string `yaml:"status,omitempty" json:"status,omitempty"`
	Workaround  string `yaml:"workaround,omitempty" json:"workaround,omitempty"`
}

// ValidationError represents a structured validation error from manifest validation.
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// ProtocolState represents the current state of the IMPL manifest in the SAW protocol state machine.
type ProtocolState string

const (
	StateScoutPending    ProtocolState = "SCOUT_PENDING"
	StateScoutValidating ProtocolState = "SCOUT_VALIDATING"
	StateReviewed        ProtocolState = "REVIEWED"
	StateScaffoldPending ProtocolState = "SCAFFOLD_PENDING"
	StateWavePending     ProtocolState = "WAVE_PENDING"
	StateWaveExecuting   ProtocolState = "WAVE_EXECUTING"
	StateWaveMerging     ProtocolState = "WAVE_MERGING"
	StateWaveVerified    ProtocolState = "WAVE_VERIFIED"
	StateBlocked         ProtocolState = "BLOCKED"
	StateComplete        ProtocolState = "COMPLETE"
	StateNotSuitable     ProtocolState = "NOT_SUITABLE"
)

// MergeState represents the state of the merge operation for a wave.
type MergeState string

const (
	MergeStateIdle       MergeState = "idle"
	MergeStateInProgress MergeState = "in_progress"
	MergeStateCompleted  MergeState = "completed"
	MergeStateFailed     MergeState = "failed"
)
