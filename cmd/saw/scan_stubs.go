package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newScanStubsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan-stubs <file> [file...]",
		Short: "Scan files for stub/TODO patterns (E20)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := protocol.ScanStubs(args)
			if err != nil {
				return fmt.Errorf("scan-stubs: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}
