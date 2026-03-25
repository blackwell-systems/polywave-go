package engine

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// RunTestCommand loads the IMPL manifest from implPath, extracts the
// test_command, and runs it in repoPath via "sh -c". Output is streamed
// line-by-line through the onOutput callback. The process is killed on
// context cancellation. Returns nil on success; on failure the error
// includes accumulated output for diagnostics.
func RunTestCommand(ctx context.Context, implPath, repoPath string, onOutput func(line string)) error {
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	testCommand := manifest.TestCommand
	if testCommand == "" {
		return fmt.Errorf("no test_command defined in IMPL manifest")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", testCommand)
	cmd.Dir = repoPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Combine stdout and stderr for streaming.
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout pipe

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start test command: %w", err)
	}

	// Stream output line-by-line.
	var outputLines []string
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		outputLines = append(outputLines, line)
		if onOutput != nil {
			onOutput(line)
		}
	}

	// Wait for the command to finish.
	if err := cmd.Wait(); err != nil {
		// On context cancellation, kill the process group.
		if ctx.Err() != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			return fmt.Errorf("test command cancelled: %w", ctx.Err())
		}
		accumulated := strings.Join(outputLines, "\n")
		return fmt.Errorf("test command failed: %w\nOutput:\n%s", err, accumulated)
	}

	return nil
}
