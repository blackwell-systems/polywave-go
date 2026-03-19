package protocol

import "sort"

// UnblockBonusPerIMPL is the weight applied per transitively unblocked IMPL.
const UnblockBonusPerIMPL = 100

// buildReverseDepGraph builds a reverse dependency map from manifest.Impls.
// For each ProgramIMPL, for each slug in DependsOn, the IMPL is added to
// the reverse graph entry for that dep slug.
// Returns map[depSlug -> []slugsThatDependOnIt].
func buildReverseDepGraph(manifest *PROGRAMManifest) map[string][]string {
	rev := make(map[string][]string)
	for _, impl := range manifest.Impls {
		for _, dep := range impl.DependsOn {
			rev[dep] = append(rev[dep], impl.Slug)
		}
	}
	return rev
}

// countTransitivelyUnblocked performs a BFS from implSlug over the reverse dep
// graph and counts how many reachable IMPLs have status "pending" or "blocked".
// Completed and in_progress IMPLs are not counted (already running or done).
// Handles cycles gracefully via a visited set.
func countTransitivelyUnblocked(implSlug string, rev map[string][]string, statusBySlug map[string]string) int {
	visited := make(map[string]bool)
	queue := []string{implSlug}
	visited[implSlug] = true
	count := 0

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, dependent := range rev[curr] {
			if visited[dependent] {
				continue
			}
			visited[dependent] = true

			status := statusBySlug[dependent]
			if status == "pending" || status == "blocked" {
				count++
			}

			queue = append(queue, dependent)
		}
	}

	return count
}

// UnblockingScore computes a priority score for implSlug based on how many
// downstream IMPLs it transitively unblocks in manifest.
//
// Score formula (mirrors Formic's prioritizer):
//
//	score = unblocking_potential * UnblockBonusPerIMPL + age_bonus
//
// where:
//
//	unblocking_potential = BFS count of transitively unblocked pending/blocked IMPLs
//	UnblockBonusPerIMPL  = 100 (constant)
//	age_bonus            = min(daysSinceQueued, 10)  — tie-breaker, 0 if no timestamp
//
// Returns 0 if implSlug has no downstream dependents or is not found.
//
// TODO: Add age_bonus once ProgramIMPL gains a QueuedAt timestamp field.
func UnblockingScore(manifest *PROGRAMManifest, implSlug string) int {
	rev := buildReverseDepGraph(manifest)

	statusBySlug := make(map[string]string, len(manifest.Impls))
	for _, impl := range manifest.Impls {
		statusBySlug[impl.Slug] = impl.Status
	}

	unblockingPotential := countTransitivelyUnblocked(implSlug, rev, statusBySlug)

	// age_bonus is always 0 until ProgramIMPL gains a QueuedAt field.
	ageBonus := 0

	return unblockingPotential*UnblockBonusPerIMPL + ageBonus
}

// PrioritizeIMPLs returns implSlugs sorted by descending UnblockingScore.
// Stable: equal-scored IMPLs retain their original order (FIFO tiebreak).
func PrioritizeIMPLs(manifest *PROGRAMManifest, implSlugs []string) []string {
	result := make([]string, len(implSlugs))
	copy(result, implSlugs)

	sort.SliceStable(result, func(i, j int) bool {
		si := UnblockingScore(manifest, result[i])
		sj := UnblockingScore(manifest, result[j])
		return si > sj
	})

	return result
}
