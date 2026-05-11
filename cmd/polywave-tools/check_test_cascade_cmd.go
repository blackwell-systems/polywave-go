package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCheckTestCascadeCmd() *cobra.Command {
	var repoDir string

	cmd := &cobra.Command{
		Use:   "check-test-cascade <manifest-path>",
		Short: "Pre-flight gate: verify test files for changed symbols are in file_ownership",
		Long: `After Scout writes the IMPL, before wave execution begins:
verify that every _test.go file calling a function with a changed
signature is assigned to an agent in file_ownership.

Exit code 0: all test callers are covered.
Exit code 1: orphaned test callers found.

Example:
  polywave-tools check-test-cascade docs/IMPL/IMPL-feature.yaml --repo-dir /path/to/repo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %w", err)
			}
			res := protocol.CheckTestCascade(context.TODO(), m, repoDir)
			if res.IsFatal() {
				for _, e := range res.Errors {
					fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
				}
				return fmt.Errorf("check-test-cascade: internal error")
			}
			cascadeErrors := res.GetData()
			out, err := json.MarshalIndent(cascadeErrors, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			if len(cascadeErrors) > 0 {
				return fmt.Errorf("check-test-cascade: %d orphaned test caller(s) found", len(cascadeErrors))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoDir, "repo-dir", ".", "Repository root directory")
	return cmd
}
