package engine

import (
	"sort"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// PrioritizeAgents analyzes the dependency graph and agent file counts
// to determine optimal launch order. Agents with deeper critical path
// depths launch first to unblock downstream work earlier.
//
// Returns a slice of agent IDs in launch order. For single-agent waves,
// returns the agent ID unchanged (no reordering needed).
//
// Tie-breaker: when two agents have equal critical path depth, the agent
// with fewer files launches first (lower implementation risk).
func PrioritizeAgents(manifest *protocol.IMPLManifest, waveNum int) []string {
	// Find the target wave
	var targetWave *protocol.Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}

	// Defensive: wave not found or empty
	if targetWave == nil || len(targetWave.Agents) == 0 {
		return []string{}
	}

	// Fast path: single agent wave needs no reordering
	if len(targetWave.Agents) == 1 {
		return []string{targetWave.Agents[0].ID}
	}

	// Build dependency graph: agent -> list of agents that depend on it
	// (i.e., which agents are blocked by this agent)
	dependents := make(map[string][]string)
	allAgents := make(map[string]bool)

	for _, agent := range targetWave.Agents {
		allAgents[agent.ID] = true
		if _, exists := dependents[agent.ID]; !exists {
			dependents[agent.ID] = []string{}
		}
	}

	// Build the dependents graph from FileOwnership
	for _, fo := range manifest.FileOwnership {
		if fo.Wave != waveNum {
			continue
		}
		// fo.Agent is the owner of the file
		// fo.DependsOn lists agents whose files this agent depends on
		for _, depID := range fo.DependsOn {
			// depID blocks fo.Agent, so fo.Agent is a dependent of depID
			if allAgents[depID] && allAgents[fo.Agent] {
				dependents[depID] = append(dependents[depID], fo.Agent)
			}
		}
	}

	// Calculate critical path depth for each agent
	criticalDepth := make(map[string]int)
	visited := make(map[string]bool)

	var calculateDepth func(agentID string) int
	calculateDepth = func(agentID string) int {
		if depth, exists := criticalDepth[agentID]; exists {
			return depth
		}

		// Detect cycles: if we're visiting an agent we're already processing,
		// we have a cycle. Return 0 to break recursion.
		if visited[agentID] {
			return 0
		}

		visited[agentID] = true
		defer func() { visited[agentID] = false }()

		// Base case: no dependents means critical path depth is 1
		deps := dependents[agentID]
		if len(deps) == 0 {
			criticalDepth[agentID] = 1
			return 1
		}

		// Recursive case: 1 + max depth of all dependents
		maxDepth := 0
		for _, depAgent := range deps {
			depth := calculateDepth(depAgent)
			if depth > maxDepth {
				maxDepth = depth
			}
		}

		criticalDepth[agentID] = 1 + maxDepth
		return criticalDepth[agentID]
	}

	// Calculate depth for all agents
	for agentID := range allAgents {
		calculateDepth(agentID)
	}

	// Build file count map for tie-breaking
	fileCount := make(map[string]int)
	for _, agent := range targetWave.Agents {
		fileCount[agent.ID] = len(agent.Files)
	}

	// Create sortable slice with agent metadata
	type agentMeta struct {
		id           string
		criticalPath int
		fileCount    int
		originalIdx  int
	}

	agentList := make([]agentMeta, 0, len(targetWave.Agents))
	for i, agent := range targetWave.Agents {
		agentList = append(agentList, agentMeta{
			id:           agent.ID,
			criticalPath: criticalDepth[agent.ID],
			fileCount:    fileCount[agent.ID],
			originalIdx:  i,
		})
	}

	// Sort by:
	// 1. Critical path depth (descending) - deeper paths first
	// 2. File count (ascending) - fewer files first
	// 3. Original index (ascending) - stable sort for ties
	sort.SliceStable(agentList, func(i, j int) bool {
		if agentList[i].criticalPath != agentList[j].criticalPath {
			return agentList[i].criticalPath > agentList[j].criticalPath
		}
		if agentList[i].fileCount != agentList[j].fileCount {
			return agentList[i].fileCount < agentList[j].fileCount
		}
		return agentList[i].originalIdx < agentList[j].originalIdx
	})

	// Extract sorted agent IDs
	result := make([]string, len(agentList))
	for i, meta := range agentList {
		result[i] = meta.id
	}

	return result
}
