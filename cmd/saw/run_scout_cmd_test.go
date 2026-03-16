package main

import (
	"testing"

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
