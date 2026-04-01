package idgen

import (
	"fmt"
	"regexp"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// agentIDRegex matches valid agent IDs: A-Z optionally followed by 2-9
var agentIDRegex = regexp.MustCompile(`^[A-Z][2-9]?$`)

// AssignAgentIDs generates agent IDs following the ^[A-Z][2-9]?$ pattern.
// If grouping is nil, uses sequential mode (A-Z, A2-Z2, ...).
// If grouping is provided, groups by category tags and assigns multi-gen IDs.
//
// Parameters:
//   - count: Total number of agents (must be > 0)
//   - grouping: Optional array of category tags (length must equal count if provided)
//
// Returns:
//   - result.Result[[]string]: Success with ordered agent IDs (e.g., ["A", "B", "C"])
//     or failure with structured error codes:
//     - I001_AGENT_COUNT_INVALID: count <= 0
//     - I002_AGENT_COUNT_MISMATCH: grouping length != count
//     - I003_AGENT_LIMIT_EXCEEDED: count > 234
//     - I004_CATEGORY_LIMIT_EXCEEDED: >9 agents in one category
//     - I005_CATEGORY_COUNT_EXCEEDED: >26 distinct categories
//     - I006_INVALID_AGENT_ID_GENERATED: invalid ID format generated
//
// Behavior:
//   - If grouping is nil: Sequential mode (A-Z for first 26, then A2-Z2, etc.)
//   - If grouping provided: Group by category tags, assign multi-generation IDs
//     (e.g., tags ["data", "data", "api"] → IDs ["A", "A2", "B"])
func AssignAgentIDs(count int, grouping [][]string) result.Result[[]string] {
	// Validation
	if count <= 0 {
		return result.NewFailure[[]string]([]result.SAWError{result.NewError(result.CodeAgentCountInvalid, fmt.Sprintf("count must be > 0, got %d", count))})
	}

	if grouping != nil && len(grouping) != count {
		return result.NewFailure[[]string]([]result.SAWError{result.NewError(result.CodeAgentCountMismatch, fmt.Sprintf("grouping length (%d) must match count (%d)", len(grouping), count))})
	}

	// Check for >234 agents (26 letters * 9 generations = 234 max)
	if count > 234 {
		return result.NewFailure[[]string]([]result.SAWError{result.NewError(result.CodeAgentLimitExceeded, fmt.Sprintf("count %d exceeds maximum 234 agents (26 letters × 9 generations)", count))})
	}

	// Upfront validation for grouped mode
	if grouping != nil {
		// Count agents per category and track distinct categories
		categoryCounts := make(map[string]int)
		for _, tags := range grouping {
			category := ""
			if len(tags) > 0 {
				category = tags[0]
			}
			categoryCounts[category]++
		}

		// Check per-category agent cap
		for category, n := range categoryCounts {
			if n > 9 {
				return result.NewFailure[[]string]([]result.SAWError{result.NewError(result.CodeCategoryLimitExceeded, fmt.Sprintf("category %q has %d agents, maximum is 9 per category", category, n))})
			}
		}

		// Check category count cap
		if len(categoryCounts) > 26 {
			return result.NewFailure[[]string]([]result.SAWError{result.NewError(result.CodeCategoryCountExceeded, fmt.Sprintf("grouped mode requires ≤ 26 distinct categories, got %d", len(categoryCounts)))})
		}
	}

	var ids []string

	// Sequential mode: grouping is nil
	if grouping == nil {
		ids = generateSequentialIDs(count)
	} else {
		ids = generateGroupedIDs(grouping)
	}

	// Validate all generated IDs
	for i, id := range ids {
		if !agentIDRegex.MatchString(id) {
			return result.NewFailure[[]string]([]result.SAWError{result.NewError(result.CodeInvalidAgentIDGenerated, fmt.Sprintf("generated invalid agent ID %q at index %d", id, i))})
		}
	}

	return result.NewSuccess(ids)
}

// generateSequentialIDs creates agent IDs in sequential mode: A-Z, then A2-Z2, etc.
func generateSequentialIDs(count int) []string {
	ids := make([]string, count)

	for i := 0; i < count; i++ {
		generation := i / 26
		letter := rune('A' + (i % 26))

		if generation == 0 {
			ids[i] = string(letter)
		} else {
			ids[i] = fmt.Sprintf("%c%d", letter, generation+1)
		}
	}

	return ids
}

// generateGroupedIDs creates agent IDs in grouped mode, assigning multi-generation
// IDs within each category group.
func generateGroupedIDs(grouping [][]string) []string {
	// Step 1: Build order of first appearance
	seen := make(map[string]bool)
	categoryOrder := []string{} // Track order of first appearance

	for _, tags := range grouping {
		// Use first tag as category key (or empty string if no tags)
		category := ""
		if len(tags) > 0 {
			category = tags[0]
		}

		// Track first appearance order
		if !seen[category] {
			seen[category] = true
			categoryOrder = append(categoryOrder, category)
		}
	}

	// Step 2: Assign letters to categories in order of first appearance
	categoryLetters := make(map[string]rune)
	for idx, category := range categoryOrder {
		categoryLetters[category] = rune('A' + idx)
	}

	// Step 3: Assign generation numbers within each category
	ids := make([]string, len(grouping))
	categoryGeneration := make(map[string]int) // Track generation counter per category

	for i, tags := range grouping {
		category := ""
		if len(tags) > 0 {
			category = tags[0]
		}

		letter := categoryLetters[category]
		gen := categoryGeneration[category]
		categoryGeneration[category]++

		if gen == 0 {
			ids[i] = string(letter)
		} else {
			ids[i] = fmt.Sprintf("%c%d", letter, gen+1)
		}
	}

	return ids
}
