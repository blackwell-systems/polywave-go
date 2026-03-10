package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newRunGatesCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "run-gates <manifest-path>",
		Short: "Run quality gates from manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("run-gates: %w", err)
			}

			results, err := protocol.RunGates(m, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("run-gates: %w", err)
			}

			out, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(out))

			for _, r := range results {
				if r.Required && !r.Passed {
					os.Exit(1)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number")

	return cmd
}
