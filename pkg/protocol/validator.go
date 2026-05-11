package protocol

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// typedBlockRe matches the opening fence of a typed block, e.g.:
//
//	```yaml type=impl-file-ownership
var typedBlockRe = regexp.MustCompile("^```yaml\\s+type=(impl-[a-z-]+)")

// plainFenceRe matches a plain opening fence with no type= annotation.
var plainFenceRe = regexp.MustCompile("^```[a-zA-Z]*$")

// e16cAgentRe and e16cWaveRe are used together to detect likely dep-graph content.
var e16cAgentRe = regexp.MustCompile(`\[[A-Z]\]`)
var e16cWaveRe = regexp.MustCompile(`Wave`)

// e16aRequiredBlocks lists the typed block types that must appear in every IMPL doc
// that uses typed blocks (i.e. block_count > 0).
var e16aRequiredBlocks = []string{"impl-file-ownership", "impl-dep-graph", "impl-wave-structure"}

// agentLineRe matches agent lines like "    [A] some/file" or "    [A2] some/file"
// (leading whitespace + [LETTER] or [LETTER+DIGIT]).
var agentLineRe = regexp.MustCompile(`^\s+\[([A-Z][2-9]?)\]`)

// waveHeaderRe matches "Wave N" at the start of a line.
var waveHeaderRe = regexp.MustCompile(`^Wave [0-9]+`)

// agentRefRe matches any [A-Z] or [A2]-style reference.
var agentRefRe = regexp.MustCompile(`\[[A-Z][2-9]?\]`)

// waveStructureRe matches "Wave N:" at the start of a line.
var waveStructureRe = regexp.MustCompile(`^Wave [0-9]+:`)

// rootOrDependsRe matches either "✓ root" or "depends on:" within agent block lines.
var rootOrDependsRe = regexp.MustCompile(`✓ root|depends on:`)

// I2: validAgentIDRe matches valid agent IDs following ^[A-Z][2-9]?$ pattern.
// This is the canonical pattern enforced by M1 (assign-agent-ids).
var validAgentIDRe = regexp.MustCompile(`^[A-Z][2-9]?$`)

// I2: invalidAgentIDRe detects common invalid patterns:
// - [A1] (generation 1 must use bare letter)
// - [A0], [A10], [AB], [a], [1A] (malformed)
var invalidAgentIDRe = regexp.MustCompile(`\[([A-Z]1|[A-Z]0|[A-Z][0-9]{2,}|[A-Z]{2,}|[a-z]|[0-9][A-Z])\]`)

// ValidateIMPLDoc runs E16 typed-block validation on the IMPL doc at path.
// It reads the file directly (not via ParseIMPLDoc) to preserve line numbers.
//
// Checks performed:
//   - E16A: required block presence (impl-file-ownership, impl-dep-graph, impl-wave-structure)
//   - E16B: structural content within each typed block
//   - E16C: out-of-band dep graph detection in plain fenced blocks (warn only)
//   - I2: agent ID format validation across all blocks
//
// Called by: FullValidate (after Validate). Does not call Validate itself.
// Do not call directly unless you specifically need E16 block validation without
// the full invariant suite.
func ValidateIMPLDoc(path string) ([]result.PolywaveError, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ValidateIMPLDoc: cannot open %q: %w", path, err)
	}
	defer f.Close()

	// First pass: scan all lines into memory so we can extract block contents
	// with correct line numbers.
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err2 := scanner.Err(); err2 != nil {
		return nil, fmt.Errorf("ValidateIMPLDoc: scanner error reading %q: %w", path, err2)
	}

	var errs []result.PolywaveError
	seenBlocks := map[string]bool{}
	blockCount := 0

	// I2: Track unique agent IDs across all blocks for count-based suggestion
	allAgentIDs := make(map[string]bool)

	for i, line := range lines {
		m := typedBlockRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		blockType := m[1]
		lineNumber := i + 1 // 1-based

		// Extract block content: lines after the opening fence until closing ```.
		var blockLines []string
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimRight(lines[j], " \t") == "```" {
				break
			}
			blockLines = append(blockLines, lines[j])
		}

		seenBlocks[blockType] = true
		blockCount++

		var blockErrs []result.PolywaveError
		var blockAgentIDs []string
		switch blockType {
		case "impl-file-ownership":
			blockErrs, blockAgentIDs = validateFileOwnership(blockLines, lineNumber)
		case "impl-dep-graph":
			blockErrs, blockAgentIDs = validateDepGraph(blockLines, lineNumber)
		case "impl-wave-structure":
			blockErrs, blockAgentIDs = validateWaveStructure(blockLines, lineNumber)
		case "impl-completion-report":
			blockErrs, blockAgentIDs = validateCompletionReport(blockLines, lineNumber)
		}
		errs = append(errs, blockErrs...)

		// I2: Collect agent IDs for global count
		for _, id := range blockAgentIDs {
			allAgentIDs[id] = true
		}
	}

	// I2: If any agent ID errors found, append suggestion with count
	hasAgentIDErrors := false
	for _, err := range errs {
		if err.Code == "agent-id" {
			hasAgentIDErrors = true
			break
		}
	}
	if hasAgentIDErrors && len(allAgentIDs) > 0 {
		errs = append(errs, result.PolywaveError{
			Code:     "agent-id",
			Severity: "error",
			Line:     0,
			Message:  fmt.Sprintf("Run: sawtools assign-agent-ids --count %d", len(allAgentIDs)),
		})
	}

	// E16A: Required block presence — only when at least one typed block exists.
	if blockCount > 0 {
		for _, req := range e16aRequiredBlocks {
			if !seenBlocks[req] {
				errs = append(errs, result.PolywaveError{
					Code:     "e16a",
					Severity: "error",
					Line:     0,
					Message:  fmt.Sprintf("missing required block: %s", req),
				})
			}
		}
	}

	// E16C: Out-of-band dep graph detection (warn only).
	// Scan plain fenced blocks for content that looks like a dep graph.
	// Must skip typed blocks so their closing ``` fences are not mistaken for
	// plain fence openings.
	{
		inTypedBlock := false
		inPlainBlock := false
		plainBlockStart := 0
		var plainBlockLines []string
		for i, line := range lines {
			lineNum := i + 1
			closingFence := strings.TrimRight(line, " \t") == "```"

			if inTypedBlock {
				if closingFence {
					inTypedBlock = false
				}
				continue
			}
			if strings.Contains(line, "type=") && strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
				inTypedBlock = true
				continue
			}

			if !inPlainBlock {
				if plainFenceRe.MatchString(line) {
					inPlainBlock = true
					plainBlockStart = lineNum
					plainBlockLines = nil
				}
			} else {
				if closingFence {
					content := strings.Join(plainBlockLines, "\n")
					if e16cAgentRe.MatchString(content) && e16cWaveRe.MatchString(content) {
						errs = append(errs, result.PolywaveError{
							Code:     "warning",
							Severity: "warning",
							Line:     plainBlockStart,
							Message:  fmt.Sprintf("WARNING: possible dep-graph content found outside typed block at line %d — use ```yaml type=impl-dep-graph```", plainBlockStart),
						})
					}
					inPlainBlock = false
					plainBlockLines = nil
				} else {
					plainBlockLines = append(plainBlockLines, line)
				}
			}
		}
	}

	if len(errs) == 0 {
		return nil, nil
	}
	return errs, nil
}

// validateFileOwnership validates an impl-file-ownership block.
// I2: Returns (errors, agentIDs) where agentIDs are all agent IDs found in this block.
func validateFileOwnership(lines []string, lineNumber int) ([]result.PolywaveError, []string) {
	var errs []result.PolywaveError
	var agentIDs []string
	content := strings.Join(lines, "\n")

	// Must have a header row containing "| File "
	if !strings.Contains(content, "| File ") {
		errs = append(errs, result.PolywaveError{
			Code:     "impl-file-ownership",
			Severity: "error",
			Line:     lineNumber,
			Message:  fmt.Sprintf("impl-file-ownership block (line %d): missing header row — expected '| File | Agent | Wave | Depends On |'", lineNumber),
		})
		return errs, agentIDs
	}

	// Must have at least one data row (not header, not separator |---|)
	dataRows := 0
	for _, row := range lines {
		if !strings.HasPrefix(row, "|") {
			continue
		}
		if strings.Contains(row, "File") {
			continue
		}
		if strings.Contains(row, "---") {
			continue
		}
		if strings.TrimSpace(row) == "" {
			continue
		}
		dataRows++
	}
	if dataRows == 0 {
		errs = append(errs, result.PolywaveError{
			Code:     "impl-file-ownership",
			Severity: "error",
			Line:     lineNumber,
			Message:  fmt.Sprintf("impl-file-ownership block (line %d): no data rows found — table must have at least one file entry", lineNumber),
		})
	}

	// Each data row must have at least 4 pipe characters
	for _, row := range lines {
		if strings.Contains(row, "File") {
			continue
		}
		if strings.Contains(row, "----") {
			continue
		}
		if strings.TrimSpace(row) == "" {
			continue
		}
		pipeCount := strings.Count(row, "|")
		if pipeCount < 4 {
			errs = append(errs, result.PolywaveError{
				Code:     "impl-file-ownership",
				Severity: "error",
				Line:     lineNumber,
				Message:  fmt.Sprintf("impl-file-ownership block (line %d): row has fewer than 4 columns: %s", lineNumber, row),
			})
		}

		// I2: Extract and validate agent IDs from the Agent column (column 2)
		cells := strings.Split(row, "|")
		if len(cells) >= 3 {
			agentCell := strings.TrimSpace(cells[2])
			if agentCell != "" && agentCell != "Agent" && agentCell != "---" {
				agentIDs = append(agentIDs, agentCell)
				if !validAgentIDRe.MatchString(agentCell) {
					errs = append(errs, result.PolywaveError{
						Code:     "agent-id",
						Severity: "error",
						Line:     lineNumber,
						Message:  fmt.Sprintf("impl-file-ownership block (line %d): invalid agent ID '%s' — must match ^[A-Z][2-9]?$ (bare letter for generation 1, e.g., A, B, C; multi-gen like A2, B3 for generation 2+)", lineNumber, agentCell),
					})
				}
			}
		}
	}

	return errs, agentIDs
}

// validateDepGraph validates an impl-dep-graph block.
// I2: Returns (errors, agentIDs) where agentIDs are all agent IDs found in this block.
func validateDepGraph(lines []string, lineNumber int) ([]result.PolywaveError, []string) {
	var errs []result.PolywaveError
	var agentIDs []string
	content := strings.Join(lines, "\n")

	// Must have at least one line matching "^Wave [0-9]+"
	if !waveHeaderRe.MatchString(content) {
		errs = append(errs, result.PolywaveError{
			Code:     "impl-dep-graph",
			Severity: "error",
			Line:     lineNumber,
			Message:  fmt.Sprintf("impl-dep-graph block (line %d): missing 'Wave N (...):' header — each wave must start with 'Wave N'", lineNumber),
		})
		return errs, agentIDs
	}

	// Must have at least one line matching "[A-Z]"
	if !agentRefRe.MatchString(content) {
		errs = append(errs, result.PolywaveError{
			Code:     "impl-dep-graph",
			Severity: "error",
			Line:     lineNumber,
			Message:  fmt.Sprintf("impl-dep-graph block (line %d): no agent lines found — expected lines like '    [A] path/to/file'", lineNumber),
		})
		return errs, agentIDs
	}

	// Each agent block must contain either "✓ root" or "depends on:"
	// Collect agent lines; for each agent accumulate its block until the next agent line.
	type agentBlock struct {
		letter string
		lines  []string
	}
	var blocks []agentBlock
	var current *agentBlock

	for _, ln := range lines {
		if m := agentLineRe.FindStringSubmatch(ln); m != nil {
			// Flush previous agent
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &agentBlock{letter: m[1], lines: []string{ln}}
			// I2: Collect agent ID
			agentIDs = append(agentIDs, m[1])
		} else if current != nil {
			current.lines = append(current.lines, ln)
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}

	for _, block := range blocks {
		blockContent := strings.Join(block.lines, "\n")
		if !rootOrDependsRe.MatchString(blockContent) {
			errs = append(errs, result.PolywaveError{
				Code:     "impl-dep-graph",
				Severity: "error",
				Line:     lineNumber,
				Message:  fmt.Sprintf("impl-dep-graph block (line %d): agent [%s] has neither '✓ root' nor 'depends on:' — one is required", lineNumber, block.letter),
			})
		}

		// I2: Validate agent ID format
		if !validAgentIDRe.MatchString(block.letter) {
			errs = append(errs, result.PolywaveError{
				Code:     "agent-id",
				Severity: "error",
				Line:     lineNumber,
				Message:  fmt.Sprintf("impl-dep-graph block (line %d): invalid agent ID '[%s]' — must match ^[A-Z][2-9]?$ (bare letter for generation 1, e.g., A, B, C; multi-gen like A2, B3 for generation 2+)", lineNumber, block.letter),
			})
		}
	}

	return errs, agentIDs
}

// validateWaveStructure validates an impl-wave-structure block.
// I2: Returns (errors, agentIDs) where agentIDs are all agent IDs found in this block.
func validateWaveStructure(lines []string, lineNumber int) ([]result.PolywaveError, []string) {
	var errs []result.PolywaveError
	var agentIDs []string
	content := strings.Join(lines, "\n")

	// Must have at least one line matching "^Wave [0-9]+:"
	if !waveStructureRe.MatchString(content) {
		errs = append(errs, result.PolywaveError{
			Code:     "impl-wave-structure",
			Severity: "error",
			Line:     lineNumber,
			Message:  fmt.Sprintf("impl-wave-structure block (line %d): missing 'Wave N:' lines — each wave must appear as 'Wave N: [A] [B]'", lineNumber),
		})
		return errs, agentIDs
	}

	// Must reference at least one agent letter [A-Z]
	if !agentRefRe.MatchString(content) {
		errs = append(errs, result.PolywaveError{
			Code:     "impl-wave-structure",
			Severity: "error",
			Line:     lineNumber,
			Message:  fmt.Sprintf("impl-wave-structure block (line %d): no agent letters found — expected [A], [B], etc.", lineNumber),
		})
		return errs, agentIDs
	}

	// I2: Extract all agent IDs from wave structure and validate format
	// Pattern: "Wave N: [A] [B2] [C] ..." or "Wave N: [A], [B2], [C], ..."
	for _, line := range lines {
		matches := agentRefRe.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 0 {
				// Extract agent ID from [X] or [X2] format
				id := strings.Trim(match[0], "[]")
				agentIDs = append(agentIDs, id)

				// Validate format
				if !validAgentIDRe.MatchString(id) {
					errs = append(errs, result.PolywaveError{
						Code:     "agent-id",
						Severity: "error",
						Line:     lineNumber,
						Message:  fmt.Sprintf("impl-wave-structure block (line %d): invalid agent ID '[%s]' — must match ^[A-Z][2-9]?$ (bare letter for generation 1, e.g., A, B, C; multi-gen like A2, B3 for generation 2+)", lineNumber, id),
					})
				}
			}
		}
	}

	return errs, agentIDs
}

// completionReportRequiredFields lists the required field prefixes for impl-completion-report.
var completionReportRequiredFields = []string{
	"status:",
	"worktree:",
	"branch:",
	"commit:",
	"files_changed:",
	"interface_deviations:",
	"verification:",
}

// validateCompletionReport validates an impl-completion-report block.
// I2: Returns (errors, agentIDs) where agentIDs is empty (completion reports don't define new agent IDs).
func validateCompletionReport(lines []string, lineNumber int) ([]result.PolywaveError, []string) {
	var errs []result.PolywaveError

	// Check required fields
	for _, field := range completionReportRequiredFields {
		found := false
		for _, ln := range lines {
			if strings.HasPrefix(ln, field) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, result.PolywaveError{
				Code:     "impl-completion-report",
				Severity: "error",
				Line:     lineNumber,
				Message:  fmt.Sprintf("impl-completion-report block (line %d): missing required field '%s'", lineNumber, field),
			})
		}
	}

	// Validate status value
	for _, ln := range lines {
		if !strings.HasPrefix(ln, "status:") {
			continue
		}
		// Extract the raw value after "status:" for template-placeholder detection.
		rawVal := strings.TrimSpace(strings.TrimPrefix(ln, "status:"))

		// Detect the template placeholder "complete | partial | blocked" (contains "|").
		// The bash script's tr -d '[:space:]' + cut -d'|' -f1 would silently pass this,
		// but we explicitly reject multi-value placeholders.
		if strings.Contains(rawVal, "|") {
			errs = append(errs, result.PolywaveError{
				Code:     "impl-completion-report",
				Severity: "error",
				Line:     lineNumber,
				Message:  fmt.Sprintf("impl-completion-report block (line %d): status must be 'complete', 'partial', or 'blocked' — got: '%s'", lineNumber, rawVal),
			})
			break
		}

		val := rawVal
		if val != "complete" && val != "partial" && val != "blocked" {
			errs = append(errs, result.PolywaveError{
				Code:     "impl-completion-report",
				Severity: "error",
				Line:     lineNumber,
				Message:  fmt.Sprintf("impl-completion-report block (line %d): status must be 'complete', 'partial', or 'blocked' — got: '%s'", lineNumber, val),
			})
		}
		break
	}

	return errs, nil // No agent IDs in completion reports
}
