package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newSetCriticVerdictCmd() *cobra.Command {
	var verdict string

	cmd := &cobra.Command{
		Use:   "set-critic-verdict <impl-path>",
		Short: "Update critic_report.verdict in an IMPL doc",
		Long: `Atomically updates critic_report.verdict in an existing IMPL doc.
Use after the Orchestrator corrects IMPL issues flagged by the critic, to
transition the verdict from ISSUES to PASS without manually editing YAML.

Exits 1 if critic_report does not exist in the IMPL doc.

Example:
  sawtools set-critic-verdict /path/to/IMPL-feature.yaml --verdict pass`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implPath := args[0]

			normalized := strings.ToUpper(verdict)
			switch normalized {
			case "PASS", "ISSUES":
				// valid
			default:
				return fmt.Errorf("set-critic-verdict: invalid --verdict %q: must be one of pass, issues", verdict)
			}

			doc, err := protocol.Load(context.TODO(), implPath)
			if err != nil {
				return fmt.Errorf("set-critic-verdict: failed to load IMPL doc: %w", err)
			}

			if doc.CriticReport == nil {
				return fmt.Errorf("set-critic-verdict: critic_report does not exist in IMPL doc")
			}

			oldVerdict := doc.CriticReport.Verdict
			doc.CriticReport.Verdict = normalized

			if saveRes := protocol.Save(context.TODO(), doc, implPath); saveRes.IsFatal() {
				saveErrMsg := "save failed"
				if len(saveRes.Errors) > 0 {
					saveErrMsg = saveRes.Errors[0].Message
				}
				return fmt.Errorf("set-critic-verdict: failed to save IMPL doc: %s", saveErrMsg)
			}

			out, _ := json.Marshal(map[string]interface{}{
				"impl_path":   implPath,
				"old_verdict": oldVerdict,
				"new_verdict": normalized,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&verdict, "verdict", "", "New verdict: pass or issues (required)")
	_ = cmd.MarkFlagRequired("verdict")

	return cmd
}
