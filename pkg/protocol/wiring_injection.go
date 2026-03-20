package protocol

import (
	"fmt"
	"strings"
)

// InjectWiringInstructions formats wiring obligations for an agent into markdown
// for injection into .saw-agent-brief.md (E35 Layer 3C).
//
// It filters wiring declarations by both agentID and waveNum to ensure agents
// only see obligations relevant to their current wave.
//
// Returns an empty string if the agent has no wiring obligations in this wave.
func InjectWiringInstructions(doc *IMPLManifest, agentID string, waveNum int) string {
	var entries []WiringDeclaration
	for _, w := range doc.Wiring {
		if w.Agent == agentID && w.Wave == waveNum {
			entries = append(entries, w)
		}
	}
	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Wiring Obligations (E35)\n\n")
	sb.WriteString("You must wire the following exports into existing aggregation points.\n")
	sb.WriteString("These declarations are specified in the IMPL doc `wiring:` block and will be\n")
	sb.WriteString("validated post-merge by `validate-integration` (severity: error if missing).\n\n")
	sb.WriteString("| Symbol | Defined In | Must Be Called From | Pattern |\n")
	sb.WriteString("|--------|------------|---------------------|--------|\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			e.Symbol, e.DefinedIn, e.MustBeCalledFrom, e.IntegrationPattern))
	}
	sb.WriteString("\n")

	return sb.String()
}
