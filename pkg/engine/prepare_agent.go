package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/hooks"
	"github.com/blackwell-systems/polywave-go/pkg/journal"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// PrepareAgentOpts configures engine.PrepareAgent. Consolidates brief generation,
// ownership manifest write, context_source detection, and journal init.
type PrepareAgentOpts struct {
	ManifestPath string       // absolute path to IMPL doc (required)
	ProjectRoot  string       // absolute path to repo root (required)
	WaveNum      int          // wave number (required)
	AgentID      string       // agent letter e.g. "A" (required)
	NoWorktree   bool         // solo agent mode
	Logger       *slog.Logger // optional
}

// PrepareAgentResult is the structured result returned by engine.PrepareAgent.
type PrepareAgentResult struct {
	BriefPath     string `json:"brief_path"`
	BriefLength   int    `json:"brief_length"`
	JournalDir    string `json:"journal_dir"`
	CursorPath    string `json:"cursor_path"`
	IndexPath     string `json:"index_path"`
	ResultsDir    string `json:"results_dir"`
	AgentID       string `json:"agent_id"`
	Wave          int    `json:"wave"`
	FilesOwned    int    `json:"files_owned"`
	ContextSource string `json:"context_source"`
	// JournalContextAvailable is true when the agent has prior session history
	// that was successfully synced and serialized into JournalContextFile.
	JournalContextAvailable bool `json:"journal_context_available"`
	// JournalContextFile is the absolute path to context.md when
	// JournalContextAvailable is true; empty otherwise.
	JournalContextFile string `json:"journal_context_file"`
}

// PrepareAgent performs all prepare-agent work: parse IMPL doc, find agent task,
// build brief, write .polywave-agent-brief.md and .polywave-ownership.json, detect
// context_source, persist to IMPL doc, initialize journal observer.
func PrepareAgent(ctx context.Context, opts PrepareAgentOpts) result.Result[PrepareAgentResult] {
	var res PrepareAgentResult

	// Parse IMPL doc
	doc, err := protocol.Load(ctx, opts.ManifestPath)
	if err != nil {
		return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
			result.NewFatal(result.CodeIMPLParseFailed, fmt.Sprintf("failed to parse IMPL doc: %v", err)),
		})
	}

	// Find the agent's wave and task
	var agentTask string
	var agentFiles []string
	for _, wave := range doc.Waves {
		if wave.Number != opts.WaveNum {
			continue
		}
		for _, agent := range wave.Agents {
			if agent.ID == opts.AgentID {
				agentTask = agent.Task
				agentFiles = agent.Files
				break
			}
		}
	}

	if agentTask == "" {
		return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
			result.NewFatal(result.CodePrepareWaveFailed, fmt.Sprintf("agent %s not found in wave %d", opts.AgentID, opts.WaveNum)),
		})
	}

	// Pre-launch gate: validate preconditions before writing brief.
	// For solo/no-worktree agents, wtPath is empty; checkWorktreeBranch
	// gracefully skips when wtPath is "".
	wtPath := ""
	gateRes := hooks.PreLaunchGate(doc, opts.WaveNum, opts.AgentID, opts.ProjectRoot, wtPath)
	if gateRes.IsPartial() || !gateRes.IsSuccess() {
		var errParts []string
		for _, e := range gateRes.Errors {
			errParts = append(errParts, e.Message)
		}
		return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveNotReady, fmt.Sprintf("pre-launch gate failed: %s", strings.Join(errParts, "; "))),
		})
	}

	// Extract interface contracts
	contractsSection := ""
	if len(doc.InterfaceContracts) > 0 {
		contractsSection = "\n\n## Interface Contracts\n\n"
		for _, contract := range doc.InterfaceContracts {
			contractsSection += fmt.Sprintf("### %s\n\n%s\n\n```\n%s\n```\n\n",
				contract.Name, contract.Description, contract.Definition)
		}
	}

	// Extract quality gates
	gatesSection := ""
	if doc.QualityGates != nil && doc.QualityGates.Level != "" {
		gatesSection = "\n\n## Quality Gates\n\n"
		gatesSection += fmt.Sprintf("Level: %s\n\n", doc.QualityGates.Level)
		for _, gate := range doc.QualityGates.Gates {
			gatesSection += fmt.Sprintf("- **%s**: `%s` (required: %t)\n",
				gate.Type, gate.Command, gate.Required)
			if gate.Description != "" {
				gatesSection += fmt.Sprintf("  %s\n", gate.Description)
			}
		}
	}

	// Extract first line of task for SAW name
	taskFirstLine := agentTask
	if idx := strings.Index(agentTask, "\n"); idx > 0 {
		taskFirstLine = agentTask[:idx]
	}
	if len(taskFirstLine) > 80 {
		taskFirstLine = taskFirstLine[:77] + "..."
	}

	// Generate SAW-formatted name for Agent tool
	sawName := fmt.Sprintf("[polywave:wave%d:agent-%s] %s", opts.WaveNum, opts.AgentID, taskFirstLine)
	// Wrap in single quotes for valid YAML: values containing brackets (e.g. [polywave:...])
	// are invalid bare YAML scalars and must be quoted.
	sawNameYAML := fmt.Sprintf("'%s'", sawName)

	// Build the agent brief with frontmatter
	brief := fmt.Sprintf(`---
polywave_name: %s
---

# Agent %s Brief - Wave %d

**IMPL Doc:** %s

## Files Owned

%s

## Task

%s
%s%s
`,
		sawNameYAML,
		opts.AgentID,
		opts.WaveNum,
		opts.ManifestPath,
		formatFileList(agentFiles),
		agentTask,
		contractsSection,
		gatesSection,
	)

	// Determine output path
	var briefPath string
	if opts.NoWorktree {
		// Solo agent - write to .polywave-state
		stateDir := protocol.PolywaveStateAgentDir(opts.ProjectRoot, opts.WaveNum, fmt.Sprintf("agent-%s", opts.AgentID))
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
				result.NewFatal(result.CodePrepareWaveFailed, fmt.Sprintf("failed to create state dir: %v", err)),
			})
		}
		briefPath = filepath.Join(stateDir, "brief.md")
	} else {
		// Worktree agent - write to worktree root (slug-scoped path)
		worktreePath := protocol.WorktreeDir(opts.ProjectRoot, doc.FeatureSlug, opts.WaveNum, opts.AgentID)
		briefPath = filepath.Join(worktreePath, ".polywave-agent-brief.md")
	}

	// Write brief
	if err := os.WriteFile(briefPath, []byte(brief), 0644); err != nil {
		return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
			result.NewFatal(result.CodePrepareWaveFailed, fmt.Sprintf("failed to write brief: %v", err)),
		})
	}

	// Collect owned files from both agent.Files and file_ownership table.
	// Many IMPL docs only populate file_ownership, so we merge both sources.
	ownedSet := make(map[string]struct{})
	for _, f := range agentFiles {
		ownedSet[f] = struct{}{}
	}
	for _, fo := range doc.FileOwnership {
		if fo.Agent == opts.AgentID && fo.Wave == opts.WaveNum {
			ownedSet[fo.File] = struct{}{}
		}
	}
	ownedFiles := make([]string, 0, len(ownedSet))
	for f := range ownedSet {
		ownedFiles = append(ownedFiles, f)
	}

	// Write .polywave-ownership.json to the same directory as the brief.
	// The check_wave_ownership PreToolUse hook reads this at write time.
	type ownershipManifest struct {
		Agent      string   `json:"agent"`
		Wave       int      `json:"wave"`
		ImplDoc    string   `json:"impl_doc"`
		OwnedFiles []string `json:"owned_files"`
		RepoRoot   string   `json:"repo_root"`
	}
	ownershipData, _ := json.MarshalIndent(ownershipManifest{
		Agent:      opts.AgentID,
		Wave:       opts.WaveNum,
		ImplDoc:    opts.ManifestPath,
		OwnedFiles: ownedFiles,
		RepoRoot:   opts.ProjectRoot,
	}, "", "  ")
	ownershipPath := filepath.Join(filepath.Dir(briefPath), ".polywave-ownership.json")
	if err := os.WriteFile(ownershipPath, ownershipData, 0644); err != nil {
		return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
			result.NewFatal(result.CodePrepareWaveFailed, fmt.Sprintf("failed to write ownership manifest: %v", err)),
		})
	}

	// Determine and persist context_source for this agent.
	// Detection: if any owned file belongs to a repo different from the manifest repo,
	// use cross-repo-full; otherwise use prepared-brief (normal worktree path).
	manifestRepoName := filepath.Base(opts.ProjectRoot)
	contextSource := protocol.ContextSourcePreparedBrief
	for _, fo := range doc.FileOwnership {
		if fo.Agent == opts.AgentID && fo.Wave == opts.WaveNum {
			if fo.Repo != "" && fo.Repo != manifestRepoName {
				contextSource = protocol.ContextSourceCrossRepoFull
				break
			}
		}
	}

	// Write context_source to the agent entry in the IMPL doc.
	for i, wave := range doc.Waves {
		if wave.Number != opts.WaveNum {
			continue
		}
		for j, agent := range wave.Agents {
			if agent.ID == opts.AgentID {
				doc.Waves[i].Agents[j].ContextSource = contextSource
				break
			}
		}
	}
	if saveRes := protocol.Save(ctx, doc, opts.ManifestPath); saveRes.IsFatal() {
		// Non-fatal: log but don't abort agent preparation
		saveErrMsg := "save failed"
		if len(saveRes.Errors) > 0 {
			saveErrMsg = saveRes.Errors[0].Message
		}
		fmt.Fprintf(os.Stderr, "prepare-agent: warning: failed to persist context_source: %v\n", saveErrMsg)
	}

	// Advance IMPL state to REVIEWED for solo-wave agents (NoWorktree path).
	// PrepareWave does this via its advance_state step; solo paths skip PrepareWave.
	if opts.NoWorktree && (doc.State == protocol.StateScoutPending || doc.State == protocol.StateScoutValidating) {
		if warnMsg := advanceStateToReviewed(ctx, opts.ManifestPath); warnMsg != "" {
			fmt.Fprintf(os.Stderr, "prepare-agent: warning: could not advance to REVIEWED: %s\n", warnMsg)
		}
	}

	// Initialize journal observer
	fullAgentID := fmt.Sprintf("wave%d-agent-%s", opts.WaveNum, opts.AgentID)
	obsRes := journal.NewObserver(opts.ProjectRoot, fullAgentID)
	if obsRes.IsFatal() {
		return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
			result.NewFatal(result.CodeJournalInitFail, fmt.Sprintf("failed to create journal observer: %s", obsRes.Errors[0].Message)),
		})
	}
	observer := obsRes.GetData()

	// Initialize cursor if it doesn't exist
	if _, err := os.Stat(observer.CursorPath); os.IsNotExist(err) {
		emptyCursor := journal.SessionCursor{
			SessionFile: "",
			Offset:      0,
		}
		cursorData, _ := json.MarshalIndent(emptyCursor, "", "  ")
		if err := os.WriteFile(observer.CursorPath, cursorData, 0644); err != nil {
			return result.NewFailure[PrepareAgentResult]([]result.PolywaveError{
				result.NewFatal(result.CodePrepareWaveFailed, fmt.Sprintf("failed to write cursor file: %v", err)),
			})
		}
	}

	// Sync journal and surface context availability for the LLM orchestrator.
	// Non-fatal: fresh agents have no session history; errors are silently skipped.
	journalContextAvailable := false
	journalContextFile := ""
	if syncRes := observer.Sync(); syncRes.IsSuccess() {
		sr := syncRes.GetData()
		if sr != nil && sr.NewToolUses > 0 {
			if ctxRes := observer.GenerateContext(); ctxRes.IsSuccess() {
				contextFile := filepath.Join(observer.JournalDir, "context.md")
				if writeErr := os.WriteFile(contextFile, []byte(ctxRes.GetData()), 0644); writeErr == nil {
					journalContextAvailable = true
					journalContextFile = contextFile
				}
			}
		}
	}

	res = PrepareAgentResult{
		BriefPath:               briefPath,
		BriefLength:             len(brief),
		JournalDir:              observer.JournalDir,
		CursorPath:              observer.CursorPath,
		IndexPath:               observer.IndexPath,
		ResultsDir:              observer.ResultsDir,
		AgentID:                 opts.AgentID,
		Wave:                    opts.WaveNum,
		FilesOwned:              len(ownedSet),
		ContextSource:           string(contextSource),
		JournalContextAvailable: journalContextAvailable,
		JournalContextFile:      journalContextFile,
	}

	return result.NewSuccess(res)
}

// advanceStateToReviewed transitions the IMPL doc from SCOUT_PENDING or
// SCOUT_VALIDATING to REVIEWED. Best-effort: failure returns a warning
// message without aborting the caller. Returns "" on success.
func advanceStateToReviewed(ctx context.Context, implPath string) string {
	os.Setenv("POLYWAVE_ALLOW_MAIN_COMMIT", "1")
	defer os.Unsetenv("POLYWAVE_ALLOW_MAIN_COMMIT")
	stateRes := protocol.SetImplState(ctx, implPath, protocol.StateReviewed, protocol.SetImplStateOpts{
		Commit:    true,
		CommitMsg: "chore: advance IMPL state to REVIEWED (solo-wave prepare-agent)",
	})
	if stateRes.IsFatal() {
		if len(stateRes.Errors) > 0 {
			return stateRes.Errors[0].Message
		}
		return "unknown error advancing to REVIEWED"
	}
	return ""
}
