// Package protocol — extract.go provides E23 context extraction: trimming an
// IMPL doc down to the per-agent payload that is passed as the agent prompt
// parameter when launching wave agents.
package protocol

import (
	"context"
	"fmt"
)

// AgentContextJSONPayload is the structured JSON output of 'polywave-tools extract-context' (E23).
type AgentContextJSONPayload struct {
	IMPLDocPath        string              `json:"impl_doc_path"`
	AgentID            string              `json:"agent_id"`
	AgentTask          string              `json:"agent_task"`
	FileOwnership      []FileOwnership     `json:"file_ownership"`
	InterfaceContracts []InterfaceContract `json:"interface_contracts"`
	Scaffolds          []ScaffoldFile      `json:"scaffolds"`
	QualityGates       *QualityGates       `json:"quality_gates"`
}

// ExtractAgentContextFromManifest builds a per-agent context payload from a parsed
// IMPLManifest. YAML-mode equivalent of ExtractAgentContext. Returns ErrAgentNotFound
// (wrapped) if agentID is not in any wave.
func ExtractAgentContextFromManifest(ctx context.Context, m *IMPLManifest, agentID string) (*AgentContextJSONPayload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Search all waves for the agent with matching ID.
	var foundAgent *Agent
	for i := range m.Waves {
		for j := range m.Waves[i].Agents {
			if m.Waves[i].Agents[j].ID == agentID {
				foundAgent = &m.Waves[i].Agents[j]
				break
			}
		}
		if foundAgent != nil {
			break
		}
	}

	if foundAgent == nil {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, agentID)
	}

	// Filter file ownership entries to only those belonging to this agent.
	var agentFiles []FileOwnership
	for _, fo := range m.FileOwnership {
		if fo.Agent == agentID {
			agentFiles = append(agentFiles, fo)
		}
	}
	if agentFiles == nil {
		agentFiles = []FileOwnership{}
	}

	// All interface contracts and scaffolds are shared across all agents.
	contracts := m.InterfaceContracts
	if contracts == nil {
		contracts = []InterfaceContract{}
	}
	scaffolds := m.Scaffolds
	if scaffolds == nil {
		scaffolds = []ScaffoldFile{}
	}

	return &AgentContextJSONPayload{
		IMPLDocPath:        "",
		AgentID:            agentID,
		AgentTask:          foundAgent.Task,
		FileOwnership:      agentFiles,
		InterfaceContracts: contracts,
		Scaffolds:          scaffolds,
		QualityGates:       m.QualityGates,
	}, nil
}

