package main

import (
	"encoding/json"
	"errors"
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
			res := protocol.ScanStubs(args)
			if !res.IsSuccess() {
				return fmt.Errorf("scan-stubs: %w", errors.Join(sawErrsToErrors(res.Errors)...))
			}

			if appendImpl != "" {
				waveKey := fmt.Sprintf("wave%d", waveNum)
				if appendRes := protocol.AppendStubReport(appendImpl, waveKey, res); appendRes.IsFatal() {
					return fmt.Errorf("scan-stubs: %w", errors.Join(sawErrsToErrors(appendRes.Errors)...))
				}
			}

			result := res.GetData()

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&appendImpl, "append-impl", "", "Append stub report to manifest at this path")
	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number for stub report key (used with --append-impl)")

	return cmd
}
