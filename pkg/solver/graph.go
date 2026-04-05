package solver

import (
	"fmt"
	"sort"
)

// detectCycles uses DFS with a recursion stack to find all cycles in the
// dependency graph. Returns a slice of cycle paths, where each path is a
// slice of agent IDs forming the cycle (e.g. ["A", "B", "A"]). Returns nil
// if no cycles exist. Cycles are reported in sorted order (by first element)
// for determinism.
//
// Note: pkg/analyzer also has a private detectCycles function operating on
// map[string][]string (file-level adjacency). The two are intentionally separate:
// this function operates on []DepNode (agent-level dependency graph), while the
// analyzer operates on file import graphs. The input types differ by design.
func detectCycles(nodes []DepNode) [][]string {
	// Build adjacency map: agent -> deps (edges point from agent to its dependencies)
	adj := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		adj[n.AgentID] = n.DependsOn
	}

	// Collect all agent IDs and sort for deterministic iteration
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.AgentID)
	}
	sort.Strings(ids)

	var cycles [][]string
	visited := make(map[string]bool, len(ids))
	inStack := make(map[string]bool, len(ids))
	path := make([]string, 0)

	var dfs func(id string)
	dfs = func(id string) {
		visited[id] = true
		inStack[id] = true
		path = append(path, id)

		deps := adj[id]
		// Sort deps for deterministic traversal
		sorted := make([]string, len(deps))
		copy(sorted, deps)
		sort.Strings(sorted)

		for _, dep := range sorted {
			if inStack[dep] {
				// Found a cycle — extract the cycle path from the recursion stack
				cycleStart := -1
				for i, p := range path {
					if p == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path[cycleStart:]))
					copy(cycle, path[cycleStart:])
					cycle = append(cycle, dep) // close the cycle
					cycles = append(cycles, cycle)
				}
			} else if !visited[dep] {
				if _, exists := adj[dep]; exists {
					dfs(dep)
				}
			}
		}

		path = path[:len(path)-1]
		inStack[id] = false
	}

	for _, id := range ids {
		if !visited[id] {
			dfs(id)
		}
	}

	// Sort cycles by first element for determinism
	sort.Slice(cycles, func(i, j int) bool {
		ci, cj := cycles[i], cycles[j]
		minLen := len(ci)
		if len(cj) < minLen {
			minLen = len(cj)
		}
		for k := 0; k < minLen; k++ {
			if ci[k] != cj[k] {
				return ci[k] < cj[k]
			}
		}
		return len(ci) < len(cj)
	})

	if len(cycles) == 0 {
		return nil
	}
	return cycles
}

// transitiveDeps returns all transitive dependencies of the given agent
// (not just direct deps). Result is sorted alphabetically for determinism.
// Returns nil if agentID is not found in the node set or has no transitive deps.
// Unexported: no production callers exist outside this package.
func transitiveDeps(nodes []DepNode, agentID string) []string {
	// Build adjacency map
	adj := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		adj[n.AgentID] = n.DependsOn
	}

	// Check agent exists
	if _, exists := adj[agentID]; !exists {
		return nil
	}

	// BFS to find all transitive dependencies
	seen := make(map[string]bool)
	queue := make([]string, len(adj[agentID]))
	copy(queue, adj[agentID])
	for _, d := range queue {
		seen[d] = true
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if deps, ok := adj[current]; ok {
			for _, dep := range deps {
				if !seen[dep] {
					seen[dep] = true
					queue = append(queue, dep)
				}
			}
		}
	}

	// Don't include the agent itself
	delete(seen, agentID)

	if len(seen) == 0 {
		return nil
	}

	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// CriticalPath returns the longest dependency chain in the graph as a slice
// of agent IDs (from root to leaf along the longest path). This represents
// the minimum number of waves needed. Returns nil if the graph has cycles.
func CriticalPath(nodes []DepNode) []string {
	if cycles := detectCycles(nodes); cycles != nil {
		return nil
	}

	// Build adjacency map and reverse map (dependant -> dependencies becomes
	// dependency -> dependants for forward traversal)
	adj := make(map[string][]string, len(nodes))
	revAdj := make(map[string][]string, len(nodes))
	inDegree := make(map[string]int, len(nodes))

	for _, n := range nodes {
		adj[n.AgentID] = n.DependsOn
		if _, ok := inDegree[n.AgentID]; !ok {
			inDegree[n.AgentID] = 0
		}
		for _, dep := range n.DependsOn {
			revAdj[dep] = append(revAdj[dep], n.AgentID)
			inDegree[n.AgentID]++
		}
	}

	// Topological sort using Kahn's algorithm, computing longest path via DP
	dist := make(map[string]int, len(nodes))
	prev := make(map[string]string, len(nodes))

	// Initialize roots
	var queue []string
	for _, n := range nodes {
		if len(n.DependsOn) == 0 {
			dist[n.AgentID] = 1
			queue = append(queue, n.AgentID)
		}
	}
	sort.Strings(queue)

	// Process in topological order
	for len(queue) > 0 {
		// Sort queue for determinism
		sort.Strings(queue)
		current := queue[0]
		queue = queue[1:]

		// For each node that depends on current
		dependants := revAdj[current]
		sort.Strings(dependants)
		for _, dep := range dependants {
			newDist := dist[current] + 1
			if newDist > dist[dep] {
				dist[dep] = newDist
				prev[dep] = current
			}
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(dist) == 0 {
		return nil
	}

	// Find the node with the maximum distance
	var maxNode string
	maxDist := 0
	// Sort for determinism
	ids := make([]string, 0, len(dist))
	for id := range dist {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if dist[id] > maxDist {
			maxDist = dist[id]
			maxNode = id
		}
	}

	// Reconstruct path from maxNode back to root
	path := []string{maxNode}
	for {
		p, ok := prev[path[len(path)-1]]
		if !ok {
			break
		}
		path = append(path, p)
	}

	// Reverse to get root-to-leaf order
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

// ValidateRefs checks that all DependsOn entries reference agents that exist
// in the node set. Returns a slice of error messages for each invalid reference
// (sorted for determinism). Returns nil if all references are valid.
func ValidateRefs(nodes []DepNode) []string {
	// Build set of known agent IDs
	known := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		known[n.AgentID] = true
	}

	var errors []string
	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			if !known[dep] {
				errors = append(errors, fmt.Sprintf("agent %q depends on %q which does not exist", n.AgentID, dep))
			}
		}
	}

	sort.Strings(errors)

	if len(errors) == 0 {
		return nil
	}
	return errors
}
