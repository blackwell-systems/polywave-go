package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newListErrorRangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-error-ranges",
		Short: "List all allocated error code ranges from pkg/result/codes.go",
		Long: `Parses pkg/result/codes.go and returns all allocated error code ranges
as JSON. Lets Scout and agents choose an unoccupied range without guessing.

Example:
  sawtools list-error-ranges --repo-dir /path/to/repo`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res := protocol.ListErrorRanges(context.TODO(), repoDir)
			if res.IsFatal() {
				for _, e := range res.Errors {
					fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
				}
				return fmt.Errorf("list-error-ranges failed")
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
