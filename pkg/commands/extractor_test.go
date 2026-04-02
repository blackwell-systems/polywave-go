package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Mock CI parser for testing
type mockCIParser struct {
	commandSet *CommandSet
	priority   int
	err        error
}

func (m *mockCIParser) ParseCI(repoRoot string) result.Result[ParseCIData] {
	if m.err != nil {
		return result.NewFailure[ParseCIData]([]result.SAWError{result.NewFatal("MOCK_ERROR", m.err.Error())})
	}
	return result.NewSuccess(ParseCIData{CommandSet: m.commandSet})
}

func (m *mockCIParser) Priority() int {
	return m.priority
}

// Mock build system parser for testing
type mockBuildSystemParser struct {
	commandSet *CommandSet
	priority   int
	err        error
}

func (m *mockBuildSystemParser) ParseBuildSystem(repoRoot string) result.Result[ParseBuildSystemData] {
	if m.err != nil {
		return result.NewFailure[ParseBuildSystemData]([]result.SAWError{result.NewFatal("MOCK_ERROR", m.err.Error())})
	}
	return result.NewSuccess(ParseBuildSystemData{CommandSet: m.commandSet})
}

func (m *mockBuildSystemParser) Priority() int {
	return m.priority
}

// Mock LanguageDefaults function for testing
var originalLanguageDefaults = LanguageDefaults

func mockLanguageDefaults(repoRoot string) result.Result[LanguageDefaultsData] {
	return result.NewSuccess(LanguageDefaultsData{
		CommandSet: &CommandSet{
			Toolchain: "go",
			Commands: Commands{
				Build: "go build ./...",
				Test: TestCommands{
					Full: "go test ./...",
				},
			},
			DetectionSources: []string{"defaults"},
		},
	})
}

func TestExtractor_PriorityResolution(t *testing.T) {
	// Setup: CI parser with priority 100 vs build system parser with priority 50
	extractor := New()

	ciCommandSet := &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "make ci-build",
			Test: TestCommands{
				Full: "make ci-test",
			},
		},
		DetectionSources: []string{".github/workflows/ci.yml"},
	}

	buildSystemCommandSet := &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "make build",
			Test: TestCommands{
				Full: "make test",
			},
		},
		DetectionSources: []string{"Makefile"},
	}

	extractor.RegisterCIParser(&mockCIParser{
		commandSet: ciCommandSet,
		priority:   100,
	})

	extractor.RegisterBuildSystemParser(&mockBuildSystemParser{
		commandSet: buildSystemCommandSet,
		priority:   50,
	})

	// Execute
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("Expected no error, got %v", r.Errors)
	}

	result := r.GetData().CommandSet
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// CI parser should win (priority 100 > 50)
	if result.Commands.Build != "make ci-build" {
		t.Errorf("Expected CI parser result (make ci-build), got %s", result.Commands.Build)
	}

	if len(result.DetectionSources) != 1 || result.DetectionSources[0] != ".github/workflows/ci.yml" {
		t.Errorf("Expected CI parser detection source, got %v", result.DetectionSources)
	}
}

func TestExtractor_FallbackToDefaults(t *testing.T) {
	// Setup: Replace LanguageDefaults with mock
	defer func() { LanguageDefaults = originalLanguageDefaults }()
	LanguageDefaults = mockLanguageDefaults

	extractor := New()

	// Register parsers that return nil
	extractor.RegisterCIParser(&mockCIParser{
		commandSet: nil,
		priority:   100,
	})

	extractor.RegisterBuildSystemParser(&mockBuildSystemParser{
		commandSet: nil,
		priority:   50,
	})

	// Execute
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("Expected no error, got %v", r.Errors)
	}

	result := r.GetData().CommandSet
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should fall back to language defaults
	if result.Commands.Build != "go build ./..." {
		t.Errorf("Expected language defaults (go build ./...), got %s", result.Commands.Build)
	}

	if len(result.DetectionSources) != 1 || result.DetectionSources[0] != "defaults" {
		t.Errorf("Expected defaults detection source, got %v", result.DetectionSources)
	}
}

func TestExtractor_NilResults(t *testing.T) {
	// Setup: Mix of nil and non-nil results
	extractor := New()

	validCommandSet := &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "make build",
		},
		DetectionSources: []string{"Makefile"},
	}

	// Register nil parser first (higher priority)
	extractor.RegisterCIParser(&mockCIParser{
		commandSet: nil,
		priority:   100,
	})

	// Register valid parser second (lower priority)
	extractor.RegisterBuildSystemParser(&mockBuildSystemParser{
		commandSet: validCommandSet,
		priority:   50,
	})

	// Execute
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("Expected no error, got %v", r.Errors)
	}

	result := r.GetData().CommandSet
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should use the valid parser (nil results filtered out)
	if result.Commands.Build != "make build" {
		t.Errorf("Expected valid parser result (make build), got %s", result.Commands.Build)
	}
}

func TestExtractor_EmptyParsers(t *testing.T) {
	// Setup: Replace LanguageDefaults with mock
	defer func() { LanguageDefaults = originalLanguageDefaults }()
	LanguageDefaults = mockLanguageDefaults

	extractor := New()

	// Execute with no parsers registered
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("Expected no error, got %v", r.Errors)
	}

	result := r.GetData().CommandSet
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should fall back to language defaults
	if result.Commands.Build != "go build ./..." {
		t.Errorf("Expected language defaults (go build ./...), got %s", result.Commands.Build)
	}
}

func TestExtractor_ErrorHandling(t *testing.T) {
	// Setup: Parsers that return errors
	extractor := New()

	validCommandSet := &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "make build",
		},
		DetectionSources: []string{"Makefile"},
	}

	// Register parser that errors
	extractor.RegisterCIParser(&mockCIParser{
		err:      errors.New("parse error"),
		priority: 100,
	})

	// Register valid parser
	extractor.RegisterBuildSystemParser(&mockBuildSystemParser{
		commandSet: validCommandSet,
		priority:   50,
	})

	// Execute
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("Expected no error (errors should be ignored), got %v", r.Errors)
	}

	result := r.GetData().CommandSet
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should use the valid parser (errors ignored)
	if result.Commands.Build != "make build" {
		t.Errorf("Expected valid parser result (make build), got %s", result.Commands.Build)
	}
}

func TestExtractor_PriorityTies(t *testing.T) {
	// Setup: Two parsers with same priority
	extractor := New()

	firstCommandSet := &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "first",
		},
		DetectionSources: []string{"first"},
	}

	secondCommandSet := &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "second",
		},
		DetectionSources: []string{"second"},
	}

	// Register parsers with same priority (first should win due to stable sort)
	extractor.RegisterCIParser(&mockCIParser{
		commandSet: firstCommandSet,
		priority:   100,
	})

	extractor.RegisterCIParser(&mockCIParser{
		commandSet: secondCommandSet,
		priority:   100,
	})

	// Execute
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("Expected no error, got %v", r.Errors)
	}

	result := r.GetData().CommandSet
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// First parser should win (stable sort)
	if result.Commands.Build != "first" {
		t.Errorf("Expected first parser result (first), got %s", result.Commands.Build)
	}
}

func TestExtractor_ContextCancellation(t *testing.T) {
	// Setup: pre-cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	extractor := New()
	extractor.RegisterCIParser(&mockCIParser{
		commandSet: &CommandSet{Toolchain: "go"},
		priority:   100,
	})

	// Execute with cancelled context
	r := extractor.Extract(ctx, "/fake/repo")

	// Verify cancellation is propagated
	if !r.IsFatal() {
		t.Fatal("Expected error from cancelled context, got success")
	}

	if len(r.Errors) == 0 {
		t.Error("Expected error details, got empty error list")
	}
}

func TestExtractor_LanguageDefaultsError(t *testing.T) {
	// Setup: Replace LanguageDefaults with error-returning mock
	defer func() { LanguageDefaults = originalLanguageDefaults }()
	LanguageDefaults = func(repoRoot string) result.Result[LanguageDefaultsData] {
		return result.NewFailure[LanguageDefaultsData]([]result.SAWError{
			result.NewFatal("LANG_DETECT_FAILED", "language detection failed"),
		})
	}

	extractor := New()

	// Execute with no valid parsers
	r := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if !r.IsFatal() {
		t.Fatal("Expected error when language defaults fail")
	}

	if len(r.Errors) == 0 {
		t.Error("Expected error details, got empty error list")
	}
}

func TestExtractCommands(t *testing.T) {
	// Setup: Replace LanguageDefaults with mock
	defer func() { LanguageDefaults = originalLanguageDefaults }()
	LanguageDefaults = mockLanguageDefaults

	// Execute: ExtractCommands wrapper should delegate to Extract
	r := ExtractCommands(context.Background(), "/fake/repo")

	// Verify
	if r.IsFatal() {
		t.Fatalf("expected success, got errors: %v", r.Errors)
	}

	commandSet := r.GetData().CommandSet
	if commandSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}

	if commandSet.Toolchain != "go" {
		t.Errorf("expected go toolchain, got %s", commandSet.Toolchain)
	}

	if commandSet.Commands.Build != "go build ./..." {
		t.Errorf("expected 'go build ./...', got %s", commandSet.Commands.Build)
	}
}
