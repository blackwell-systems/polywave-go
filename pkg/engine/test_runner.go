package engine

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// TestData holds data returned by RunTestCommand.
type TestData struct {
	Command     string   `json:"command"`
	OutputLines []string `json:"output_lines"`
}

// RunTestCommand loads the IMPL manifest from implPath, extracts the
// test_command, and runs it in repoPath via "sh -c". Output is streamed
// line-by-line through the onOutput callback. The process is killed on
// context cancellation. Returns fatal result on failure; on success the result
// includes accumulated output for diagnostics.
func RunTestCommand(ctx context.Context, implPath, repoPath string, onOutput func(line string)) result.Result[TestData] {
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		return result.NewFailure[TestData]([]result.SAWError{
			result.NewFatal(result.CodeTestLoadFailed,
				fmt.Sprintf("load manifest: %v", err)).
				WithContext("impl_path", implPath),
		})
	}

	testCommand := manifest.TestCommand
	if testCommand == "" {
		return result.NewFailure[TestData]([]result.SAWError{
			result.NewFatal(result.CodeTestNoCommand,
				"no test_command defined in IMPL manifest").
				WithContext("impl_path", implPath),
		})
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", testCommand)
	cmd.Dir = repoPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Combine stdout and stderr for streaming.
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return result.NewFailure[TestData]([]result.SAWError{
			result.NewFatal(result.CodeTestPipeFailed,
				fmt.Sprintf("create stdout pipe: %v", err)),
		})
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout pipe

	if err := cmd.Start(); err != nil {
		return result.NewFailure[TestData]([]result.SAWError{
			result.NewFatal(result.CodeTestStartFailed,
				fmt.Sprintf("start test command: %v", err)).
				WithContext("command", testCommand),
		})
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
			return result.NewFailure[TestData]([]result.SAWError{
				result.NewFatal(result.CodeContextCancelled,
					fmt.Sprintf("test command cancelled: %v", ctx.Err())).
					WithContext("command", testCommand),
			})
		}
		accumulated := strings.Join(outputLines, "\n")
		return result.NewFailure[TestData]([]result.SAWError{
			result.NewFatal(result.CodeTestCommandFailed,
				fmt.Sprintf("test command failed: %v\nOutput:\n%s", err, accumulated)).
				WithContext("command", testCommand),
		})
	}

	return result.NewSuccess(TestData{
		Command:     testCommand,
		OutputLines: outputLines,
	})
}
