package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var useSolver bool
	var autoFix bool
	cmd := &cobra.Command{
		Use:   "validate <manifest-path>",
		Short: "Validate a YAML IMPL manifest against protocol invariants",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			res := protocol.FullValidate(manifestPath, protocol.FullValidateOpts{
				AutoFix:   autoFix,
				UseSolver: useSolver,
			})

			data := res.GetData()
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))

			if !data.Valid {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useSolver, "solver", false, "use CSP solver for wave assignment validation")
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-correct fixable issues (e.g. invalid gate types -> custom, unknown keys -> stripped)")
	return cmd
}
