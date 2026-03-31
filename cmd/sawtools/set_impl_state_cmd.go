package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newSetImplStateCmd() *cobra.Command {
	var (
		state     string
		commit    bool
		commitMsg string
	)
	cmd := &cobra.Command{
		Use:   "set-impl-state <manifest-path>",
		Short: "Atomically transition an IMPL manifest to a new protocol state",
		Long: `Validates the state transition against the protocol state machine,
then atomically writes the new state to the manifest. Optionally commits
the change to git with --commit.

Valid states: SCOUT_PENDING, SCOUT_VALIDATING, REVIEWED, SCAFFOLD_PENDING,
WAVE_PENDING, WAVE_EXECUTING, WAVE_MERGING, WAVE_VERIFIED, BLOCKED, COMPLETE,
NOT_SUITABLE

Output: JSON with previous_state, new_state, committed, commit_sha.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			manifestPath := args[0]
			newState := protocol.ProtocolState(state)
			opts := protocol.SetImplStateOpts{
				Commit:    commit,
				CommitMsg: commitMsg,
			}
			res := protocol.SetImplState(ctx, manifestPath, newState, opts)
			if res.IsFatal() {
				return fmt.Errorf("set-impl-state: %s", res.Errors[0].Message)
			}
			data := res.GetData()
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Target state (required)")
	_ = cmd.MarkFlagRequired("state")
	cmd.Flags().BoolVar(&commit, "commit", false, "Git commit the state change")
	cmd.Flags().StringVar(&commitMsg, "commit-msg", "", "Commit message override")
	return cmd
}
