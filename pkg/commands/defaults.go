package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

// LanguageDefaults is a variable that holds the language defaults implementation.
// It can be reassigned for testing purposes.
var LanguageDefaults = languageDefaultsImpl

// languageDefaultsImpl detects the project's toolchain from marker files
// and returns standard commands for that language. This is the lowest-priority
// fallback (priority 0) used when no CI or build system configs are found.
func languageDefaultsImpl(repoRoot string) (*CommandSet, error) {
	// Check for Go
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
		return &CommandSet{
			Toolchain: "go",
			Commands: Commands{
				Build: "go build ./...",
				Test: TestCommands{
					Full:           "go test ./...",
					FocusedPattern: "go test ./{package} -run {test_name}",
				},
				Lint: LintCommands{
					Check: "go vet ./...",
					Fix:   "",
				},
				Format: FormatCommands{
					Check: "",
					Fix:   "gofmt -w .",
				},
			},
			DetectionSources: []string{"go.mod"},
			ModuleMap:        []Module{},
		}, nil
	}

	// Check for Rust
	if _, err := os.Stat(filepath.Join(repoRoot, "Cargo.toml")); err == nil {
		return &CommandSet{
			Toolchain: "rust",
			Commands: Commands{
				Build: "cargo build",
				Test: TestCommands{
					Full:           "cargo test",
					FocusedPattern: "cargo test {test_name}",
				},
				Lint: LintCommands{
					Check: "cargo clippy -- -D warnings",
					Fix:   "",
				},
				Format: FormatCommands{
					Check: "cargo fmt -- --check",
					Fix:   "cargo fmt",
				},
			},
			DetectionSources: []string{"Cargo.toml"},
			ModuleMap:        []Module{},
		}, nil
	}

	// Check for Node
	if _, err := os.Stat(filepath.Join(repoRoot, "package.json")); err == nil {
		return &CommandSet{
			Toolchain: "node",
			Commands: Commands{
				Build: "npm run build",
				Test: TestCommands{
					Full:           "npm test",
					FocusedPattern: "npm test -- {test_name}",
				},
				Lint: LintCommands{
					Check: "npm run lint",
					Fix:   "",
				},
				Format: FormatCommands{
					Check: "npm run format:check",
					Fix:   "",
				},
			},
			DetectionSources: []string{"package.json"},
			ModuleMap:        []Module{},
		}, nil
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(repoRoot, "pyproject.toml")); err == nil {
		return &CommandSet{
			Toolchain: "python",
			Commands: Commands{
				Build: "", // Python is interpreted - no build step
				Test: TestCommands{
					Full:           "pytest",
					FocusedPattern: "pytest {test_file}::{test_name}",
				},
				Lint: LintCommands{
					Check: "ruff check .",
					Fix:   "",
				},
				Format: FormatCommands{
					Check: "",
					Fix:   "black .",
				},
			},
			DetectionSources: []string{"pyproject.toml"},
			ModuleMap:        []Module{},
		}, nil
	}

	// No toolchain detected
	return nil, fmt.Errorf("no toolchain detected: no go.mod, Cargo.toml, package.json, or pyproject.toml found in %s", repoRoot)
}
