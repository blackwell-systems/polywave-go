package main

import (
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name    string
		feature string
		want    string
	}{
		{
			name:    "Simple feature name",
			feature: "Add audit logging",
			want:    "add-audit-logging",
		},
		{
			name:    "Feature with special characters",
			feature: "Fix bug #123: auth fails",
			want:    "fix-bug-123-auth-fails",
		},
		{
			name:    "Feature with multiple spaces",
			feature: "Add   multiple   spaces",
			want:    "add-multiple-spaces",
		},
		{
			name:    "Feature with leading/trailing spaces",
			feature: "  leading and trailing  ",
			want:    "leading-and-trailing",
		},
		{
			name:    "Long feature name (>50 chars, truncated to 49)",
			feature: "This is a very long feature description that exceeds fifty characters",
			want:    "this-is-a-very-long-feature-description-that-exce", // 49 chars (index 0-48)
		},
		{
			name:    "Feature with only numbers",
			feature: "123456",
			want:    "123456",
		},
		{
			name:    "Feature with mixed case",
			feature: "Add OAuth Integration",
			want:    "add-oauth-integration",
		},
		{
			name:    "Feature with underscores",
			feature: "fix_bug_in_auth_module",
			want:    "fix-bug-in-auth-module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateSlug(tt.feature)
			if got != tt.want {
				t.Errorf("generateSlug(%q) = %q, want %q", tt.feature, got, tt.want)
			}
		})
	}
}

// TestRunScoutCmd_ProgramFlag tests that the --program flag is parsed and accepted.
func TestRunScoutCmd_ProgramFlag(t *testing.T) {
	// Create command
	cmd := newRunScoutCmd()

	// Check that the flag exists
	programFlag := cmd.Flags().Lookup("program")
	if programFlag == nil {
		t.Fatal("expected --program flag to be defined, got nil")
	}

	// Check flag properties
	if programFlag.Usage == "" {
		t.Error("expected --program flag to have usage documentation")
	}

	// Verify the flag accepts a string value
	if programFlag.Value.Type() != "string" {
		t.Errorf("expected --program flag type 'string', got '%s'", programFlag.Value.Type())
	}

	// Verify the flag description mentions PROGRAM manifest
	usage := strings.ToLower(programFlag.Usage)
	if !strings.Contains(usage, "program") || !strings.Contains(usage, "manifest") {
		t.Errorf("expected --program flag usage to mention PROGRAM manifest, got: %s", programFlag.Usage)
	}

	// The flag should have a default value of empty string
	defaultValue := programFlag.DefValue
	if defaultValue != "" {
		t.Errorf("expected default value '', got '%s'", defaultValue)
	}
}

func TestCountAgentsFromErrors(t *testing.T) {
	tests := []struct {
		name string
		errs []types.ValidationError
		want int
	}{
		{
			name: "No errors",
			errs: []types.ValidationError{},
			want: 0,
		},
		{
			name: "Non-agent-id errors",
			errs: []types.ValidationError{
				{BlockType: "impl-file-ownership", LineNumber: 10, Message: "missing header"},
			},
			want: 0,
		},
		{
			name: "Agent ID errors without suggestion",
			errs: []types.ValidationError{
				{BlockType: "agent-id", LineNumber: 10, Message: "invalid agent ID 'A1'"},
			},
			want: 0,
		},
		{
			name: "Agent ID errors with suggestion",
			errs: []types.ValidationError{
				{BlockType: "agent-id", LineNumber: 10, Message: "invalid agent ID 'A1'"},
				{BlockType: "agent-id", LineNumber: 0, Message: "Run: sawtools assign-agent-ids --count 5"},
			},
			want: 5,
		},
		{
			name: "Multiple errors with suggestion",
			errs: []types.ValidationError{
				{BlockType: "agent-id", LineNumber: 10, Message: "invalid agent ID 'A1'"},
				{BlockType: "agent-id", LineNumber: 15, Message: "invalid agent ID 'B1'"},
				{BlockType: "agent-id", LineNumber: 20, Message: "invalid agent ID 'C1'"},
				{BlockType: "agent-id", LineNumber: 0, Message: "Run: sawtools assign-agent-ids --count 12"},
			},
			want: 12,
		},
		{
			name: "Suggestion with large count",
			errs: []types.ValidationError{
				{BlockType: "agent-id", LineNumber: 0, Message: "Run: sawtools assign-agent-ids --count 234"},
			},
			want: 234,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countAgentsFromErrors(tt.errs)
			if got != tt.want {
				t.Errorf("countAgentsFromErrors() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCriticThresholdMet(t *testing.T) {
	tests := []struct {
		name     string
		manifest *protocol.IMPLManifest
		want     bool
	}{
		{
			name: "Wave 1 with 3 agents triggers threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"}, {ID: "C"},
					}},
				},
			},
			want: true,
		},
		{
			name: "Wave 1 with 4 agents triggers threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"},
					}},
				},
			},
			want: true,
		},
		{
			name: "Wave 1 with 2 agents does not trigger threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"},
					}},
				},
			},
			want: false,
		},
		{
			name: "Wave 1 with 1 agent does not trigger threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"},
					}},
				},
			},
			want: false,
		},
		{
			name: "No waves does not trigger threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{},
			},
			want: false,
		},
		{
			name: "File ownership spanning 2 repos triggers threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"},
					}},
				},
				FileOwnership: []protocol.FileOwnership{
					{File: "pkg/foo.go", Agent: "A", Wave: 1, Repo: "/repo/alpha"},
					{File: "pkg/bar.go", Agent: "B", Wave: 1, Repo: "/repo/beta"},
				},
			},
			want: true,
		},
		{
			name: "File ownership in single repo does not trigger repo threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"},
					}},
				},
				FileOwnership: []protocol.FileOwnership{
					{File: "pkg/foo.go", Agent: "A", Wave: 1, Repo: "/repo/alpha"},
					{File: "pkg/bar.go", Agent: "B", Wave: 1, Repo: "/repo/alpha"},
				},
			},
			want: false,
		},
		{
			name: "File ownership with no repo fields does not trigger repo threshold",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"},
					}},
				},
				FileOwnership: []protocol.FileOwnership{
					{File: "pkg/foo.go", Agent: "A", Wave: 1},
					{File: "pkg/bar.go", Agent: "B", Wave: 1},
				},
			},
			want: false,
		},
		{
			name: "Wave 2 with 3 agents does not trigger threshold (only wave 1 counts)",
			manifest: &protocol.IMPLManifest{
				Waves: []protocol.Wave{
					{Number: 1, Agents: []protocol.Agent{
						{ID: "A"}, {ID: "B"},
					}},
					{Number: 2, Agents: []protocol.Agent{
						{ID: "C"}, {ID: "D"}, {ID: "E"},
					}},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := criticThresholdMet(tt.manifest)
			if got != tt.want {
				t.Errorf("criticThresholdMet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunScoutCmd_CriticFlags(t *testing.T) {
	cmd := newRunScoutCmd()

	// --no-critic flag
	noCriticFlag := cmd.Flags().Lookup("no-critic")
	if noCriticFlag == nil {
		t.Fatal("expected --no-critic flag to be defined, got nil")
	}
	if noCriticFlag.Value.Type() != "bool" {
		t.Errorf("expected --no-critic flag type 'bool', got '%s'", noCriticFlag.Value.Type())
	}
	if noCriticFlag.DefValue != "false" {
		t.Errorf("expected --no-critic default 'false', got '%s'", noCriticFlag.DefValue)
	}

	// --critic-model flag
	criticModelFlag := cmd.Flags().Lookup("critic-model")
	if criticModelFlag == nil {
		t.Fatal("expected --critic-model flag to be defined, got nil")
	}
	if criticModelFlag.Value.Type() != "string" {
		t.Errorf("expected --critic-model flag type 'string', got '%s'", criticModelFlag.Value.Type())
	}
	if criticModelFlag.DefValue != "" {
		t.Errorf("expected --critic-model default '', got '%s'", criticModelFlag.DefValue)
	}
	if !strings.Contains(strings.ToLower(criticModelFlag.Usage), "model") {
		t.Errorf("expected --critic-model usage to mention model, got: %s", criticModelFlag.Usage)
	}
}
