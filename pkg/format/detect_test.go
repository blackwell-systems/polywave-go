package format

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFormatter(t *testing.T) {
	tests := []struct {
		name        string
		markers     []string // files to create in temp dir
		wantTool    string
		wantCheck   string
		wantFix     string
		wantDesc    string
	}{
		{
			name:      "go.mod → gofmt",
			markers:   []string{"go.mod"},
			wantTool:  "gofmt",
			wantCheck: "gofmt -l .",
			wantFix:   "gofmt -w .",
			wantDesc:  "Go: gofmt",
		},
		{
			name:      "package.json → prettier",
			markers:   []string{"package.json"},
			wantTool:  "prettier",
			wantCheck: "npx prettier --check .",
			wantFix:   "npx prettier --write .",
			wantDesc:  "TypeScript/JS: prettier",
		},
		{
			name:      "pyproject.toml → ruff",
			markers:   []string{"pyproject.toml"},
			wantTool:  "ruff",
			wantCheck: "ruff format --check .",
			wantFix:   "ruff format .",
			wantDesc:  "Python: ruff format",
		},
		{
			name:      "setup.py → ruff",
			markers:   []string{"setup.py"},
			wantTool:  "ruff",
			wantCheck: "ruff format --check .",
			wantFix:   "ruff format .",
			wantDesc:  "Python: ruff format",
		},
		{
			name:      "Cargo.toml → cargo-fmt",
			markers:   []string{"Cargo.toml"},
			wantTool:  "cargo-fmt",
			wantCheck: "cargo fmt --check",
			wantFix:   "cargo fmt",
			wantDesc:  "Rust: cargo fmt",
		},
		{
			name:     "empty dir → zero value",
			markers:  []string{},
			wantTool: "",
		},
		{
			name:      "go.mod takes precedence over package.json",
			markers:   []string{"go.mod", "package.json"},
			wantTool:  "gofmt",
			wantCheck: "gofmt -l .",
			wantFix:   "gofmt -w .",
			wantDesc:  "Go: gofmt",
		},
		{
			name:      "go.mod takes precedence over pyproject.toml",
			markers:   []string{"go.mod", "pyproject.toml"},
			wantTool:  "gofmt",
			wantCheck: "gofmt -l .",
			wantFix:   "gofmt -w .",
			wantDesc:  "Go: gofmt",
		},
		{
			name:      "package.json takes precedence over Cargo.toml",
			markers:   []string{"package.json", "Cargo.toml"},
			wantTool:  "prettier",
			wantCheck: "npx prettier --check .",
			wantFix:   "npx prettier --write .",
			wantDesc:  "TypeScript/JS: prettier",
		},
		{
			name:      "pyproject.toml takes precedence over Cargo.toml",
			markers:   []string{"pyproject.toml", "Cargo.toml"},
			wantTool:  "ruff",
			wantCheck: "ruff format --check .",
			wantFix:   "ruff format .",
			wantDesc:  "Python: ruff format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "format-detect-*")
			if err != nil {
				t.Fatalf("MkdirTemp: %v", err)
			}
			defer os.RemoveAll(dir)

			for _, marker := range tc.markers {
				if err := os.WriteFile(filepath.Join(dir, marker), []byte(""), 0644); err != nil {
					t.Fatalf("WriteFile %s: %v", marker, err)
				}
			}

			got := DetectFormatter(dir)

			if got.Tool != tc.wantTool {
				t.Errorf("Tool: got %q, want %q", got.Tool, tc.wantTool)
			}
			if tc.wantTool == "" {
				return // zero value, no further checks
			}
			if got.CheckCmd != tc.wantCheck {
				t.Errorf("CheckCmd: got %q, want %q", got.CheckCmd, tc.wantCheck)
			}
			if got.FixCmd != tc.wantFix {
				t.Errorf("FixCmd: got %q, want %q", got.FixCmd, tc.wantFix)
			}
			if got.Description != tc.wantDesc {
				t.Errorf("Description: got %q, want %q", got.Description, tc.wantDesc)
			}
		})
	}
}

func TestDetectFormatter_NonexistentDir(t *testing.T) {
	got := DetectFormatter("/nonexistent/path/that/does/not/exist")
	if got.Tool != "" {
		t.Errorf("expected zero FormatConfig for nonexistent dir, got Tool=%q", got.Tool)
	}
}
