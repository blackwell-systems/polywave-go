package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newFinalizeImplCmd() *cobra.Command {
	var repoRoot string

	cmd := &cobra.Command{
		Use:   "finalize-impl <manifest-path>",
		Short: "Finalize IMPL doc: validate, populate verification gates, validate again",
		Long: `Atomic batching command that combines:
1. Initial validation (E16 structure check)
2. Extract build/test/lint commands from project (H2)
3. Populate verification gate blocks for all agents
4. Final validation (confirm gates are valid)

Transactional: rolls back manifest on failure (no partial writes).
Idempotent: safe to run multiple times.

Typical usage (orchestrator workflow after Scout completes):
  sawtools finalize-impl docs/IMPL/IMPL-feature-x.yaml

Output: JSON with validation results and gate population stats.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Call FinalizeIMPL from pkg/protocol
			result, err := protocol.FinalizeIMPL(manifestPath, repoRoot)
			if err != nil {
				return fmt.Errorf("finalize-impl: %w", err)
			}

			// Marshal to pretty JSON
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("finalize-impl: failed to marshal result: %w", err)
			}

			// Write to stdout (testable via cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Exit code 1 if not successful (return error instead of os.Exit for testability)
			if !result.Success {
				return fmt.Errorf("finalize-impl failed: validation or gate population errors (see JSON output)")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo-root", ".", "Repository root directory (defaults to current directory)")

	return cmd
}
