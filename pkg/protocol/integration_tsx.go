package protocol

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// TSX/TypeScript integration support for validate-integration and wiring validation.
// Extends the Go-only heuristic scanner to detect React prop additions and
// verify JSX prop-pass at call sites.

var (
	// Matches: interface FooProps { or type FooProps = {
	propsInterfacePattern = regexp.MustCompile(`(?:interface|type)\s+(\w+Props)\s*(?:=\s*)?\{`)

	// Matches prop declarations inside Props interfaces:
	//   propName: Type
	//   propName?: Type
	propDeclPattern = regexp.MustCompile(`^\s+(\w+)\s*\??\s*:\s*`)

	// Matches: export default function ComponentName
	exportDefaultFuncRe = regexp.MustCompile(`export\s+default\s+function\s+(\w+)`)

	// Matches: export function ComponentName
	exportNamedFuncRe = regexp.MustCompile(`export\s+function\s+(\w+)`)

	// Matches: export const ComponentName
	exportNamedConstRe = regexp.MustCompile(`export\s+const\s+(\w+)`)

	// Matches: export type TypeName or export interface InterfaceName
	exportTypeRe = regexp.MustCompile(`export\s+(?:type|interface)\s+(\w+)`)
)

// extractTSXExports parses a .tsx/.ts file and returns exported symbols.
// Uses regex (no TypeScript parser) — reliable enough for integration detection.
func extractTSXExports(filePath string) ([]exportInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	content := string(data)
	lines := strings.Split(content, "\n")

	var exports []exportInfo
	seen := map[string]bool{}

	// Extract exported functions/components
	for _, re := range []*regexp.Regexp{exportDefaultFuncRe, exportNamedFuncRe, exportNamedConstRe} {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			name := match[1]
			if !seen[name] {
				exports = append(exports, exportInfo{Name: name, Kind: "func"})
				seen[name] = true
			}
		}
	}

	// Extract exported types/interfaces
	for _, match := range exportTypeRe.FindAllStringSubmatch(content, -1) {
		name := match[1]
		if !seen[name] {
			exports = append(exports, exportInfo{Name: name, Kind: "type"})
			seen[name] = true
		}
	}

	// Extract new props from *Props interfaces (these are "prop" kind exports)
	inProps := false
	braceDepth := 0
	for _, line := range lines {
		if !inProps {
			if propsInterfacePattern.MatchString(line) {
				inProps = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		if match := propDeclPattern.FindStringSubmatch(line); match != nil {
			propName := match[1]
			if !seen[propName] {
				exports = append(exports, exportInfo{Name: propName, Kind: "prop"})
				seen[propName] = true
			}
		}

		if braceDepth <= 0 {
			inProps = false
		}
	}

	return exports, nil
}

// symbolFoundViaTSXProp checks whether symbol is passed as a JSX prop attribute
// in the given file. This is stricter than grep — it looks for the pattern
// `symbol={` or `symbol="` which indicates an actual prop pass, not a comment,
// TODO, or type definition.
//
// Returns true if the symbol appears as a JSX attribute assignment.
func symbolFoundViaTSXProp(absPath, symbol string) (bool, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Patterns that indicate actual prop usage (not just mention):
	// 1. JSX attribute:     `propName={`  or  `propName="`
	// 2. Destructured use:  `props.propName` or `{ propName }` in function args
	// 3. Direct call:       `propName(` or `propName?.(`
	propPassRe := regexp.MustCompile(
		fmt.Sprintf(`\b%s\s*=\s*[{"'{(]`, regexp.QuoteMeta(symbol)),
	)
	propCallRe := regexp.MustCompile(
		fmt.Sprintf(`\b%s\s*\??\.?\s*\(`, regexp.QuoteMeta(symbol)),
	)
	propAccessRe := regexp.MustCompile(
		fmt.Sprintf(`\bprops\.%s\b`, regexp.QuoteMeta(symbol)),
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and TODO lines
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// Skip interface/type definitions (we want usage, not declaration)
		if strings.Contains(line, "interface ") || strings.Contains(line, "type ") {
			if strings.Contains(line, symbol) && strings.Contains(line, ":") && !strings.Contains(line, "={") {
				continue
			}
		}

		if propPassRe.MatchString(line) || propCallRe.MatchString(line) || propAccessRe.MatchString(line) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// isTSXFile returns true if the file path ends with .tsx or .ts
func isTSXFile(path string) bool {
	return strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts")
}
