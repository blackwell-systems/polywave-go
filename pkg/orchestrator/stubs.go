package orchestrator

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// RunStubScan implements E20: collects all files_changed and files_created from
// wave agent completion reports, invokes scan-stubs.sh, and appends the
// ## Stub Report — Wave {N} section to the IMPL doc at implDocPath.
//
// sawRepoPath locates scan-stubs.sh: falls back to $SAW_REPO env var, then
// ~/code/scout-and-wave (same fallback as RunScout).
//
// Always returns nil — stub detection is informational only (E20).
func RunStubScan(implDocPath string, waveNum int, reports map[string]*protocol.CompletionReport, sawRepoPath string) error {
	// 1. Collect the union of all FilesChanged and FilesCreated, deduplicated,
	//    skipping any files under docs/IMPL/.
	seen := make(map[string]struct{})
	var files []string
	for _, report := range reports {
		for _, f := range report.FilesChanged {
			if strings.HasPrefix(f, "docs/IMPL/") {
				continue
			}
			if _, ok := seen[f]; !ok {
				seen[f] = struct{}{}
				files = append(files, f)
			}
		}
		for _, f := range report.FilesCreated {
			if strings.HasPrefix(f, "docs/IMPL/") {
				continue
			}
			if _, ok := seen[f]; !ok {
				seen[f] = struct{}{}
				files = append(files, f)
			}
		}
	}

	// 2. Resolve sawRepoPath.
	if sawRepoPath == "" {
		sawRepoPath = os.Getenv("SAW_REPO")
	}
	if sawRepoPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			slog.Default().Warn("RunStubScan: could not determine home dir", "err", err)
			homeDir = "~"
		}
		sawRepoPath = filepath.Join(homeDir, "code", "scout-and-wave")
	}

	// 3. Locate scan-stubs.sh.
	scriptPath := filepath.Join(sawRepoPath, "implementations", "claude-code", "scripts", "scan-stubs.sh")

	// 4. If the script does not exist, write a stub report noting the missing script.
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		section := fmt.Sprintf("\n## Stub Report — Wave %d\n\nscan-stubs.sh not found at %s\n", waveNum, scriptPath)
		if appendErr := appendToFile(implDocPath, section); appendErr != nil {
			slog.Default().Warn("RunStubScan: failed to append stub report", "err", appendErr)
		}
		return nil
	}

	// 5. Run the script with files as arguments. If no files, still run to get a clean report.
	var output string
	if len(files) == 0 {
		output = ""
	} else {
		args := append([]string{scriptPath}, files...)
		cmd := exec.Command("bash", args...)
		cmd.Dir = filepath.Dir(implDocPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			// E20: exit code is always treated as 0 (informational only)
			slog.Default().Warn("RunStubScan: scan-stubs.sh exited with error (ignored)", "err", err)
		}
		output = strings.TrimSpace(string(out))
	}

	// 6. Format and append the stub report section.
	var body string
	if output == "" {
		body = "No stub patterns detected."
	} else {
		body = output
	}
	section := fmt.Sprintf("\n## Stub Report — Wave %d\n\n%s\n", waveNum, body)
	if appendErr := appendToFile(implDocPath, section); appendErr != nil {
		slog.Default().Warn("RunStubScan: failed to append stub report", "err", appendErr)
	}

	// 7. Always return nil — stub detection is informational only.
	return nil
}

// appendToFile opens the file at path in append mode and writes content.
func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
