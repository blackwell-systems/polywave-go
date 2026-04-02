package commands

import (
	"context"
	"fmt"
	"sort"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

// ExtractData wraps the result of Extract operation
type ExtractData struct {
	CommandSet *CommandSet
}

// Extract is the main entry point for command extraction. It orchestrates
// priority-based resolution across CI parsers and build system parsers.
// Returns the highest-priority CommandSet found, or language defaults if
// no config files are present. The ctx parameter allows cancellation of the
// extraction process between parser invocations.
func (e *Extractor) Extract(ctx context.Context, repoRoot string) result.Result[ExtractData] {
	var results []parserResult

	// Collect results from CI parsers
	for _, parser := range e.ciParsers {
		if err := ctx.Err(); err != nil {
			return result.NewFailure[ExtractData]([]result.SAWError{
				result.NewFatal(result.CodeCommandExtractCancelled, fmt.Sprintf("extraction cancelled: %v", err)),
			})
		}
		r := parser.ParseCI(repoRoot)
		if r.IsFatal() {
			// Log error but continue with other parsers
			continue
		}
		commandSet := r.GetData().CommandSet
		if commandSet != nil {
			results = append(results, parserResult{
				commandSet: commandSet,
				priority:   parser.Priority(),
			})
		}
	}

	// Collect results from build system parsers
	for _, parser := range e.buildSystemParsers {
		if err := ctx.Err(); err != nil {
			return result.NewFailure[ExtractData]([]result.SAWError{
				result.NewFatal(result.CodeCommandExtractCancelled, fmt.Sprintf("extraction cancelled: %v", err)),
			})
		}
		r := parser.ParseBuildSystem(repoRoot)
		if r.IsFatal() {
			// Log error but continue with other parsers
			continue
		}
		commandSet := r.GetData().CommandSet
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
		return result.NewSuccess(ExtractData{CommandSet: results[0].commandSet})
	}

	// Fall back to language defaults
	r := LanguageDefaults(repoRoot)
	if r.IsFatal() {
		return result.NewFailure[ExtractData](r.Errors)
	}
	return result.NewSuccess(ExtractData{CommandSet: r.GetData().CommandSet})
}
