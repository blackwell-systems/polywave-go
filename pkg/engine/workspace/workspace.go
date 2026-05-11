package workspace

import (
	"fmt"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// WorkspaceManager manages workspace configuration files for a specific language
// so that LSP tools can resolve cross-package types in wave agent worktrees.
type WorkspaceManager interface {
	// Language returns the canonical name: "go", "typescript", "rust", "python"
	Language() string
	// Detect returns true if this manager applies to the repo at repoRoot.
	Detect(repoRoot string) bool
	// Setup modifies workspace configuration to include all worktreePaths for LSP resolution.
	// Backs up existing config to BackupDir(repoRoot, waveNum) before modification.
	Setup(repoRoot string, waveNum int, worktreePaths []string) error
	// Restore reverts workspace configuration to its pre-wave state.
	// If a backup exists, it is restored and deleted. If no backup, the SAW-created file is deleted.
	Restore(repoRoot string, waveNum int) error
}

// StepResult mirrors engine.StepResult to avoid a circular import.
type StepResult struct {
	Step   string
	Status string
	Detail string
}

// BackupDir returns the .polywave-state/wave{N}/ directory for backup files.
func BackupDir(repoRoot string, waveNum int) string {
	return filepath.Join(protocol.PolywaveStateDir(repoRoot), fmt.Sprintf("wave%d", waveNum))
}
