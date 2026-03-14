package idgen

import (
	"fmt"
	"regexp"
)

// agentIDRegex matches valid agent IDs: A-Z optionally followed by 2-9
var agentIDRegex = regexp.MustCompile(`^[A-Z][2-9]?$`)

// AssignAgentIDs generates agent IDs following the ^[A-Z][2-9]?$ pattern.
// If grouping is nil/empty, uses sequential mode (A-Z, A2-Z2, ...).
// If grouping is provided, groups by category tags and assigns multi-gen IDs.
//
// Parameters:
//   - count: Total number of agents (must be > 0)
//   - grouping: Optional array of category tags (length must equal count if provided)
//
// Returns:
//   - []string: Ordered agent IDs (e.g., ["A", "B", "C"] or ["A", "A2", "B"])
//   - error: If count <= 0 or grouping length mismatch
//
// Behavior:
//   - If grouping is nil/empty: Sequential mode (A-Z for first 26, then A2-Z2, etc.)
//   - If grouping provided: Group by category tags, assign multi-generation IDs
//     (e.g., tags ["data", "data", "api"] → IDs ["A", "A2", "B"])
func AssignAgentIDs(count int, grouping [][]string) ([]string, error) {
	// Validation
	if count <= 0 {
		return nil, fmt.Errorf("count must be > 0, got %d", count)
	}

	if grouping != nil && len(grouping) > 0 && len(grouping) != count {
		return nil, fmt.Errorf("grouping length (%d) must match count (%d)", len(grouping), count)
	}

	// Check for >234 agents (26 letters * 9 generations = 234 max)
	if count > 234 {
		return nil, fmt.Errorf("count %d exceeds maximum 234 agents (26 letters × 9 generations)", count)
	}

	var ids []string

	// Sequential mode: grouping is nil or empty
	if grouping == nil || len(grouping) == 0 {
		ids = generateSequentialIDs(count)
	} else {
		ids = generateGroupedIDs(count, grouping)
	}

	// Validate all generated IDs
	for i, id := range ids {
		if !agentIDRegex.MatchString(id) {
			return nil, fmt.Errorf("generated invalid agent ID %q at index %d", id, i)
		}
	}

	return ids, nil
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
func generateGroupedIDs(count int, grouping [][]string) []string {
	// Step 1: Build map of category → agent indices
	categoryIndices := make(map[string][]int)
	categoryOrder := []string{} // Track order of first appearance

	for i, tags := range grouping {
		// Use first tag as category key (or empty string if no tags)
		category := ""
		if len(tags) > 0 {
			category = tags[0]
		}

		// Track first appearance order
		if _, exists := categoryIndices[category]; !exists {
			categoryOrder = append(categoryOrder, category)
		}

		categoryIndices[category] = append(categoryIndices[category], i)
	}

	// Step 2: Assign letters to categories in order of first appearance
	categoryLetters := make(map[string]rune)
	for idx, category := range categoryOrder {
		categoryLetters[category] = rune('A' + idx)
	}

	// Step 3: Assign generation numbers within each category
	ids := make([]string, count)
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
