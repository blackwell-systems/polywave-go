package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newVerifyBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify-build <manifest-path>",
		Short: "Run test and lint commands from manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			res := protocol.VerifyBuild(manifestPath, repoDir)
			if !res.IsSuccess() {
				return fmt.Errorf("verify-build: %w", errors.Join(sawErrsToErrors(res.Errors)...))
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
