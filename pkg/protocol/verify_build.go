package protocol

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VerifyBuildResult captures the outcome of running test and lint commands
// from the IMPL manifest's top-level test_command and lint_command fields.
type VerifyBuildResult struct {
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
// Returns VerifyBuildResult with all execution details.
// Returns an error only for system-level failures (e.g., cannot load manifest).
func VerifyBuild(manifestPath string, repoDir string) (*VerifyBuildResult, error) {
	// Load the manifest
	manifest, err := Load(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	result := &VerifyBuildResult{
		TestCommand: manifest.TestCommand,
		LintCommand: manifest.LintCommand,
	}

	// Run test command if present and applicable to this repo
	if manifest.TestCommand != "" && commandApplies(manifest.TestCommand, repoDir) {
		testPassed, testOutput := runCommand(manifest.TestCommand, repoDir)
		result.TestPassed = testPassed
		result.TestOutput = testOutput
	} else {
		// Empty command or not applicable to this repo: skip and mark as passed
		result.TestPassed = true
	}

	// Run lint command if present and applicable to this repo
	if manifest.LintCommand != "" && commandApplies(manifest.LintCommand, repoDir) {
		lintPassed, lintOutput := runCommand(manifest.LintCommand, repoDir)
		result.LintPassed = lintPassed
		result.LintOutput = lintOutput
	} else {
		// Empty command or not applicable to this repo: skip and mark as passed
		result.LintPassed = true
	}

	return result, nil
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
