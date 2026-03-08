package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	runVerificationFunc = runVerification
}

// runVerification runs go vet then testCommand in o.repoPath.
// Returns nil only when both pass.
func runVerification(o *Orchestrator, testCommand string) error {
	// Lint pass: go vet ./... (skip if no go.mod in repoPath — e.g. in tests)
	if _, err := os.Stat(filepath.Join(o.repoPath, "go.mod")); err == nil {
		vet := exec.Command("go", "vet", "./...")
		vet.Dir = o.repoPath
		if out, err := vet.CombinedOutput(); err != nil {
			return fmt.Errorf("runVerification: go vet failed: %w\noutput: %s", err, string(out))
		}
	}

	parts := strings.Fields(testCommand)
	if len(parts) == 0 {
		return fmt.Errorf("runVerification: empty test command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = o.repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("runVerification: command %q failed: %w\noutput: %s", testCommand, err, string(out))
	}

	return nil
}
