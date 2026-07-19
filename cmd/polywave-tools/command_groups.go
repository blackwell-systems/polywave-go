package main

import "github.com/spf13/cobra"

// Command group IDs for tiered help output.
const (
	groupWorkflows = "workflows"
	groupSetup     = "setup"
)

// workflowCommands are the primary lifecycle entrypoints a user invokes directly.
// Everything else is plumbing called by these or by the /polywave skill.
var workflowCommands = map[string]bool{
	"auto":            true, // scout + confirm + wave, full feature flow
	"run-scout":       true, // analyze codebase, produce IMPL
	"prepare-wave":    true, // set up worktrees + briefs for a wave
	"finalize-wave":   true, // verify + merge + build + cleanup a wave
	"run-wave":        true, // full wave lifecycle
	"close-impl":      true, // mark complete + archive + cleanup
	"program-execute": true, // multi-IMPL tier-gated execution
}

// setupCommands are one-time or diagnostic setup entrypoints.
var setupCommands = map[string]bool{
	"init":           true,
	"verify-install": true,
	"install-hooks":  true,
}

// applyCommandGroups registers help groups and assigns each workflow/setup
// command to its group. Commands not in either map keep cobra's default
// "Additional Commands" section, so the ~75 plumbing commands stay available
// but out of the way.
func applyCommandGroups(root *cobra.Command) {
	root.AddGroup(
		&cobra.Group{ID: groupWorkflows, Title: "Workflows (start here):"},
		&cobra.Group{ID: groupSetup, Title: "Setup & verification:"},
	)
	for _, c := range root.Commands() {
		switch {
		case workflowCommands[c.Name()]:
			c.GroupID = groupWorkflows
		case setupCommands[c.Name()]:
			c.GroupID = groupSetup
		}
	}
}
