package hooks

import (
	"os"
	"path/filepath"
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
		{"Absolute path valid", "/repo/docs/IMPL/IMPL-feature.yaml", true},
		{"Absolute path deep", "/home/user/projects/myrepo/docs/IMPL/IMPL-x.yaml", true},
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

	t.Run("No violations returns Success", func(t *testing.T) {
		implPath := filepath.Join(docsIMPL, "IMPL-feature.yaml")
		if err := os.WriteFile(implPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		res := ValidateScoutWrites(tmpDir, implPath, startTime)
		if !res.IsSuccess() {
			t.Errorf("Expected SUCCESS, got code=%q errors=%v", res.Code, res.Errors)
		}
		data := res.GetData()
		if data.ValidatedPath != tmpDir {
			t.Errorf("Expected ValidatedPath=%q, got %q", tmpDir, data.ValidatedPath)
		}
		if len(data.UnexpectedWrites) != 0 {
			t.Errorf("Expected no UnexpectedWrites, got %v", data.UnexpectedWrites)
		}
	})

	t.Run("CONTEXT.md violation returns Partial", func(t *testing.T) {
		contextPath := filepath.Join(tmpDir, "docs", "CONTEXT.md")
		if err := os.WriteFile(contextPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		implPath := filepath.Join(docsIMPL, "IMPL-test.yaml")
		res := ValidateScoutWrites(tmpDir, implPath, startTime)
		if !res.IsPartial() {
			t.Errorf("Expected PARTIAL result, got code=%q", res.Code)
		}
		data := res.GetData()
		found := false
		for _, w := range data.UnexpectedWrites {
			if w == "docs/CONTEXT.md" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected CONTEXT.md in UnexpectedWrites, got %v", data.UnexpectedWrites)
		}
		if len(data.Violations) == 0 {
			t.Error("Expected Violations to be populated")
		}
		for _, v := range data.Violations {
			if v.Code != "SCOUT_BOUNDARY_VIOLATION" {
				t.Errorf("Expected error code SCOUT_BOUNDARY_VIOLATION, got %q", v.Code)
			}
		}
	})

	t.Run("Partial result lists all violations", func(t *testing.T) {
		// Add two additional unauthorized files
		extraDir := filepath.Join(tmpDir, "docs", "extra")
		if err := os.MkdirAll(extraDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extraDir, "notes.txt"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "docs", "README.md"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		implPath := filepath.Join(docsIMPL, "IMPL-test2.yaml")
		res := ValidateScoutWrites(tmpDir, implPath, startTime)
		if !res.IsPartial() {
			t.Errorf("Expected PARTIAL result, got code=%q", res.Code)
		}
		data := res.GetData()
		if len(data.UnexpectedWrites) < 2 {
			t.Errorf("Expected at least 2 unexpected writes, got %d: %v", len(data.UnexpectedWrites), data.UnexpectedWrites)
		}
		if len(data.Violations) != len(data.UnexpectedWrites) {
			t.Errorf("Violations count (%d) should match UnexpectedWrites count (%d)", len(data.Violations), len(data.UnexpectedWrites))
		}
	})

	t.Run("Old files ignored", func(t *testing.T) {
		oldPath := filepath.Join(docsIMPL, "IMPL-old.yaml")
		if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		oldTime := startTime.Add(-1 * time.Hour)
		if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		// Use a fresh temp dir to avoid interference from other subtests
		freshDir := t.TempDir()
		freshIMPL := filepath.Join(freshDir, "docs", "IMPL")
		if err := os.MkdirAll(freshIMPL, 0755); err != nil {
			t.Fatal(err)
		}
		oldFile := filepath.Join(freshIMPL, "IMPL-old.yaml")
		if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		implPath := filepath.Join(freshIMPL, "IMPL-new.yaml")
		res := ValidateScoutWrites(freshDir, implPath, startTime)
		if !res.IsSuccess() {
			t.Errorf("Expected SUCCESS when only old files present, got code=%q errors=%v", res.Code, res.Errors)
		}
	})

	t.Run("No docs dir returns Success", func(t *testing.T) {
		emptyDir := t.TempDir()
		res := ValidateScoutWrites(emptyDir, filepath.Join(emptyDir, "docs/IMPL/IMPL-x.yaml"), startTime)
		if !res.IsSuccess() {
			t.Errorf("Expected SUCCESS when no docs dir, got code=%q", res.Code)
		}
	})

	t.Run("File under docs still flagged when expectedIMPLPath is outside docs/IMPL", func(t *testing.T) {
		// Use a fresh temp dir
		freshDir := t.TempDir()
		freshIMPL := filepath.Join(freshDir, "docs", "IMPL")
		if err := os.MkdirAll(freshIMPL, 0755); err != nil {
			t.Fatal(err)
		}

		// Write a file that Scout is allowed to write
		validImpl := filepath.Join(freshIMPL, "IMPL-allowed.yaml")
		if err := os.WriteFile(validImpl, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Use a bad expectedIMPLPath (outside docs/IMPL/)
		badExpected := filepath.Join("/tmp", "bad-path.yaml")
		res := ValidateScoutWrites(freshDir, badExpected, startTime)

		// The valid IMPL file was written by Scout but it matches the boundary rule,
		// so it should NOT be a violation (IsValidScoutPath(relPath) returns true).
		// The function should return Success because the file is in docs/IMPL/.
		if !res.IsSuccess() {
			t.Errorf("Expected SUCCESS: file in docs/IMPL/ should be allowed regardless of expectedIMPLPath, got code=%q errors=%v", res.Code, res.Errors)
		}
	})

	t.Run("Any IMPL doc in docs/IMPL is allowed even if not expectedIMPLPath", func(t *testing.T) {
		freshDir := t.TempDir()
		freshIMPL := filepath.Join(freshDir, "docs", "IMPL")
		if err := os.MkdirAll(freshIMPL, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(freshIMPL, "IMPL-different.yaml"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		// expectedIMPLPath points to a different IMPL file
		differentExpected := filepath.Join(freshIMPL, "IMPL-expected.yaml")
		res := ValidateScoutWrites(freshDir, differentExpected, startTime)
		if !res.IsSuccess() {
			t.Errorf("Expected SUCCESS for IMPL doc in docs/IMPL/, got code=%q", res.Code)
		}
	})
}
