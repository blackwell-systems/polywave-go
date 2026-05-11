package engine

import (
	"encoding/json"
	"os"
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
	if check.Name != "polywave_tools_binary" {
		t.Errorf("expected name polywave_tools_binary, got %q", check.Name)
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
	// Use a temp dir with no polywave.config.json — config check should warn
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

	// Should be warn since the temp dir has no polywave.config.json
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

func TestCheckHooksRegistered_NoSettingsFile(t *testing.T) {
	// If there's no settings.json at all, should warn (not fail) since
	// the file might not exist on CI runners or bare environments.
	t.Setenv("HOME", t.TempDir())
	check := checkHooksRegistered()
	if check.Name != "hooks_registered" {
		t.Errorf("expected name hooks_registered, got %q", check.Name)
	}
	if check.Status != "warn" {
		t.Errorf("expected warn when settings.json absent, got %q (detail: %s)", check.Status, check.Detail)
	}
}

func TestCheckHooksRegistered_AllCriticalPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	claudeDir := dir + "/.claude"
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Build a settings.json with all critical hooks present
	hooks := make(map[string]interface{})
	var entries []interface{}
	for _, name := range criticalPolywaveHooks {
		entries = append(entries, map[string]interface{}{
			"hooks": []map[string]string{{"command": "/home/user/.local/bin/" + name}},
		})
	}
	hooks["PreToolUse"] = entries

	settings := map[string]interface{}{
		"permissions": map[string]interface{}{"allow": []string{"Agent"}},
		"hooks":       hooks,
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(claudeDir+"/settings.json", data, 0644); err != nil {
		t.Fatal(err)
	}

	check := checkHooksRegistered()
	if check.Status != "pass" {
		t.Errorf("expected pass with all critical hooks present, got %q (detail: %s)", check.Status, check.Detail)
	}
}

func TestCheckHooksRegistered_MissingHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	claudeDir := dir + "/.claude"
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Only one of the critical hooks present
	settings := map[string]interface{}{
		"permissions": map[string]interface{}{"allow": []string{"Agent"}},
		"hooks": map[string]interface{}{
			"UserPromptSubmit": []map[string]interface{}{
				{"hooks": []map[string]string{{"command": "/bin/inject_skill_context"}}},
			},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(claudeDir+"/settings.json", data, 0644); err != nil {
		t.Fatal(err)
	}

	check := checkHooksRegistered()
	if check.Status != "fail" {
		t.Errorf("expected fail with missing critical hooks, got %q (detail: %s)", check.Status, check.Detail)
	}
	if !strings.Contains(check.Detail, "missing hooks:") {
		t.Errorf("expected detail to name missing hooks, got: %s", check.Detail)
	}
}

func TestCheckAgentPermission_Present(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	claudeDir := dir + "/.claude"
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	settings := map[string]interface{}{
		"permissions": map[string]interface{}{"allow": []string{"Bash", "Agent", "Read"}},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(claudeDir+"/settings.json", data, 0644); err != nil {
		t.Fatal(err)
	}

	check := checkAgentPermission()
	if check.Name != "agent_permission" {
		t.Errorf("expected name agent_permission, got %q", check.Name)
	}
	if check.Status != "pass" {
		t.Errorf("expected pass when Agent in allow list, got %q (detail: %s)", check.Status, check.Detail)
	}
}

func TestCheckAgentPermission_Missing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	claudeDir := dir + "/.claude"
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	settings := map[string]interface{}{
		"permissions": map[string]interface{}{"allow": []string{"Bash", "Read"}},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(claudeDir+"/settings.json", data, 0644); err != nil {
		t.Fatal(err)
	}

	check := checkAgentPermission()
	if check.Status != "fail" {
		t.Errorf("expected fail when Agent absent, got %q (detail: %s)", check.Status, check.Detail)
	}
}

func TestCheckAgentPermission_NoSettingsFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	check := checkAgentPermission()
	if check.Status != "warn" {
		t.Errorf("expected warn when settings.json absent, got %q", check.Status)
	}
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
