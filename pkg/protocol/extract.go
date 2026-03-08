// Package protocol — extract.go provides E23 context extraction: trimming an
// IMPL doc down to the per-agent payload that is passed as the agent prompt
// parameter when launching wave agents.
package protocol

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AgentContextPayload is the trimmed per-agent context passed to wave agents (E23).
type AgentContextPayload struct {
	IMPLDocPath        string
	AgentPrompt        string
	InterfaceContracts string
	FileOwnership      string
	Scaffolds          string
	QualityGates       string
}

// ExtractAgentContext parses implDocPath and returns the per-agent payload for
// agentLetter. It extracts: the agent's 9-field prompt section, Interface
// Contracts, File Ownership, Scaffolds, and Quality Gates sections.
// Returns error if the agent section is not found in the document.
// Absent optional sections (Scaffolds, Quality Gates) are returned as empty
// strings without error.
func ExtractAgentContext(implDocPath string, agentLetter string) (*AgentContextPayload, error) {
	f, err := os.Open(implDocPath)
	if err != nil {
		return nil, fmt.Errorf("ExtractAgentContext: cannot open %q: %w", implDocPath, err)
	}
	defer f.Close()

	// Section buffers.
	var (
		agentPromptLines   []string
		interfaceContracts strings.Builder
		fileOwnership      strings.Builder
		scaffolds          strings.Builder
		qualityGates       strings.Builder
	)

	agentFound := false

	type secState int
	const (
		secTop               secState = iota
		secAgent                      // inside this agent's prompt section
		secInterfaceContracts         // inside ## Interface Contracts
		secFileOwnership              // inside ## File Ownership
		secScaffolds                  // inside ## Scaffolds
		secQualityGates               // inside ## Quality Gates
		secOther                      // inside a section we don't capture
	)

	state := secTop
	inCodeFence := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track code fences so we don't treat their contents as section headers.
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
			switch state {
			case secAgent:
				agentPromptLines = append(agentPromptLines, line)
			case secInterfaceContracts:
				interfaceContracts.WriteString(line)
				interfaceContracts.WriteByte('\n')
			case secFileOwnership:
				fileOwnership.WriteString(line)
				fileOwnership.WriteByte('\n')
			case secScaffolds:
				scaffolds.WriteString(line)
				scaffolds.WriteByte('\n')
			case secQualityGates:
				qualityGates.WriteString(line)
				qualityGates.WriteByte('\n')
			}
			continue
		}

		// While inside a code fence, accumulate into the current section.
		if inCodeFence {
			switch state {
			case secAgent:
				agentPromptLines = append(agentPromptLines, line)
			case secInterfaceContracts:
				interfaceContracts.WriteString(line)
				interfaceContracts.WriteByte('\n')
			case secFileOwnership:
				fileOwnership.WriteString(line)
				fileOwnership.WriteByte('\n')
			case secScaffolds:
				scaffolds.WriteString(line)
				scaffolds.WriteByte('\n')
			case secQualityGates:
				qualityGates.WriteString(line)
				qualityGates.WriteByte('\n')
			}
			continue
		}

		// ── ## -level headings: switch top-level section ──────────────────────
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			switch {
			case strings.EqualFold(heading, "Interface Contracts"):
				state = secInterfaceContracts
			case strings.EqualFold(heading, "File Ownership"):
				state = secFileOwnership
			case strings.EqualFold(heading, "Scaffolds"):
				state = secScaffolds
			case strings.EqualFold(heading, "Quality Gates"):
				state = secQualityGates
			default:
				state = secOther
			}
			continue
		}

		// ── ### Agent headers ─────────────────────────────────────────────────
		if isAgentHeader(line) {
			// Skip completion-report headers.
			if isCompletionReportHeader(line) {
				state = secOther
				continue
			}
			letter := extractAgentLetter(line)
			if letter == agentLetter {
				agentFound = true
				// Include the header line itself in the agent prompt.
				agentPromptLines = append(agentPromptLines, line)
				state = secAgent
			} else {
				// Different agent — leave agent section if we were in it.
				if state == secAgent {
					state = secOther
				} else if state != secTop {
					// still in a non-agent section, keep state
				}
				state = secOther
			}
			continue
		}

		// ── Accumulate into current section ───────────────────────────────────
		switch state {
		case secAgent:
			agentPromptLines = append(agentPromptLines, line)
		case secInterfaceContracts:
			interfaceContracts.WriteString(line)
			interfaceContracts.WriteByte('\n')
		case secFileOwnership:
			fileOwnership.WriteString(line)
			fileOwnership.WriteByte('\n')
		case secScaffolds:
			scaffolds.WriteString(line)
			scaffolds.WriteByte('\n')
		case secQualityGates:
			qualityGates.WriteString(line)
			qualityGates.WriteByte('\n')
		}
	}

	if err2 := scanner.Err(); err2 != nil {
		return nil, fmt.Errorf("ExtractAgentContext: scanner error reading %q: %w", implDocPath, err2)
	}

	if !agentFound {
		return nil, fmt.Errorf("ExtractAgentContext: agent %q not found in %q", agentLetter, implDocPath)
	}

	return &AgentContextPayload{
		IMPLDocPath:        implDocPath,
		AgentPrompt:        strings.TrimSpace(strings.Join(agentPromptLines, "\n")),
		InterfaceContracts: strings.TrimSpace(interfaceContracts.String()),
		FileOwnership:      strings.TrimSpace(fileOwnership.String()),
		Scaffolds:          strings.TrimSpace(scaffolds.String()),
		QualityGates:       strings.TrimSpace(qualityGates.String()),
	}, nil
}

// FormatAgentContextPayload renders an AgentContextPayload as the markdown
// string passed as the agent prompt parameter (E23 payload format from
// message-formats.md).
func FormatAgentContextPayload(payload *AgentContextPayload) string {
	var b strings.Builder
	b.WriteString("<!-- IMPL doc: ")
	b.WriteString(payload.IMPLDocPath)
	b.WriteString(" -->\n\n")
	b.WriteString(payload.AgentPrompt)
	b.WriteString("\n\n## Interface Contracts\n\n")
	b.WriteString(payload.InterfaceContracts)
	b.WriteString("\n\n## File Ownership\n\n")
	b.WriteString(payload.FileOwnership)
	b.WriteString("\n\n## Scaffolds\n\n")
	b.WriteString(payload.Scaffolds)
	b.WriteString("\n\n## Quality Gates\n\n")
	b.WriteString(payload.QualityGates)
	return b.String()
}
