package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newValidateScaffoldsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate-scaffolds <manifest-path>",
		Short: "Validate that all scaffold files are committed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("validate-scaffolds: %w", err)
			}

			statuses := protocol.ValidateScaffolds(m)
			allCommitted := protocol.AllScaffoldsCommitted(m)

			result := struct {
				AllCommitted   bool                       `json:"all_committed"`
				ScaffoldCount  int                        `json:"scaffold_count"`
				Statuses       []protocol.ScaffoldStatus  `json:"statuses"`
			}{
				AllCommitted:  allCommitted,
				ScaffoldCount: len(statuses),
				Statuses:      statuses,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !allCommitted {
				return fmt.Errorf("validate-scaffolds: not all scaffolds are committed")
			}
			return nil
		},
	}
}
