package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// GithubActionsParser extracts commands from .github/workflows/*.yml files
type GithubActionsParser struct{}

// ParseWorkflowData wraps the result of parseWorkflowFile operation
type ParseWorkflowData struct {
	Commands []string
}

// ParseCI implements CIParser interface
func (p *GithubActionsParser) ParseCI(repoRoot string) result.Result[ParseCIData] {
	workflowsDir := filepath.Join(repoRoot, ".github", "workflows")

	// Return nil (not error) when .github/workflows/ doesn't exist
	if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
		return result.NewSuccess(ParseCIData{CommandSet: nil})
	}

	// Find all YAML workflow files
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return result.NewFailure[ParseCIData]([]result.PolywaveError{
			result.NewFatal(result.CodeCommandExtractWorkflowRead, fmt.Sprintf("reading workflows directory: %v", err)),
		})
	}

	var allCommands []string
	var detectionSources []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		workflowPath := filepath.Join(workflowsDir, name)
		r := p.parseWorkflowFile(workflowPath)
		if r.IsFatal() {
			// Skip malformed files rather than failing entirely
			continue
		}
		commands := r.GetData().Commands

		if len(commands) > 0 {
			allCommands = append(allCommands, commands...)
			detectionSources = append(detectionSources, workflowPath)
		}
	}

	if len(allCommands) == 0 {
		return result.NewSuccess(ParseCIData{CommandSet: nil})
	}

	// Classify commands into build/test/lint/format
	cmdSet := &CommandSet{
		DetectionSources: detectionSources,
	}

	cmdSet.Commands = p.classifyCommands(allCommands)
	cmdSet.Toolchain = p.detectToolchain(allCommands)

	return result.NewSuccess(ParseCIData{CommandSet: cmdSet})
}

// Priority returns 100 (higher than Makefile)
func (p *GithubActionsParser) Priority() int {
	return 100
}

// parseWorkflowFile parses a single GitHub Actions workflow file
func (p *GithubActionsParser) parseWorkflowFile(path string) result.Result[ParseWorkflowData] {
	data, err := os.ReadFile(path)
	if err != nil {
		return result.NewFailure[ParseWorkflowData]([]result.PolywaveError{
			result.NewFatal(result.CodeCommandExtractWorkflowRead, fmt.Sprintf("reading workflow file %s: %v", path, err)),
		})
	}

	var workflow struct {
		Jobs map[string]struct {
			Env      map[string]string `yaml:"env"`
			Strategy *struct {
				Matrix map[string][]interface{} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
				Run string            `yaml:"run"`
				Env map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}

	// Cannot use protocol.LoadYAML: data is already-read bytes from the caller, not a file path.
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return result.NewFailure[ParseWorkflowData]([]result.PolywaveError{
			result.NewFatal(result.CodeCommandExtractWorkflowParse, fmt.Sprintf("parsing YAML in %s: %v", path, err)),
		})
	}

	var commands []string
	hostOS := runtime.GOOS

	for _, job := range workflow.Jobs {
		// Check if this job uses a matrix strategy
		if job.Strategy != nil && job.Strategy.Matrix != nil {
			// Try to find a matching OS in the matrix
			if osValues, ok := job.Strategy.Matrix["os"]; ok {
				matchesHost := false
				for _, osVal := range osValues {
					osStr, ok := osVal.(string)
					if !ok {
						continue
					}
					// Match host platform (darwin -> macos, linux -> ubuntu)
					if strings.Contains(osStr, "macos") && hostOS == "darwin" {
						matchesHost = true
						break
					}
					if strings.Contains(osStr, "ubuntu") && hostOS == "linux" {
						matchesHost = true
						break
					}
					if strings.Contains(osStr, "windows") && hostOS == "windows" {
						matchesHost = true
						break
					}
				}
				// Skip this job if it doesn't match our host OS
				if !matchesHost && len(osValues) > 0 {
					continue
				}
			}
		}

		// Extract commands from steps, merging job-level env with step-level env.
		// Job env is the base; step env overrides. The merged env is prepended
		// to each extracted command so the gate runner sees the correct environment
		// even when GOWORK or similar vars are declared at the job level.
		for _, step := range job.Steps {
			if step.Run != "" {
				merged := mergeEnvMaps(job.Env, step.Env)
				envPrefix := envMapToPrefix(merged)
				// Split multi-line run commands
				lines := strings.Split(step.Run, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					// Ignore empty lines and comments
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					if envPrefix != "" {
						line = envPrefix + line
					}
					commands = append(commands, line)
				}
			}
		}
	}

	return result.NewSuccess(ParseWorkflowData{Commands: commands})
}

// classifyCommands categorizes commands by pattern matching
func (p *GithubActionsParser) classifyCommands(commands []string) Commands {
	var result Commands

	for _, cmd := range commands {
		cmdLower := strings.ToLower(cmd)

		// Build commands
		if strings.Contains(cmdLower, "go build") ||
			strings.Contains(cmdLower, "cargo build") ||
			strings.Contains(cmdLower, "npm run build") {
			if result.Build == "" {
				result.Build = cmd
			}
		}

		// Test commands
		if strings.Contains(cmdLower, "go test") ||
			strings.Contains(cmdLower, "cargo test") ||
			strings.Contains(cmdLower, "npm test") ||
			strings.Contains(cmdLower, "pytest") {
			if result.Test.Full == "" {
				result.Test.Full = cmd
				// Extract focused pattern if present
				if strings.Contains(cmdLower, "go test") && strings.Contains(cmd, "-run") {
					result.Test.FocusedPattern = cmd
				}
			}
		}

		// Lint commands (check vs fix)
		if strings.Contains(cmdLower, "go vet") ||
			strings.Contains(cmdLower, "golangci-lint") ||
			strings.Contains(cmdLower, "cargo clippy") ||
			strings.Contains(cmdLower, "eslint") ||
			strings.Contains(cmdLower, "ruff check") {

			// Determine if this is check or fix mode
			if strings.Contains(cmd, "--fix") {
				if result.Lint.Fix == "" {
					result.Lint.Fix = cmd
				}
			} else {
				if result.Lint.Check == "" {
					result.Lint.Check = cmd
				}
			}
		}

		// Format commands (check vs fix)
		if strings.Contains(cmdLower, "gofmt") ||
			strings.Contains(cmdLower, "cargo fmt") ||
			strings.Contains(cmdLower, "prettier") ||
			strings.Contains(cmdLower, "black") {

			// Check for check mode flags
			if strings.Contains(cmd, "--check") || strings.Contains(cmd, "-l") {
				if result.Format.Check == "" {
					result.Format.Check = cmd
				}
			} else {
				if result.Format.Fix == "" {
					result.Format.Fix = cmd
				}
			}
		}
	}

	return result
}

// mergeEnvMaps merges base and override into a new map.
// Keys in override take precedence over keys in base.
func mergeEnvMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// envMapToPrefix converts an env map to a shell-inline prefix string like "KEY=val ".
// Keys are sorted for determinism. Values containing spaces or shell metacharacters
// are not quoted; callers should ensure values are safe for inline use (typical CI
// env vars like GOWORK=off are always simple).
func envMapToPrefix(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+env[k])
	}
	return strings.Join(parts, " ") + " "
}

// detectToolchain infers the primary toolchain from commands
func (p *GithubActionsParser) detectToolchain(commands []string) string {
	goCount := 0
	rustCount := 0
	nodeCount := 0
	pythonCount := 0

	for _, cmd := range commands {
		cmdLower := strings.ToLower(cmd)

		// Check Rust first (to avoid "cargo" matching "go")
		if strings.Contains(cmdLower, "cargo ") || strings.Contains(cmdLower, "rustc") {
			rustCount++
			continue
		}

		// Check Go (use word boundaries to avoid matching "cargo")
		if strings.HasPrefix(cmdLower, "go ") ||
		   strings.Contains(cmdLower, " go ") ||
		   strings.HasPrefix(cmdLower, "gofmt") {
			goCount++
			continue
		}

		// Check Node/npm
		if strings.Contains(cmdLower, "npm ") || strings.Contains(cmdLower, "node ") {
			nodeCount++
			continue
		}

		// Check Python
		if strings.Contains(cmdLower, "python ") || strings.Contains(cmdLower, "pytest") {
			pythonCount++
			continue
		}
	}

	// Find the toolchain with the highest count.
	// Tie-breaking priority order: Go > Rust > Node > Python.
	// Each language block uses strictly-greater-than (>) so in ties the first
	// language encountered in this order wins.
	max := 0
	toolchain := ""

	if goCount > max {
		max = goCount
		toolchain = "go"
	}
	if rustCount > max {
		max = rustCount
		toolchain = "rust"
	}
	if nodeCount > max {
		max = nodeCount
		toolchain = "node"
	}
	if pythonCount > max {
		max = pythonCount
		toolchain = "python"
	}

	// Default to empty string if no toolchain detected
	return toolchain
}
