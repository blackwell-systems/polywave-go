package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

// --- TestValidateWiringDeclarations ---

func TestValidateWiringDeclarations(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(t *testing.T) (repoPath string, decl WiringDeclaration)
		wantValid bool
		wantGaps  int
		wantErr   bool
	}{
		{
			name: "symbol found as AST call (plain ident)",
			setup: func(t *testing.T) (string, WiringDeclaration) {
				dir := t.TempDir()
				src := `package main
func main() {
	MyFunc()
}`
				if err := os.WriteFile(filepath.Join(dir, "caller.go"), []byte(src), 0644); err != nil {
					t.Fatal(err)
				}
				return dir, WiringDeclaration{
					Symbol:           "MyFunc",
					DefinedIn:        "impl.go",
					MustBeCalledFrom: "caller.go",
					Agent:            "A",
					Wave:             1,
				}
			},
			wantValid: true,
			wantGaps:  0,
		},
		{
			name: "symbol found as AST call (selector expr)",
			setup: func(t *testing.T) (string, WiringDeclaration) {
				dir := t.TempDir()
				src := `package main
import "mypkg"
func main() {
	mypkg.DoWork()
}`
				if err := os.WriteFile(filepath.Join(dir, "caller.go"), []byte(src), 0644); err != nil {
					t.Fatal(err)
				}
				return dir, WiringDeclaration{
					Symbol:           "DoWork",
					DefinedIn:        "impl.go",
					MustBeCalledFrom: "caller.go",
					Agent:            "A",
					Wave:             1,
				}
			},
			wantValid: true,
			wantGaps:  0,
		},
		{
			name: "symbol not found",
			setup: func(t *testing.T) (string, WiringDeclaration) {
				dir := t.TempDir()
				src := `package main
func main() {
	OtherFunc()
}`
				if err := os.WriteFile(filepath.Join(dir, "caller.go"), []byte(src), 0644); err != nil {
					t.Fatal(err)
				}
				return dir, WiringDeclaration{
					Symbol:           "MissingFunc",
					DefinedIn:        "impl.go",
					MustBeCalledFrom: "caller.go",
					Agent:            "A",
					Wave:             1,
				}
			},
			wantValid: false,
			wantGaps:  1,
		},
		{
			name: "non-Go file falls back to grep and finds symbol",
			setup: func(t *testing.T) (string, WiringDeclaration) {
				dir := t.TempDir()
				content := "# config\nsome_symbol: true\ncall_my_func: yes\nMyFunc called here"
				if err := os.WriteFile(filepath.Join(dir, "caller.yaml"), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir, WiringDeclaration{
					Symbol:           "MyFunc",
					DefinedIn:        "impl.go",
					MustBeCalledFrom: "caller.yaml",
					Agent:            "A",
					Wave:             1,
				}
			},
			wantValid: true,
			wantGaps:  0,
		},
		{
			name: "empty manifest.Wiring",
			setup: func(t *testing.T) (string, WiringDeclaration) {
				return t.TempDir(), WiringDeclaration{} // won't be used
			},
			wantValid: true,
			wantGaps:  0,
		},
		{
			name: "file does not exist",
			setup: func(t *testing.T) (string, WiringDeclaration) {
				dir := t.TempDir()
				return dir, WiringDeclaration{
					Symbol:           "MyFunc",
					DefinedIn:        "impl.go",
					MustBeCalledFrom: "nonexistent.go",
					Agent:            "A",
					Wave:             1,
				}
			},
			wantValid: false,
			wantGaps:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoPath, decl := tc.setup(t)

			manifest := &IMPLManifest{}
			if tc.name != "empty manifest.Wiring" {
				manifest.Wiring = []WiringDeclaration{decl}
			}

			result, err := ValidateWiringDeclarations(manifest, repoPath)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tc.wantValid {
				t.Errorf("Valid: got %v, want %v", result.Valid, tc.wantValid)
			}
			if len(result.Gaps) != tc.wantGaps {
				t.Errorf("len(Gaps): got %d, want %d; gaps: %+v", len(result.Gaps), tc.wantGaps, result.Gaps)
			}
			if tc.wantGaps > 0 {
				for _, g := range result.Gaps {
					if g.Severity != "error" {
						t.Errorf("gap severity: got %q, want \"error\"", g.Severity)
					}
				}
			}
			if result.Summary == "" {
				t.Error("Summary should not be empty")
			}
		})
	}
}

// --- TestCheckWiringOwnership ---

func TestCheckWiringOwnership(t *testing.T) {
	cases := []struct {
		name       string
		manifest   *IMPLManifest
		wave       int
		wantViolations int
	}{
		{
			name: "must_be_called_from in agent's files",
			manifest: &IMPLManifest{
				FileOwnership: []FileOwnership{
					{File: "pkg/handler.go", Agent: "A", Wave: 1},
				},
				Wiring: []WiringDeclaration{
					{Symbol: "Register", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/handler.go", Agent: "A", Wave: 1},
				},
			},
			wave:           1,
			wantViolations: 0,
		},
		{
			name: "must_be_called_from NOT in agent's files",
			manifest: &IMPLManifest{
				FileOwnership: []FileOwnership{
					{File: "pkg/handler.go", Agent: "A", Wave: 1},
				},
				Wiring: []WiringDeclaration{
					{Symbol: "Register", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/router.go", Agent: "A", Wave: 1},
				},
			},
			wave:           1,
			wantViolations: 1,
		},
		{
			name: "wave mismatch — entry skipped",
			manifest: &IMPLManifest{
				FileOwnership: []FileOwnership{
					{File: "pkg/handler.go", Agent: "A", Wave: 1},
				},
				Wiring: []WiringDeclaration{
					// entry.Wave=2, we check wave=1, should be skipped
					{Symbol: "Register", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/router.go", Agent: "A", Wave: 2},
				},
			},
			wave:           1,
			wantViolations: 0,
		},
		{
			name: "multiple agents, one violation",
			manifest: &IMPLManifest{
				FileOwnership: []FileOwnership{
					{File: "pkg/a.go", Agent: "A", Wave: 1},
					{File: "pkg/b.go", Agent: "B", Wave: 1},
				},
				Wiring: []WiringDeclaration{
					{Symbol: "Foo", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/a.go", Agent: "A", Wave: 1},
					{Symbol: "Bar", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/c.go", Agent: "B", Wave: 1},
				},
			},
			wave:           1,
			wantViolations: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations := CheckWiringOwnership(tc.manifest, tc.wave)
			if len(violations) != tc.wantViolations {
				t.Errorf("violations: got %d, want %d; %v", len(violations), tc.wantViolations, violations)
			}
			for _, v := range violations {
				if !strings.Contains(v, "must_be_called_from") {
					t.Errorf("violation message missing must_be_called_from: %q", v)
				}
			}
		})
	}
}

// --- TestFormatWiringBriefSection ---

func TestFormatWiringBriefSection(t *testing.T) {
	cases := []struct {
		name      string
		manifest  *IMPLManifest
		agentID   string
		wantEmpty bool
		wantHas   []string
	}{
		{
			name: "agent has wiring obligations",
			manifest: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{Symbol: "Register", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/router.go", Agent: "A", Wave: 1, IntegrationPattern: "append"},
					{Symbol: "Init", DefinedIn: "pkg/init.go", MustBeCalledFrom: "pkg/main.go", Agent: "A", Wave: 1, IntegrationPattern: "call"},
				},
			},
			agentID:   "A",
			wantEmpty: false,
			wantHas: []string{
				"## Wiring Obligations",
				"Register",
				"pkg/impl.go",
				"pkg/router.go",
				"append",
				"Init",
				"| Symbol | Defined In | Must Be Called From | Pattern |",
			},
		},
		{
			name: "agent has no obligations",
			manifest: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{Symbol: "Register", DefinedIn: "pkg/impl.go", MustBeCalledFrom: "pkg/router.go", Agent: "B", Wave: 1},
				},
			},
			agentID:   "A",
			wantEmpty: true,
		},
		{
			name:      "empty wiring slice",
			manifest:  &IMPLManifest{},
			agentID:   "A",
			wantEmpty: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatWiringBriefSection(tc.manifest, tc.agentID)
			if tc.wantEmpty {
				if got != "" {
					t.Errorf("expected empty string, got: %q", got)
				}
				return
			}
			if got == "" {
				t.Fatal("expected non-empty section, got empty string")
			}
			for _, want := range tc.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, got)
				}
			}
		})
	}
}
