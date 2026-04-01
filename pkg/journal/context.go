package journal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// FileModification represents a detected file change from Edit/Write tool use
type FileModification struct {
	Path         string
	LinesAdded   int
	LinesDeleted int
	LastEdited   time.Time
	Operation    string // "added", "modified", "deleted"
}

// TestRun represents a detected test execution from Bash tool results
type TestRun struct {
	Command   string
	Passed    int
	Failed    int
	Timestamp time.Time
	ExitCode  int
}

// GitCommit represents a detected git commit from Bash tool results
type GitCommit struct {
	SHA        string
	Message    string
	Branch     string
	Files      int
	Insertions int
	Deletions  int
	Timestamp  time.Time
}

// GateStatus represents verification gate execution status
type GateStatus struct {
	Command   string
	Status    string // "PASS", "FAIL", "PENDING"
	Timestamp time.Time
}

// GenerateContext analyzes journal entries and produces markdown summary for agent context injection.
// maxEntries limits how many entries to process; if 0, all entries are processed.
func GenerateContext(entries []ToolEntry, maxEntries int) (string, error) {
	if len(entries) == 0 {
		return "## Session Context (Recovered from Tool Journal)\n\n**No tool activity recorded yet.**\n", nil
	}

	// Apply max entries limit if specified
	processEntries := entries
	if maxEntries > 0 && len(entries) > maxEntries {
		processEntries = entries[len(entries)-maxEntries:]
	}

	// Extract data
	filesModified := extractFilesModified(processEntries)
	testResults := extractTestResults(processEntries)
	gitCommits := extractGitCommits(processEntries)
	scaffoldImports := extractScaffoldImports(processEntries)
	verificationGates := extractVerificationGates(processEntries)
	completionReportWritten := extractCompletionReportStatus(processEntries)

	// Calculate session stats
	lastActivity := entries[len(entries)-1].Timestamp
	firstActivity := entries[0].Timestamp
	duration := lastActivity.Sub(firstActivity)
	totalTools := len(entries)

	var sb strings.Builder
	sb.WriteString("## Session Context (Recovered from Tool Journal)\n\n")

	// Session stats
	sb.WriteString(fmt.Sprintf("**Last activity:** %s (%s ago)\n",
		lastActivity.Format("2006-01-02 15:04:05"),
		formatRelativeTime(time.Since(lastActivity))))
	sb.WriteString(fmt.Sprintf("**Total tool calls:** %d\n", totalTools))
	sb.WriteString(fmt.Sprintf("**Session duration:** %s\n\n", formatDuration(duration)))

	// Files modified section
	if len(filesModified) > 0 {
		sb.WriteString(fmt.Sprintf("### Files Modified (%d)\n", len(filesModified)))
		for _, fm := range filesModified {
			changeDesc := formatFileChange(fm)
			timeAgo := formatRelativeTime(time.Since(fm.LastEdited))
			sb.WriteString(fmt.Sprintf("- `%s` (%s) — last edited %s ago\n",
				fm.Path, changeDesc, timeAgo))
		}
		sb.WriteString("\n")
	}

	// Test results section
	if len(testResults) > 0 {
		sb.WriteString("### Tests Run\n")
		for _, tr := range testResults {
			status := "✓"
			if tr.Failed > 0 {
				status = "✗"
			}
			timeStr := tr.Timestamp.Format("15:04")
			if tr.Failed > 0 {
				sb.WriteString(fmt.Sprintf("- `%s` → %d passed, %d failed %s — %s\n",
					tr.Command, tr.Passed, tr.Failed, status, timeStr))
			} else {
				sb.WriteString(fmt.Sprintf("- `%s` → %d passed %s — %s\n",
					tr.Command, tr.Passed, status, timeStr))
			}
		}
		sb.WriteString("\n")
	}

	// Git commits section
	if len(gitCommits) > 0 {
		sb.WriteString("### Git Commits\n")
		for _, gc := range gitCommits {
			timeStr := gc.Timestamp.Format("15:04")
			sb.WriteString(fmt.Sprintf("- **%s** \"%s\"\n", gc.SHA[:7], gc.Message))
			sb.WriteString(fmt.Sprintf("  (committed %s, branch: %s, %d files changed",
				timeStr, gc.Branch, gc.Files))
			if gc.Insertions > 0 || gc.Deletions > 0 {
				sb.WriteString(fmt.Sprintf(", %d insertions, %d deletions",
					gc.Insertions, gc.Deletions))
			}
			sb.WriteString(")\n")
		}
		sb.WriteString("\n")
	}

	// Scaffold imports section
	if len(scaffoldImports) > 0 {
		sb.WriteString("### Scaffold Files Imported\n")
		for _, path := range scaffoldImports {
			sb.WriteString(fmt.Sprintf("- `%s`\n", path))
		}
		sb.WriteString("\n")
	}

	// Verification gates section
	if len(verificationGates) > 0 {
		sb.WriteString("### Verification Status (Field 6 Gates)\n")
		// Sort gates by status (PASS first, then FAIL, then PENDING)
		sortedGates := make([]string, 0, len(verificationGates))
		for name := range verificationGates {
			sortedGates = append(sortedGates, name)
		}
		sort.Slice(sortedGates, func(i, j int) bool {
			statusOrder := map[string]int{"PASS": 0, "FAIL": 1, "PENDING": 2}
			si := statusOrder[verificationGates[sortedGates[i]].Status]
			sj := statusOrder[verificationGates[sortedGates[j]].Status]
			return si < sj
		})

		for _, name := range sortedGates {
			gate := verificationGates[name]
			icon := "⏳"
			if gate.Status == "PASS" {
				icon = "✓"
			} else if gate.Status == "FAIL" {
				icon = "✗"
			}
			sb.WriteString(fmt.Sprintf("- %s %s: `%s` — %s (%s)\n",
				icon, name, gate.Command, gate.Status, gate.Timestamp.Format("15:04")))
		}
		sb.WriteString("\n")
	}

	// Completion report status
	sb.WriteString("### Completion Report Status\n")
	if completionReportWritten {
		sb.WriteString("- ✓ Completion report written to IMPL doc\n")
	} else {
		sb.WriteString("- ⏳ Not yet written (next step after verification gates pass)\n")
	}
	sb.WriteString("\n")

	// Next steps hint (if report not written)
	if !completionReportWritten {
		pendingGates := []string{}
		for name, gate := range verificationGates {
			if gate.Status == "PENDING" {
				pendingGates = append(pendingGates, name)
			}
		}
		if len(pendingGates) > 0 {
			sort.Strings(pendingGates)
			sb.WriteString(fmt.Sprintf("**What's next:** Run pending gates (%s), then write completion report.\n",
				strings.Join(pendingGates, ", ")))
		} else {
			sb.WriteString("**What's next:** Write completion report to IMPL doc.\n")
		}
	}

	return sb.String(), nil
}

// extractFilesModified parses Edit and Write tool calls to detect file modifications
func extractFilesModified(entries []ToolEntry) []FileModification {
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
				// Check if this is a new file or existing file
				if existing, exists := fileMap[path]; exists && existing.Operation == "modified" {
					operation = "modified"
				} else {
					operation = "added"
				}
			}
		}

		if path != "" {
			// Count lines if content available
			linesAdded := 0
			linesDeleted := 0

			if entry.ToolName == "Edit" {
				if newStr, ok := entry.Input["new_string"].(string); ok {
					linesAdded = strings.Count(newStr, "\n") + 1
				}
				if oldStr, ok := entry.Input["old_string"].(string); ok {
					linesDeleted = strings.Count(oldStr, "\n") + 1
				}
			} else if entry.ToolName == "Write" {
				if content, ok := entry.Input["content"].(string); ok {
					linesAdded = strings.Count(content, "\n") + 1
				}
			}

			if existing, exists := fileMap[path]; exists {
				existing.LinesAdded += linesAdded
				existing.LinesDeleted += linesDeleted
				existing.LastEdited = entry.Timestamp
			} else {
				fileMap[path] = &FileModification{
					Path:         path,
					LinesAdded:   linesAdded,
					LinesDeleted: linesDeleted,
					LastEdited:   entry.Timestamp,
					Operation:    operation,
				}
			}
		}
	}

	// Convert map to sorted slice (by timestamp, newest first)
	result := make([]FileModification, 0, len(fileMap))
	for _, fm := range fileMap {
		result = append(result, *fm)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastEdited.After(result[j].LastEdited)
	})

	return result
}

// extractTestResults parses Bash tool results looking for test command patterns
func extractTestResults(entries []ToolEntry) []TestRun {
	var results []TestRun

	// Patterns for test commands
	testCmdPattern := regexp.MustCompile(`(?i)(go test|npm test|cargo test|pytest|python -m pytest)`)
	// Patterns for test output
	goTestCountPattern := regexp.MustCompile(`(\d+) passed?|(\d+) failed?`)
	exitCodePattern := regexp.MustCompile(`exit status (\d+)`)

	for i, entry := range entries {
		if entry.Kind != "tool_use" || entry.ToolName != "Bash" {
			continue
		}

		// Check if this is a test command
		command, ok := entry.Input["command"].(string)
		if !ok || !testCmdPattern.MatchString(command) {
			continue
		}

		// Look for the corresponding tool_result
		var result *ToolEntry
		for j := i + 1; j < len(entries) && j < i+5; j++ {
			if entries[j].Kind == "tool_result" && entries[j].ToolUseID == entry.ToolUseID {
				result = &entries[j]
				break
			}
		}

		if result == nil {
			continue
		}

		// Parse test results from preview
		passed := 0
		failed := 0

		// Try Go test format
		if strings.Contains(command, "go test") {
			matches := goTestCountPattern.FindAllStringSubmatch(result.Preview, -1)
			for _, match := range matches {
				if match[1] != "" {
					fmt.Sscanf(match[1], "%d", &passed)
				}
				if match[2] != "" {
					fmt.Sscanf(match[2], "%d", &failed)
				}
			}
			// Also check for PASS/FAIL markers
			if passed == 0 && failed == 0 {
				if strings.Contains(result.Preview, "PASS") {
					passed = 1
				}
				if strings.Contains(result.Preview, "FAIL") {
					failed = 1
				}
			}
		}

		exitCode := 0
		if m := exitCodePattern.FindStringSubmatch(result.Preview); len(m) > 1 {
			fmt.Sscanf(m[1], "%d", &exitCode)
		}
		results = append(results, TestRun{
			Command:   command,
			Passed:    passed,
			Failed:    failed,
			Timestamp: result.Timestamp,
			ExitCode:  exitCode,
		})
	}

	return results
}

// extractGitCommits parses Bash tool results looking for git commit commands
func extractGitCommits(entries []ToolEntry) []GitCommit {
	var commits []GitCommit

	gitCommitPattern := regexp.MustCompile(`git\s+(?:-C\s+\S+\s+)?commit`)
	shaPattern := regexp.MustCompile(`\[([^\s\]]+)\s+([a-f0-9]{7,40})\]`)
	statsPattern := regexp.MustCompile(`(\d+)\s+files?\s+changed(?:,\s+(\d+)\s+insertions?\([+]\))?(?:,\s+(\d+)\s+deletions?\([-]\))?`)

	for i, entry := range entries {
		if entry.Kind != "tool_use" || entry.ToolName != "Bash" {
			continue
		}

		command, ok := entry.Input["command"].(string)
		if !ok || !gitCommitPattern.MatchString(command) {
			continue
		}

		// Look for the corresponding tool_result
		var result *ToolEntry
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

		// Extract commit message from command
		message := "commit"
		if strings.Contains(command, "-m") {
			// Try to extract message from -m flag
			msgPattern := regexp.MustCompile(`-m\s+["']([^"']+)["']`)
			if msgMatch := msgPattern.FindStringSubmatch(command); len(msgMatch) > 1 {
				message = msgMatch[1]
			}
		}

		// Parse stats
		files := 0
		insertions := 0
		deletions := 0
		if statsMatch := statsPattern.FindStringSubmatch(result.Preview); len(statsMatch) > 1 {
			fmt.Sscanf(statsMatch[1], "%d", &files)
			if len(statsMatch) > 2 && statsMatch[2] != "" {
				fmt.Sscanf(statsMatch[2], "%d", &insertions)
			}
			if len(statsMatch) > 3 && statsMatch[3] != "" {
				fmt.Sscanf(statsMatch[3], "%d", &deletions)
			}
		}

		commits = append(commits, GitCommit{
			SHA:        sha,
			Message:    message,
			Branch:     branch,
			Files:      files,
			Insertions: insertions,
			Deletions:  deletions,
			Timestamp:  result.Timestamp,
		})
	}

	return commits
}

// extractScaffoldImports detects Read tool calls for scaffold files
func extractScaffoldImports(entries []ToolEntry) []string {
	seen := make(map[string]bool)
	var imports []string

	scaffoldPattern := regexp.MustCompile(`(?i)scaffold|types\.go|interface|contract`)

	for _, entry := range entries {
		if entry.Kind != "tool_use" || entry.ToolName != "Read" {
			continue
		}

		filePath, ok := entry.Input["file_path"].(string)
		if !ok {
			continue
		}

		// Check if this looks like a scaffold file
		if scaffoldPattern.MatchString(filePath) || strings.Contains(filePath, "types.go") {
			if !seen[filePath] {
				seen[filePath] = true
				imports = append(imports, filePath)
			}
		}
	}

	return imports
}

// extractVerificationGates detects Field 6 gate commands (build/test/lint)
func extractVerificationGates(entries []ToolEntry) map[string]GateStatus {
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

		// Look for the corresponding tool_result
		var result *ToolEntry
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
			// Check if command succeeded (look for error patterns)
			if strings.Contains(result.Preview, "FAIL") ||
				strings.Contains(result.Preview, "error:") ||
				strings.Contains(result.Preview, "Error:") {
				status = "FAIL"
			} else {
				status = "PASS"
			}
		}

		// Update gate status (keep most recent)
		if existing, exists := gates[gateName]; !exists || timestamp.After(existing.Timestamp) {
			gates[gateName] = GateStatus{
				Command:   command,
				Status:    status,
				Timestamp: timestamp,
			}
		}
	}

	return gates
}

// extractCompletionReportStatus checks if completion report was written
func extractCompletionReportStatus(entries []ToolEntry) bool {
	reportPattern := regexp.MustCompile(`(?i)## Agent .* Completion Report|type=impl-completion-report`)

	for _, entry := range entries {
		if entry.Kind != "tool_use" {
			continue
		}

		// Check Edit/Write tools for completion report
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

// Helper functions for formatting

func formatRelativeTime(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
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
	if fm.LinesDeleted == 0 {
		return fmt.Sprintf("added %d lines", fm.LinesAdded)
	}
	if fm.LinesAdded == 0 {
		return fmt.Sprintf("deleted %d lines", fm.LinesDeleted)
	}
	return fmt.Sprintf("+%d/-%d lines", fm.LinesAdded, fm.LinesDeleted)
}
