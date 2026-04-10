package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SawConfigParser reads lintCommand, testCommand, and buildCommand from
// saw.config.json at the repo root. Priority 200 (above CI at 100 and
// Makefile at 50) makes saw.config.json the authoritative override for
// repos that need commands the CI parser cannot faithfully reproduce
// (e.g. commands that depend on job-level env vars not yet visible to
// the extractor, or non-standard toolchain invocations).
type SawConfigParser struct{}

// sawConfigCommands is the subset of saw.config.json fields read by this parser.
// Using a local struct avoids importing pkg/config and introducing a cycle.
type sawConfigCommands struct {
	BuildCommand string `json:"buildCommand"`
	TestCommand  string `json:"testCommand"`
	LintCommand  string `json:"lintCommand"`
}

// ParseBuildSystem implements BuildSystemParser.
func (p *SawConfigParser) ParseBuildSystem(repoRoot string) result.Result[ParseBuildSystemData] {
	configPath := filepath.Join(repoRoot, "saw.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file — not an error, just no override available.
		return result.NewSuccess(ParseBuildSystemData{CommandSet: nil})
	}

	var cfg sawConfigCommands
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Malformed JSON — skip silently; other parsers will take over.
		return result.NewSuccess(ParseBuildSystemData{CommandSet: nil})
	}

	if cfg.BuildCommand == "" && cfg.TestCommand == "" && cfg.LintCommand == "" {
		return result.NewSuccess(ParseBuildSystemData{CommandSet: nil})
	}

	cmdSet := &CommandSet{
		Toolchain:        detectToolchainFromCmds(cfg.BuildCommand, cfg.TestCommand, cfg.LintCommand),
		DetectionSources: []string{configPath},
		Commands: Commands{
			Build: cfg.BuildCommand,
			Test:  TestCommands{Full: cfg.TestCommand},
			Lint:  LintCommands{Check: cfg.LintCommand},
		},
	}
	return result.NewSuccess(ParseBuildSystemData{CommandSet: cmdSet})
}

// Priority returns 200 — higher than GithubActionsParser (100), MakefileParser (50),
// and PackageJSONParser (40). saw.config.json is always the explicit override.
func (p *SawConfigParser) Priority() int {
	return 200
}

// detectToolchainFromCmds infers the primary toolchain from up to three command strings.
func detectToolchainFromCmds(cmds ...string) string {
	for _, cmd := range cmds {
		lower := strings.ToLower(cmd)
		if strings.Contains(lower, "cargo ") || strings.Contains(lower, "rustc") {
			return "rust"
		}
		if strings.HasPrefix(lower, "go ") || strings.Contains(lower, " go ") ||
			strings.HasPrefix(lower, "gowork=") && strings.Contains(lower, " go ") {
			return "go"
		}
		if strings.Contains(lower, "npm ") || strings.Contains(lower, "node ") {
			return "node"
		}
		if strings.Contains(lower, "python ") || strings.Contains(lower, "pytest") {
			return "python"
		}
	}
	return ""
}
