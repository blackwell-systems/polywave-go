package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/config"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// AgentStatusRow is one row in the agent-status table output.
// It combines IMPL doc data with git state.
type AgentStatusRow struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Branch      string `json:"branch"`
	CommitCount int    `json:"commit_count"`
	HasReport   bool   `json:"has_report"`
	Status      string `json:"status"`
}

// AgentStatusResult is the top-level output for agent-status.
type AgentStatusResult struct {
	IMPLSlug string           `json:"impl_slug"`
	State    string           `json:"state"`
	Agents   []AgentStatusRow `json:"agents"`
}

// newAgentStatusCmd returns the agent-status cobra command.
func newAgentStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "agent-status <manifest-path>",
		Short: "Show per-agent git and IMPL doc status for a wave",
		Long:  "Reads the IMPL doc and git state, then prints a table or JSON of agent progress.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("agent-status: %w", err)
			}

			// Resolve primary repo path. Use manifest.Repository if set, otherwise
			// try to find it via polywave.config.json starting from the manifest's directory.
			repoDir := m.Repository
			if repoDir == "" {
				// Try config lookup based on manifest path directory.
				cfgResult := config.Load(manifestPath)
				if cfgResult.IsSuccess() {
					cfg := cfgResult.GetData()
					if len(cfg.Repos) > 0 {
						repoDir = cfg.Repos[0].Path
					}
				}
			}

			result := buildAgentStatusResult(m, repoDir)

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			return printAgentStatusTable(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON instead of table")

	return cmd
}

// buildAgentStatusResult constructs an AgentStatusResult from the manifest and git state.
func buildAgentStatusResult(m *protocol.IMPLManifest, repoDir string) AgentStatusResult {
	result := AgentStatusResult{
		IMPLSlug: m.FeatureSlug,
		State:    string(m.State),
		Agents:   []AgentStatusRow{},
	}

	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			row := buildAgentRow(m, wave, agent, repoDir)
			result.Agents = append(result.Agents, row)
		}

		// Add critic row if CriticReport is present and non-empty.
		if m.CriticReport != nil && m.CriticReport.Verdict != "" {
			criticRow := buildCriticRow(m, wave, repoDir)
			result.Agents = append(result.Agents, criticRow)
			// Only add critic row once (for the first wave).
			break
		}
	}

	return result
}

// buildAgentRow constructs a single agent row using manifest data and git state.
func buildAgentRow(m *protocol.IMPLManifest, wave protocol.Wave, agent protocol.Agent, repoDir string) AgentStatusRow {
	branch := protocol.BranchName(m.FeatureSlug, wave.Number, agent.ID)
	role := fmt.Sprintf("wave%d", wave.Number)

	commitCount := 0
	branchExists := false
	branchMerged := false

	if repoDir != "" {
		branchExists = git.BranchExists(repoDir, branch)
		if branchExists {
			// Count commits on this branch since HEAD (base). Gracefully degrade on error.
			count, err := git.CommitCount(repoDir, "HEAD", branch)
			if err == nil {
				commitCount = count
			}
		}

		// Check if branch was merged (reachable from HEAD even if branch no longer exists).
		if !branchExists {
			// Branch may have been merged and deleted. Check if the report commit is reachable.
			if m.CompletionReports != nil {
				if report, ok := m.CompletionReports[agent.ID]; ok && report.Commit != "" {
					branchMerged = git.IsAncestor(repoDir, report.Commit, "HEAD")
				}
			}
		}
	}

	hasReport := false
	reportStatus := ""
	if m.CompletionReports != nil {
		if report, ok := m.CompletionReports[agent.ID]; ok {
			hasReport = true
			reportStatus = string(report.Status)
		}
	}

	status := deriveAgentStatus(hasReport, commitCount, branchExists, branchMerged, reportStatus)

	return AgentStatusRow{
		ID:          agent.ID,
		Role:        role,
		Branch:      branch,
		CommitCount: commitCount,
		HasReport:   hasReport,
		Status:      status,
	}
}

// buildCriticRow constructs the critic row for the critic report.
func buildCriticRow(m *protocol.IMPLManifest, wave protocol.Wave, repoDir string) AgentStatusRow {
	commitCount := -1

	if repoDir != "" && len(m.Waves) > 0 {
		baseCommit := wave.BaseCommit
		if baseCommit != "" {
			// Count commits matching [polywave:critic:<slug>] since the base commit.
			lines, err := git.LogOneline(repoDir, baseCommit+"..HEAD")
			if err == nil {
				count := 0
				searchStr := "[polywave:critic:" + m.FeatureSlug + "]"
				for _, line := range lines {
					if strings.Contains(line, searchStr) {
						count++
					}
				}
				commitCount = count
			}
		}
	}

	hasReport := m.CriticReport != nil && m.CriticReport.Verdict != ""

	return AgentStatusRow{
		ID:          "critic",
		Role:        "critic",
		Branch:      "(main)",
		CommitCount: commitCount,
		HasReport:   hasReport,
		Status:      "complete",
	}
}

// deriveAgentStatus computes the status string from git and report state.
func deriveAgentStatus(hasReport bool, commitCount int, branchExists, branchMerged bool, reportStatus string) string {
	// "blocked" if report status is blocked or partial
	if hasReport && (reportStatus == "blocked" || reportStatus == "partial") {
		return "blocked"
	}

	// "complete" if has report AND (commits > 0 OR branch merged/absent with reachable SHA)
	if hasReport && (commitCount > 0 || branchMerged || !branchExists) {
		return "complete"
	}

	// "in_progress" if branch exists AND commits > 0 AND no report
	if !hasReport && branchExists && commitCount > 0 {
		return "in_progress"
	}

	return "not_started"
}

// printAgentStatusTable writes the human-readable table to w.
func printAgentStatusTable(w interface{ Write(p []byte) (n int, err error) }, result AgentStatusResult) error {
	fmt.Fprintf(w, "IMPL: %s  State: %s\n\n", result.IMPLSlug, result.State)

	if len(result.Agents) == 0 {
		fmt.Fprintln(w, "(no agents)")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "Agent\tRole\tBranch\tCommits\tReport\tStatus\n")
	for _, row := range result.Agents {
		reportStr := "no"
		if row.HasReport {
			reportStr = "yes"
		}
		commitsStr := fmt.Sprintf("%d", row.CommitCount)
		if row.CommitCount == -1 {
			commitsStr = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			row.ID, row.Role, row.Branch, commitsStr, reportStr, row.Status,
		)
	}
	return tw.Flush()
}
