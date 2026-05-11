package engine

import "github.com/blackwell-systems/polywave-go/pkg/protocol"

// ScoutData contains metadata from a successful RunScout operation.
type ScoutData struct {
	IMPLOutPath string `json:"impl_out_path"`
	Feature     string `json:"feature"`
}

// PlannerData contains metadata from a successful RunPlanner operation.
type PlannerData struct {
	ProgramOutPath string `json:"program_out_path"`
	Description    string `json:"description"`
}

// StartWaveData contains metadata from a successful StartWave operation.
type StartWaveData struct {
	Slug   string `json:"slug"`
	Waves  int    `json:"waves"`
	Agents int    `json:"agents"`
}

// ScaffoldData contains metadata from a successful RunScaffold operation.
type ScaffoldData struct {
	IMPLPath       string `json:"impl_path"`
	ScaffoldsFound int    `json:"scaffolds_found"`
}

// WaveData contains metadata from a successful RunSingleWave operation.
type WaveData struct {
	IMPLPath string `json:"impl_path"`
	WaveNum  int    `json:"wave_num"`
}

// AgentData contains metadata from a successful RunSingleAgent operation.
type AgentData struct {
	IMPLPath    string `json:"impl_path"`
	WaveNum     int    `json:"wave_num"`
	AgentLetter string `json:"agent_letter"`
}

// MergeData contains metadata from a successful MergeWave operation.
type MergeData struct {
	IMPLPath string `json:"impl_path"`
	WaveNum  int    `json:"wave_num"`
}

// VerificationData contains metadata from a successful RunVerification operation.
type VerificationData struct {
	RepoPath    string `json:"repo_path"`
	TestCommand string `json:"test_command"`
}

// UpdateData contains metadata from a successful UpdateIMPLStatus operation.
type UpdateData struct {
	IMPLDocPath      string   `json:"impl_doc_path"`
	CompletedLetters []string `json:"completed_letters"`
}

// ValidateData contains metadata from a successful ValidateInvariants operation.
type ValidateData struct {
	Valid bool `json:"valid"`
}

// CheckData contains metadata from a successful checkPreviousWaveVerified operation.
type CheckData struct {
	PrevWaveNum int `json:"prev_wave_num"`
}

// VerifyHookData contains metadata from a successful verifyHookInWorktree operation.
type VerifyHookData struct {
	WorktreePath string `json:"worktree_path"`
}

// MarkCompleteData contains metadata from a successful MarkIMPLComplete operation.
type MarkCompleteData struct {
	IMPLPath string `json:"impl_path"`
	Date     string `json:"date"`
}

// ChatData contains metadata from a successful RunChat operation.
type ChatData struct {
	IMPLPath string `json:"impl_path"`
	Message  string `json:"message"`
}

// DaemonData contains metadata from a completed RunDaemon operation.
type DaemonData struct {
	RepoPath       string `json:"repo_path"`
	CompletedCount int    `json:"completed_count"`
}

// FixBuildData contains metadata from a successful FixBuildFailure operation.
type FixBuildData struct {
	IMPLPath string `json:"impl_path"`
	WaveNum  int    `json:"wave_num"`
	GateType string `json:"gate_type"`
}

// IntegrationAgentData contains metadata from a successful RunIntegrationAgent operation.
type IntegrationAgentData struct {
	IMPLPath string `json:"impl_path"`
	WaveNum  int    `json:"wave_num"`
	GapCount int    `json:"gap_count"`
}

// --- Typed event payload structs for engine event bus (Issue 19) ---
// These replace ad-hoc map[string]string / map[string]interface{} payloads
// in runner.go publish calls. engine.Event.Data is typed as any.

// RunStartedPayload is the Data for "run_started" events.
type RunStartedPayload struct {
	Slug     string `json:"slug"`
	IMPLPath string `json:"impl_path"`
}

// RunFailedPayload is the Data for "run_failed" events.
type RunFailedPayload struct {
	Error string `json:"error"`
}

// ScaffoldStartedPayload is the Data for "scaffold_started" events.
type ScaffoldStartedPayload struct {
	IMPLPath string `json:"impl_path"`
}

// ScaffoldOutputPayload is the Data for "scaffold_output" events.
type ScaffoldOutputPayload struct {
	Chunk string `json:"chunk"`
}

// ScaffoldFailedPayload is the Data for "scaffold_failed" events.
type ScaffoldFailedPayload struct {
	Error string `json:"error"`
}

// ScaffoldCompletePayload is the Data for "scaffold_complete" events.
type ScaffoldCompletePayload struct {
	IMPLPath string `json:"impl_path"`
}

// IntegrationGapsPayload is the Data for "integration_gaps_detected" events.
// Fields: wave (int), gaps (count int), report (*protocol.IntegrationReport).
type IntegrationGapsPayload struct {
	Wave   int                          `json:"wave"`
	Gaps   int                          `json:"gaps"`
	Report *protocol.IntegrationReport `json:"report,omitempty"`
}

// IntegrationAgentWarningPayload is the Data for "integration_agent_warning" events.
type IntegrationAgentWarningPayload struct {
	Error string `json:"error"`
}

// UpdateStatusFailedPayload is the Data for "update_status_failed" events.
// Slug is the IMPL slug string identifying which IMPL failed the status update.
type UpdateStatusFailedPayload struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

// WaveGatePendingPayload is the Data for "wave_gate_pending" events.
type WaveGatePendingPayload struct {
	Wave     int    `json:"wave"`
	NextWave int    `json:"next_wave"`
	Slug     string `json:"slug"`
}

// WaveGateResolvedPayload is the Data for "wave_gate_resolved" events.
type WaveGateResolvedPayload struct {
	Wave   int    `json:"wave"`
	Action string `json:"action"`
	Slug   string `json:"slug"`
}
