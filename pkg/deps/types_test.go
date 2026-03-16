package deps

import (
	"testing"
)

// TestLockFileParserInterface verifies the interface contract
func TestLockFileParserInterface(t *testing.T) {
	// Create a mock parser
	mock := &mockParser{
		detectResult: true,
		parseResult: []PackageInfo{
			{Name: "test-pkg", Version: "1.0.0", Source: "https://example.com"},
		},
	}

	// Test Detect
	if !mock.Detect("test.lock") {
		t.Error("Expected Detect to return true")
	}

	// Test Parse
	packages, err := mock.Parse("test.lock")
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}
	if len(packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(packages))
	}
	if packages[0].Name != "test-pkg" {
		t.Errorf("Expected package name 'test-pkg', got '%s'", packages[0].Name)
	}
}

// mockParser implements LockFileParser for testing
type mockParser struct {
	detectResult bool
	parseResult  []PackageInfo
	parseError   error
}

func (m *mockParser) Detect(filePath string) bool {
	return m.detectResult
}

func (m *mockParser) Parse(filePath string) ([]PackageInfo, error) {
	if m.parseError != nil {
		return nil, m.parseError
	}
	return m.parseResult, nil
}

func TestConflictReportStructure(t *testing.T) {
	report := ConflictReport{
		MissingDeps: []MissingDep{
			{Agent: "A", Package: "missing-pkg", RequiredBy: "file.go", AvailableVer: ""},
		},
		VersionConflicts: []VersionConflict{
			{Agents: []string{"A", "B"}, Package: "conflict-pkg", Versions: []string{"1.0", "2.0"}, ResolutionNeeded: true},
		},
		Recommendations: []string{"Install missing dependencies"},
	}

	if len(report.MissingDeps) != 1 {
		t.Errorf("Expected 1 missing dep, got %d", len(report.MissingDeps))
	}
	if len(report.VersionConflicts) != 1 {
		t.Errorf("Expected 1 version conflict, got %d", len(report.VersionConflicts))
	}
	if len(report.Recommendations) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(report.Recommendations))
	}
}
