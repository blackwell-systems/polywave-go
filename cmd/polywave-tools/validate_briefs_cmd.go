package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newValidateBriefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-briefs <manifest-path>",
		Short: "Validate agent briefs for symbol and line accuracy",
		Long: `Validate agent briefs in an IMPL manifest.

Checks:
  1. Symbols referenced in briefs exist in owned files
  2. Line number references are valid
  3. Provides actionable suggestions for missing symbols

Language-agnostic validation using grep and line counting.
Run this in Scout step 17 to catch errors before critic gate.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Call protocol.ValidateBriefs
			data, err := protocol.ValidateBriefs(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// JSON output
			jsonOut, _ := json.MarshalIndent(data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(jsonOut))

			// Exit 1 if invalid
			if !data.Valid {
				return fmt.Errorf("brief validation failed")
			}
			return nil
		},
	}
	return cmd
}
