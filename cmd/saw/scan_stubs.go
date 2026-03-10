package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newScanStubsCmd() *cobra.Command {
	var appendImpl string
	var waveNum int

	cmd := &cobra.Command{
		Use:   "scan-stubs <file> [file...]",
		Short: "Scan files for stub/TODO patterns (E20)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := protocol.ScanStubs(args)
			if err != nil {
				return fmt.Errorf("scan-stubs: %w", err)
			}

			if appendImpl != "" {
				waveKey := fmt.Sprintf("wave%d", waveNum)
				if err := protocol.AppendStubReport(appendImpl, waveKey, result); err != nil {
					return fmt.Errorf("scan-stubs: %w", err)
				}
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&appendImpl, "append-impl", "", "Append stub report to manifest at this path")
	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number for stub report key (used with --append-impl)")

	return cmd
}
