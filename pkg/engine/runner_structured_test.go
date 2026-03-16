package engine

import (
	"context"
	"strings"
	"testing"
)

// TestRunScoutStructuredBedrock_NilModel verifies that runScoutStructuredBedrock
// returns an error when ScoutModel strips to an empty string (e.g. "bedrock:" with
// nothing after the prefix).
func TestRunScoutStructuredBedrock_NilModel(t *testing.T) {
	opts := RunScoutOpts{
		Feature:             "test feature",
		RepoPath:            t.TempDir(),
		IMPLOutPath:         t.TempDir() + "/IMPL-test.yaml",
		ScoutModel:          "bedrock:", // prefix present but bare model is empty
		UseStructuredOutput: true,
	}

	_, err := runScoutStructuredBedrock(context.Background(), opts, "test prompt", nil)
	if err == nil {
		t.Fatal("expected error for empty model after stripping prefix, got nil")
	}
}

// TestRunScout_BedrockStructuredRouting verifies that RunScout with a "bedrock:" prefixed
// model and UseStructuredOutput=true routes to the Bedrock path. We detect this by
// checking the error message: the Bedrock client will fail with an AWS credentials error
// (not an "API" or "runScoutStructured" error) in a test environment without AWS creds.
func TestRunScout_BedrockStructuredRouting(t *testing.T) {
	dir := t.TempDir()
	opts := RunScoutOpts{
		Feature:             "test feature for routing",
		RepoPath:            dir,
		IMPLOutPath:         dir + "/IMPL-routing.yaml",
		ScoutModel:          "bedrock:claude-sonnet-4-5",
		UseStructuredOutput: true,
		// Provide a trivial schema override so we don't need protocol.GenerateScoutSchema to work.
		OutputSchemaOverride: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
		},
	}

	// The function should fail because AWS credentials are not available in CI/test,
	// but the error path should trace through runScoutStructuredBedrock, NOT through
	// the generic API backend. The error will contain "Bedrock run" or "AWS credentials"
	// rather than "API run".
	err := RunScout(context.Background(), opts, func(string) {})
	if err == nil {
		// In an environment where AWS is configured and Bedrock is accessible this
		// could theoretically succeed, which is also fine.
		t.Log("RunScout with Bedrock backend succeeded (AWS credentials available)")
		return
	}

	// We expect the error to originate from the Bedrock path.
	errStr := err.Error()
	if strings.Contains(errStr, "runScoutStructuredBedrock") || strings.Contains(errStr, "Bedrock run") ||
		strings.Contains(errStr, "AWS credentials") || strings.Contains(errStr, "bedrock") ||
		strings.Contains(errStr, "AWS config failed") {
		// Error came from the Bedrock path — routing is correct.
		return
	}

	// If the error mentions "runScoutStructured" without "Bedrock" it means we hit the
	// API path instead, which would indicate a routing bug.
	if strings.Contains(errStr, "runScoutStructured:") && !strings.Contains(errStr, "runScoutStructuredBedrock") {
		t.Errorf("expected error from Bedrock path but got API-path error: %v", err)
	}
}

// TestRunScout_APIStructuredRouting verifies that RunScout with a non-prefixed model
// and UseStructuredOutput=true routes to the existing API backend path (runScoutStructured).
// In a test environment without API credentials, the error should reference the API path.
func TestRunScout_APIStructuredRouting(t *testing.T) {
	dir := t.TempDir()
	opts := RunScoutOpts{
		Feature:             "test feature for api routing",
		RepoPath:            dir,
		IMPLOutPath:         dir + "/IMPL-api-routing.yaml",
		ScoutModel:          "claude-sonnet-4-5", // no "bedrock:" prefix
		UseStructuredOutput: true,
		OutputSchemaOverride: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
		},
	}

	err := RunScout(context.Background(), opts, func(string) {})
	if err == nil {
		t.Log("RunScout with API backend succeeded (API credentials available)")
		return
	}

	// Error should NOT mention the Bedrock path.
	errStr := err.Error()
	if strings.Contains(errStr, "runScoutStructuredBedrock") {
		t.Errorf("expected error from API path but got Bedrock-path error: %v", err)
	}
}
