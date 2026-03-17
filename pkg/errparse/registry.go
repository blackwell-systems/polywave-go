package errparse

import "strings"

// registry is the package-level map from tool name to Parser.
var registry = map[string]Parser{}

// Register adds a parser to the registry, keyed by its Name().
func Register(parser Parser) {
	registry[parser.Name()] = parser
}

// GetParser returns the parser for a tool name, or nil if not found.
func GetParser(toolName string) Parser {
	if p, ok := registry[toolName]; ok {
		return p
	}
	return nil
}

// DetectTool returns the tool name from a gate type + command string.
// Returns empty string if no known tool is detected.
func DetectTool(gateType string, command string) string {
	cmd := strings.TrimSpace(command)

	// Check for golangci-lint before generic "go" checks
	if strings.Contains(cmd, "golangci-lint") {
		return "golangci-lint"
	}

	// Go tools
	if strings.HasPrefix(cmd, "go ") || cmd == "go" {
		if strings.Contains(cmd, "go build") {
			return "go-build"
		}
		if strings.Contains(cmd, "go test") {
			return "go-test"
		}
		if strings.Contains(cmd, "go vet") {
			return "go-vet"
		}
	}

	// Use gateType as a hint for "go" commands
	if strings.ToLower(gateType) == "build" && strings.Contains(cmd, "go") {
		return "go-build"
	}

	// TypeScript
	if strings.Contains(cmd, "tsc") {
		return "tsc"
	}

	// ESLint
	if strings.Contains(cmd, "eslint") {
		return "eslint"
	}

	// npm test / npx jest / npx vitest
	if strings.Contains(cmd, "npm test") ||
		strings.Contains(cmd, "npx jest") ||
		strings.Contains(cmd, "npx vitest") {
		return "npm-test"
	}

	// Python tools
	if strings.Contains(cmd, "pytest") {
		return "pytest"
	}
	if strings.Contains(cmd, "mypy") {
		return "mypy"
	}
	if strings.Contains(cmd, "ruff") {
		return "ruff"
	}

	return ""
}

// ParseOutput auto-detects the tool from gate type/command and dispatches
// to the correct parser. Returns nil for unknown tools.
func ParseOutput(gateType string, command string, stdout string, stderr string) *ParseResult {
	toolName := DetectTool(gateType, command)
	if toolName == "" {
		return nil
	}

	parser := GetParser(toolName)
	if parser == nil {
		return nil
	}

	return parser.Parse(stdout, stderr)
}
