package main

import (
	"testing"
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
