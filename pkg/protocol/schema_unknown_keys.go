package protocol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// knownKeys maps each YAML context to its set of valid keys.
// These are derived from the yaml struct tags on the corresponding Go types.
var knownKeys = map[string]map[string]bool{
	"top": {
		"title":                  true,
		"feature_slug":           true,
		"feature":                true,
		"repository":             true,
		"repositories":           true,
		"plan_reference":         true,
		"verdict":                true,
		"suitability_assessment": true,
		"test_command":           true,
		"lint_command":           true,
		"file_ownership":         true,
		"interface_contracts":    true,
		"waves":                  true,
		"quality_gates":          true,
		"post_merge_checklist":   true,
		"scaffolds":              true,
		"wiring":                 true,
		"completion_reports":     true,
		"critic_report":          true,
		"stub_reports":           true,
		"integration_reports":    true,
		"integration_connectors": true,
		"pre_mortem":             true,
		"reactions":              true,
		"known_issues":          true,
		"state":                  true,
		"merge_state":            true,
		"worktrees_created_at":   true,
		"frozen_contracts_hash":  true,
		"frozen_scaffolds_hash":  true,
		"completion_date":        true,
		"injection_method":       true,
	},
	"file_ownership": {
		"file":       true,
		"agent":      true,
		"wave":       true,
		"action":     true,
		"depends_on": true,
		"repo":       true,
	},
	"wave": {
		"number":             true,
		"type":               true,
		"agents":             true,
		"agent_launch_order": true,
		"base_commit":        true,
	},
	"agent": {
		"id":             true,
		"task":           true,
		"files":          true,
		"dependencies":   true,
		"model":          true,
		"context_source": true,
	},
	"interface_contract": {
		"name":        true,
		"description": true,
		"definition":  true,
		"location":    true,
	},
	"quality_gates": {
		"level": true,
		"gates": true,
	},
	"quality_gate": {
		"type":        true,
		"command":     true,
		"required":    true,
		"description": true,
		"repo":        true,
		"fix":         true,
		"timing":      true,
	},
	"scaffold": {
		"file_path":   true,
		"contents":    true,
		"import_path": true,
		"status":      true,
		"commit":      true,
	},
	"pre_mortem": {
		"overall_risk": true,
		"rows":         true,
	},
	"pre_mortem_row": {
		"scenario":   true,
		"likelihood": true,
		"impact":     true,
		"mitigation": true,
	},
	"known_issue": {
		"title":       true,
		"description": true,
		"status":      true,
		"workaround":  true,
	},
	"reactions": {
		"transient":    true,
		"timeout":      true,
		"fixable":      true,
		"needs_replan": true,
		"escalate":     true,
	},
	"reaction_entry": {
		"action":       true,
		"max_attempts": true,
	},
	"wiring_declaration": {
		"symbol":              true,
		"defined_in":          true,
		"must_be_called_from": true,
		"agent":               true,
		"wave":                true,
		"integration_pattern": true,
	},
	"completion_report": {
		"status":               true,
		"worktree":             true,
		"branch":               true,
		"commit":               true,
		"files_changed":        true,
		"files_created":        true,
		"interface_deviations": true,
		"out_of_scope_deps":    true,
		"tests_added":          true,
		"verification":         true,
		"failure_type":         true,
		"notes":                true,
		"repo":                 true,
	},
}

// DetectUnknownKeys detects unknown/typo YAML keys by parsing raw YAML into
// a yaml.Node tree and comparing keys at each level against the known schema.
// It returns V013_UNKNOWN_KEY errors for any unrecognized keys.
// This operates on raw YAML bytes (not the parsed struct) to catch keys that
// Go's YAML unmarshaling silently ignores.
func DetectUnknownKeys(yamlData []byte) []result.SAWError {
	// Cannot use LoadYAML: operates on raw bytes in memory and unmarshals into a yaml.Node
	// for key-walking — LoadYAML unmarshals into a typed struct, not a Node.
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return nil
	}

	// doc is a DocumentNode; its first child is the root MappingNode
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}

	var errs []result.SAWError
	checkMapping(root, "top", "", &errs)
	return errs
}

// checkMapping validates all keys in a MappingNode against the known key set
// for the given context, and recursively checks nested structures.
func checkMapping(node *yaml.Node, context, pathPrefix string, errs *[]result.SAWError) {
	known := knownKeys[context]
	if known == nil {
		return
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value

		if !known[key] {
			path := key
			if pathPrefix != "" {
				path = pathPrefix + "." + key
			}
			*errs = append(*errs, result.SAWError{
				Code:     result.CodeUnknownKey,
				Message:  fmt.Sprintf("unknown key '%s' at %s", key, path),
				Severity: "error",
				Field:    path,
				Line:     keyNode.Line,
			})
			continue
		}

		// Recurse into known nested structures
		if context == "top" {
			checkTopLevelValue(key, valNode, errs)
		}
	}
}

// checkTopLevelValue handles recursion into nested YAML structures under top-level keys.
func checkTopLevelValue(key string, valNode *yaml.Node, errs *[]result.SAWError) {
	switch key {
	case "file_ownership":
		checkSequenceOfMappings(valNode, "file_ownership", key, errs)
	case "waves":
		checkSequenceOfMappings(valNode, "wave", key, errs)
	case "interface_contracts":
		checkSequenceOfMappings(valNode, "interface_contract", key, errs)
	case "scaffolds":
		checkSequenceOfMappings(valNode, "scaffold", key, errs)
	case "wiring":
		checkSequenceOfMappings(valNode, "wiring_declaration", key, errs)
	case "known_issues":
		checkSequenceOfMappings(valNode, "known_issue", key, errs)
	case "quality_gates":
		if valNode.Kind == yaml.MappingNode {
			checkMapping(valNode, "quality_gates", key, errs)
			// Also check nested gates sequence
			for i := 0; i+1 < len(valNode.Content); i += 2 {
				if valNode.Content[i].Value == "gates" {
					checkSequenceOfMappings(valNode.Content[i+1], "quality_gate", key+".gates", errs)
				}
			}
		}
	case "pre_mortem":
		if valNode.Kind == yaml.MappingNode {
			checkMapping(valNode, "pre_mortem", key, errs)
			for i := 0; i+1 < len(valNode.Content); i += 2 {
				if valNode.Content[i].Value == "rows" {
					checkSequenceOfMappings(valNode.Content[i+1], "pre_mortem_row", key+".rows", errs)
				}
			}
		}
	case "completion_reports":
		// completion_reports is a map[string]CompletionReport
		if valNode.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(valNode.Content); i += 2 {
				agentKey := valNode.Content[i].Value
				agentVal := valNode.Content[i+1]
				if agentVal.Kind == yaml.MappingNode {
					checkMapping(agentVal, "completion_report", key+"."+agentKey, errs)
				}
			}
		}
	case "reactions":
		if valNode.Kind == yaml.MappingNode {
			checkMapping(valNode, "reactions", key, errs)
			for i := 0; i+1 < len(valNode.Content); i += 2 {
				ftKey := valNode.Content[i].Value
				ftVal := valNode.Content[i+1]
				if ftVal.Kind == yaml.MappingNode {
					checkMapping(ftVal, "reaction_entry", key+"."+ftKey, errs)
				}
			}
		}
	}
}

// checkSequenceOfMappings checks each item in a YAML sequence against the given context.
func checkSequenceOfMappings(node *yaml.Node, context, parentPath string, errs *[]result.SAWError) {
	if node.Kind != yaml.SequenceNode {
		return
	}
	for idx, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		itemPath := fmt.Sprintf("%s[%d]", parentPath, idx)
		checkMapping(item, context, itemPath, errs)

		// For wave items, recurse into agents
		if context == "wave" {
			for i := 0; i+1 < len(item.Content); i += 2 {
				if item.Content[i].Value == "agents" {
					agentsNode := item.Content[i+1]
					checkSequenceOfMappings(agentsNode, "agent", itemPath+".agents", errs)
				}
			}
		}
	}
}

// formatKeyPath builds a dot-separated path with array indices for error messages.
func formatKeyPath(parts []string) string {
	return strings.Join(parts, ".")
}

// StripUnknownKeys removes unknown top-level YAML keys from raw YAML bytes.
// It parses into a yaml.Node tree, removes key-value pairs where the key is
// not in knownKeys["top"], and re-marshals. This preserves YAML structure for
// known keys. Returns the cleaned YAML bytes, the list of stripped key names,
// and any error.
func StripUnknownKeys(yamlData []byte) ([]byte, []string, error) {
	// Cannot use LoadYAML/SaveYAML: operates on raw bytes in memory and uses the yaml.Node
	// tree API to remove key-value pairs while preserving YAML structure. Re-marshals the
	// modified Node tree (a yaml.Node, not a typed struct) back to bytes for the caller.
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return nil, nil, fmt.Errorf("StripUnknownKeys: parse YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return yamlData, nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return yamlData, nil, nil
	}

	known := knownKeys["top"]
	var stripped []string
	var cleaned []*yaml.Node

	// Walk key-value pairs, keeping only known keys
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]

		if known[keyNode.Value] {
			cleaned = append(cleaned, keyNode, valNode)
		} else {
			stripped = append(stripped, keyNode.Value)
		}
	}

	if len(stripped) == 0 {
		return yamlData, nil, nil
	}

	sort.Strings(stripped)
	root.Content = cleaned

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, nil, fmt.Errorf("StripUnknownKeys: marshal YAML: %w", err)
	}

	return out, stripped, nil
}
