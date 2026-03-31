package commands

import (
	"context"
	"errors"
	"testing"
)

// Mock CI parser for testing
type mockCIParser struct {
	commandSet *CommandSet
	priority   int
	err        error
}

func (m *mockCIParser) ParseCI(repoRoot string) (*CommandSet, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commandSet, nil
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

func (m *mockBuildSystemParser) ParseBuildSystem(repoRoot string) (*CommandSet, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.commandSet, nil
}

func (m *mockBuildSystemParser) Priority() int {
	return m.priority
}

// Mock LanguageDefaults function for testing
var originalLanguageDefaults = LanguageDefaults

func mockLanguageDefaults(repoRoot string) (*CommandSet, error) {
	return &CommandSet{
		Toolchain: "go",
		Commands: Commands{
			Build: "go build ./...",
			Test: TestCommands{
				Full: "go test ./...",
			},
		},
		DetectionSources: []string{"defaults"},
	}, nil
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
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

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
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

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
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

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
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

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
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error (errors should be ignored), got %v", err)
	}

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
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

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
	result, err := extractor.Extract(ctx, "/fake/repo")

	// Verify cancellation is propagated
	if err == nil {
		t.Fatal("Expected error from cancelled context, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result on cancellation, got %v", result)
	}
}

func TestExtractor_LanguageDefaultsError(t *testing.T) {
	// Setup: Replace LanguageDefaults with error-returning mock
	defer func() { LanguageDefaults = originalLanguageDefaults }()
	LanguageDefaults = func(repoRoot string) (*CommandSet, error) {
		return nil, errors.New("language detection failed")
	}

	extractor := New()

	// Execute with no valid parsers
	result, err := extractor.Extract(context.Background(), "/fake/repo")

	// Verify
	if err == nil {
		t.Fatal("Expected error when language defaults fail")
	}

	if result != nil {
		t.Errorf("Expected nil result on error, got %v", result)
	}

	// Error message should mention both parser failure and defaults failure
	expectedMsg := "all parsers returned nil and language defaults failed"
	if err.Error()[:len(expectedMsg)] != expectedMsg {
		t.Errorf("Expected error message starting with '%s', got '%s'", expectedMsg, err.Error())
	}
}
