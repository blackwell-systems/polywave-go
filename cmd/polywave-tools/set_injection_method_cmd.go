package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newSetInjectionMethodCmd() *cobra.Command {
	var method string

	cmd := &cobra.Command{
		Use:   "set-injection-method <manifest-path>",
		Short: "Set the injection_method field on an IMPL doc",
		Long: `Records how the Scout agent received its reference file content.

The injection_method field captures the context delivery mechanism:
  hook            - validate_agent_launch injected references via updatedInput
  manual-fallback - Scout read reference files explicitly (hook absent/failed)
  unknown         - Scout did not support this field (old IMPL)

Written by the Scout agent to the IMPL doc before completing.

Example:
  polywave-tools set-injection-method docs/IMPL/IMPL-feature.yaml --method hook`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --method before loading
			validMethods := map[string]protocol.InjectionMethod{
				"hook":            protocol.InjectionMethodHook,
				"manual-fallback": protocol.InjectionMethodManualFallback,
				"unknown":         protocol.InjectionMethodUnknown,
			}
			im, ok := validMethods[method]
			if !ok {
				return fmt.Errorf("invalid --method %q: must be one of hook, manual-fallback, unknown", method)
			}

			manifestPath := args[0]

			doc, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load IMPL doc: %w", err)
			}

			doc.InjectionMethod = im

			if saveRes := protocol.Save(context.TODO(), doc, manifestPath); saveRes.IsFatal() {
				saveErrMsg := "save failed"
				if len(saveRes.Errors) > 0 {
					saveErrMsg = saveRes.Errors[0].Message
				}
				return fmt.Errorf("failed to save IMPL doc: %s", saveErrMsg)
			}

			out, _ := json.Marshal(map[string]interface{}{
				"manifest":         manifestPath,
				"injection_method": string(im),
				"saved":            true,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&method, "method", "", "Injection method: one of hook, manual-fallback, unknown (required)")
	_ = cmd.MarkFlagRequired("method")

	return cmd
}
