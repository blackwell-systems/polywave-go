package main

import (
	"testing"
)

func TestValidateIntegrationCmd_FlagParsing(t *testing.T) {
	cmd := newValidateIntegrationCmd()

	// Verify the command has the expected use string
	if cmd.Use != "validate-integration <manifest-path>" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	// Verify --wave flag exists
	waveFlag := cmd.Flags().Lookup("wave")
	if waveFlag == nil {
		t.Fatal("expected --wave flag to exist")
	}
	if waveFlag.DefValue != "0" {
		t.Errorf("expected --wave default to be 0, got %s", waveFlag.DefValue)
	}

	// Verify --wave is marked required
	// cobra marks required flags with annotations
	annotations := waveFlag.Annotations
	if annotations == nil {
		t.Fatal("expected --wave flag to have annotations (marked as required)")
	}
	if _, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("expected --wave flag to be marked as required")
	}

	// Verify the command requires exactly 1 positional argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error when no args provided")
	}
	if err := cmd.Args(cmd, []string{"manifest.yaml"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error when 2 args provided")
	}

	// Verify --wiring flag exists with default true
	wiringFlag := cmd.Flags().Lookup("wiring")
	if wiringFlag == nil {
		t.Fatal("expected --wiring flag to exist")
	}
	if wiringFlag.DefValue != "true" {
		t.Errorf("expected --wiring default to be true, got %s", wiringFlag.DefValue)
	}

	// Verify --wiring can be disabled
	if err := cmd.Flags().Set("wiring", "false"); err != nil {
		t.Errorf("failed to set --wiring=false: %v", err)
	}
	wiringVal, err := cmd.Flags().GetBool("wiring")
	if err != nil {
		t.Errorf("failed to get --wiring flag value: %v", err)
	}
	if wiringVal != false {
		t.Errorf("expected wiring=false, got %v", wiringVal)
	}

	// Reset and verify --wiring=true
	if err := cmd.Flags().Set("wiring", "true"); err != nil {
		t.Errorf("failed to set --wiring=true: %v", err)
	}

	// Verify --repo-dir is available (inherited from root)
	// Note: --repo-dir is a persistent flag on the root command, so it won't
	// appear on this subcommand directly unless we build the full command tree.
	// We verify the command structure is correct by checking it accepts the wave flag.
	if err := cmd.Flags().Set("wave", "3"); err != nil {
		t.Errorf("failed to set --wave flag: %v", err)
	}
	val, err := cmd.Flags().GetInt("wave")
	if err != nil {
		t.Errorf("failed to get --wave flag value: %v", err)
	}
	if val != 3 {
		t.Errorf("expected wave=3, got %d", val)
	}
}
