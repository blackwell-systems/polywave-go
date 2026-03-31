package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newFreezeCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "freeze-check <manifest-path>",
		Short: "Check manifest for freeze violations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("freeze-check: %w", err)
			}

			violations, err := protocol.CheckFreeze(m)
			if err != nil {
				return fmt.Errorf("freeze-check: %w", err)
			}

			frozen := m.WorktreesCreatedAt != nil

			result := struct {
				Frozen         bool                        `json:"frozen"`
				ViolationCount int                         `json:"violation_count"`
				Violations     []protocol.FreezeViolation  `json:"violations"`
			}{
				Frozen:         frozen,
				ViolationCount: len(violations),
				Violations:     violations,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if len(violations) > 0 {
				return fmt.Errorf("freeze-check: %d violation(s) detected", len(violations))
			}
			return nil
		},
	}
}
