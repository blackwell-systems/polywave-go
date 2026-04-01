package scaffold

import "regexp"

// typeNameRe is a compiled package-level regex matching type definition keywords
// across Go, Rust, TypeScript, and Python.
// Matches: "type TypeName struct", "type TypeName interface", "struct TypeName",
// "interface TypeName", "class TypeName", "enum TypeName".
var typeNameRe = regexp.MustCompile(`(?m)^\s*(?:type|struct|interface|class|enum)\s+(\w+)\s+(?:struct|interface|enum|class|\{)`)

// extractTypeNames returns deduplicated type names found in text.
// It uses the package-level typeNameRe to avoid repeated compilation on hot paths.
func extractTypeNames(text string) []string {
	matches := typeNameRe.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var names []string
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	return names
}
