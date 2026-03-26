package main

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newPreWaveGateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pre-wave-gate <manifest-path>",
		Short: "Run pre-wave readiness checks on an IMPL manifest",
		Long: `Runs all pre-wave readiness checks on a manifest and returns structured JSON.

Checks performed:
  validation    - Validates the manifest structure and content
  critic_review - Checks if a critic review has been performed (E37)
  scaffolds     - Verifies all scaffold files have been committed
  state         - Confirms the IMPL state allows wave execution

Exit code 0 if all checks pass (ready=true).
Exit code 1 if any check fails (ready=false).`,
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := protocol.Load(args[0])
			if err != nil {
				return fmt.Errorf("load manifest: %w", err)
			}

			gateResult := protocol.PreWaveGate(doc)

			if err := printJSON(gateResult); err != nil {
				return err
			}

			if !gateResult.Ready {
				return fmt.Errorf("pre-wave-gate: not ready")
			}
			return nil
		},
	}

	return cmd
}
