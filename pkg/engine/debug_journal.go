package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/journal"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// DebugJournalOpts holds options for the DebugJournal engine function.
type DebugJournalOpts struct {
	RepoPath     string // absolute path to the repository root
	AgentPath    string // e.g. "wave1/agent-A" or "wave2-agent-B"
	Summary      bool
	FailuresOnly bool
	Last         int
	Export       string // output path for HTML timeline export
	Force        bool   // overwrite export file if it exists
}

// DebugJournalResult contains filtered entries and summary stats returned by DebugJournal.
type DebugJournalResult struct {
	AgentPath   string              `json:"agent_path"`
	Entries     []journal.ToolEntry `json:"entries"`
	TotalCount  int                 `json:"total_count"`
	FilterCount int                 `json:"filter_count"`
	ExportPath  string              `json:"export_path,omitempty"`
	Summary     *JournalSummary     `json:"summary,omitempty"`
}

// JournalSummary holds a human-readable summary of a journal session.
type JournalSummary struct {
	Duration      string             `json:"duration"`
	FilesModified []FileModification `json:"files_modified"`
	CommandsRun   []CommandRun       `json:"commands_run"`
	GitCommits    []GitCommit        `json:"git_commits"`
	ReportWritten bool               `json:"report_written"`
}

// FileModification records a file touched by an agent.
type FileModification struct {
	Path       string
	LinesAdded int
	Operation  string
}

// CommandRun records a Bash command executed by an agent.
type CommandRun struct {
	Command string
	Status  string
	Runs    int
}

// GitCommit records a git commit made by an agent.
type GitCommit struct {
	SHA     string
	Message string
	Branch  string
}

// GateStatus records the last observed status of a verification gate.
type GateStatus struct {
	Status    string
	Timestamp time.Time
}

// ExportData holds data returned by exportHTMLTimeline.
type ExportData struct {
	ExportPath string `json:"export_path"`
	EntryCount int    `json:"entry_count"`
}

// DebugJournal loads, filters, and summarises a wave-agent journal.
// It is the single entry point for all debug-journal business logic.
func DebugJournal(opts DebugJournalOpts) result.Result[DebugJournalResult] {
	partial := DebugJournalResult{AgentPath: opts.AgentPath}

	// Resolve repo root to absolute path.
	absRepoRoot, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return result.NewFailure[DebugJournalResult]([]result.PolywaveError{
			result.NewFatal(result.CodeExportWriteFailed,
				"failed to resolve repo root: "+err.Error()).WithCause(err),
		})
	}

	journalPath := filepath.Join(absRepoRoot, ".polywave-state", opts.AgentPath, "index.jsonl")

	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		return result.NewFailure[DebugJournalResult]([]result.PolywaveError{
			result.NewFatal(result.CodeExportNoEntries,
				fmt.Sprintf("journal not found for agent %s\n\nExpected location: %s\n\nHint: Run 'polywave-tools list-impls' to see available IMPL docs, then check .polywave-state/ for agent journals.", opts.AgentPath, journalPath)),
		})
	}

	entriesResult := LoadJournalEntries(journalPath)
	if entriesResult.IsFatal() {
		return result.NewFailure[DebugJournalResult](entriesResult.Errors)
	}
	entries := entriesResult.GetData()

	partial.TotalCount = len(entries)

	// Apply filters.
	filtered := entries
	if opts.FailuresOnly {
		filtered = filterFailedEntries(entries)
	}
	if opts.Last > 0 && len(filtered) > opts.Last {
		filtered = filtered[len(filtered)-opts.Last:]
	}

	partial.FilterCount = len(filtered)
	partial.Entries = filtered

	// Handle HTML export.
	if opts.Export != "" {
		exportRes := exportHTMLTimeline(filtered, opts.AgentPath, opts.Export, opts.Force)
		if exportRes.IsFatal() {
			return result.NewFailure[DebugJournalResult](exportRes.Errors)
		}
		partial.ExportPath = opts.Export
		return result.NewSuccess(partial)
	}

	// Populate summary when requested.
	if opts.Summary {
		partial.Summary = buildJournalSummary(opts.AgentPath, entries, filtered)
	}

	return result.NewSuccess(partial)
}

// LoadJournalEntries reads all entries from index.jsonl.
// Returns empty slice (not error) if index.jsonl doesn't exist yet.
// Exported so cmd-layer code (e.g. journal_context.go) can delegate to it.
func LoadJournalEntries(journalPath string) result.Result[[]journal.ToolEntry] {
	entries, err := loadJournalEntries(journalPath)
	if err != nil {
		return result.NewFailure[[]journal.ToolEntry]([]result.PolywaveError{
			result.NewFatal(result.CodeExportWriteFailed,
				"failed to load journal: "+err.Error()).WithCause(err),
		})
	}
	return result.NewSuccess(entries)
}

// loadJournalEntries is the internal implementation.
func loadJournalEntries(journalPath string) ([]journal.ToolEntry, error) {
	f, err := os.Open(journalPath)
	if os.IsNotExist(err) {
		return []journal.ToolEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []journal.ToolEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10 MB max line size

	for scanner.Scan() {
		var entry journal.ToolEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip malformed lines.
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// filterFailedEntries returns tool_result entries that match error patterns,
// together with their preceding tool_use entries for context.
func filterFailedEntries(entries []journal.ToolEntry) []journal.ToolEntry {
	var failed []journal.ToolEntry

	errorPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)error:`),
		regexp.MustCompile(`(?i)failed`),
		regexp.MustCompile(`(?i)fatal`),
		regexp.MustCompile(`FAIL`),
		regexp.MustCompile(`exit status [1-9]`),
		regexp.MustCompile(`exit code: [1-9]`),
	}

	for _, entry := range entries {
		if entry.Kind != "tool_result" {
			continue
		}

		hasError := false
		for _, pattern := range errorPatterns {
			if pattern.MatchString(entry.Preview) {
				hasError = true
				break
			}
		}

		if hasError {
			// Check whether the corresponding tool_use was already added.
			alreadyAdded := false
			for i := len(failed) - 1; i >= 0; i-- {
				if failed[i].Kind == "tool_use" && failed[i].ToolUseID == entry.ToolUseID {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				// Find the tool_use in the original slice.
				for _, e := range entries {
					if e.Kind == "tool_use" && e.ToolUseID == entry.ToolUseID {
						failed = append(failed, e)
						break
					}
				}
			}
			failed = append(failed, entry)
		}
	}

	return failed
}

// buildJournalSummary constructs a JournalSummary from all entries.
func buildJournalSummary(agentPath string, all []journal.ToolEntry, display []journal.ToolEntry) *JournalSummary {
	if len(all) == 0 {
		return &JournalSummary{}
	}

	firstEntry := all[0]
	lastEntry := all[len(all)-1]
	duration := lastEntry.Timestamp.Sub(firstEntry.Timestamp)

	summary := &JournalSummary{
		Duration:      formatDuration(duration),
		FilesModified: extractFilesModified(all),
		CommandsRun:   extractCommandsRun(all),
		GitCommits:    extractGitCommits(all),
		ReportWritten: checkCompletionReportWritten(all),
	}

	return summary
}

// exportHTMLTimeline writes an interactive HTML timeline to exportPath.
func exportHTMLTimeline(entries []journal.ToolEntry, agentPath, exportPath string, force bool) result.Result[ExportData] {
	if !force {
		if _, err := os.Stat(exportPath); err == nil {
			return result.NewFailure[ExportData]([]result.PolywaveError{
				result.NewFatal(result.CodeExportFileExists,
					fmt.Sprintf("export file already exists: %s (use --force to overwrite)", exportPath)).
					WithContext("export_path", exportPath),
			})
		}
	}

	if len(entries) == 0 {
		return result.NewFailure[ExportData]([]result.PolywaveError{
			result.NewFatal(result.CodeExportNoEntries,
				"no entries to export"),
		})
	}

	htmlContent := generateTimelineHTML(entries, agentPath)

	if err := os.WriteFile(exportPath, []byte(htmlContent), 0644); err != nil {
		return result.NewFailure[ExportData]([]result.PolywaveError{
			result.NewFatal(result.CodeExportWriteFailed,
				fmt.Sprintf("failed to write HTML file: %v", err)).
				WithContext("export_path", exportPath),
		})
	}

	return result.NewSuccess(ExportData{
		ExportPath: exportPath,
		EntryCount: len(entries),
	})
}

// generateTimelineHTML creates a self-contained HTML timeline string.
func generateTimelineHTML(entries []journal.ToolEntry, agentPath string) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Journal Timeline: ` + html.EscapeString(agentPath) + `</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 20px; background: #f5f5f5; }
        h1 { color: #333; }
        .timeline { position: relative; padding: 20px 0; margin: 20px 0; }
        .timeline-line { position: absolute; left: 50px; top: 0; bottom: 0; width: 2px; background: #ddd; }
        .entry { position: relative; padding: 10px 0 10px 80px; min-height: 40px; cursor: pointer; }
        .entry:hover { background: #fff; }
        .entry-dot { position: absolute; left: 42px; top: 15px; width: 16px; height: 16px; border-radius: 50%; border: 2px solid #fff; }
        .entry-dot.success { background: #4CAF50; }
        .entry-dot.failure { background: #f44336; }
        .entry-dot.tool-use { background: #2196F3; }
        .entry-time { font-size: 12px; color: #666; margin-right: 10px; }
        .entry-tool { font-weight: bold; color: #333; }
        .entry-detail { display: none; margin-top: 10px; padding: 10px; background: #fff; border-left: 3px solid #2196F3; white-space: pre-wrap; font-family: monospace; font-size: 12px; max-height: 400px; overflow-y: auto; }
        .entry.expanded .entry-detail { display: block; }
        .stats { background: #fff; padding: 15px; border-radius: 4px; margin-bottom: 20px; }
        .stats-item { display: inline-block; margin-right: 30px; }
        .stats-label { font-weight: bold; color: #666; }
    </style>
</head>
<body>
    <h1>Journal Timeline: ` + html.EscapeString(agentPath) + `</h1>
`)

	firstEntry := entries[0]
	lastEntry := entries[len(entries)-1]
	duration := lastEntry.Timestamp.Sub(firstEntry.Timestamp)

	toolUseCount := 0
	toolResultCount := 0
	failureCount := 0
	for _, entry := range entries {
		if entry.Kind == "tool_use" {
			toolUseCount++
		} else if entry.Kind == "tool_result" {
			toolResultCount++
			if strings.Contains(entry.Preview, "error:") || strings.Contains(entry.Preview, "FAIL") {
				failureCount++
			}
		}
	}

	sb.WriteString(`    <div class="stats">`)
	sb.WriteString(fmt.Sprintf(`<div class="stats-item"><span class="stats-label">Duration:</span> %s</div>`, formatDuration(duration)))
	sb.WriteString(fmt.Sprintf(`<div class="stats-item"><span class="stats-label">Tool calls:</span> %d</div>`, toolUseCount))
	sb.WriteString(fmt.Sprintf(`<div class="stats-item"><span class="stats-label">Results:</span> %d</div>`, toolResultCount))
	sb.WriteString(fmt.Sprintf(`<div class="stats-item"><span class="stats-label">Failures:</span> %d</div>`, failureCount))
	sb.WriteString(`    </div>`)

	sb.WriteString(`    <div class="timeline">`)
	sb.WriteString(`        <div class="timeline-line"></div>`)

	for i, entry := range entries {
		dotClass := "tool-use"
		if entry.Kind == "tool_result" {
			if strings.Contains(entry.Preview, "error:") || strings.Contains(entry.Preview, "FAIL") {
				dotClass = "failure"
			} else {
				dotClass = "success"
			}
		}

		timeStr := entry.Timestamp.Format("15:04:05")
		toolLabel := entry.Kind
		if entry.ToolName != "" {
			toolLabel = entry.ToolName
		}

		sb.WriteString(fmt.Sprintf(`        <div class="entry" onclick="toggleDetail(%d)">`, i))
		sb.WriteString(fmt.Sprintf(`            <div class="entry-dot %s"></div>`, dotClass))
		sb.WriteString(fmt.Sprintf(`            <span class="entry-time">%s</span>`, timeStr))
		sb.WriteString(fmt.Sprintf(`            <span class="entry-tool">%s</span>`, html.EscapeString(toolLabel)))

		sb.WriteString(fmt.Sprintf(`            <div class="entry-detail" id="detail-%d">`, i))
		sb.WriteString(fmt.Sprintf(`<strong>Timestamp:</strong> %s\n`, entry.Timestamp.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf(`<strong>Kind:</strong> %s\n`, entry.Kind))
		sb.WriteString(fmt.Sprintf(`<strong>Tool Use ID:</strong> %s\n\n`, entry.ToolUseID))

		if entry.Kind == "tool_use" && len(entry.Input) > 0 {
			sb.WriteString(`<strong>Input:</strong>` + "\n")
			inputJSON, _ := json.MarshalIndent(entry.Input, "", "  ")
			sb.WriteString(html.EscapeString(string(inputJSON)))
		} else if entry.Kind == "tool_result" {
			sb.WriteString(`<strong>Output:</strong>` + "\n")
			preview := entry.Preview
			if len(preview) > 2000 {
				preview = preview[:2000] + "\n... (truncated)"
			}
			sb.WriteString(html.EscapeString(preview))
			if entry.Truncated {
				sb.WriteString(fmt.Sprintf("\n\n<em>Full output saved to: %s</em>", entry.ContentFile))
			}
		}

		sb.WriteString(`            </div>`)
		sb.WriteString(`        </div>`)
	}

	sb.WriteString(`    </div>`)

	sb.WriteString(`
    <script>
        function toggleDetail(index) {
            const entry = document.querySelectorAll('.entry')[index];
            entry.classList.toggle('expanded');
        }
    </script>
</body>
</html>`)

	return sb.String()
}

// --- Helper extract functions ---

func extractFilesModified(entries []journal.ToolEntry) []FileModification {
	fileMap := make(map[string]*FileModification)

	for _, entry := range entries {
		if entry.Kind != "tool_use" {
			continue
		}

		var path string
		var operation string

		switch entry.ToolName {
		case "Edit":
			if fp, ok := entry.Input["file_path"].(string); ok {
				path = fp
				operation = "modified"
			}
		case "Write":
			if fp, ok := entry.Input["file_path"].(string); ok {
				path = fp
				operation = "added"
			}
		}

		if path != "" {
			linesAdded := 0
			if entry.ToolName == "Edit" {
				if newStr, ok := entry.Input["new_string"].(string); ok {
					linesAdded = strings.Count(newStr, "\n") + 1
				}
			} else if entry.ToolName == "Write" {
				if content, ok := entry.Input["content"].(string); ok {
					linesAdded = strings.Count(content, "\n") + 1
				}
			}

			if existing, exists := fileMap[path]; exists {
				existing.LinesAdded += linesAdded
			} else {
				fileMap[path] = &FileModification{
					Path:       path,
					LinesAdded: linesAdded,
					Operation:  operation,
				}
			}
		}
	}

	result := make([]FileModification, 0, len(fileMap))
	for _, fm := range fileMap {
		result = append(result, *fm)
	}
	return result
}

func extractCommandsRun(entries []journal.ToolEntry) []CommandRun {
	cmdMap := make(map[string]*CommandRun)

	for i, entry := range entries {
		if entry.Kind != "tool_use" || entry.ToolName != "Bash" {
			continue
		}

		command, ok := entry.Input["command"].(string)
		if !ok {
			continue
		}

		if len(command) > 40 {
			command = command[:37] + "..."
		}

		status := "unknown"
		for j := i + 1; j < len(entries) && j < i+5; j++ {
			if entries[j].Kind == "tool_result" && entries[j].ToolUseID == entry.ToolUseID {
				if strings.Contains(entries[j].Preview, "error:") || strings.Contains(entries[j].Preview, "FAIL") {
					status = "failed"
				} else {
					status = "passed"
				}
				break
			}
		}

		if existing, exists := cmdMap[command]; exists {
			existing.Runs++
			if status == "failed" {
				existing.Status = fmt.Sprintf("%d runs: %d failed, %d passed",
					existing.Runs,
					countFailures(existing.Status)+1,
					existing.Runs-countFailures(existing.Status)-1)
			}
		} else {
			cmdMap[command] = &CommandRun{
				Command: command,
				Status:  fmt.Sprintf("1 run: %s", status),
				Runs:    1,
			}
		}
	}

	result := make([]CommandRun, 0, len(cmdMap))
	for _, cmd := range cmdMap {
		result = append(result, *cmd)
	}
	return result
}

func countFailures(status string) int {
	pattern := regexp.MustCompile(`(\d+) failed`)
	matches := pattern.FindStringSubmatch(status)
	if len(matches) > 1 {
		var count int
		fmt.Sscanf(matches[1], "%d", &count)
		return count
	}
	return 0
}

func extractGitCommits(entries []journal.ToolEntry) []GitCommit {
	var commits []GitCommit

	gitCommitPattern := regexp.MustCompile(`git\s+(?:-C\s+\S+\s+)?commit`)
	shaPattern := regexp.MustCompile(`\[([^\s\]]+)\s+([a-f0-9]{7,40})\]`)

	for i, entry := range entries {
		if entry.Kind != "tool_use" || entry.ToolName != "Bash" {
			continue
		}

		command, ok := entry.Input["command"].(string)
		if !ok || !gitCommitPattern.MatchString(command) {
			continue
		}

		var res *journal.ToolEntry
		for j := i + 1; j < len(entries) && j < i+5; j++ {
			if entries[j].Kind == "tool_result" && entries[j].ToolUseID == entry.ToolUseID {
				res = &entries[j]
				break
			}
		}

		if res == nil {
			continue
		}

		shaMatch := shaPattern.FindStringSubmatch(res.Preview)
		if len(shaMatch) < 3 {
			continue
		}

		branch := shaMatch[1]
		sha := shaMatch[2]

		message := "commit"
		msgPattern := regexp.MustCompile(`-m\s+["']([^"']+)["']`)
		if msgMatch := msgPattern.FindStringSubmatch(command); len(msgMatch) > 1 {
			message = msgMatch[1]
		}

		commits = append(commits, GitCommit{
			SHA:     sha,
			Message: message,
			Branch:  branch,
		})
	}

	return commits
}

func checkCompletionReportWritten(entries []journal.ToolEntry) bool {
	reportPattern := regexp.MustCompile(`(?i)## Agent .* Completion Report|type=impl-completion-report`)

	for _, entry := range entries {
		if entry.Kind != "tool_use" {
			continue
		}

		if entry.ToolName == "Edit" || entry.ToolName == "Write" {
			if content, ok := entry.Input["content"].(string); ok {
				if reportPattern.MatchString(content) {
					return true
				}
			}
			if newStr, ok := entry.Input["new_string"].(string); ok {
				if reportPattern.MatchString(newStr) {
					return true
				}
			}
		}
	}

	return false
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
