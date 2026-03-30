package main

import (
	"os"
	"strings"
	"testing"
)

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

// TestRunScoutCmd_ThinAdapter verifies that run_scout_cmd.go is a thin adapter:
// it must call engine.RunScoutFull and must NOT contain inline orchestration logic
// (generateSlug, waitForFile, criticThresholdMet, FinalizeIMPLEngine, ValidateIMPLDoc).
func TestRunScoutCmd_ThinAdapter(t *testing.T) {
	src, err := os.ReadFile("run_scout_cmd.go")
	if err != nil {
		t.Fatalf("failed to read run_scout_cmd.go: %v", err)
	}
	content := string(src)

	// Must call engine.RunScoutFull.
	if !strings.Contains(content, "engine.RunScoutFull(") {
		t.Error("expected run_scout_cmd.go to call engine.RunScoutFull()")
	}

	// Must NOT contain inline orchestration that belongs in engine.
	forbidden := []string{
		"generateSlug(",
		"waitForFile(",
		"criticThresholdMet(",
		"countAgentsFromErrors(",
		"protocol.ValidateIMPLDoc(",
		"engine.FinalizeIMPLEngine(",
		"engine.ScoutCorrectionLoop(",
		"idgen.AssignAgentIDs(",
	}
	for _, f := range forbidden {
		if strings.Contains(content, f) {
			t.Errorf("run_scout_cmd.go must not contain inline orchestration %q (belongs in engine.RunScoutFull)", f)
		}
	}
}

// TestRunScoutCmd_FinalizeImplCmdWired verifies that the finalize-impl subcommand
// is registered in main.go, confirming it is reachable as a standalone CLI command.
func TestRunScoutCmd_FinalizeImplCmdWired(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("failed to read main.go: %v", err)
	}
	content := string(src)

	if !strings.Contains(content, "newFinalizeImplCmd()") {
		t.Error("expected newFinalizeImplCmd() to be registered in main.go AddCommand block")
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
