// Package protocol parses and updates IMPL documents — the single source of
// truth for SAW protocol execution. It extracts wave/agent structure,
// reads completion reports written by agents, and ticks status checkboxes
// once agents report completion. All IMPL doc I/O is concentrated here.
package protocol

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
	"gopkg.in/yaml.v3"
)

// sawCompleteRe matches <!-- SAW:COMPLETE YYYY-MM-DD --> and captures the date.
var sawCompleteRe = regexp.MustCompile(`<!--\s*SAW:COMPLETE\s+(\d{4}-\d{2}-\d{2})\s*-->`)

// ParseIMPLDoc parses a markdown IMPL doc at path and returns a structured
// IMPLDoc. It uses a line-by-line state machine — no full CommonMark parser.
//
// Partial results: if the file is readable but some sections are malformed,
// ParseIMPLDoc may return a non-nil *types.IMPLDoc together with a non-nil
// error. Callers should check both return values and use whichever partial
// data is available.
func ParseIMPLDoc(path string) (*types.IMPLDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ParseIMPLDoc: cannot open %q: %w", path, err)
	}
	defer f.Close()

	doc := &types.IMPLDoc{
		FileOwnership: make(map[string]types.FileOwnershipInfo),
	}

	type parserState int
	const (
		stateTop         parserState = iota // scanning top-level headers
		stateFileOwner                      // inside ### File Ownership table
		stateWave                           // inside a ## Wave N section
		stateAgent                          // inside a ### Agent X: section
		stateCompletion                     // inside ### Agent X - Completion Report
	)

	state := stateTop
	var currentWave *types.Wave
	var currentAgent *types.AgentSpec
	var agentPromptLines []string

	// completionBlocks accumulates raw YAML lines for a completion report.
	// Not used in ParseIMPLDoc itself but we skip those sections cleanly.
	inYAMLBlock := false

	// Typed-block override flags: when a typed block provides a section,
	// the heading-based fallback must not overwrite it.
	hasTypedFileOwnership := false
	hasTypedDepGraph := false

	scanner := bufio.NewScanner(f)
	lineNum := 0

	flushAgent := func() {
		if currentAgent != nil && currentWave != nil {
			currentAgent.Prompt = strings.TrimSpace(strings.Join(agentPromptLines, "\n"))
			currentWave.Agents = append(currentWave.Agents, *currentAgent)
			currentAgent = nil
			agentPromptLines = nil
		}
	}

	flushWave := func() {
		flushAgent()
		if currentWave != nil {
			doc.Waves = append(doc.Waves, *currentWave)
			currentWave = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Track YAML fences so we don't misinterpret their content as headers.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			// Check for typed block: ```yaml type=impl-* or ```type=impl-*
			if strings.Contains(trimmed, "type=impl-") {
				// Extract the blockType: substring after "type=" to end-of-line or first space.
				typeIdx := strings.Index(trimmed, "type=")
				blockType := trimmed[typeIdx+len("type="):]
				if spIdx := strings.Index(blockType, " "); spIdx >= 0 {
					blockType = blockType[:spIdx]
				}
				// Read block content until closing ```.
				blockLines := readUntilClosingFence(scanner)
				lineNum += len(blockLines) + 1 // +1 for the closing fence line

				// Dispatch to the appropriate typed-block section parser.
				switch blockType {
				case "impl-file-ownership":
					// Parse table rows from block content into doc.FileOwnership.
					for _, bl := range blockLines {
						if strings.HasPrefix(bl, "|") {
							parseFileOwnershipRow(bl, doc.FileOwnership, &doc.FileOwnershipCol4)
						}
					}
					hasTypedFileOwnership = true
				case "impl-dep-graph":
					doc.DependencyGraphText = strings.Join(blockLines, "\n")
					hasTypedDepGraph = true
				case "impl-wave-structure":
					// Store raw content (ignored by parser beyond storage).
					// Currently no field in IMPLDoc for this; skip silently.
				case "impl-completion-report":
					// Skip: completion reports are parsed by ParseCompletionReport.
				}
				// Do NOT set inYAMLBlock — the helper already consumed the block.
				continue
			}

			inYAMLBlock = !inYAMLBlock
			if state == stateAgent {
				agentPromptLines = append(agentPromptLines, line)
			}
			continue
		}
		if inYAMLBlock {
			if state == stateAgent {
				agentPromptLines = append(agentPromptLines, line)
			}
			continue
		}

		switch {
		// ── SAW:COMPLETE tag: <!-- SAW:COMPLETE 2026-03-07 -->
		case sawCompleteRe.MatchString(trimmed):
			if m := sawCompleteRe.FindStringSubmatch(trimmed); len(m) == 2 {
				doc.DocStatus = "COMPLETE"
				doc.CompletedAt = m[1]
			}

		// ── Top-level title: # IMPL: {name}
		case strings.HasPrefix(line, "# IMPL:"):
			doc.FeatureName = strings.TrimSpace(strings.TrimPrefix(line, "# IMPL:"))
			state = stateTop

		// ── Suitability verdict: Verdict: SUITABLE / NOT SUITABLE
		// Also handles **Verdict: ...** (bold markdown from scout output).
		case state == stateTop && strings.Contains(trimmed, "Verdict:"):
			stripped := strings.ReplaceAll(trimmed, "**", "")
			val := strings.TrimSpace(strings.SplitN(stripped, "Verdict:", 2)[1])
			doc.Status = val

		// ── Metadata: **Test Command:** go test ./...  (or without bold)
		case state == stateTop && (strings.HasPrefix(trimmed, "**Test Command:**") ||
			strings.HasPrefix(trimmed, "Test Command:")):
			val := trimmed
			if idx := strings.Index(val, ":"); idx >= 0 {
				val = strings.TrimSpace(val[idx+1:])
			}
			val = strings.Trim(val, "`")
			doc.TestCommand = val

		// ── Metadata: **Lint Command:** go vet ./...  (or without bold, or lint_command:)
		case state == stateTop && (strings.HasPrefix(trimmed, "**Lint Command:**") ||
			strings.HasPrefix(trimmed, "Lint Command:") ||
			strings.HasPrefix(trimmed, "lint_command:")):
			val := trimmed
			if idx := strings.Index(val, ":"); idx >= 0 {
				val = strings.TrimSpace(val[idx+1:])
			}
			val = strings.TrimLeft(val, "* ")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, "`")
			doc.LintCommand = val

		// ── Wave section: ## Wave N
		// Guard: "## Wave N / Agent X Completion Report" appears in some IMPL
		// doc formats — not a wave boundary, skip it as a completion report.
		case strings.HasPrefix(line, "## Wave ") &&
			!strings.Contains(line, "Completion Report") &&
			!strings.Contains(line, "/ Agent"):
			flushWave()
			rest := strings.TrimPrefix(line, "## Wave ")
			rest = strings.Fields(rest)[0] // take first token (the number)
			var n int
			if _, err2 := fmt.Sscanf(rest, "%d", &n); err2 != nil {
				// best-effort: skip malformed wave header
				state = stateTop
				continue
			}
			currentWave = &types.Wave{Number: n}
			state = stateWave

		// ── Pre-Mortem section: ## Pre-Mortem or ### Pre-Mortem
		case trimmed == "## Pre-Mortem" || trimmed == "### Pre-Mortem":
			flushAgent()
			doc.PreMortem = parsePreMortemSection(scanner)
			state = stateTop

		// ── Other ## headers (non-Wave): leave wave context
		case strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Wave "):
			flushWave()
			state = stateTop

		// ── Agent completion report: ### Agent X - Completion Report
		case isCompletionReportHeader(line):
			flushAgent()
			state = stateCompletion

		// ── File ownership table header: ### File Ownership
		case trimmed == "### File Ownership":
			flushAgent()
			if hasTypedFileOwnership {
				state = stateTop // typed block already parsed; skip heading-based fallback
			} else {
				state = stateFileOwner
			}

		// ── Known Issues section: ### Known Issues
		case trimmed == "### Known Issues":
			flushAgent()
			doc.KnownIssues = parseKnownIssuesSection(scanner)
			state = stateTop

		// ── Scaffolds section: ### Scaffolds
		case trimmed == "### Scaffolds":
			flushAgent()
			doc.ScaffoldsDetail = parseScaffoldsDetailSection(scanner)
			// DEBUG: fmt.Printf("DEBUG: Parsed %d scaffolds\n", len(doc.ScaffoldsDetail))
			state = stateTop

		// ── Interface Contracts section: ### Interface Contracts
		case trimmed == "### Interface Contracts":
			flushAgent()
			doc.InterfaceContractsText = parseInterfaceContractsSection(scanner)
			state = stateTop

		// ── Dependency Graph section: ### Dependency Graph
		case trimmed == "### Dependency Graph":
			flushAgent()
			if hasTypedDepGraph {
				state = stateTop // typed block already parsed; skip heading-based fallback
			} else {
				doc.DependencyGraphText = parseDependencyGraphSection(scanner)
				state = stateTop
			}

		// ── Post-Merge Checklist section: ### Orchestrator Post-Merge Checklist
		case strings.HasPrefix(trimmed, "### Orchestrator Post-Merge Checklist"):
			flushAgent()
			doc.PostMergeChecklistText = parsePostMergeChecklistSection(scanner)
			state = stateTop

		// ── Stub Report section: ## Stub Report
		case trimmed == "## Stub Report" || trimmed == "### Stub Report":
			flushAgent()
			doc.StubReportText = parseInterfaceContractsSection(scanner)
			state = stateTop

		// ── Quality Gates section: ## Quality Gates
		case trimmed == "## Quality Gates" || trimmed == "### Quality Gates":
			flushAgent()
			doc.QualityGates = parseQualityGatesSection(scanner)
			state = stateTop

		// ── Agent subsection: ### Agent X: Description  or  #### Agent X — Description
		//    Accepted in any state (wave, agent, or top-level). If no ## Wave N
		//    section is active, auto-create wave 1.
		//    Completion-report headers are handled above and are excluded here.
		case isAgentHeader(line) && !isCompletionReportHeader(line):
			flushAgent()
			if currentWave == nil {
				currentWave = &types.Wave{Number: 1}
			}
			letter := extractAgentLetter(line)
			currentAgent = &types.AgentSpec{Letter: letter}
			agentPromptLines = nil
			state = stateAgent

		// ── Any other ### header inside an agent prompt — accumulate as prompt text
		case strings.HasPrefix(line, "### ") && state == stateAgent:
			agentPromptLines = append(agentPromptLines, line)

		// ── File ownership table rows
		case state == stateFileOwner && strings.HasPrefix(line, "|"):
			parseFileOwnershipRow(line, doc.FileOwnership, &doc.FileOwnershipCol4)

		// ── Accumulate agent prompt text; extract **wave:** N metadata
		case state == stateAgent:
			if strings.HasPrefix(trimmed, "**wave:**") {
				waveVal := strings.TrimSpace(strings.TrimPrefix(trimmed, "**wave:**"))
				var n int
				if _, err2 := fmt.Sscanf(waveVal, "%d", &n); err2 == nil && currentWave != nil {
					currentWave.Number = n
				}
			}
			agentPromptLines = append(agentPromptLines, line)
		}
	}

	flushWave()

	if err2 := scanner.Err(); err2 != nil {
		return doc, fmt.Errorf("ParseIMPLDoc: scanner error reading %q: %w", path, err2)
	}

	if doc.FeatureName == "" {
		// Tolerate missing title: derive from filename (IMPL-<slug>.md → slug).
		base := filepath.Base(path)
		base = strings.TrimPrefix(base, "IMPL-")
		base = strings.TrimSuffix(base, ".md")
		doc.FeatureName = base
	}

	// If no agent headers were parsed, reconstruct waves from the file ownership
	// table directly. Also handles the single-wave case where maxWave > 1.
	if len(doc.Waves) <= 1 && len(doc.FileOwnership) > 0 {
		// Build agent -> wave number from ownership table.
		agentWave := make(map[string]int)
		agentSeen := make(map[string]bool)
		maxWave := 1
		for _, info := range doc.FileOwnership {
			if info.Wave > 0 {
				if prev, ok := agentWave[info.Agent]; !ok || info.Wave < prev {
					agentWave[info.Agent] = info.Wave
				}
				if info.Wave > maxWave {
					maxWave = info.Wave
				}
			}
			agentSeen[info.Agent] = true
		}

		// When 0 waves: build entirely from ownership table agents.
		// When 1 wave: only regroup if maxWave > 1 (existing behavior).
		if len(doc.Waves) == 0 || maxWave > 1 {
			var origAgents []types.AgentSpec
			if len(doc.Waves) == 1 {
				origAgents = doc.Waves[0].Agents
			} else {
				// Reconstruct agent list from ownership table in letter order.
				seen := make(map[string]bool)
				for _, info := range doc.FileOwnership {
					if !seen[info.Agent] && info.Agent != "" {
						origAgents = append(origAgents, types.AgentSpec{Letter: info.Agent})
						seen[info.Agent] = true
					}
				}
			}
			waveMap := make(map[int][]types.AgentSpec)
			for _, a := range origAgents {
				wn := agentWave[a.Letter]
				if wn == 0 {
					wn = 1
				}
				waveMap[wn] = append(waveMap[wn], a)
			}
			doc.Waves = nil
			for wn := 1; wn <= maxWave; wn++ {
				if agents, ok := waveMap[wn]; ok {
					doc.Waves = append(doc.Waves, types.Wave{Number: wn, Agents: agents})
				}
			}
		}
	}

	// Populate FilesOwned for each agent from the authoritative FileOwnership table.
	for i := range doc.Waves {
		for j := range doc.Waves[i].Agents {
			agent := &doc.Waves[i].Agents[j]
			agent.FilesOwned = nil
			for file, info := range doc.FileOwnership {
				if info.Agent == agent.Letter {
					agent.FilesOwned = append(agent.FilesOwned, file)
				}
			}
		}
	}

	return doc, nil
}

// ValidateInvariants checks Invariant I1: no file appears in two different
// agents' ownership lists within the same wave. Returns a descriptive error
// for the first violation found, or nil if the document is clean.
func ValidateInvariants(doc *types.IMPLDoc) error {
	if doc == nil {
		return nil
	}
	for _, wave := range doc.Waves {
		seen := make(map[string]string) // file -> agent letter
		for _, agent := range wave.Agents {
			for _, file := range agent.FilesOwned {
				if prev, ok := seen[file]; ok {
					return fmt.Errorf(
						"I1 violation in Wave %d: file %q claimed by both Agent %s and Agent %s",
						wave.Number, file, prev, agent.Letter,
					)
				}
				seen[file] = agent.Letter
			}
		}
	}
	return nil
}

// ParseCompletionReport extracts the named agent's completion report from the
// IMPL doc at path. agentLetter is a single uppercase letter ("A", "B", etc.).
// Returns ErrReportNotFound if the section does not exist in the file.
func ParseCompletionReport(path string, agentLetter string) (*types.CompletionReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ParseCompletionReport: cannot open %q: %w", path, err)
	}
	defer f.Close()

	headerTarget := fmt.Sprintf("### Agent %s - Completion Report", agentLetter)

	type scanState int
	const (
		scanLooking  scanState = iota // searching for the section header
		scanInSection                 // found header, looking for yaml fence
		scanInYAML                    // inside ```yaml ... ```
	)

	state := scanLooking
	var yamlLines []string
	inYAML := false
	_ = inYAML

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		switch state {
		case scanLooking:
			if strings.TrimSpace(line) == headerTarget {
				state = scanInSection
			}

		case scanInSection:
			if strings.HasPrefix(trimmed, "```yaml") {
				state = scanInYAML
			} else if strings.HasPrefix(trimmed, "```") {
				// plain fence — treat as yaml start anyway (spec says ```yaml)
				state = scanInYAML
			} else if strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") {
				// Next section header — no yaml block found
				break
			}

		case scanInYAML:
			if strings.HasPrefix(trimmed, "```") {
				// Closing fence — we have all the YAML
				goto done
			}
			yamlLines = append(yamlLines, line)
		}
	}

done:
	if err2 := scanner.Err(); err2 != nil {
		return nil, fmt.Errorf("ParseCompletionReport: scanner error reading %q: %w", path, err2)
	}

	if state == scanLooking {
		return nil, ErrReportNotFound
	}
	if len(yamlLines) == 0 {
		return nil, fmt.Errorf("ParseCompletionReport: agent %s section found in %q but YAML block is empty", agentLetter, path)
	}

	// Unmarshal the YAML into a raw map first so we can handle snake_case keys.
	raw := struct {
		Status              string   `yaml:"status"`
		Worktree            string   `yaml:"worktree"`
		Branch              string   `yaml:"branch"`
		Commit              string   `yaml:"commit"`
		FilesChanged        []string `yaml:"files_changed"`
		FilesCreated        []string `yaml:"files_created"`
		InterfaceDeviations []struct {
			Description              string   `yaml:"description"`
			DownstreamActionRequired bool     `yaml:"downstream_action_required"`
			Affects                  []string `yaml:"affects"`
		} `yaml:"interface_deviations"`
		OutOfScopeDeps []string `yaml:"out_of_scope_deps"`
		TestsAdded     []string `yaml:"tests_added"`
		Verification   string   `yaml:"verification"`
	}{}

	yamlStr := strings.Join(yamlLines, "\n")
	if err3 := yaml.Unmarshal([]byte(yamlStr), &raw); err3 != nil {
		return nil, fmt.Errorf("ParseCompletionReport: agent %s YAML in %q: %w", agentLetter, path, err3)
	}

	report := &types.CompletionReport{
		Status:         types.CompletionStatus(raw.Status),
		Worktree:       raw.Worktree,
		Branch:         raw.Branch,
		Commit:         raw.Commit,
		FilesChanged:   raw.FilesChanged,
		FilesCreated:   raw.FilesCreated,
		OutOfScopeDeps: raw.OutOfScopeDeps,
		TestsAdded:     raw.TestsAdded,
		Verification:   raw.Verification,
	}
	for _, d := range raw.InterfaceDeviations {
		report.InterfaceDeviations = append(report.InterfaceDeviations, types.InterfaceDeviation{
			Description:              d.Description,
			DownstreamActionRequired: d.DownstreamActionRequired,
			Affects:                  d.Affects,
		})
	}

	return report, nil
}

// parseKnownIssuesSection extracts "### Known Issues" content as []types.KnownIssue.
// Format: Free-form text (bullets, paragraphs, or "None identified"). Parse heuristically.
func parseKnownIssuesSection(scanner *bufio.Scanner) []types.KnownIssue {
	var issues []types.KnownIssue
	var currentIssue strings.Builder
	inYAMLBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track code fences
		if strings.HasPrefix(trimmed, "```") {
			inYAMLBlock = !inYAMLBlock
		}

		// Stop at next header
		if !inYAMLBlock && (strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ")) {
			break
		}

		// Stop at horizontal rule between sections
		if !inYAMLBlock && trimmed == "---" {
			break
		}

		if inYAMLBlock || trimmed == "" {
			continue
		}

		// Accumulate text
		currentIssue.WriteString(line)
		currentIssue.WriteString("\n")
	}

	// Simple heuristic: if text contains "None identified" or "None", return empty list
	text := currentIssue.String()
	if strings.Contains(strings.ToLower(text), "none identified") ||
		(strings.Contains(strings.ToLower(text), "none") && len(text) < 100) {
		return issues
	}

	// Otherwise, treat entire section as a single issue description
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		issues = append(issues, types.KnownIssue{
			Description: trimmed,
			Status:      "",
			Workaround:  "",
		})
	}

	return issues
}

// parseScaffoldsDetailSection extracts "### Scaffolds" table as []types.ScaffoldFile.
// Format: markdown table with columns: File | Contents | Import path | Status
func parseScaffoldsDetailSection(scanner *bufio.Scanner) []types.ScaffoldFile {
	var scaffolds []types.ScaffoldFile
	inTable := false
	tableLinesSeen := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Stop at next header
		if strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") {
			break
		}

		// Stop at horizontal rule only if we've seen at least 2 table lines (header + separator)
		// and we're not currently in a table row
		if strings.HasPrefix(trimmed, "---") && !strings.Contains(line, "|") && tableLinesSeen >= 2 {
			break
		}

		if !strings.HasPrefix(line, "|") {
			// If we were in a table and hit non-table content after seeing data rows, we're done
			if inTable && tableLinesSeen > 2 {
				break
			}
			continue
		}

		tableLinesSeen++

		// Parse table rows
		parts := strings.Split(line, "|")
		// A 4-column table like | A | B | C | D | has 6 parts when split by |
		// (empty string before first | and after last |)
		if len(parts) < 4 {
			continue
		}

		file := strings.TrimSpace(parts[1])
		contents := ""
		importPath := ""
		if len(parts) > 2 {
			contents = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			importPath = strings.TrimSpace(parts[3])
		}

		// Skip header and separator rows
		if strings.Contains(file, "---") || strings.ToLower(file) == "file" {
			inTable = true
			continue
		}

		if file != "" {
			// Clean backticks from file path, contents, and import path
			file = strings.Trim(file, "`")
			importPath = strings.Trim(importPath, "`")
			scaffolds = append(scaffolds, types.ScaffoldFile{
				FilePath:   file,
				Contents:   contents,
				ImportPath: importPath,
			})
		}
	}

	return scaffolds
}

// parseInterfaceContractsSection extracts "### Interface Contracts" as raw markdown.
// Captures everything until next ### or ## header.
func parseInterfaceContractsSection(scanner *bufio.Scanner) string {
	var buf strings.Builder
	inYAMLBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track code fences to avoid stopping inside them
		if strings.HasPrefix(trimmed, "```") {
			inYAMLBlock = !inYAMLBlock
			buf.WriteString(line)
			buf.WriteString("\n")
			continue
		}

		// Stop at next section header (but not if inside code block)
		if !inYAMLBlock && (strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ")) {
			break
		}

		// Stop at horizontal rule between sections
		if !inYAMLBlock && trimmed == "---" {
			break
		}

		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return strings.TrimSpace(buf.String())
}

// parseDependencyGraphSection extracts "### Dependency Graph" as raw markdown.
func parseDependencyGraphSection(scanner *bufio.Scanner) string {
	var buf strings.Builder
	inYAMLBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track code fences
		if strings.HasPrefix(trimmed, "```") {
			inYAMLBlock = !inYAMLBlock
			buf.WriteString(line)
			buf.WriteString("\n")
			continue
		}

		// Stop at next section header
		if !inYAMLBlock && (strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ")) {
			break
		}

		// Stop at horizontal rule between sections
		if !inYAMLBlock && trimmed == "---" {
			break
		}

		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return strings.TrimSpace(buf.String())
}

// parsePostMergeChecklistSection extracts "### Orchestrator Post-Merge Checklist" as raw markdown.
func parsePostMergeChecklistSection(scanner *bufio.Scanner) string {
	var buf strings.Builder
	inYAMLBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track code fences
		if strings.HasPrefix(trimmed, "```") {
			inYAMLBlock = !inYAMLBlock
			buf.WriteString(line)
			buf.WriteString("\n")
			continue
		}

		// Stop at next section header
		if !inYAMLBlock && (strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ")) {
			break
		}

		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return strings.TrimSpace(buf.String())
}

// ── helpers ──────────────────────────────────────────────────────────────────

// isAgentHeader returns true for lines like "### Agent A: Description"
// or "#### Agent A — Description".
func isAgentHeader(line string) bool {
	rest := ""
	switch {
	case strings.HasPrefix(line, "#### Agent "):
		rest = strings.TrimPrefix(line, "#### Agent ")
	case strings.HasPrefix(line, "### Agent "):
		rest = strings.TrimPrefix(line, "### Agent ")
	default:
		return false
	}
	if len(rest) == 0 {
		return false
	}
	letter := rest[0]
	if letter < 'A' || letter > 'Z' {
		return false
	}
	if len(rest) == 1 {
		return true
	}
	return rest[1] == ':' || rest[1] == ' ' || rest[1] == 0xE2 // '—' starts with 0xE2 in UTF-8
}

// isCompletionReportHeader returns true for lines like
// "### Agent A - Completion Report".
func isCompletionReportHeader(line string) bool {
	if !strings.HasPrefix(line, "### Agent ") {
		return false
	}
	return strings.Contains(line, "- Completion Report")
}

// extractAgentLetter returns the single uppercase letter from a header like
// "### Agent A: ...", "#### Agent A — ...", or "### Agent A - Completion Report".
func extractAgentLetter(line string) string {
	var rest string
	switch {
	case strings.HasPrefix(line, "#### Agent "):
		rest = strings.TrimPrefix(line, "#### Agent ")
	case strings.HasPrefix(line, "### Agent "):
		rest = strings.TrimPrefix(line, "### Agent ")
	}
	if len(rest) == 0 {
		return ""
	}
	return string(rest[0])
}

// parseFileOwnershipRow parses a markdown table row like:
//
//	| pkg/protocol/parser.go | A | 1 | — |
//
// and populates the ownership map (file -> FileOwnershipInfo).
// col4Header is detected from the header row and stored for the caller.
func parseFileOwnershipRow(line string, ownership map[string]types.FileOwnershipInfo, col4Header *string) {
	// Split on "|" and trim whitespace from each cell.
	parts := strings.Split(line, "|")
	// A valid data row has at least 4 cells (leading empty + 3+ columns).
	if len(parts) < 4 {
		return
	}
	file := strings.TrimSpace(parts[1])
	agent := strings.TrimSpace(parts[2])

	// Skip separator rows.
	if strings.Contains(file, "---") || strings.Contains(agent, "---") {
		return
	}

	// Detect header row and extract 4th column name.
	fileLower := strings.ToLower(file)
	if fileLower == "file" || fileLower == "" {
		if len(parts) >= 6 && col4Header != nil {
			h := strings.TrimSpace(parts[4])
			if h != "" {
				*col4Header = h
			}
		}
		return
	}
	agentLower := strings.ToLower(agent)
	if agentLower == "agent" || agentLower == "agent-letter" {
		return
	}

	agent = strings.Trim(agent, "` ")

	info := types.FileOwnershipInfo{Agent: agent}

	// Infer action from file path annotations like "(new)" or "(modify)" before
	// stripping backticks, since annotations appear outside the backtick-wrapped path.
	switch {
	case strings.Contains(fileLower, "(new)"):
		info.Action = "new"
		file = strings.Replace(file, "(new)", "", 1)
		file = strings.Replace(file, "(New)", "", 1)
	case strings.Contains(fileLower, "(create)"):
		info.Action = "new"
		file = strings.Replace(file, "(create)", "", 1)
		file = strings.Replace(file, "(Create)", "", 1)
	case strings.Contains(fileLower, "(modify)"):
		info.Action = "modify"
		file = strings.Replace(file, "(modify)", "", 1)
		file = strings.Replace(file, "(Modify)", "", 1)
	}

	// Normalise backtick-wrapped paths: `pkg/protocol/parser.go` -> pkg/protocol/parser.go
	file = strings.Trim(strings.TrimSpace(file), "` ")

	// Parse wave number from 3rd column if present.
	if len(parts) >= 5 {
		waveStr := strings.TrimSpace(parts[3])
		var n int
		if _, err := fmt.Sscanf(waveStr, "%d", &n); err == nil {
			info.Wave = n
		}
	}

	// Parse 4th column based on detected header.
	if len(parts) >= 6 {
		val := strings.TrimSpace(parts[4])
		val = strings.Trim(val, "`")

		col4Name := ""
		if col4Header != nil {
			col4Name = strings.ToLower(*col4Header)
		}

		if val != "" && val != "—" && val != "-" {
			switch {
			case strings.Contains(col4Name, "depends"):
				info.DependsOn = val
			case strings.Contains(col4Name, "action"):
				info.Action = classifyAction(val)
			default:
				// No header detected or unknown — try to classify as action,
				// fall back to DependsOn.
				classified := classifyAction(val)
				if classified != val {
					info.Action = classified
				} else {
					info.DependsOn = val
				}
			}
		}
	}

	ownership[file] = info
}

// readUntilClosingFence reads scanner lines until a closing ``` fence is found
// (or EOF). It returns the content lines (not including the closing fence line).
// The caller must account for the consumed lines when tracking line numbers.
func readUntilClosingFence(scanner *bufio.Scanner) []string {
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			// Closing fence found — stop and do not include it in content.
			break
		}
		lines = append(lines, line)
	}
	return lines
}

// parsePreMortemSection parses a ## Pre-Mortem or ### Pre-Mortem section.
// It scans until the next ## or ### header or EOF. It looks for:
//   - **Overall risk:** <level>  (case-insensitive)
//   - A markdown table with 4 columns: Scenario | Likelihood | Impact | Mitigation
//
// Returns nil if no table rows are found.
func parsePreMortemSection(scanner *bufio.Scanner) *types.PreMortem {
	pm := &types.PreMortem{}
	inTable := false
	headerSeen := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Stop at next section header.
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			break
		}

		// Detect **Overall risk:** line (case-insensitive).
		lowerTrimmed := strings.ToLower(trimmed)
		if strings.Contains(lowerTrimmed, "overall risk:") {
			idx := strings.Index(lowerTrimmed, "overall risk:")
			val := strings.TrimSpace(trimmed[idx+len("overall risk:"):])
			// Strip trailing ** if present (bold markup).
			val = strings.Trim(val, "* ")
			val = strings.ToLower(val)
			pm.OverallRisk = val
			continue
		}

		// Parse markdown table rows.
		if !strings.HasPrefix(line, "|") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		col1 := strings.TrimSpace(parts[1])

		// Skip header row (contains "Scenario") and separator row (contains "---").
		if strings.Contains(col1, "---") {
			continue
		}
		if !headerSeen && strings.EqualFold(col1, "scenario") {
			headerSeen = true
			inTable = true
			continue
		}
		if !inTable {
			// Treat first | row as header if we haven't seen one yet.
			inTable = true
			if strings.EqualFold(col1, "scenario") {
				headerSeen = true
				continue
			}
		}

		// Data row.
		scenario := col1
		likelihood := ""
		impact := ""
		mitigation := ""
		if len(parts) > 2 {
			likelihood = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			impact = strings.TrimSpace(parts[3])
		}
		if len(parts) > 4 {
			mitigation = strings.TrimSpace(parts[4])
		}

		if scenario == "" {
			continue
		}

		pm.Rows = append(pm.Rows, types.PreMortemRow{
			Scenario:   scenario,
			Likelihood: likelihood,
			Impact:     impact,
			Mitigation: mitigation,
		})
	}

	if len(pm.Rows) == 0 && pm.OverallRisk == "" {
		return nil
	}
	return pm
}

// ParseQualityGates extracts the ## Quality Gates section from an IMPL doc at
// path. Returns nil, nil if the section is absent (quality gates are optional).
// The section is parsed by hand (no external YAML library) consistent with the
// rest of the parser.
func ParseQualityGates(implDocPath string) (*types.QualityGates, error) {
	f, err := os.Open(implDocPath)
	if err != nil {
		return nil, fmt.Errorf("ParseQualityGates: cannot open %q: %w", implDocPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// Scan for the ## Quality Gates heading.
	found := false
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "## Quality Gates" || trimmed == "### Quality Gates" {
			found = true
			break
		}
	}
	if err2 := scanner.Err(); err2 != nil {
		return nil, fmt.Errorf("ParseQualityGates: scanner error reading %q: %w", implDocPath, err2)
	}
	if !found {
		return nil, nil
	}

	return parseQualityGatesSection(scanner), nil
}

// parseQualityGatesSection parses a Quality Gates section from an already-
// positioned scanner (the heading line has been consumed). It reads until the
// next ## or ### heading, or EOF.
//
// Expected format:
//
//	level: standard
//	gates:
//	  - type: build
//	    command: go build ./...
//	    required: true
//	    description: must compile
func parseQualityGatesSection(scanner *bufio.Scanner) *types.QualityGates {
	qg := &types.QualityGates{}
	inGatesList := false
	var current *types.QualityGate

	flushGate := func() {
		if current != nil {
			qg.Gates = append(qg.Gates, *current)
			current = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Stop at the next ## or ### heading.
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			break
		}

		// Skip blank lines and code fences.
		if trimmed == "" || strings.HasPrefix(trimmed, "```") {
			continue
		}

		// level: <value>
		if strings.HasPrefix(trimmed, "level:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "level:"))
			qg.Level = val
			continue
		}

		// gates: (list marker — just toggle mode)
		if trimmed == "gates:" {
			inGatesList = true
			continue
		}

		if !inGatesList {
			continue
		}

		// New gate entry: "  - type: build"
		if strings.HasPrefix(trimmed, "- type:") {
			flushGate()
			current = &types.QualityGate{}
			current.Type = strings.TrimSpace(strings.TrimPrefix(trimmed, "- type:"))
			continue
		}

		// Properties of the current gate (may be indented with spaces).
		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "type:"):
			current.Type = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
		case strings.HasPrefix(trimmed, "command:"):
			current.Command = strings.TrimSpace(strings.TrimPrefix(trimmed, "command:"))
		case strings.HasPrefix(trimmed, "required:"):
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "required:"))
			current.Required = strings.EqualFold(val, "true")
		case strings.HasPrefix(trimmed, "description:"):
			current.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		}
	}

	flushGate()

	if qg.Level == "" && len(qg.Gates) == 0 {
		return nil
	}
	return qg
}

// classifyAction normalizes an action string to "new", "modify", or "delete".
// Returns the original string if it doesn't match any known action.
func classifyAction(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "new") || strings.HasPrefix(lower, "create"):
		return "new"
	case strings.HasPrefix(lower, "mod") || strings.HasPrefix(lower, "edit"):
		return "modify"
	case strings.HasPrefix(lower, "del") || strings.HasPrefix(lower, "remove"):
		return "delete"
	default:
		return s
	}
}
