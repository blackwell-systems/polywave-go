package commands

import (
	"fmt"
	"sort"
)

// Extractor orchestrates priority-based command resolution across CI parsers
// and build system parsers. It collects results from all registered parsers,
// sorts by priority descending, and returns the first non-nil CommandSet.
type Extractor struct {
	ciParsers          []CIParser
	buildSystemParsers []BuildSystemParser
}

// New creates a new Extractor with no parsers registered.
func New() *Extractor {
	return &Extractor{
		ciParsers:          make([]CIParser, 0),
		buildSystemParsers: make([]BuildSystemParser, 0),
	}
}

// RegisterCIParser adds a CI parser to the extractor.
func (e *Extractor) RegisterCIParser(parser CIParser) {
	e.ciParsers = append(e.ciParsers, parser)
}

// RegisterBuildSystemParser adds a build system parser to the extractor.
func (e *Extractor) RegisterBuildSystemParser(parser BuildSystemParser) {
	e.buildSystemParsers = append(e.buildSystemParsers, parser)
}

// parserResult holds a CommandSet and its priority for sorting.
type parserResult struct {
	commandSet *CommandSet
	priority   int
}

// Extract is the main entry point for command extraction. It orchestrates
// priority-based resolution across CI parsers and build system parsers.
// Returns the highest-priority CommandSet found, or language defaults if
// no config files are present.
func (e *Extractor) Extract(repoRoot string) (*CommandSet, error) {
	var results []parserResult

	// Collect results from CI parsers
	for _, parser := range e.ciParsers {
		commandSet, err := parser.ParseCI(repoRoot)
		if err != nil {
			// Log error but continue with other parsers
			continue
		}
		if commandSet != nil {
			results = append(results, parserResult{
				commandSet: commandSet,
				priority:   parser.Priority(),
			})
		}
	}

	// Collect results from build system parsers
	for _, parser := range e.buildSystemParsers {
		commandSet, err := parser.ParseBuildSystem(repoRoot)
		if err != nil {
			// Log error but continue with other parsers
			continue
		}
		if commandSet != nil {
			results = append(results, parserResult{
				commandSet: commandSet,
				priority:   parser.Priority(),
			})
		}
	}

	// Sort by priority descending (stable sort for ties)
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].priority > results[j].priority
	})

	// Return highest-priority result if available
	if len(results) > 0 {
		return results[0].commandSet, nil
	}

	// Fall back to language defaults
	commandSet, err := LanguageDefaults(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("all parsers returned nil and language defaults failed: %w", err)
	}
	return commandSet, nil
}
