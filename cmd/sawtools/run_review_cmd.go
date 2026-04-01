package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/codereview"
	"github.com/spf13/cobra"
)

func newRunReviewCmd() *cobra.Command {
	var model string
	var threshold int
	var blocking bool

	cmd := &cobra.Command{
		Use:   "run-review",
		Short: "Run AI code review on the current diff (post-merge gate)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := codereview.CodeReviewConfig{
				Enabled:    true,
				Model:      model,
				Threshold:  threshold,
				Blocking:   blocking,
				Dimensions: nil,
			}

			res := codereview.RunCodeReview(context.Background(), repoDir, cfg)
			if res.IsFatal() {
				return fmt.Errorf("run-review: %v", res.Errors[0].Message)
			}
			got := res.GetData()
			out, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				return fmt.Errorf("run-review: marshal result: %w", err)
			}
			fmt.Println(string(out))

			if blocking && !got.Skipped && !got.Passed {
				return fmt.Errorf("run-review: code review failed (overall score %d < threshold %d)", got.Overall, threshold)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Anthropic model (defaults to claude-haiku-4-5)")
	cmd.Flags().IntVar(&threshold, "threshold", 70, "Minimum overall score (0-100) to pass")
	cmd.Flags().BoolVar(&blocking, "blocking", false, "Exit code 1 on failing review")

	return cmd
}
