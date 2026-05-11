// Package collision provides AST-based type collision detection for Polywave Wave agents.
// It scans agent branches for new type declarations and detects naming conflicts
// within the same package across multiple agents.
package collision

// CollisionReport is a structured report of type name collisions across agent branches.
// Valid is false if any collisions are detected.
type CollisionReport struct {
	Collisions []TypeCollision `json:"collisions"`
	Valid      bool            `json:"valid"` // false if any collisions detected
}

// TypeCollision represents a single type name collision within a package.
// It includes the colliding type name, the package path, the list of agents
// that define it, and a suggested resolution strategy.
type TypeCollision struct {
	TypeName   string   `json:"type_name"`  // e.g. "RepoEntry"
	Package    string   `json:"package"`    // e.g. "pkg/service"
	Agents     []string `json:"agents"`     // e.g. ["A", "B", "C"]
	Resolution string   `json:"resolution"` // e.g. "Keep A, remove from B and C"
}

// TypeDeclaration represents a single type declaration extracted from AST parsing.
type TypeDeclaration struct {
	Name    string `json:"name"`    // Type name (e.g. "Logger")
	Package string `json:"package"` // Package name (e.g. "service")
	Kind    string `json:"kind"`    // "struct" | "interface" | "alias"
}
