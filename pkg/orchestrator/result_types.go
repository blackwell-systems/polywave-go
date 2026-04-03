// Package orchestrator — result data types for all orchestrator operations.
//
// Each exported function that returns result.Result[T] uses one of these
// data types as its type parameter T.
package orchestrator

// ValidateData is returned by validateManifestInvariantsAdapter on success.
type ValidateData struct {
	// WavesChecked is the number of waves whose file ownership was validated.
	WavesChecked int `json:"waves_checked"`
}

// ModelData is returned by validateModelName on success.
type ModelData struct {
	// Model is the validated model name.
	Model string `json:"model"`
}

// TransitionData is returned by TransitionTo on success.
type TransitionData struct {
	// From is the previous protocol state.
	From string `json:"from"`
	// To is the new protocol state after transition.
	To string `json:"to"`
}

// WaveData is returned by RunWave on success.
type WaveData struct {
	// WaveNum is the wave number that was executed.
	WaveNum int `json:"wave_num"`
	// AgentCount is the number of agents that were launched.
	AgentCount int `json:"agent_count"`
}

// AgentData is returned by RunAgent on success.
type AgentData struct {
	// WaveNum is the wave number.
	WaveNum int `json:"wave_num"`
	// AgentLetter is the agent letter (e.g. "A").
	AgentLetter string `json:"agent_letter"`
}

// MergeData is returned by MergeWave on success.
type MergeData struct {
	// WaveNum is the wave number that was merged.
	WaveNum int `json:"wave_num"`
}

// VerificationData is returned by RunVerification on success.
type VerificationData struct {
	// TestCommand is the command that was run.
	TestCommand string `json:"test_command"`
}

// UpdateData is returned by UpdateIMPLStatus on success.
type UpdateData struct {
	// WaveNum is the wave number whose status was updated.
	WaveNum int `json:"wave_num"`
	// CompletedAgents lists the agent letters that were marked complete.
	CompletedAgents []string `json:"completed_agents"`
}

// ConflictData is returned by predictConflicts on success.
type ConflictData struct {
	// FilesChecked is the number of unique files that were checked.
	FilesChecked int `json:"files_checked"`
}

// VerifyCommitsData is returned by verifyAgentCommits on success.
type VerifyCommitsData struct {
	// AgentsVerified is the number of agent branches that were verified.
	AgentsVerified int `json:"agents_verified"`
}

// UpdateContextData is returned by UpdateContextMD on success.
type UpdateContextData struct {
	// Slug is the feature slug that was recorded.
	Slug string `json:"slug"`
	// ContextPath is the path to docs/CONTEXT.md that was written.
	ContextPath string `json:"context_path"`
}

// PrepareContextData is returned by PrepareAgentContext on success.
type PrepareContextData struct {
	// ContextMD is the generated session context markdown (may be empty if no journal).
	ContextMD string `json:"context_md"`
}

// WriteJournalData is returned by WriteJournalEntry on success.
type WriteJournalData struct {
	// JournalPath is the path to the index.jsonl file that was written.
	JournalPath string `json:"journal_path"`
}

// RunStubData is returned by RunStubScan on success.
// RunStubScan is informational only (E20) and always succeeds.
type RunStubData struct {
	// WaveNum is the wave number that was scanned.
	WaveNum int `json:"wave_num"`
	// FilesScanned is the number of files that were scanned.
	FilesScanned int `json:"files_scanned"`
}
