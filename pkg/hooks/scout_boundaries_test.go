package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsValidScoutPath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Valid IMPL doc", "docs/IMPL/IMPL-feature-name.yaml", true},
		{"Valid with hyphens", "docs/IMPL/IMPL-multi-word-feature.yaml", true},
		{"Source code", "src/main.go", false},
		{"CONTEXT.md", "docs/CONTEXT.md", false},
		{"REQUIREMENTS.md", "docs/REQUIREMENTS.md", false},
		{"Complete subdir", "docs/IMPL/complete/IMPL-old.yaml", false},
		{"Non-IMPL yaml", "docs/IMPL/other.yaml", false},
		{"Wrong prefix", "docs/IMPL/impl-lowercase.yaml", false},
		{"Wrong extension", "docs/IMPL/IMPL-feature.yml", false},
		{"Root IMPL", "IMPL-feature.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidScoutPath(tt.filePath)
			if got != tt.want {
				t.Errorf("IsValidScoutPath(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestValidateScoutWrites(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	docsIMPL := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(docsIMPL, 0755); err != nil {
		t.Fatal(err)
	}

	startTime := time.Now()
	time.Sleep(10 * time.Millisecond) // Ensure file mtime > startTime

	t.Run("No violations", func(t *testing.T) {
		// Write valid IMPL doc
		implPath := filepath.Join(docsIMPL, "IMPL-feature.yaml")
		if err := os.WriteFile(implPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		err := ValidateScoutWrites(tmpDir, implPath, startTime)
		if err != nil {
			t.Errorf("Expected no violations, got: %v", err)
		}
	})

	t.Run("CONTEXT.md violation", func(t *testing.T) {
		contextPath := filepath.Join(tmpDir, "docs", "CONTEXT.md")
		if err := os.WriteFile(contextPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		implPath := filepath.Join(docsIMPL, "IMPL-test.yaml")
		err := ValidateScoutWrites(tmpDir, implPath, startTime)
		if err == nil {
			t.Error("Expected violation error, got nil")
		}
		if !strings.Contains(err.Error(), "I6 VIOLATION") {
			t.Errorf("Expected I6 VIOLATION, got: %v", err)
		}
		if !strings.Contains(err.Error(), "CONTEXT.md") {
			t.Errorf("Expected CONTEXT.md in error, got: %v", err)
		}
	})

	t.Run("Old files ignored", func(t *testing.T) {
		// Create file with old mtime
		oldPath := filepath.Join(docsIMPL, "IMPL-old.yaml")
		if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		oldTime := startTime.Add(-1 * time.Hour)
		if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		implPath := filepath.Join(docsIMPL, "IMPL-new.yaml")
		err := ValidateScoutWrites(tmpDir, implPath, startTime)
		// Should not report old file as violation
		if err != nil && strings.Contains(err.Error(), "IMPL-old.yaml") {
			t.Errorf("Old file should be ignored, got: %v", err)
		}
	})
}
