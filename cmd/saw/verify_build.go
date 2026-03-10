package main

import (
	"encoding/json"
	"fmt"
	"os"

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
			result, err := protocol.VerifyBuild(manifestPath, repoDir)
			if err != nil {
				return fmt.Errorf("verify-build: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.TestPassed || !result.LintPassed {
				os.Exit(1)
			}
			return nil
		},
	}
}
