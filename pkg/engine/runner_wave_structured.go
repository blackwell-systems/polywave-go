package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// reportWaveMu serializes concurrent writes to the IMPL doc's completion_reports
// from wave agents using structured output. Mirrors orchestrator.reportMu.
var reportWaveMu sync.Mutex

// runWaveAgentStructuredAPI is a seam for tests: constructs the API backend call
// for structured wave agent execution. Tests replace this var to inject a mock
// server URL without running real Anthropic API calls.
var runWaveAgentStructuredAPI = func(ctx context.Context, model, prompt, wtPath string, schema map[string]any, onChunk func(string)) (string, error) {
	apiClient := apiclient.New("", backend.Config{Model: model})
	apiClient.WithOutputConfig(schema)

	jsonStr, err := apiClient.RunStreaming(ctx, "", prompt, wtPath, onChunk)
	if err != nil {
		return "", fmt.Errorf("API run: %w", err)
	}
	return jsonStr, nil
}

// runWaveAgentStructuredBedrock is a seam for tests: constructs the Bedrock
// backend call for structured wave agent execution.
var runWaveAgentStructuredBedrock = func(ctx context.Context, model, prompt, wtPath string, schema map[string]any, onChunk func(string)) (string, error) {
	bareModel := strings.TrimPrefix(model, "bedrock:")
	if bareModel == "" {
		return "", fmt.Errorf("Bedrock run: model is required after 'bedrock:' prefix")
	}

	bedrockClient := bedrockbackend.New(backend.Config{Model: bareModel})
	bedrockClient.WithOutputConfig(schema)

	// Non-streaming Converse API; structured output returns complete JSON at once.
	jsonStr, err := bedrockClient.Run(ctx, "", prompt, wtPath)
	if err != nil {
		if strings.Contains(err.Error(), "no EC2 IMDS role found") ||
			strings.Contains(err.Error(), "failed to refresh cached credentials") ||
			strings.Contains(err.Error(), "NoCredentialProviders") ||
			strings.Contains(err.Error(), "AWS config failed to load") {
			return "", fmt.Errorf("Bedrock run: AWS credentials not configured: %w\n"+
				"Hint: run `aws configure` or set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY environment variables", err)
		}
		return "", fmt.Errorf("Bedrock run: %w", err)
	}

	// Forward full JSON response as a single chunk for SSE visibility.
	if onChunk != nil {
		onChunk(jsonStr)
	}

	return jsonStr, nil
}

// runWaveAgentStructured runs a wave agent via the API or Bedrock backend with
// structured output enabled for direct completion report parsing.
//
// It mirrors the runScoutStructured pattern:
//  1. Calls protocol.GenerateCompletionReportSchema() to get the schema.
//  2. Creates the appropriate client with WithOutputConfig(schema).
//  3. Runs the agent and collects the JSON response.
//  4. Unmarshals the JSON response directly into protocol.CompletionReport.
//  5. Saves the report via protocol.SetCompletionReport.
//
// The model in opts.WaveModel determines the backend:
//   - "bedrock:*" prefix → Bedrock backend (non-streaming Run)
//   - everything else   → API backend (streaming RunStreaming)
//
// wtPath is the worktree path where the agent should operate.
// onChunk, if non-nil, receives streaming output chunks.
func runWaveAgentStructured(ctx context.Context, opts RunWaveOpts, agentSpec types.AgentSpec, wtPath string, onChunk func(string)) (*protocol.CompletionReport, error) {
	// Generate completion report schema.
	schema, err := protocol.GenerateCompletionReportSchema()
	if err != nil {
		return nil, fmt.Errorf("runWaveAgentStructured: generate schema: %w", err)
	}

	// Resolve effective model: per-agent override takes precedence over default.
	model := opts.WaveModel
	if agentSpec.Model != "" {
		model = agentSpec.Model
	}

	// Route to Bedrock or API backend based on provider prefix.
	var jsonStr string
	if strings.HasPrefix(model, "bedrock:") {
		jsonStr, err = runWaveAgentStructuredBedrock(ctx, model, agentSpec.Prompt, wtPath, schema, onChunk)
	} else {
		jsonStr, err = runWaveAgentStructuredAPI(ctx, model, agentSpec.Prompt, wtPath, schema, onChunk)
	}
	if err != nil {
		return nil, fmt.Errorf("runWaveAgentStructured: agent %s: %w", agentSpec.Letter, err)
	}

	// Unmarshal the structured JSON response into CompletionReport.
	var report protocol.CompletionReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("runWaveAgentStructured: unmarshal report for agent %s: %w", agentSpec.Letter, err)
	}

	// Validate that the status field is populated; fall back to "complete".
	if report.Status == "" {
		report.Status = "complete"
	}

	// Persist the report to the IMPL doc.
	reportWaveMu.Lock()
	defer reportWaveMu.Unlock()

	manifest, loadErr := protocol.Load(opts.IMPLPath)
	if loadErr != nil {
		return nil, fmt.Errorf("runWaveAgentStructured: load manifest for agent %s: %w", agentSpec.Letter, loadErr)
	}
	if setErr := protocol.SetCompletionReport(manifest, agentSpec.Letter, report); setErr != nil {
		return nil, fmt.Errorf("runWaveAgentStructured: set completion report for agent %s: %w", agentSpec.Letter, setErr)
	}
	if saveErr := protocol.Save(manifest, opts.IMPLPath); saveErr != nil {
		return nil, fmt.Errorf("runWaveAgentStructured: save manifest for agent %s: %w", agentSpec.Letter, saveErr)
	}

	return &report, nil
}
