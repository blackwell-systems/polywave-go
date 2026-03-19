package format

import (
	"os"
	"path/filepath"
)

// FormatConfig describes the formatter for a project.
type FormatConfig struct {
	Tool        string // "gofmt", "prettier", "ruff", "cargo-fmt", ""
	CheckCmd    string // command to run in check/report mode (exit 1 if unformatted)
	FixCmd      string // command to run in fix/write mode (rewrites files in place)
	Description string // human-readable description, e.g. "Go: gofmt -l ./..."
}

// DetectFormatter inspects the project root for known toolchain marker files
// and returns the appropriate FormatConfig. Returns zero FormatConfig if
// no known formatter is detected.
//
// Detection priority:
//
//	go.mod      → gofmt (CheckCmd: "gofmt -l .", FixCmd: "gofmt -w .")
//	package.json → prettier (CheckCmd: "npx prettier --check .", FixCmd: "npx prettier --write .")
//	pyproject.toml or setup.py → ruff (CheckCmd: "ruff format --check .", FixCmd: "ruff format .")
//	Cargo.toml  → cargo fmt (CheckCmd: "cargo fmt --check", FixCmd: "cargo fmt")
//
// When Command is set on the gate (non-empty), DetectFormatter is bypassed
// and the caller's explicit Command is used as-is.
func DetectFormatter(projectRoot string) FormatConfig {
	// Check go.mod → gofmt
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
		return FormatConfig{
			Tool:        "gofmt",
			CheckCmd:    "gofmt -l .",
			FixCmd:      "gofmt -w .",
			Description: "Go: gofmt",
		}
	}
	// Check package.json → prettier
	if _, err := os.Stat(filepath.Join(projectRoot, "package.json")); err == nil {
		return FormatConfig{
			Tool:        "prettier",
			CheckCmd:    "npx prettier --check .",
			FixCmd:      "npx prettier --write .",
			Description: "TypeScript/JS: prettier",
		}
	}
	// Check pyproject.toml or setup.py → ruff
	for _, marker := range []string{"pyproject.toml", "setup.py"} {
		if _, err := os.Stat(filepath.Join(projectRoot, marker)); err == nil {
			return FormatConfig{
				Tool:        "ruff",
				CheckCmd:    "ruff format --check .",
				FixCmd:      "ruff format .",
				Description: "Python: ruff format",
			}
		}
	}
	// Check Cargo.toml → cargo fmt
	if _, err := os.Stat(filepath.Join(projectRoot, "Cargo.toml")); err == nil {
		return FormatConfig{
			Tool:        "cargo-fmt",
			CheckCmd:    "cargo fmt --check",
			FixCmd:      "cargo fmt",
			Description: "Rust: cargo fmt",
		}
	}
	return FormatConfig{} // no formatter detected
}
