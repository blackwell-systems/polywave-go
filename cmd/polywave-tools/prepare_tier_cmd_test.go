package main

import (
	"strings"
	"testing"
)

func TestPrepareTierCmd_HasSkipCriticFlag(t *testing.T) {
	cmd := newPrepareTierCmd()
	flag := cmd.Flags().Lookup("skip-critic")
	if flag == nil {
		t.Fatal("expected --skip-critic flag to be registered")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected --skip-critic default to be false, got %q", flag.DefValue)
	}
}

func TestPrepareTierCmd_RequiresTierFlag(t *testing.T) {
	cmd := newPrepareTierCmd()
	cmd.SetArgs([]string{"/tmp/nonexistent.yaml"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --tier is not provided")
	}
	if !strings.Contains(err.Error(), "tier") {
		t.Errorf("expected error about tier flag, got: %s", err.Error())
	}
}

func TestPrepareWaveCmd_HasSkipCriticFlag(t *testing.T) {
	cmd := newPrepareWaveCmd()
	flag := cmd.Flags().Lookup("skip-critic")
	if flag == nil {
		t.Fatal("expected --skip-critic flag to be registered on prepare-wave")
	}
}
