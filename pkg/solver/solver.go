package solver

import (
	"fmt"
	"sort"
	"strings"
)

// Solve computes wave assignments from dependency declarations using
// topological sort with level assignment (Kahn's algorithm variant).
// Wave numbers are 1-based. Returns Valid=false with descriptive errors
// if the graph contains cycles or missing dependency references.
func Solve(nodes []DepNode) SolveResult {
	if len(nodes) == 0 {
		return SolveResult{
			Assignments: nil,
			WaveCount:   0,
			Valid:       true,
		}
	}

	// Build node set for existence checks.
	nodeSet := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeSet[n.AgentID] = struct{}{}
	}

	// Validate: all DependsOn references must exist.
	if refErrs := ValidateRefs(nodes); refErrs != nil {
		return SolveResult{Valid: false, Errors: refErrs}
	}

	// Build adjacency list (dependee -> dependents) and in-degree map.
	inDegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		if _, ok := inDegree[n.AgentID]; !ok {
			inDegree[n.AgentID] = 0
		}
		for _, dep := range n.DependsOn {
			dependents[dep] = append(dependents[dep], n.AgentID)
			inDegree[n.AgentID]++
		}
	}

	// Kahn's algorithm with wave-level assignment.
	assignments := make(map[string]int, len(nodes))
	assigned := 0
	maxWave := 0

	// Collect initial zero in-degree nodes.
	var queue []string
	for _, n := range nodes {
		if inDegree[n.AgentID] == 0 {
			queue = append(queue, n.AgentID)
		}
	}

	wave := 1
	for len(queue) > 0 {
		// Sort current wave's nodes for determinism.
		sort.Strings(queue)

		if wave > maxWave {
			maxWave = wave
		}

		var nextQueue []string
		for _, id := range queue {
			assignments[id] = wave
			assigned++

			// Get dependents sorted for deterministic iteration.
			deps := dependents[id]
			sort.Strings(deps)
			for _, dep := range deps {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					nextQueue = append(nextQueue, dep)
				}
			}
		}
		queue = nextQueue
		wave++
	}

	// If not all nodes assigned, a cycle exists.
	if assigned < len(nodes) {
		var cycleAgents []string
		for _, n := range nodes {
			if _, ok := assignments[n.AgentID]; !ok {
				cycleAgents = append(cycleAgents, n.AgentID)
			}
		}
		sort.Strings(cycleAgents)
		return SolveResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("dependency cycle detected: agents %s are in a cycle", strings.Join(cycleAgents, ", "))},
		}
	}

	// Build sorted assignment slice: (Wave ASC, AgentID ASC).
	result := make([]Assignment, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, Assignment{
			AgentID: n.AgentID,
			Wave:    assignments[n.AgentID],
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Wave != result[j].Wave {
			return result[i].Wave < result[j].Wave
		}
		return result[i].AgentID < result[j].AgentID
	})

	return SolveResult{
		Assignments: result,
		WaveCount:   maxWave,
		Valid:       true,
	}
}
