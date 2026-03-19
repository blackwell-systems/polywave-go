// Package format provides toolchain detection for format gates.
// This file is a stub created by Agent B to allow pkg/protocol to compile
// before Agent A's implementation is merged. At merge time, Agent A's
// implementation will replace this stub.
package format

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
//
// NOTE: This is a stub. Agent A's full implementation will replace this file at merge.
func DetectFormatter(projectRoot string) FormatConfig {
	return FormatConfig{}
}
