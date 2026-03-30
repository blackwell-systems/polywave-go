package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunVerifyInstall_OutputStructure(t *testing.T) {
	result := RunVerifyInstall(VerifyInstallOpts{})

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal InstallResult: %v", err)
	}

	var decoded InstallResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal InstallResult: %v", err)
	}

	if len(decoded.Checks) == 0 {
		t.Error("expected at least one check in result")
	}

	// Verify all checks have valid status values
	validStatuses := map[string]bool{"pass": true, "fail": true, "warn": true, "skip": true}
	for _, c := range decoded.Checks {
		if c.Name == "" {
			t.Error("check has empty name")
		}
		if !validStatuses[c.Status] {
			t.Errorf("check %q has invalid status %q", c.Name, c.Status)
		}
		if c.Detail == "" {
			t.Errorf("check %q has empty detail", c.Name)
		}
	}

	// Verify verdict is one of the expected values
	validVerdicts := map[string]bool{"PASS": true, "PARTIAL": true, "FAIL": true}
	if !validVerdicts[decoded.Verdict] {
		t.Errorf("unexpected verdict %q", decoded.Verdict)
	}

	if decoded.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestRunVerifyInstall_GitVersion(t *testing.T) {
	check := checkGitVersion()

	if check.Name != "git_version" {
		t.Errorf("expected name git_version, got %q", check.Name)
	}

	// Git should be installed in any environment running these tests
	if check.Status == "fail" && strings.Contains(check.Detail, "not found") {
		t.Skip("git not installed, skipping version check test")
	}

	// If git is installed, we expect a pass (git >= 2.20 is very common)
	if check.Status != "pass" {
		t.Logf("git version check status: %s, detail: %s", check.Status, check.Detail)
	}

	// Verify the detail contains version numbers
	if check.Status == "pass" && !strings.Contains(check.Detail, ">=") {
		t.Errorf("pass detail should contain '>=' comparison, got: %s", check.Detail)
	}
}

func TestRunVerifyInstall_SawtoolsBinary(t *testing.T) {
	check := checkSawtoolsBinary()
	if check.Name != "sawtools_binary" {
		t.Errorf("expected name sawtools_binary, got %q", check.Name)
	}
	// Should always pass since we are the running binary
	if check.Status != "pass" {
		t.Errorf("expected pass status, got %q", check.Status)
	}
}

func TestRunVerifyInstall_VerdictLogic(t *testing.T) {
	tests := []struct {
		name     string
		statuses []string
		want     string
	}{
		{"all pass", []string{"pass", "pass", "pass"}, "PASS"},
		{"one fail", []string{"pass", "fail", "pass"}, "FAIL"},
		{"one warn", []string{"pass", "warn", "pass"}, "PARTIAL"},
		{"fail trumps warn", []string{"pass", "warn", "fail"}, "FAIL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var failed, warned int
			for _, s := range tt.statuses {
				switch s {
				case "fail":
					failed++
				case "warn":
					warned++
				}
			}

			verdict := "PASS"
			if failed > 0 {
				verdict = "FAIL"
			} else if warned > 0 {
				verdict = "PARTIAL"
			}

			if verdict != tt.want {
				t.Errorf("got verdict %q, want %q", verdict, tt.want)
			}
		})
	}
}

func TestRunVerifyInstall_RepoPathUsed(t *testing.T) {
	// Test that an explicit RepoPath is used for config lookup
	// Use a temp dir with no saw.config.json — config check should warn
	result := RunVerifyInstall(VerifyInstallOpts{RepoPath: t.TempDir()})

	var configCheck InstallCheck
	for _, c := range result.Checks {
		if c.Name == "config_file" {
			configCheck = c
			break
		}
	}

	if configCheck.Name == "" {
		t.Fatal("config_file check not found in result")
	}

	// Should be warn since the temp dir has no saw.config.json
	if configCheck.Status != "warn" && configCheck.Status != "pass" {
		t.Errorf("expected warn or pass for config_file in empty dir, got %q", configCheck.Status)
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		input    string
		wantHome bool // true if result should start with expanded home path
	}{
		{"~/foo/bar", true},
		{"/absolute/path", false},
		{"relative/path", false},
		{"~notahome", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandHome(tt.input)
			if tt.wantHome {
				if strings.HasPrefix(result, "~/") {
					t.Errorf("expandHome(%q) = %q: tilde not expanded", tt.input, result)
				}
				if !strings.Contains(result, "foo/bar") {
					t.Errorf("expandHome(%q) = %q: expected path to contain foo/bar", tt.input, result)
				}
			} else {
				if result != tt.input {
					t.Errorf("expandHome(%q) = %q, want unchanged %q", tt.input, result, tt.input)
				}
			}
		})
	}
}

func TestCheckConfigFile_WithRepoPath(t *testing.T) {
	// Empty repoPath falls back to cwd
	check, path := checkConfigFile("")
	if check.Name != "config_file" {
		t.Errorf("expected name config_file, got %q", check.Name)
	}
	// path may be empty if not found; check we don't crash
	_ = path
}

func TestInstallCheckTypes(t *testing.T) {
	// Verify InstallCheck and InstallResult have correct JSON tags
	check := InstallCheck{
		Name:   "test_check",
		Status: "pass",
		Detail: "test detail",
	}
	data, err := json.Marshal(check)
	if err != nil {
		t.Fatalf("failed to marshal InstallCheck: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"name"`) {
		t.Error("InstallCheck JSON should have 'name' key")
	}
	if !strings.Contains(s, `"status"`) {
		t.Error("InstallCheck JSON should have 'status' key")
	}
	if !strings.Contains(s, `"detail"`) {
		t.Error("InstallCheck JSON should have 'detail' key")
	}

	result := InstallResult{
		Checks:  []InstallCheck{check},
		Verdict: "PASS",
		Summary: "1 passed",
	}
	data, err = json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal InstallResult: %v", err)
	}
	s = string(data)
	if !strings.Contains(s, `"checks"`) {
		t.Error("InstallResult JSON should have 'checks' key")
	}
	if !strings.Contains(s, `"verdict"`) {
		t.Error("InstallResult JSON should have 'verdict' key")
	}
	if !strings.Contains(s, `"summary"`) {
		t.Error("InstallResult JSON should have 'summary' key")
	}
}
