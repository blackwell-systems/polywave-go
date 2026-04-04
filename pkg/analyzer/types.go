// Package analyzer provides data structures for file-level dependency graphs.
package analyzer

// DepGraph represents the file-level dependency graph with wave assignments.
// Nodes are files; edges are import dependencies; wave assignments come from
// topological sort depth (files with no deps = wave 1).
type DepGraph struct {
	Nodes             []FileNode
	Waves             map[int][]string   // wave number -> file paths
	CascadeCandidates []CascadeCandidate // was []CascadeFile; unified by P1-9
}

// FileNode represents a single Go source file in the dependency graph.
type FileNode struct {
	File          string   // Absolute path to .go file
	DependsOn     []string // Files this file imports (local packages only)
	DependedBy    []string // Files that import this file
	WaveCandidate int      // Topological depth (0 = no deps, 1+ = depends on wave N-1)
}
