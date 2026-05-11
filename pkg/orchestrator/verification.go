package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func init() {
	runVerificationFunc = runVerification
}

// runVerification runs go vet then testCommand in o.repoPath.
// Returns failure only when either pass fails.
func runVerification(ctx context.Context, o *Orchestrator, testCommand string) result.Result[VerificationData] {
	// Lint pass: go vet ./... (skip if no go.mod in repoPath — e.g. in tests)
	if _, err := os.Stat(filepath.Join(o.repoPath, "go.mod")); err == nil {
		vet := exec.CommandContext(ctx, "go", "vet", "./...")
		vet.Dir = o.repoPath
		if out, err := vet.CombinedOutput(); err != nil {
			return result.NewFailure[VerificationData]([]result.PolywaveError{
				result.NewFatal(result.CodeLintFailed, fmt.Sprintf(
					"runVerification: go vet failed: %s\noutput: %s", err.Error(), string(out))),
			})
		}
	}

	parts := strings.Fields(testCommand)
	if len(parts) == 0 {
		return result.NewFailure[VerificationData]([]result.PolywaveError{
			result.NewFatal(result.CodeGateCommandMissing, "runVerification: empty test command"),
		})
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = o.repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return result.NewFailure[VerificationData]([]result.PolywaveError{
			result.NewFatal(result.CodeTestFailed, fmt.Sprintf(
				"runVerification: command %q failed: %s\noutput: %s", testCommand, err.Error(), string(out))),
		})
	}

	return result.NewSuccess(VerificationData{TestCommand: testCommand})
}
