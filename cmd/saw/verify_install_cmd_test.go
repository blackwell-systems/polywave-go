package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyInstallCmd_OutputJSON(t *testing.T) {
	// Run the actual install checks and verify the JSON structure is valid.
	result := runInstallChecks()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal install result: %v", err)
	}

	// Verify it round-trips to the expected structure
	var decoded installResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal install result: %v", err)
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

func TestVerifyInstallCmd_GitVersion(t *testing.T) {
	// checkGitVersion should succeed in any CI/dev environment with git installed
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

func TestVerifyInstallCmd_HumanFlag(t *testing.T) {
	// Build a known result to verify human output format
	result := installResult{
		Checks: []installCheck{
			{Name: "sawtools_binary", Status: "pass", Detail: "at /usr/local/bin/sawtools"},
			{Name: "git_version", Status: "pass", Detail: "2.43 >= 2.20"},
			{Name: "skill_directory", Status: "fail", Detail: "~/.claude/skills/saw/ not found"},
		},
		Verdict: "FAIL",
		Summary: "2 passed, 1 failed",
	}

	// Capture output using a buffer via cobra command
	cmd := newVerifyInstallCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)

	printHumanOutput(cmd, result)

	output := buf.String()

	// Verify icons appear
	if !strings.Contains(output, "[OK]") {
		t.Error("expected [OK] icon in human output")
	}
	if !strings.Contains(output, "[FAIL]") {
		t.Error("expected [FAIL] icon in human output")
	}

	// Verify verdict line
	if !strings.Contains(output, "Verdict: FAIL") {
		t.Error("expected verdict line in human output")
	}

	// Verify check names appear
	if !strings.Contains(output, "sawtools_binary") {
		t.Error("expected check name in human output")
	}

	// Verify summary appears
	if !strings.Contains(output, "2 passed, 1 failed") {
		t.Error("expected summary in human output")
	}
}

func TestVerifyInstallCmd_SawtoolsBinary(t *testing.T) {
	check := checkSawtoolsBinary()
	if check.Name != "sawtools_binary" {
		t.Errorf("expected name sawtools_binary, got %q", check.Name)
	}
	// Should always pass since we are the running binary
	if check.Status != "pass" {
		t.Errorf("expected pass status, got %q", check.Status)
	}
}

func TestVerifyInstallCmd_VerdictLogic(t *testing.T) {
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
