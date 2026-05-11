package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
	"github.com/spf13/cobra"
)

func newVerifyBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify-build <manifest-path>",
		Short: "Run test and lint commands from manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			manifestPath := args[0]
			res := protocol.VerifyBuild(ctx, manifestPath, repoDir)
			if !res.IsSuccess() {
				return fmt.Errorf("verify-build: %w", errors.Join(result.ToErrors(res.Errors)...))
			}
			result := res.GetData()

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.TestPassed || !result.LintPassed {
				return fmt.Errorf("verify-build: test or lint failed")
			}
			return nil
		},
	}
}
