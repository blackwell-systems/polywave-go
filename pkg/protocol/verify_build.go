package protocol

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// VerifyBuildData captures the outcome of running test and lint commands
// from the IMPL manifest's top-level test_command and lint_command fields.
type VerifyBuildData struct {
	TestCommand string `json:"test_command"`
	LintCommand string `json:"lint_command"`
	TestPassed  bool   `json:"test_passed"`
	LintPassed  bool   `json:"lint_passed"`
	TestOutput  string `json:"test_output,omitempty"`
	LintOutput  string `json:"lint_output,omitempty"`
}

// VerifyBuild loads the IMPL manifest and runs the test and lint commands.
// It returns pass/fail status and combined stdout+stderr for each command.
//
// Command selection: quality_gates entries (type "test" / "lint") take precedence
// over the top-level test_command / lint_command fields. Quality gates are set by
// the scout after examining the repo's CI config and typically exclude integration-
// only packages (e.g. ./test/). When a matching quality gate is present it is used
// as the canonical command; test_command / lint_command are fallbacks.
//
// If a command is an empty string, it is skipped and marked as passed.
// The repoDir parameter is the working directory for command execution.
//
// Returns Result[VerifyBuildData] with all execution details.
// Returns FATAL result for system-level failures (e.g., cannot load manifest).
func VerifyBuild(ctx context.Context, manifestPath string, repoDir string) result.Result[VerifyBuildData] {
	// Load the manifest
	manifest, err := Load(ctx, manifestPath)
	if err != nil {
		return result.NewFailure[VerifyBuildData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  fmt.Sprintf("failed to load manifest: %v", err),
				Severity: "fatal",
				File:     manifestPath,
			},
		})
	}

	// Prefer quality gate commands over top-level test_command / lint_command.
	// Quality gates are derived from the repo's actual CI config by the scout and
	// explicitly scope out integration-only packages. test_command is the fallback.
	testCmd := manifest.TestCommand
	lintCmd := manifest.LintCommand
	testGateRepo := ""
	lintGateRepo := ""
	if manifest.QualityGates != nil {
		for _, gate := range manifest.QualityGates.Gates {
			if gate.Type == "test" && gate.Command != "" && testCmd == manifest.TestCommand {
				testCmd = gate.Command
				testGateRepo = gate.Repo
			}
			if gate.Type == "lint" && gate.Command != "" && lintCmd == manifest.LintCommand {
				lintCmd = gate.Command
				lintGateRepo = gate.Repo
			}
		}
	}
	configRepos := loadSAWConfigRepos(filepath.Dir(manifestPath))
	testDir := repoDir
	if testGateRepo != "" {
		if p, ok := configRepos[testGateRepo]; ok {
			testDir = p
		}
	}
	lintDir := repoDir
	if lintGateRepo != "" {
		if p, ok := configRepos[lintGateRepo]; ok {
			lintDir = p
		}
	}

	data := VerifyBuildData{
		TestCommand: testCmd,
		LintCommand: lintCmd,
	}

	// Run test command if present and applicable to this repo
	if isRealCommand(testCmd) && commandApplies(testCmd, testDir) {
		testPassed, testOutput := runCommand(ctx, testCmd, testDir)
		data.TestPassed = testPassed
		data.TestOutput = testOutput
	} else {
		// Empty command or not applicable to this repo: skip and mark as passed
		data.TestPassed = true
	}

	// Run lint command if present and applicable to this repo
	if isRealCommand(lintCmd) && commandApplies(lintCmd, lintDir) {
		lintPassed, lintOutput := runCommand(ctx, lintCmd, lintDir)
		data.LintPassed = lintPassed
		data.LintOutput = lintOutput
	} else {
		// Empty command or not applicable to this repo: skip and mark as passed
		data.LintPassed = true
	}

	return result.NewSuccess(data)
}

// isRealCommand returns false for empty strings and sentinel values like "none"
// that indicate no command should be run.
func isRealCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(cmd))
	return lower != "none" && lower != "n/a" && lower != "skip"
}

// commandApplies returns false when a command is ecosystem-specific but the repo
// doesn't support that ecosystem. Currently handles Go: if the command starts with
// "go " and there is no go.mod in repoDir, the command is not applicable.
func commandApplies(command, repoDir string) bool {
	if strings.HasPrefix(command, "go ") {
		_, err := os.Stat(filepath.Join(repoDir, "go.mod"))
		return err == nil
	}
	return true
}

// resolveCommandPaths rewrites "cd <relative-path> [&& ...]" to use an
// absolute path. This ensures commands like "cd web && npx vitest run"
// behave identically to quality gate commands that use absolute paths.
// Commands that already use absolute paths, or don't start with "cd ", are
// returned unchanged.
func resolveCommandPaths(command, repoDir string) string {
	if !strings.HasPrefix(command, "cd ") {
		return command
	}
	rest := strings.TrimPrefix(command, "cd ")
	// Split at first whitespace or shell operator to isolate the directory
	idx := strings.IndexAny(rest, " \t&;|")
	if idx == -1 {
		dir := rest
		if !filepath.IsAbs(dir) {
			return "cd " + filepath.Join(repoDir, dir)
		}
		return command
	}
	dir := rest[:idx]
	suffix := rest[idx:]
	if filepath.IsAbs(dir) {
		return command
	}
	return "cd " + filepath.Join(repoDir, dir) + suffix
}

// runCommand executes a shell command and returns (passed, combinedOutput).
// Follows the exact pattern from gates.go: sh -c, combined stdout+stderr, exit code check.
func runCommand(ctx context.Context, command string, repoDir string) (bool, string) {
	cmd := exec.CommandContext(ctx, "sh", "-c", resolveCommandPaths(command, repoDir))
	cmd.Dir = repoDir

	// Capture stdout and stderr combined
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Execute the command
	err := cmd.Run()

	// Determine pass/fail status
	if err != nil {
		// Command failed
		return false, output.String()
	}

	// Command succeeded
	return true, output.String()
}
