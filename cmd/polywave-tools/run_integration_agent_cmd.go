package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/blackwell-systems/polywave-go/pkg/config"
	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newRunIntegrationAgentCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "run-integration-agent <manifest-path>",
		Short: "Launch integration agent to wire integration gaps (E26)",
		Long: `Automated integration agent workflow (E26):

1. Load manifest from <manifest-path>
2. Run validate-integration (or use existing integration_report from manifest)
3. If no gaps (valid=true), exit 0 with message "No integration gaps detected"
4. If gaps found, read agent.integration_model from polywave.config.json
5. Call engine.RunIntegrationAgent() with opts
6. Verify build: run test_command from manifest
7. Read completion report from manifest (agent ID: "integrator")
8. Return structured JSON result

Examples:
  # Basic usage
  sawtools run-integration-agent docs/IMPL/IMPL-feature.yaml --wave 1

  # After finalize-wave detects gaps
  sawtools run-integration-agent docs/IMPL/IMPL-feature.yaml --wave 2

Output:
  - JSON with success status, gap count, and completion report
  - Integration agent modifies integration_connectors files to wire exports
  - Build verification runs to ensure changes are valid`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Load manifest
			manifest, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("run-integration-agent: failed to load manifest: %w", err)
			}

			// Check if integration_report already exists in manifest
			waveKey := fmt.Sprintf("wave%d", waveNum)
			existingReport := manifest.IntegrationReports[waveKey]

			// If no existing report, run validate-integration
			if existingReport == nil {
				fmt.Fprintf(os.Stderr, "Running integration validation for wave %d...\n", waveNum)
				report, err := protocol.ValidateIntegration(manifest, waveNum, repoDir)
				if err != nil {
					return fmt.Errorf("run-integration-agent: integration validation failed: %w", err)
				}
				existingReport = report
			}

			// If no gaps, exit early
			if existingReport.Valid {
				result := map[string]interface{}{
					"success":        true,
					"gap_count":      0,
					"agent_launched": false,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			// Read agent.integration_model from polywave.config.json
			var integrationModel string
			cfgRes := config.Load(repoDir)
			if cfgRes.IsSuccess() {
				cfg := cfgRes.GetData()
				integrationModel = cfg.Agent.IntegrationModel
			}
			// If still empty, inherit from parent (orchestrator will use its own model)
			// Empty string is valid for engine.RunIntegrationAgent — it will use the default

			// Launch integration agent
			fmt.Fprintf(os.Stderr, "Launching integration agent to fix %d gap(s)...\n", len(existingReport.Gaps))

			intAgentRes := engine.RunIntegrationAgent(context.Background(), engine.RunIntegrationAgentOpts{
				IMPLPath: manifestPath,
				RepoPath: repoDir,
				WaveNum:  waveNum,
				Report:   existingReport,
				Model:    integrationModel,
				Logger:   newSawLogger(),
			}, func(ev engine.Event) {
				// Print progress events to stderr
				if ev.Event == "integration_agent_output" {
					if data, ok := ev.Data.(map[string]string); ok {
						fmt.Fprint(os.Stderr, data["chunk"])
					}
				}
			})

			if intAgentRes.IsFatal() {
				errMsg := "integration agent failed"
				if len(intAgentRes.Errors) > 0 {
					errMsg = intAgentRes.Errors[0].Message
				}
				result := map[string]interface{}{
					"success":        false,
					"gap_count":      len(existingReport.Gaps),
					"agent_launched": true,
					"error":          errMsg,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return fmt.Errorf("run-integration-agent: %s", errMsg)
			}

			// Verify build
			buildPassed := true
			if manifest.TestCommand != "" {
				fmt.Fprintln(os.Stderr, "Verifying build...")
				cmd := exec.Command("bash", "-c", manifest.TestCommand)
				cmd.Dir = repoDir
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					buildPassed = false
					fmt.Fprintf(os.Stderr, "Warning: build verification failed: %v\n", err)
				}
			}

			// Re-load manifest to get updated completion report
			manifest, err = protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("run-integration-agent: failed to reload manifest: %w", err)
			}

			// Read completion report
			report := manifest.CompletionReports["integrator"]
			result := map[string]interface{}{
				"success":           true,
				"gap_count":         len(existingReport.Gaps),
				"agent_launched":    true,
				"build_passed":      buildPassed,
				"completion_report": report,
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
