package engine

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
