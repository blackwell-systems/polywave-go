package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDetectCascadesCmd_SingleRename(t *testing.T) {
	// Create temp test directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "auth.go")
	if err := os.WriteFile(testFile, []byte(`package auth
type AuthToken struct { Value string }
func NewAuthToken() *AuthToken { return &AuthToken{} }
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Run command
	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		tmpDir,
		"--renames", `[{"old":"AuthToken","new":"SessionToken","scope":"pkg/auth"}]`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nOutput: %s", err, buf.String())
	}

	// Parse YAML output
	output := buf.String()
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid YAML output: %v\nOutput: %s", err, output)
	}

	// Verify structure (YAML marshals as cascadecandidates without yaml struct tag)
	if _, ok := result["cascadecandidates"]; !ok {
		t.Errorf("output missing cascadecandidates field\nOutput: %s\nParsed: %+v", output, result)
	}
}

func TestDetectCascadesCmd_MultipleRenames(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "service.go")
	if err := os.WriteFile(testFile, []byte(`package service
import "auth"
type UserService struct {
    token AuthToken
    session SessionKey
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		tmpDir,
		"--renames", `[{"old":"AuthToken","new":"SessionToken","scope":"auth"},{"old":"SessionKey","new":"SessionID","scope":"session"}]`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid YAML output: %v", err)
	}
}

func TestDetectCascadesCmd_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte(`package main
func main() {
    println("hello")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		tmpDir,
		"--renames", `[{"old":"NonExistentType","new":"AlsoNonExistent","scope":"fake"}]`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid YAML output: %v", err)
	}

	// Should have cascadecandidates field (may be empty list)
	if _, ok := result["cascadecandidates"]; !ok {
		t.Errorf("output missing cascadecandidates field")
	}
}

func TestDetectCascadesCmd_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		tmpDir,
		"--renames", `{this is not valid json}`,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "invalid --renames JSON") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

func TestDetectCascadesCmd_InvalidRepoRoot(t *testing.T) {
	nonExistentPath := filepath.Join(t.TempDir(), "does-not-exist")

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		nonExistentPath,
		"--renames", `[{"old":"Foo","new":"Bar","scope":"pkg"}]`,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent repo root, got nil")
	}

	// Should fail during DetectCascades call
	if !strings.Contains(err.Error(), "detect cascades") {
		t.Errorf("expected error mentioning 'detect cascades', got: %v", err)
	}
}

func TestDetectCascadesCmd_EmptyRenames(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		tmpDir,
		"--renames", `[]`,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty renames array, got nil")
	}

	if !strings.Contains(err.Error(), "at least one rename") {
		t.Errorf("expected error about empty renames, got: %v", err)
	}
}

func TestDetectCascadesCmd_MissingFields(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		renamesJSON string
		wantErrMsg  string
	}{
		{
			name:        "missing old field",
			renamesJSON: `[{"new":"Bar","scope":"pkg"}]`,
			wantErrMsg:  "old and new fields are required",
		},
		{
			name:        "missing new field",
			renamesJSON: `[{"old":"Foo","scope":"pkg"}]`,
			wantErrMsg:  "old and new fields are required",
		},
		{
			name:        "empty old field",
			renamesJSON: `[{"old":"","new":"Bar","scope":"pkg"}]`,
			wantErrMsg:  "old and new fields are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDetectCascadesCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{
				tmpDir,
				"--renames", tt.renamesJSON,
			})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErrMsg, err)
			}
		})
	}
}

func TestDetectCascadesCmd_RequiredFlag(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --renames flag is missing, got nil")
	}

	// Cobra returns "required flag" error
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error about required flag, got: %v", err)
	}
}

func TestDetectCascadesCmd_Integration(t *testing.T) {
	// Full integration test simulating real usage
	tmpDir := t.TempDir()

	// Create realistic Go files
	files := map[string]string{
		"auth/token.go": `package auth
type AuthToken struct {
    Value string
}
`,
		"service/user.go": `package service
import "auth"
type UserService struct {
    token *auth.AuthToken
}
`,
		"handler/http.go": `package handler
import "service"
func HandleLogin(s *service.UserService) {}
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := newDetectCascadesCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		tmpDir,
		"--renames", `[{"old":"AuthToken","new":"SessionToken","scope":"auth"}]`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nOutput: %s", err, buf.String())
	}

	output := buf.String()
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid YAML output: %v\nOutput: %s", err, output)
	}

	// Verify expected structure
	candidates, ok := result["cascadecandidates"]
	if !ok {
		t.Fatal("output missing cascadecandidates field")
	}

	// Should be a list (may be empty or contain candidates)
	switch candidates.(type) {
	case []interface{}, nil:
		// Valid - either list of candidates or nil/empty
	default:
		t.Errorf("cascadecandidates should be a list, got: %T", candidates)
	}
}

// Helper to verify command is properly structured
func TestDetectCascadesCmd_Structure(t *testing.T) {
	cmd := newDetectCascadesCmd()

	if cmd.Use != "detect-cascades <repo-root>" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description is empty")
	}

	if cmd.Long == "" {
		t.Error("Long description is empty")
	}

	// Verify flags exist
	renamesFlag := cmd.Flags().Lookup("renames")
	if renamesFlag == nil {
		t.Fatal("--renames flag not found")
	}

	// Verify it's a required flag
	annotations := cmd.Annotations
	_ = annotations // Cobra handles required flags differently

	// Verify it accepts exactly one argument
	if cmd.Args == nil {
		t.Error("Args validator is nil")
	}
}
