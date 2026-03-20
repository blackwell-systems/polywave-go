package main

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestInferLanguageFromCommand(t *testing.T) {
	tests := []struct {
		name        string
		testCommand string
		want        string
	}{
		{
			name:        "Go test command",
			testCommand: "go test ./...",
			want:        "go",
		},
		{
			name:        "Go build command",
			testCommand: "go build ./cmd/...",
			want:        "go",
		},
		{
			name:        "Rust cargo test",
			testCommand: "cargo test",
			want:        "rust",
		},
		{
			name:        "Rust cargo build",
			testCommand: "cargo build --release",
			want:        "rust",
		},
		{
			name:        "JavaScript npm test",
			testCommand: "npm test",
			want:        "javascript",
		},
		{
			name:        "JavaScript jest",
			testCommand: "jest --coverage",
			want:        "javascript",
		},
		{
			name:        "JavaScript vitest",
			testCommand: "vitest run",
			want:        "javascript",
		},
		{
			name:        "Python pytest",
			testCommand: "pytest tests/",
			want:        "python",
		},
		{
			name:        "Python unittest",
			testCommand: "python -m unittest discover",
			want:        "python",
		},
		{
			name:        "Unknown command",
			testCommand: "make test",
			want:        "",
		},
		{
			name:        "Empty command",
			testCommand: "",
			want:        "",
		},
		{
			name:        "Case insensitive Go",
			testCommand: "GO TEST ./...",
			want:        "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferLanguageFromCommand(tt.testCommand)
			if got != tt.want {
				t.Errorf("inferLanguageFromCommand(%q) = %q, want %q", tt.testCommand, got, tt.want)
			}
		})
	}
}

func TestFinalizeWaveResult_CollisionReportsField(t *testing.T) {
	// Verify that FinalizeWaveResult has the CollisionReports field
	result := &FinalizeWaveResult{
		Wave:             1,
		CrossRepo:        false,
		Success:          false,
		VerifyCommits:    make(map[string]*protocol.VerifyCommitsResult),
		CollisionReports: make(map[string]*collision.CollisionReport),
		GateResults:      make(map[string][]protocol.GateResult),
		MergeResult:      make(map[string]*protocol.MergeAgentsResult),
	}

	if result.CollisionReports == nil {
		t.Error("FinalizeWaveResult.CollisionReports should be initialized")
	}
}

func TestFinalizeWaveResult_CollisionReportIntegration(t *testing.T) {
	// Verify that CollisionReport can be properly added to FinalizeWaveResult
	result := &FinalizeWaveResult{
		Wave:             1,
		CollisionReports: make(map[string]*collision.CollisionReport),
	}

	// Simulate adding a collision report for a repo
	report := &collision.CollisionReport{
		Valid: false,
		Collisions: []collision.TypeCollision{
			{
				TypeName:   "Handler",
				Package:    "pkg/service",
				Agents:     []string{"A", "B"},
				Resolution: "Keep A, remove from B",
			},
		},
	}

	result.CollisionReports["main-repo"] = report

	if result.CollisionReports["main-repo"] == nil {
		t.Error("CollisionReport should be stored in FinalizeWaveResult")
	}

	if result.CollisionReports["main-repo"].Valid {
		t.Error("CollisionReport.Valid should be false when collisions exist")
	}

	if len(result.CollisionReports["main-repo"].Collisions) != 1 {
		t.Errorf("Expected 1 collision, got %d", len(result.CollisionReports["main-repo"].Collisions))
	}
}
