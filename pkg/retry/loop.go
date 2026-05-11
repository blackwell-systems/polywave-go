package retry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// RetryLoop manages the E24 verification loop. When a quality gate fails after
// wave merge, it generates a single-agent retry IMPL doc targeting the failed
// files. Callers are responsible for executing the agent; RetryLoop only generates
// the IMPL doc and tracks attempt state.
type RetryLoop struct {
	cfg     RetryConfig
	attempt int
}

// NewRetryLoop creates a new RetryLoop with the given configuration.
// If cfg.MaxRetries is zero or negative, it defaults to 2 (so the 3rd
// failure transitions to blocked state).
func NewRetryLoop(cfg RetryConfig) *RetryLoop {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	return &RetryLoop{cfg: cfg}
}

// Run generates a retry IMPL doc for the failed quality gate, saves it to
// docs/IMPL/IMPL-{parentSlug}-retry-{attempt}.yaml (relative to RepoPath),
// and returns a *RetryAttempt describing the outcome.
//
// It does NOT execute the retry agent — that is the caller's responsibility.
//
// ctx is checked for cancellation before each significant step; if cancelled,
// Run returns a fatal Result wrapping ctx.Err().
//
// State transitions:
//   - attempt < MaxRetries → FinalState = "retrying" (more attempts available)
//   - attempt >= MaxRetries → FinalState = "blocked" (max retries exhausted)
//
// onEvent is called with lifecycle events:
//   - "retry_started"  when beginning a retry attempt
//   - "retry_blocked"  when max retries are exceeded (no IMPL saved)
func (rl *RetryLoop) Run(ctx context.Context, failedGate QualityGateFailure, onEvent func(Event)) result.Result[*RetryAttempt] {
	// Check for cancellation before doing any work.
	select {
	case <-ctx.Done():
		return result.NewFailure[*RetryAttempt]([]result.PolywaveError{
			result.NewFatal("RETRY_CONTEXT_CANCELLED", ctx.Err().Error()).WithCause(ctx.Err()),
		})
	default:
	}

	rl.attempt++

	if onEvent != nil {
		onEvent(Event{
			Event: "retry_started",
			Data: map[string]interface{}{
				"attempt":     rl.attempt,
				"max_retries": rl.cfg.MaxRetries,
				"gate_type":   failedGate.GateType,
			},
		})
	}

	// Determine the parent slug from IMPLPath or use a default.
	parentSlug := slugFromIMPLPath(rl.cfg.IMPLPath)

	// Check if we've exceeded the retry limit.
	if rl.attempt > rl.cfg.MaxRetries {
		if onEvent != nil {
			onEvent(Event{
				Event: "retry_blocked",
				Data: map[string]interface{}{
					"attempt":     rl.attempt,
					"max_retries": rl.cfg.MaxRetries,
				},
			})
		}
		return result.NewSuccess(&RetryAttempt{
			AttemptNumber: rl.attempt,
			AgentID:       "R",
			GatePassed:    false,
			GateOutput:    failedGate.Output,
			FinalState:    "blocked",
		})
	}

	// Determine which files to target — prefer explicit FailedFiles, fall back
	// to all files owned by agents in the parent IMPL.
	failedFiles := failedGate.FailedFiles
	if len(failedFiles) == 0 {
		failedFiles = rl.filesFromIMPL(ctx, onEvent)
	}

	// Generate the retry IMPL manifest.
	retryManifest := rl.GenerateRetryIMPL(ctx, failedFiles, failedGate.Output)

	// Compute the output path relative to RepoPath.
	retrySlug := fmt.Sprintf("%s-retry-%d", parentSlug, rl.attempt)
	saveRes := rl.saveRetryIMPL(ctx, retryManifest, retrySlug)
	if saveRes.IsFatal() {
		return result.NewFailure[*RetryAttempt]([]result.PolywaveError{
			result.NewFatal(result.CodeRetrySaveIMPLFailed, fmt.Sprintf("failed to save retry IMPL: %s", saveRes.Errors[0].Message)).WithCause(saveRes.Errors[0]),
		})
	}
	retryIMPLPath := saveRes.GetData()

	finalState := "retrying"

	return result.NewSuccess(&RetryAttempt{
		AttemptNumber: rl.attempt,
		AgentID:       "R",
		GatePassed:    false,
		GateOutput:    failedGate.Output,
		RetryIMPL:     retryIMPLPath,
		FinalState:    finalState,
	})
}

// GenerateRetryIMPL creates a minimal single-wave, single-agent IMPL manifest
// that targets the given failed files. The gateOutput is embedded in the
// agent task so the retry agent knows exactly what to fix.
//
// This method is also used directly by callers (e.g. CLI commands) who want
// to generate a retry IMPL without calling Run().
func (rl *RetryLoop) GenerateRetryIMPL(ctx context.Context, failedFiles []string, gateOutput string) *protocol.IMPLManifest {
	parentSlug := slugFromIMPLPath(rl.cfg.IMPLPath)
	gateCommand := gateCommandFromIMPL(ctx, rl.cfg.IMPLPath, nil)
	return GenerateRetryIMPL(parentSlug, rl.attempt, failedFiles, gateOutput, gateCommand)
}

// saveRetryIMPL writes the manifest to docs/IMPL/IMPL-{slug}.yaml under RepoPath.
// Creates the directory if it does not exist. Returns the relative path.
func (rl *RetryLoop) saveRetryIMPL(ctx context.Context, m *protocol.IMPLManifest, slug string) result.Result[string] {
	implDir := protocol.IMPLDir(rl.cfg.RepoPath)
	if err := os.MkdirAll(implDir, 0755); err != nil {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeRetryIMPLDirCreateFailed, fmt.Sprintf("cannot create IMPL dir %s: %s", implDir, err.Error())).WithCause(err),
		})
	}

	absPath := protocol.IMPLPath(rl.cfg.RepoPath, slug)
	if saveRes := protocol.Save(ctx, m, absPath); saveRes.IsFatal() {
		if len(saveRes.Errors) > 0 {
			return result.NewFailure[string]([]result.PolywaveError{
				result.NewFatal(result.CodeRetrySaveIMPLFailed, saveRes.Errors[0].Message).WithCause(saveRes.Errors[0]),
			})
		}
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeRetrySaveIMPLFailed, fmt.Sprintf("failed to save IMPL manifest to %s", absPath)),
		})
	}

	// Return the path relative to RepoPath for portability.
	relPath := filepath.Join("docs", "IMPL", fmt.Sprintf("IMPL-%s.yaml", slug))
	return result.NewSuccess(relPath)
}

// filesFromIMPL reads the parent IMPL manifest and collects all files owned by
// agents. Used as a fallback when QualityGateFailure.FailedFiles is empty.
func (rl *RetryLoop) filesFromIMPL(ctx context.Context, onEvent func(Event)) []string {
	if rl.cfg.IMPLPath == "" {
		return nil
	}
	m, err := protocol.Load(ctx, rl.cfg.IMPLPath)
	if err != nil {
		if onEvent != nil {
			onEvent(Event{
				Event: "retry_file_resolution_failed",
				Data: map[string]interface{}{
					"impl_path": rl.cfg.IMPLPath,
					"error":     err.Error(),
				},
			})
		}
		return nil
	}
	seen := make(map[string]bool)
	var files []string
	for _, fo := range m.FileOwnership {
		if !seen[fo.File] {
			seen[fo.File] = true
			files = append(files, fo.File)
		}
	}
	return files
}

// slugFromIMPLPath derives a feature slug from an IMPL file path.
// e.g. "docs/IMPL/IMPL-my-feature.yaml" → "my-feature"
// Falls back to "unknown" if the path is empty or has an unexpected format.
func slugFromIMPLPath(implPath string) string {
	if implPath == "" {
		return "unknown"
	}
	base := filepath.Base(implPath)
	// Strip extension
	name := strings.TrimSuffix(base, filepath.Ext(base))
	// Strip "IMPL-" prefix
	if strings.HasPrefix(name, "IMPL-") {
		return name[len("IMPL-"):]
	}
	return name
}

// gateCommandFromIMPL loads the parent IMPL manifest and returns its test_command,
// or a sensible default if the manifest cannot be read or has no test_command.
func gateCommandFromIMPL(ctx context.Context, implPath string, onEvent func(Event)) string {
	if implPath == "" {
		if onEvent != nil {
			onEvent(Event{
				Event: "retry_gate_command_fallback",
				Data: map[string]interface{}{
					"impl_path": implPath,
					"fallback":  "go build ./...",
				},
			})
		}
		return "go build ./..."
	}
	m, err := protocol.Load(ctx, implPath)
	if err != nil {
		if onEvent != nil {
			onEvent(Event{
				Event: "retry_gate_command_fallback",
				Data: map[string]interface{}{
					"impl_path": implPath,
					"fallback":  "go build ./...",
				},
			})
		}
		return "go build ./..."
	}
	if m.TestCommand != "" {
		return m.TestCommand
	}
	if onEvent != nil {
		onEvent(Event{
			Event: "retry_gate_command_fallback",
			Data: map[string]interface{}{
				"impl_path": implPath,
				"fallback":  "go build ./...",
				"reason":    "test_command empty in manifest",
			},
		})
	}
	return "go build ./..."
}
