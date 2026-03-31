package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/analyzer"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDetectSharedTypesCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "detect-shared-types <impl-doc-path>",
		Short: "Detect shared types that should be scaffolded",
		Long: `Analyze IMPL document to detect data structures referenced by 2+ agents.

Outputs scaffold candidates with metadata for Scout to review.
Exit code 0 if analysis succeeds (even if no candidates found).
Exit code 1 if IMPL doc is malformed or repo root cannot be determined.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implDocPath := args[0]

			// Validate format flag
			if format != "yaml" && format != "json" {
				return fmt.Errorf("invalid --format value: must be 'yaml' or 'json'")
			}

			// Load the manifest
			manifest, err := protocol.Load(context.TODO(), implDocPath)
			if err != nil {
				return fmt.Errorf("error: failed to parse IMPL doc: %w", err)
			}

			// Determine repo root
			repoRoot, err := findRepoRoot(implDocPath, manifest)
			if err != nil {
				return fmt.Errorf("error: could not determine repo root from %s: %w", implDocPath, err)
			}

			// Call DetectSharedTypes
			candidates, err := analyzer.DetectSharedTypes(cmd.Context(), manifest, repoRoot)
			if err != nil {
				return fmt.Errorf("error: detection failed: %w", err)
			}

			// Prepare output structure
			output := map[string]interface{}{
				"shared_types": candidates,
			}

			// Marshal to requested format
			var outputBytes []byte
			if format == "yaml" {
				outputBytes, err = yaml.Marshal(output)
			} else {
				outputBytes, err = json.MarshalIndent(output, "", "  ")
			}
			if err != nil {
				return fmt.Errorf("error: failed to marshal output: %w", err)
			}

			// Write to stdout
			fmt.Println(string(outputBytes))
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "yaml", "Output format: yaml or json")

	return cmd
}

// findRepoRoot determines the repository root directory.
// Priority: manifest.Repository field, then walk up from IMPL doc to find .git/go.mod/etc.
func findRepoRoot(implDocPath string, manifest *protocol.IMPLManifest) (string, error) {
	// Try manifest.Repository first
	if manifest.Repository != "" {
		// Could be absolute or relative to IMPL doc
		if filepath.IsAbs(manifest.Repository) {
			return manifest.Repository, nil
		}
		// Relative to IMPL doc directory
		implDir := filepath.Dir(implDocPath)
		absRepo := filepath.Join(implDir, manifest.Repository)
		return filepath.Clean(absRepo), nil
	}

	// Walk up from IMPL doc directory to find repo root markers
	dir := filepath.Dir(implDocPath)
	for {
		// Check for common repo root markers
		markers := []string{".git", "go.mod", "Cargo.toml", "package.json"}
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return dir, nil
			}
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding marker
			return "", fmt.Errorf("no repository root marker found")
		}
		dir = parent
	}
}
