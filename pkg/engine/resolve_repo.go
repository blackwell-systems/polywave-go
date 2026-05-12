package engine

import (
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// resolvePolywaveRepo resolves the Polywave implementation repo path using the
// standard 3-step fallback chain:
//  1. explicit path from opts
//  2. $POLYWAVE_REPO environment variable
//  3. ~/code/polywave (default)
//
// Returns the resolved path or a fatal result error.
func resolvePolywaveRepo(explicit string) (string, *result.PolywaveError) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv("POLYWAVE_REPO"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		e := result.NewFatal(result.CodeContextError, "cannot determine home directory").WithCause(err)
		return "", &e
	}
	return filepath.Join(home, "code", "polywave"), nil
}

// agentPromptPath returns the absolute path to a named agent prompt file within
// the Polywave implementation repo.
//
// Example: agentPromptPath("/home/user/code/polywave", "scout.md")
// returns "/home/user/code/polywave/implementations/claude-code/prompts/agents/scout.md"
func agentPromptPath(polywaveRepo, filename string) string {
	return filepath.Join(polywaveRepo, "implementations", "claude-code", "prompts", "agents", filename)
}
