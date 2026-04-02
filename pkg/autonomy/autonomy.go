package autonomy

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

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
	MaxQueueDepth  int   `json:"max_queue_depth" yaml:"max_queue_depth"` // Reserved for future queue depth limiting; not currently enforced
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
//
// Behavior matrix:
//   - gated:      all stages → false
//   - supervised: impl_review → false; wave_advance, gate_failure, queue_advance → true
//   - autonomous: all stages → true
func ShouldAutoApprove(level Level, stage Stage) bool {
	switch level {
	case LevelGated:
		return false
	case LevelAutonomous:
		return true
	case LevelSupervised:
		switch stage {
		case StageIMPLReview:
			return false
		case StageWaveAdvance, StageGateFailure, StageQueueAdvance:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// EffectiveLevel returns the effective autonomy level. If overrideLevel is a
// non-empty, valid Level string it takes precedence over cfg.Level. Otherwise
// cfg.Level is returned.
func EffectiveLevel(cfg Config, overrideLevel string) Level {
	if overrideLevel != "" {
		if r := ParseLevel(overrideLevel); r.IsSuccess() {
			return r.GetData().Level
		}
	}
	return cfg.Level
}

// ParseLevelData holds the result of a successful ParseLevel call.
type ParseLevelData struct {
	Level Level
}

// ParseLevel validates that s is a known autonomy level and returns it.
// Returns a Result with ParseLevelData on success, or a FATAL result with
// structured error on failure.
func ParseLevel(s string) result.Result[ParseLevelData] {
	switch Level(s) {
	case LevelGated, LevelSupervised, LevelAutonomous:
		return result.NewSuccess(ParseLevelData{Level: Level(s)})
	default:
		return result.NewFailure[ParseLevelData]([]result.SAWError{
			result.NewFatal("AUTONOMY_INVALID_LEVEL",
				fmt.Sprintf("unknown level %q (valid: gated, supervised, autonomous)", s)),
		})
	}
}
