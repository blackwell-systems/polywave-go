package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// LanguageDefaultsData wraps the result of LanguageDefaults operation
type LanguageDefaultsData struct {
	CommandSet *CommandSet
}

// LanguageDefaults is a variable that holds the language defaults implementation.
// It can be reassigned for testing purposes.
var LanguageDefaults = languageDefaultsImpl

// languageDefaultsImpl detects the project's toolchain from marker files
// and returns standard commands for that language. This is the lowest-priority
// fallback (priority 0) used when no CI or build system configs are found.
func languageDefaultsImpl(repoRoot string) result.Result[LanguageDefaultsData] {
	// Check for Go
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
		return result.NewSuccess(LanguageDefaultsData{
			CommandSet: &CommandSet{
				Toolchain: "go",
				Commands: Commands{
					Build: "go build ./...",
					Test: TestCommands{
						Full:           "go test ./...",
						FocusedPattern: "go test ./{package} -run {TestPrefix}",
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
			},
		})
	}

	// Check for Rust
	if _, err := os.Stat(filepath.Join(repoRoot, "Cargo.toml")); err == nil {
		return result.NewSuccess(LanguageDefaultsData{
			CommandSet: &CommandSet{
				Toolchain: "rust",
				Commands: Commands{
					Build: "cargo build",
					Test: TestCommands{
						Full:           "cargo test",
						FocusedPattern: "cargo test {module_name}",
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
			},
		})
	}

	// Check for Node
	if _, err := os.Stat(filepath.Join(repoRoot, "package.json")); err == nil {
		return result.NewSuccess(LanguageDefaultsData{
			CommandSet: &CommandSet{
				Toolchain: "node",
				Commands: Commands{
					Build: "npm run build",
					Test: TestCommands{
						Full:           "npm test",
						FocusedPattern: "npm test -- {module}",
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
			},
		})
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(repoRoot, "pyproject.toml")); err == nil {
		return result.NewSuccess(LanguageDefaultsData{
			CommandSet: &CommandSet{
				Toolchain: "python",
				Commands: Commands{
					Build: "", // Python is interpreted - no build step
					Test: TestCommands{
						Full:           "pytest",
						FocusedPattern: "pytest {path}",
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
			},
		})
	}

	// No toolchain detected
	return result.NewFailure[LanguageDefaultsData]([]result.PolywaveError{
		result.NewFatal(result.CodeCommandExtractNoToolchain, fmt.Sprintf("no toolchain detected: no go.mod, Cargo.toml, package.json, or pyproject.toml found in %s", repoRoot)),
	})
}
