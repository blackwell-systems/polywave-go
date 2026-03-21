package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ConflictReport describes cross-IMPL file ownership overlaps.
type ConflictReport struct {
	Conflicts      []IMPLFileConflict `json:"conflicts"`
	DisjointSets   [][]string         `json:"disjoint_sets"`
	TierSuggestion map[string]int     `json:"tier_suggestion"`
}

// IMPLFileConflict describes a single file owned by multiple IMPLs.
type IMPLFileConflict struct {
	File  string   `json:"file"`
	Impls []string `json:"impls"`
	Repos []string `json:"repos,omitempty"`
}

// CheckIMPLConflicts loads IMPL docs and performs cross-IMPL disjointness analysis.
// implSlugs: list of IMPL slugs to analyze (resolved from docs/IMPL/ and docs/IMPL/complete/)
// repoPath: repository root for resolving IMPL doc paths
func CheckIMPLConflicts(implSlugs []string, repoPath string) (*ConflictReport, error) {
	if len(implSlugs) == 0 {
		return &ConflictReport{
			Conflicts:      []IMPLFileConflict{},
			DisjointSets:   [][]string{},
			TierSuggestion: map[string]int{},
		}, nil
	}

	// Load all IMPL docs and build file -> []slug map
	fileOwners := make(map[string][]string) // qualified-file -> slugs
	for _, slug := range implSlugs {
		implPath, err := resolveIMPLPath(repoPath, slug)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve IMPL %q: %w", slug, err)
		}
		manifest, err := Load(implPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load IMPL %q: %w", slug, err)
		}

		for _, fo := range manifest.FileOwnership {
			key := fo.File
			if fo.Repo != "" {
				key = fo.Repo + ":" + fo.File
			}
			// Only add slug once per file
			found := false
			for _, existing := range fileOwners[key] {
				if existing == slug {
					found = true
					break
				}
			}
			if !found {
				fileOwners[key] = append(fileOwners[key], slug)
			}
		}
	}

	// Detect conflicts: any file owned by 2+ slugs
	var conflicts []IMPLFileConflict
	for file, owners := range fileOwners {
		if len(owners) > 1 {
			sorted := make([]string, len(owners))
			copy(sorted, owners)
			sort.Strings(sorted)
			conflicts = append(conflicts, IMPLFileConflict{
				File:  file,
				Impls: sorted,
			})
		}
	}
	// Sort conflicts for deterministic output
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].File < conflicts[j].File
	})

	// Build overlap graph: slug -> set of slugs it overlaps with
	overlaps := make(map[string]map[string]bool)
	for _, owners := range fileOwners {
		if len(owners) <= 1 {
			continue
		}
		for _, a := range owners {
			for _, b := range owners {
				if a != b {
					if overlaps[a] == nil {
						overlaps[a] = make(map[string]bool)
					}
					overlaps[a][b] = true
				}
			}
		}
	}

	// Compute DisjointSets using connected components
	disjointSets := computeDisjointSets(implSlugs, overlaps)

	// Compute TierSuggestion using greedy graph coloring
	tierSuggestion := computeTierSuggestion(implSlugs, overlaps)

	return &ConflictReport{
		Conflicts:      conflicts,
		DisjointSets:   disjointSets,
		TierSuggestion: tierSuggestion,
	}, nil
}

// resolveIMPLPath finds the IMPL doc for a given slug, checking both
// docs/IMPL/ and docs/IMPL/complete/ directories.
func resolveIMPLPath(repoPath, slug string) (string, error) {
	candidates := []string{
		filepath.Join(repoPath, "docs", "IMPL", fmt.Sprintf("IMPL-%s.yaml", slug)),
		filepath.Join(repoPath, "docs", "IMPL", "complete", fmt.Sprintf("IMPL-%s.yaml", slug)),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("IMPL doc not found for slug %q (checked %s and %s)", slug, candidates[0], candidates[1])
}

// computeDisjointSets finds connected components in the overlap graph.
// Each component contains slugs that directly or transitively overlap.
// Slugs with no overlaps are singleton sets.
func computeDisjointSets(slugs []string, overlaps map[string]map[string]bool) [][]string {
	visited := make(map[string]bool)
	var sets [][]string

	for _, slug := range slugs {
		if visited[slug] {
			continue
		}
		// BFS to find connected component
		var component []string
		queue := []string{slug}
		visited[slug] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			component = append(component, current)
			for neighbor := range overlaps[current] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		sort.Strings(component)
		sets = append(sets, component)
	}

	// Sort sets for deterministic output (by first element)
	sort.Slice(sets, func(i, j int) bool {
		return sets[i][0] < sets[j][0]
	})

	return sets
}

// computeTierSuggestion assigns each slug to the earliest tier where it has
// no conflicts with already-assigned slugs. This is identical to the algorithm
// in computeTierAssignments in cmd/saw/import_impls_cmd.go but operates on
// slugs directly instead of ImportedIMPL structs.
func computeTierSuggestion(slugs []string, overlaps map[string]map[string]bool) map[string]int {
	assignments := make(map[string]int)
	tierMembers := make(map[int][]string) // tier -> slugs assigned to it

	for _, slug := range slugs {
		assigned := false
		for tier := 1; tier <= len(slugs); tier++ {
			conflict := false
			for _, member := range tierMembers[tier] {
				if overlaps[slug] != nil && overlaps[slug][member] {
					conflict = true
					break
				}
			}
			if !conflict {
				assignments[slug] = tier
				tierMembers[tier] = append(tierMembers[tier], slug)
				assigned = true
				break
			}
		}
		if !assigned {
			// Fallback: assign to new tier
			tier := len(tierMembers) + 1
			assignments[slug] = tier
			tierMembers[tier] = append(tierMembers[tier], slug)
		}
	}

	return assignments
}
