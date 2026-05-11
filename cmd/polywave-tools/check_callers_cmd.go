package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCheckCallersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-callers <symbol-name>",
		Short: "Find all call sites of a function/method across the repo",
		Long: `Scans the entire repository for all call sites of the named symbol.
Test files are included. Returns JSON array of {file, line, context}.

Used by Scout during test cascade detection to ensure no callers are missed.

Example:
  polywave-tools check-callers "cache.Get" --repo-dir /path/to/repo
  polywave-tools check-callers "ParseFile" --repo-dir /path/to/repo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbolName := args[0]
			res := protocol.CheckCallers(context.TODO(), repoDir, symbolName)
			if res.IsFatal() {
				for _, e := range res.Errors {
					fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
				}
				return fmt.Errorf("check-callers failed")
			}
			out, err := json.MarshalIndent(res.GetData(), "", "  ")
			if err != nil {
				return fmt.Errorf("marshal output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	return cmd
}
