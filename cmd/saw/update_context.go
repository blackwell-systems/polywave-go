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
			result, err := protocol.UpdateContext(manifestPath, projectRoot)
			if err != nil {
				return fmt.Errorf("update-context: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&projectRoot, "project-root", ".", "Project root directory")

	return cmd
}
