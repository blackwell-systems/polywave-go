package errparse

import (
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// GofmtParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestGofmtParser_Name(t *testing.T) {
	p := &GofmtParser{}
	if p.Name() != "gofmt" {
		t.Errorf("Name() = %q, want %q", p.Name(), "gofmt")
	}
}

func TestGofmtParser_Parse(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		stderr     string
		wantFiles  []string
		wantLen    int
	}{
		{
			name:      "single unformatted file",
			stdout:    "pkg/foo/bar.go\n",
			wantFiles: []string{"pkg/foo/bar.go"},
			wantLen:   1,
		},
		{
			name:      "multiple unformatted files",
			stdout:    "main.go\npkg/foo/bar.go\npkg/baz/qux.go\n",
			wantFiles: []string{"main.go", "pkg/foo/bar.go", "pkg/baz/qux.go"},
			wantLen:   3,
		},
		{
			name:    "no output (all formatted)",
			stdout:  "",
			wantLen: 0,
		},
		{
			name:      "trailing whitespace in filename",
			stdout:    "  main.go  \n",
			wantFiles: []string{"main.go"},
			wantLen:   1,
		},
		{
			name:    "only blank lines",
			stdout:  "\n\n\n",
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &GofmtParser{}
			result := p.Parse(tc.stdout, tc.stderr)

			if result.Tool != "gofmt" {
				t.Errorf("Tool = %q, want %q", result.Tool, "gofmt")
			}
			if len(result.Errors) != tc.wantLen {
				t.Errorf("len(Errors) = %d, want %d", len(result.Errors), tc.wantLen)
			}
			for i, wantFile := range tc.wantFiles {
				if i >= len(result.Errors) {
					break
				}
				e := result.Errors[i]
				if e.File != wantFile {
					t.Errorf("Errors[%d].File = %q, want %q", i, e.File, wantFile)
				}
				if e.Severity != "warning" {
					t.Errorf("Errors[%d].Severity = %q, want %q", i, e.Severity, "warning")
				}
				if e.Message != "file is not gofmt-formatted" {
					t.Errorf("Errors[%d].Message = %q, want %q", i, e.Message, "file is not gofmt-formatted")
				}
				if e.Tool != "gofmt" {
					t.Errorf("Errors[%d].Tool = %q, want %q", i, e.Tool, "gofmt")
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PrettierFormatParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPrettierFormatParser_Name(t *testing.T) {
	p := &PrettierFormatParser{}
	if p.Name() != "prettier-format" {
		t.Errorf("Name() = %q, want %q", p.Name(), "prettier-format")
	}
}

func TestPrettierFormatParser_Parse(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		stderr    string
		wantFiles []string
		wantLen   int
	}{
		{
			name: "modern prettier [warn] format",
			stdout: "[warn] src/index.ts\n" +
				"[warn] src/utils.ts\n" +
				"[warn] Code style issues found in 2 files. Forgot to run Prettier?\n",
			wantFiles: []string{"src/index.ts", "src/utils.ts"},
			wantLen:   2,
		},
		{
			name:    "no output (all formatted)",
			stdout:  "",
			wantLen: 0,
		},
		{
			name: "single file",
			stdout: "[warn] index.js\n" +
				"[warn] Code style issues found in 1 file. Run Prettier with --write to fix it.\n",
			wantFiles: []string{"index.js"},
			wantLen:   1,
		},
		{
			name:    "summary line only (no files)",
			stdout:  "[warn] Code style issues found in 0 files.\n",
			wantLen: 0,
		},
		{
			name: "stderr [warn] lines",
			stdout: "",
			stderr: "[warn] src/app.tsx\n",
			wantFiles: []string{"src/app.tsx"},
			wantLen:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &PrettierFormatParser{}
			result := p.Parse(tc.stdout, tc.stderr)

			if result.Tool != "prettier-format" {
				t.Errorf("Tool = %q, want %q", result.Tool, "prettier-format")
			}
			if len(result.Errors) != tc.wantLen {
				t.Errorf("len(Errors) = %d, want %d (errors: %v)", len(result.Errors), tc.wantLen, result.Errors)
			}
			for i, wantFile := range tc.wantFiles {
				if i >= len(result.Errors) {
					break
				}
				e := result.Errors[i]
				if e.File != wantFile {
					t.Errorf("Errors[%d].File = %q, want %q", i, e.File, wantFile)
				}
				if e.Severity != "warning" {
					t.Errorf("Errors[%d].Severity = %q, want %q", i, e.Severity, "warning")
				}
				if e.Message != "file is not prettier-formatted" {
					t.Errorf("Errors[%d].Message = %q, want %q", i, e.Message, "file is not prettier-formatted")
				}
				if e.Tool != "prettier-format" {
					t.Errorf("Errors[%d].Tool = %q, want %q", i, e.Tool, "prettier-format")
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// RuffFormatParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestRuffFormatParser_Name(t *testing.T) {
	p := &RuffFormatParser{}
	if p.Name() != "ruff-format" {
		t.Errorf("Name() = %q, want %q", p.Name(), "ruff-format")
	}
}

func TestRuffFormatParser_Parse(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		stderr    string
		wantFiles []string
		wantLen   int
	}{
		{
			name:      "single file to reformat",
			stdout:    "Would reformat: src/main.py\n",
			wantFiles: []string{"src/main.py"},
			wantLen:   1,
		},
		{
			name: "multiple files",
			stdout: "Would reformat: src/main.py\n" +
				"Would reformat: src/utils.py\n" +
				"Would reformat: tests/test_main.py\n",
			wantFiles: []string{"src/main.py", "src/utils.py", "tests/test_main.py"},
			wantLen:   3,
		},
		{
			name:    "no output (all formatted)",
			stdout:  "",
			wantLen: 0,
		},
		{
			name: "mixed lines with summary",
			stdout: "Would reformat: main.py\n" +
				"1 file would be reformatted\n",
			wantFiles: []string{"main.py"},
			wantLen:   1,
		},
		{
			name:      "stderr output",
			stdout:    "",
			stderr:    "Would reformat: src/app.py\n",
			wantFiles: []string{"src/app.py"},
			wantLen:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &RuffFormatParser{}
			result := p.Parse(tc.stdout, tc.stderr)

			if result.Tool != "ruff-format" {
				t.Errorf("Tool = %q, want %q", result.Tool, "ruff-format")
			}
			if len(result.Errors) != tc.wantLen {
				t.Errorf("len(Errors) = %d, want %d", len(result.Errors), tc.wantLen)
			}
			for i, wantFile := range tc.wantFiles {
				if i >= len(result.Errors) {
					break
				}
				e := result.Errors[i]
				if e.File != wantFile {
					t.Errorf("Errors[%d].File = %q, want %q", i, e.File, wantFile)
				}
				if e.Severity != "warning" {
					t.Errorf("Errors[%d].Severity = %q, want %q", i, e.Severity, "warning")
				}
				if e.Message != "file needs ruff format" {
					t.Errorf("Errors[%d].Message = %q, want %q", i, e.Message, "file needs ruff format")
				}
				if e.Tool != "ruff-format" {
					t.Errorf("Errors[%d].Tool = %q, want %q", i, e.Tool, "ruff-format")
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CargoFmtParser tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCargoFmtParser_Name(t *testing.T) {
	p := &CargoFmtParser{}
	if p.Name() != "cargo-fmt" {
		t.Errorf("Name() = %q, want %q", p.Name(), "cargo-fmt")
	}
}

func TestCargoFmtParser_Parse(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		stderr    string
		wantFiles []string
		wantLen   int
	}{
		{
			name:      "single file diff",
			stdout:    "Diff in src/main.rs:\n--- src/main.rs\n+++ src/main.rs\n",
			wantFiles: []string{"src/main.rs"},
			wantLen:   1,
		},
		{
			name: "multiple files",
			stdout: "Diff in src/main.rs:\nsome diff\n" +
				"Diff in src/lib.rs:\nsome diff\n",
			wantFiles: []string{"src/main.rs", "src/lib.rs"},
			wantLen:   2,
		},
		{
			name:    "no output (all formatted)",
			stdout:  "",
			wantLen: 0,
		},
		{
			name:      "stderr output",
			stdout:    "",
			stderr:    "Diff in src/utils.rs:\nsome diff\n",
			wantFiles: []string{"src/utils.rs"},
			wantLen:   1,
		},
		{
			name: "diff with line number context",
			stdout: "Diff in src/main.rs at line 42:\nsome diff content\n",
			wantFiles: []string{"src/main.rs at line 42"},
			wantLen:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &CargoFmtParser{}
			result := p.Parse(tc.stdout, tc.stderr)

			if result.Tool != "cargo-fmt" {
				t.Errorf("Tool = %q, want %q", result.Tool, "cargo-fmt")
			}
			if len(result.Errors) != tc.wantLen {
				t.Errorf("len(Errors) = %d, want %d", len(result.Errors), tc.wantLen)
			}
			for i, wantFile := range tc.wantFiles {
				if i >= len(result.Errors) {
					break
				}
				e := result.Errors[i]
				if e.File != wantFile {
					t.Errorf("Errors[%d].File = %q, want %q", i, e.File, wantFile)
				}
				if e.Severity != "warning" {
					t.Errorf("Errors[%d].Severity = %q, want %q", i, e.Severity, "warning")
				}
				if e.Message != "file needs cargo fmt" {
					t.Errorf("Errors[%d].Message = %q, want %q", i, e.Message, "file needs cargo fmt")
				}
				if e.Tool != "cargo-fmt" {
					t.Errorf("Errors[%d].Tool = %q, want %q", i, e.Tool, "cargo-fmt")
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// DetectTool format detection tests
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectTool_FormatGateType(t *testing.T) {
	tests := []struct {
		name      string
		gateType  string
		command   string
		wantTool  string
	}{
		{
			name:     "format gate with prettier command",
			gateType: "format",
			command:  "npx prettier --check .",
			wantTool: "prettier-format",
		},
		{
			name:     "format gate with ruff command",
			gateType: "format",
			command:  "ruff format --check .",
			wantTool: "ruff-format",
		},
		{
			name:     "format gate with cargo fmt command",
			gateType: "format",
			command:  "cargo fmt --check",
			wantTool: "cargo-fmt",
		},
		{
			name:     "format gate defaults to gofmt",
			gateType: "format",
			command:  "gofmt -l .",
			wantTool: "gofmt",
		},
		{
			name:     "format gate with empty command defaults to gofmt",
			gateType: "format",
			command:  "",
			wantTool: "gofmt",
		},
		{
			name:     "explicit gofmt command",
			gateType: "build",
			command:  "gofmt -l .",
			wantTool: "gofmt",
		},
		{
			name:     "explicit prettier command without format gate type",
			gateType: "lint",
			command:  "npx prettier --check .",
			wantTool: "prettier-format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectTool(tc.gateType, tc.command)
			if got != tc.wantTool {
				t.Errorf("DetectTool(%q, %q) = %q, want %q", tc.gateType, tc.command, got, tc.wantTool)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// init() registration tests
// ─────────────────────────────────────────────────────────────────────────────

func TestFormatParsers_Registered(t *testing.T) {
	tools := []string{"gofmt", "prettier-format", "ruff-format", "cargo-fmt"}
	for _, tool := range tools {
		p := GetParser(tool)
		if p == nil {
			t.Errorf("parser %q not registered", tool)
		}
	}
}
