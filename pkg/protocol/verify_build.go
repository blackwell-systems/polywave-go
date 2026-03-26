package protocol

import (
	"bytes"
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

// VerifyBuild loads the IMPL manifest and runs the test_command and lint_command.
// It returns pass/fail status and combined stdout+stderr for each command.
//
// If a command is an empty string, it is skipped and marked as passed.
// The repoDir parameter is the working directory for command execution.
//
// Returns Result[VerifyBuildData] with all execution details.
// Returns FATAL result for system-level failures (e.g., cannot load manifest).
func VerifyBuild(manifestPath string, repoDir string) result.Result[VerifyBuildData] {
	// Load the manifest
	manifest, err := Load(manifestPath)
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

	data := VerifyBuildData{
		TestCommand: manifest.TestCommand,
		LintCommand: manifest.LintCommand,
	}

	// Run test command if present and applicable to this repo
	if isRealCommand(manifest.TestCommand) && commandApplies(manifest.TestCommand, repoDir) {
		testPassed, testOutput := runCommand(manifest.TestCommand, repoDir)
		data.TestPassed = testPassed
		data.TestOutput = testOutput
	} else {
		// Empty command or not applicable to this repo: skip and mark as passed
		data.TestPassed = true
	}

	// Run lint command if present and applicable to this repo
	if isRealCommand(manifest.LintCommand) && commandApplies(manifest.LintCommand, repoDir) {
		lintPassed, lintOutput := runCommand(manifest.LintCommand, repoDir)
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
func runCommand(command string, repoDir string) (bool, string) {
	cmd := exec.Command("sh", "-c", resolveCommandPaths(command, repoDir))
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
