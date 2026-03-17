package autonomy

// Level represents the autonomy level for orchestrator decision-making.
type Level string

const (
	LevelGated      Level = "gated"
	LevelSupervised Level = "supervised"
	LevelAutonomous Level = "autonomous"
)

// Stage represents a decision point where autonomy level is checked.
type Stage string

const (
	StageIMPLReview   Stage = "impl_review"
	StageWaveAdvance  Stage = "wave_advance"
	StageGateFailure  Stage = "gate_failure"
	StageQueueAdvance Stage = "queue_advance"
)

// Config holds autonomy configuration loaded from saw.config.json.
type Config struct {
	Level          Level `json:"level" yaml:"level"`
	MaxAutoRetries int   `json:"max_auto_retries" yaml:"max_auto_retries"`
	MaxQueueDepth  int   `json:"max_queue_depth" yaml:"max_queue_depth"`
}

// DefaultConfig returns the default autonomy configuration (gated level).
func DefaultConfig() Config {
	return Config{
		Level:          LevelGated,
		MaxAutoRetries: 2,
		MaxQueueDepth:  10,
	}
}

// ShouldAutoApprove returns true if the given stage should be auto-approved
// at the given autonomy level.
func ShouldAutoApprove(level Level, stage Stage) bool {
	return false // placeholder
}
