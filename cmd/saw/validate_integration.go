package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// combinedReport is the JSON output of validate-integration when --wiring is enabled.
// It includes both the heuristic scan report and the wiring declaration check report.
type combinedReport struct {
	HeuristicReport *protocol.IntegrationReport      `json:"heuristic_report,omitempty"`
	WiringReport    *protocol.WiringValidationResult `json:"wiring_report,omitempty"`
	Valid           bool                             `json:"valid"`
}

func newValidateIntegrationCmd() *cobra.Command {
	var waveNum int
	var wiringEnabled bool

	cmd := &cobra.Command{
		Use:   "validate-integration <manifest-path>",
		Short: "Validate integration gaps after a wave completes",
		Long: `Scans a completed wave for unconnected exports using Go AST analysis.
Loads the IMPL manifest, calls ValidateIntegration to detect heuristic gaps,
optionally runs wiring declaration checks (--wiring), persists reports back
to the manifest, and prints a combined JSON report.

Exits 0 if no gaps found (both reports valid), exits 1 if gaps are detected.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Step 1: Load manifest
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("validate-integration: failed to load manifest: %w", err)
			}

			combined := &combinedReport{}

			// Step 2: Run heuristic integration validation
			// Severity threshold is read from manifest.IntegrationGapSeverityThreshold.
			// Set integration_gap_severity_threshold: "error" in the IMPL doc to report only errors.
			// Default ("") reports warnings and errors; "info" reports all gaps.
			report, err := protocol.ValidateIntegration(manifest, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("validate-integration: validation failed: %w", err)
			}
			combined.HeuristicReport = report

			// Step 3: Persist heuristic report to manifest
			waveKey := fmt.Sprintf("wave%d", waveNum)
			if err := protocol.AppendIntegrationReport(manifestPath, waveKey, report); err != nil {
				return fmt.Errorf("validate-integration: failed to persist heuristic report: %w", err)
			}

			// Step 3.5: Wiring declaration check (E35 Layer 3B)
			// For each wiring: entry, verify the symbol is actually called in
			// must_be_called_from. This is severity: error (not info).
			var wiringResult *protocol.WiringValidationResult
			if wiringEnabled {
				wiringResult, err = protocol.ValidateWiringDeclarations(manifest, repoDir)
				if err != nil {
					return fmt.Errorf("validate-integration: wiring check failed: %w", err)
				}
				combined.WiringReport = wiringResult

				// Persist wiring report to manifest
				if err := appendWiringReport(manifestPath, waveKey, wiringResult); err != nil {
					fmt.Fprintf(os.Stderr, "validate-integration: failed to persist wiring report: %v\n", err)
				}
			}

			// Step 4: Determine combined validity
			// valid = heuristic valid AND (wiring disabled OR wiring valid)
			combined.Valid = report.Valid && (wiringResult == nil || wiringResult.Valid)

			// Step 5: Print combined report as indented JSON
			out, err := json.MarshalIndent(combined, "", "  ")
			if err != nil {
				return fmt.Errorf("validate-integration: failed to marshal report: %w", err)
			}
			fmt.Println(string(out))

			// Step 6: Exit 1 if any gaps found
			if !combined.Valid {
				heuristicGaps := 0
				if report != nil {
					heuristicGaps = len(report.Gaps)
				}
				wiringGaps := 0
				if wiringResult != nil {
					wiringGaps = len(wiringResult.Gaps)
				}
				return fmt.Errorf("validate-integration: %d heuristic gap(s) and %d wiring gap(s) detected",
					heuristicGaps, wiringGaps)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")
	cmd.Flags().BoolVar(&wiringEnabled, "wiring", true, "Enable wiring declaration checking (E35 Layer 3B)")

	return cmd
}

// appendWiringReport persists a WiringValidationResult to the manifest
// under wiring_validation_reports.{waveKey}. Uses raw YAML manipulation
// to avoid re-marshaling the entire manifest. Creates the
// wiring_validation_reports key if it does not yet exist.
func appendWiringReport(manifestPath, waveKey string, result *protocol.WiringValidationResult) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("appendWiringReport: failed to read manifest: %w", err)
	}

	// Parse into raw YAML node tree
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("appendWiringReport: failed to parse YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("appendWiringReport: unexpected YAML structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("appendWiringReport: root is not a mapping")
	}

	// Marshal the result to a YAML node
	resultBytes, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("appendWiringReport: failed to marshal result: %w", err)
	}
	var resultNode yaml.Node
	if err := yaml.Unmarshal(resultBytes, &resultNode); err != nil {
		return fmt.Errorf("appendWiringReport: failed to unmarshal result node: %w", err)
	}

	// Find or create wiring_validation_reports mapping
	var reportsValue *yaml.Node
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "wiring_validation_reports" {
			reportsValue = root.Content[i+1]
			break
		}
	}

	if reportsValue == nil {
		// Create the wiring_validation_reports key
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "wiring_validation_reports", Tag: "!!str"}
		valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, keyNode, valueNode)
		reportsValue = valueNode
	}

	if reportsValue.Kind != yaml.MappingNode {
		return fmt.Errorf("appendWiringReport: wiring_validation_reports is not a mapping")
	}

	// Add or replace the wave key
	found := false
	for i := 0; i < len(reportsValue.Content)-1; i += 2 {
		if reportsValue.Content[i].Value == waveKey {
			reportsValue.Content[i+1] = resultNode.Content[0]
			found = true
			break
		}
	}
	if !found {
		waveKeyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: waveKey, Tag: "!!str"}
		reportsValue.Content = append(reportsValue.Content, waveKeyNode, resultNode.Content[0])
	}

	// Write back
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("appendWiringReport: failed to marshal output: %w", err)
	}
	if err := os.WriteFile(manifestPath, out, 0644); err != nil {
		return fmt.Errorf("appendWiringReport: failed to write file: %w", err)
	}

	return nil
}
