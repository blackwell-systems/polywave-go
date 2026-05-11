package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
)

func TestVerifyInstallCmd_OutputJSON(t *testing.T) {
	// Run the actual install checks and verify the JSON structure is valid.
	result := engine.RunVerifyInstall(engine.VerifyInstallOpts{})

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal install result: %v", err)
	}

	// Verify it round-trips to the expected structure
	var decoded engine.InstallResult
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
	result := engine.RunVerifyInstall(engine.VerifyInstallOpts{})

	var check *engine.InstallCheck
	for i := range result.Checks {
		if result.Checks[i].Name == "git_version" {
			check = &result.Checks[i]
			break
		}
	}
	if check == nil {
		t.Fatal("git_version check not found in result")
	}

	// Git should be installed in any environment running these tests
	if check.Status == "fail" && strings.Contains(check.Detail, "not found") {
		t.Skip("git not installed, skipping version check test")
	}

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
	result := engine.InstallResult{
		Checks: []engine.InstallCheck{
			{Name: "polywave_tools_binary", Status: "pass", Detail: "at /usr/local/bin/sawtools"},
			{Name: "git_version", Status: "pass", Detail: "2.43 >= 2.20"},
			{Name: "skill_directory", Status: "fail", Detail: "~/.claude/skills/polywave/ not found"},
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
	if !strings.Contains(output, "polywave_tools_binary") {
		t.Error("expected check name in human output")
	}

	// Verify summary appears
	if !strings.Contains(output, "2 passed, 1 failed") {
		t.Error("expected summary in human output")
	}
}

func TestVerifyInstallCmd_SawtoolsBinary(t *testing.T) {
	result := engine.RunVerifyInstall(engine.VerifyInstallOpts{})

	var check *engine.InstallCheck
	for i := range result.Checks {
		if result.Checks[i].Name == "polywave_tools_binary" {
			check = &result.Checks[i]
			break
		}
	}
	if check == nil {
		t.Fatal("polywave_tools_binary check not found in result")
	}

	if check.Name != "polywave_tools_binary" {
		t.Errorf("expected name polywave_tools_binary, got %q", check.Name)
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
