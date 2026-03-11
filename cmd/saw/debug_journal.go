package main

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

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/spf13/cobra"
)

// DebugOpts holds options for the debug-journal command
type DebugOpts struct {
	Summary      bool
	FailuresOnly bool
	Last         int
	Export       string
	Force        bool
}

// newDebugJournalCmd returns a cobra.Command that inspects journal contents
func newDebugJournalCmd() *cobra.Command {
	opts := DebugOpts{}

	cmd := &cobra.Command{
		Use:   "debug-journal <agent-path>",
		Short: "Inspect journal contents for debugging failed agents",
		Long: `Inspect tool execution journal for a specific agent.

Agent path format: wave1/agent-A or wave2-agent-B

Examples:
  sawtools debug-journal wave1/agent-A                    # dump full journal (JSONL)
  sawtools debug-journal wave1/agent-A --summary          # human-readable summary
  sawtools debug-journal wave1/agent-A --failures-only    # show only failed tool calls
  sawtools debug-journal wave1/agent-A --last 20          # show last N entries
  sawtools debug-journal wave1/agent-A --export timeline.html  # export HTML timeline
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentPath := args[0]
			return debugJournalCommand(repoDir, agentPath, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Summary, "summary", false, "Show human-readable summary")
	cmd.Flags().BoolVar(&opts.FailuresOnly, "failures-only", false, "Show only failed tool calls")
	cmd.Flags().IntVar(&opts.Last, "last", 0, "Show last N entries only")
	cmd.Flags().StringVar(&opts.Export, "export", "", "Export HTML timeline to file")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Overwrite export file if it exists")

	return cmd
}

// debugJournalCommand implements the debug-journal command
func debugJournalCommand(repoRoot string, agentPath string, opts DebugOpts) error {
	// Resolve repo root to absolute path
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve repo root: %w", err)
	}

	// Parse agent path (e.g., "wave1/agent-A" or "wave2-agent-B")
	journalPath := filepath.Join(absRepoRoot, ".saw-state", agentPath, "index.jsonl")

	// Check if journal exists
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		return fmt.Errorf("journal not found for agent %s\n\nExpected location: %s\n\nHint: Run 'sawtools list-impls' to see available IMPL docs, then check .saw-state/ for agent journals.", agentPath, journalPath)
	}

	// Load journal entries
	entries, err := loadJournalEntries(journalPath)
	if err != nil {
		return fmt.Errorf("failed to load journal: %w", err)
	}

	if len(entries) == 0 {
		fmt.Printf("Journal for %s exists but contains no entries yet.\n", agentPath)
		return nil
	}

	// Apply filters
	filteredEntries := entries
	if opts.FailuresOnly {
		filteredEntries = filterFailedEntries(entries)
	}
	if opts.Last > 0 && len(filteredEntries) > opts.Last {
		filteredEntries = filteredEntries[len(filteredEntries)-opts.Last:]
	}

	// Handle export
	if opts.Export != "" {
		return exportHTMLTimeline(filteredEntries, agentPath, opts.Export, opts.Force)
	}

	// Handle summary
	if opts.Summary {
		return printSummary(agentPath, entries, filteredEntries)
	}

	// Default: dump raw JSONL
	for _, entry := range filteredEntries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		fmt.Println(string(data))
	}

	return nil
}

// loadJournalEntries reads all entries from index.jsonl
func loadJournalEntries(journalPath string) ([]journal.ToolEntry, error) {
	f, err := os.Open(journalPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []journal.ToolEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line size

	for scanner.Scan() {
		var entry journal.ToolEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip malformed lines
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// filterFailedEntries filters for tool_result entries with errors
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

		// Check preview for error patterns
		hasError := false
		for _, pattern := range errorPatterns {
			if pattern.MatchString(entry.Preview) {
				hasError = true
				break
			}
		}

		if hasError {
			// Include the corresponding tool_use for context
			// Find preceding tool_use with matching ID
			for i := len(failed) - 1; i >= 0; i-- {
				if failed[i].Kind == "tool_use" && failed[i].ToolUseID == entry.ToolUseID {
					// Already added, just append result
					failed = append(failed, entry)
					hasError = false
					break
				}
			}
			if hasError {
				// Need to find the tool_use in original entries
				for _, e := range entries {
					if e.Kind == "tool_use" && e.ToolUseID == entry.ToolUseID {
						failed = append(failed, e)
						break
					}
				}
				failed = append(failed, entry)
			}
		}
	}

	return failed
}

// printSummary generates and prints a human-readable summary
func printSummary(agentPath string, allEntries []journal.ToolEntry, displayEntries []journal.ToolEntry) error {
	if len(allEntries) == 0 {
		fmt.Printf("Journal: %s\nNo entries yet.\n", agentPath)
		return nil
	}

	firstEntry := allEntries[0]
	lastEntry := allEntries[len(allEntries)-1]
	duration := lastEntry.Timestamp.Sub(firstEntry.Timestamp)

	fmt.Printf("Journal: %s\n", agentPath)
	fmt.Printf("Duration: %s (%s - %s)\n",
		formatDuration(duration),
		firstEntry.Timestamp.Format("15:04:05"),
		lastEntry.Timestamp.Format("15:04:05"))
	fmt.Printf("Total tool calls: %d\n\n", len(allEntries))

	// Files modified
	files := extractFilesModified(allEntries)
	if len(files) > 0 {
		fmt.Printf("Files modified: %d\n", len(files))
		for _, fm := range files {
			changeDesc := formatFileChange(fm)
			fmt.Printf("  %-40s (%s)\n", fm.Path, changeDesc)
		}
		fmt.Println()
	}

	// Commands run
	commands := extractCommandsRun(allEntries)
	if len(commands) > 0 {
		fmt.Printf("Commands run: %d\n", len(commands))
		for _, cmd := range commands {
			fmt.Printf("  %-40s (%s)\n", cmd.Command, cmd.Status)
		}
		fmt.Println()
	}

	// Git commits
	commits := extractGitCommits(allEntries)
	if len(commits) > 0 {
		fmt.Printf("Commits: %d\n", len(commits))
		for _, gc := range commits {
			fmt.Printf("  %s \"%s\" (%s)\n", gc.SHA[:7], gc.Message, gc.Branch)
		}
		fmt.Println()
	}

	// Verification gates
	gates := extractVerificationGates(allEntries)
	if len(gates) > 0 {
		fmt.Println("Verification gates:")
		for name, gate := range gates {
			icon := "✓"
			if gate.Status == "FAIL" {
				icon = "✗"
			} else if gate.Status == "PENDING" {
				icon = "⏳"
			}
			fmt.Printf("  %s %-10s (%s)\n", icon, name, gate.Timestamp.Format("15:04"))
		}
		fmt.Println()
	}

	// Completion report status
	reportWritten := checkCompletionReportWritten(allEntries)
	fmt.Println("Completion report:")
	if reportWritten {
		fmt.Println("  ✓ Written")
	} else {
		fmt.Println("  ⏳ Not yet written")
	}

	return nil
}

// exportHTMLTimeline creates an interactive HTML timeline visualization
func exportHTMLTimeline(entries []journal.ToolEntry, agentPath string, exportPath string, force bool) error {
	// Check if file exists
	if !force {
		if _, err := os.Stat(exportPath); err == nil {
			return fmt.Errorf("export file already exists: %s (use --force to overwrite)", exportPath)
		}
	}

	if len(entries) == 0 {
		return fmt.Errorf("no entries to export")
	}

	// Generate HTML
	htmlContent := generateTimelineHTML(entries, agentPath)

	// Write to file
	if err := os.WriteFile(exportPath, []byte(htmlContent), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	fmt.Printf("Timeline exported to: %s\n", exportPath)
	return nil
}

// generateTimelineHTML creates a self-contained HTML timeline
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

	// Stats
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

	// Timeline
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

		// Detail content
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

	// JavaScript for interactivity
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

// Helper functions

type FileModification struct {
	Path       string
	LinesAdded int
	Operation  string
}

type CommandRun struct {
	Command string
	Status  string
	Runs    int
}

type GitCommit struct {
	SHA     string
	Message string
	Branch  string
}

type GateStatus struct {
	Status    string
	Timestamp time.Time
}

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

		// Truncate long commands
		if len(command) > 40 {
			command = command[:37] + "..."
		}

		// Find result
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

		// Find result
		var result *journal.ToolEntry
		for j := i + 1; j < len(entries) && j < i+5; j++ {
			if entries[j].Kind == "tool_result" && entries[j].ToolUseID == entry.ToolUseID {
				result = &entries[j]
				break
			}
		}

		if result == nil {
			continue
		}

		// Parse commit info
		shaMatch := shaPattern.FindStringSubmatch(result.Preview)
		if len(shaMatch) < 3 {
			continue
		}

		branch := shaMatch[1]
		sha := shaMatch[2]

		// Extract message
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

func extractVerificationGates(entries []journal.ToolEntry) map[string]GateStatus {
	gates := make(map[string]GateStatus)

	buildPattern := regexp.MustCompile(`(?i)go build|cargo build|npm run build|make build`)
	testPattern := regexp.MustCompile(`(?i)go test|cargo test|npm test|pytest`)
	lintPattern := regexp.MustCompile(`(?i)go vet|cargo clippy|eslint|pylint|golangci-lint`)

	for i, entry := range entries {
		if entry.Kind != "tool_use" || entry.ToolName != "Bash" {
			continue
		}

		command, ok := entry.Input["command"].(string)
		if !ok {
			continue
		}

		var gateName string
		if buildPattern.MatchString(command) {
			gateName = "Build"
		} else if testPattern.MatchString(command) {
			gateName = "Tests"
		} else if lintPattern.MatchString(command) {
			gateName = "Lint"
		} else {
			continue
		}

		// Find result
		var result *journal.ToolEntry
		for j := i + 1; j < len(entries) && j < i+5; j++ {
			if entries[j].Kind == "tool_result" && entries[j].ToolUseID == entry.ToolUseID {
				result = &entries[j]
				break
			}
		}

		status := "PENDING"
		timestamp := entry.Timestamp

		if result != nil {
			timestamp = result.Timestamp
			if strings.Contains(result.Preview, "FAIL") ||
				strings.Contains(result.Preview, "error:") ||
				strings.Contains(result.Preview, "Error:") {
				status = "FAIL"
			} else {
				status = "PASS"
			}
		}

		if existing, exists := gates[gateName]; !exists || timestamp.After(existing.Timestamp) {
			gates[gateName] = GateStatus{
				Status:    status,
				Timestamp: timestamp,
			}
		}
	}

	return gates
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

func formatFileChange(fm FileModification) string {
	if fm.Operation == "added" {
		return fmt.Sprintf("added, %d lines", fm.LinesAdded)
	}
	return fmt.Sprintf("%d lines added", fm.LinesAdded)
}
