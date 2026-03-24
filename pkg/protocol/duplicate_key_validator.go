package protocol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// ValidateDuplicateKeys detects duplicate top-level YAML keys in raw manifest bytes.
// Uses yaml.v3 Node API to walk the document and detect keys that appear more than once
// at the root mapping level.
//
// Only top-level (manifest root) keys are checked — nested duplicates are not flagged.
// Line numbers in errors are 1-indexed (yaml.Node.Line is 1-based).
//
// Returns a slice of result.SAWError, empty if no duplicates found.
func ValidateDuplicateKeys(rawYAML []byte) []result.SAWError {
	var root yaml.Node
	if err := yaml.Unmarshal(rawYAML, &root); err != nil {
		// If YAML is unparseable, return a single parse error
		return []result.SAWError{
			{
				Code:     "E16_PARSE_ERROR",
				Message:  fmt.Sprintf("failed to parse YAML: %v", err),
				Severity: "error",
			},
		}
	}

	// An empty document yields a zero-value Node with no Content
	if root.Kind == 0 || len(root.Content) == 0 {
		return nil
	}

	// The root node should be a Document node; the mapping is its first child
	docNode := &root
	if docNode.Kind != yaml.DocumentNode || len(docNode.Content) == 0 {
		return nil
	}

	mappingNode := docNode.Content[0]
	if mappingNode.Kind != yaml.MappingNode {
		return nil
	}

	// MappingNode Content is interleaved: [key1, val1, key2, val2, ...]
	// Track key -> list of line numbers (1-indexed)
	keyLines := make(map[string][]int)
	for i := 0; i+1 < len(mappingNode.Content); i += 2 {
		keyNode := mappingNode.Content[i]
		key := keyNode.Value
		keyLines[key] = append(keyLines[key], keyNode.Line)
	}

	var errs []result.SAWError
	// Collect duplicate keys in deterministic order
	keys := make([]string, 0, len(keyLines))
	for k := range keyLines {
		if len(keyLines[k]) > 1 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		lines := keyLines[key]
		lineStrs := make([]string, len(lines))
		for i, l := range lines {
			lineStrs[i] = fmt.Sprintf("%d", l)
		}
		errs = append(errs, result.SAWError{
			Code:     "E16_DUPLICATE_KEY",
			Message:  fmt.Sprintf("duplicate key %q at lines %s", key, strings.Join(lineStrs, ", ")),
			Severity: "error",
			Field:    key,
		})
	}

	return errs
}
