package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newUpdateContextCmd() *cobra.Command {
	var projectRoot string

	cmd := &cobra.Command{
		Use:   "update-context <manifest-path>",
		Short: "Update project CONTEXT.md (E18)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			res := protocol.UpdateContext(cmd.Context(), manifestPath, projectRoot)
			if res.IsFatal() {
				return fmt.Errorf("update-context: %s", res.Errors[0].Message)
			}
			data := res.GetData()

			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&projectRoot, "project-root", ".", "Project root directory")

	return cmd
}
