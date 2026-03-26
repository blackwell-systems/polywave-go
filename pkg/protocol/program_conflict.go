package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// WaveConflict describes a file owned by multiple IMPLs in the same wave number.
type WaveConflict struct {
	File    string   `json:"file"              yaml:"file"`
	WaveNum int      `json:"wave_num"          yaml:"wave_num"`
	Impls   []string `json:"impls"             yaml:"impls"`
	Repos   []string `json:"repos,omitempty"   yaml:"repos,omitempty"`
}

// WaveConflictReport extends ConflictReport with per-wave analysis.
// ConflictReport fields are still populated for backwards compatibility.
// WaveConflicts contains only concurrent-wave-level conflicts.
// SerialWaves maps each IMPL slug to the wave numbers that must serialize.
type WaveConflictReport struct {
	ConflictReport
	WaveConflicts []WaveConflict   `json:"wave_conflicts" yaml:"wave_conflicts"`
	SerialWaves   map[string][]int `json:"serial_waves"   yaml:"serial_waves"`
}

// buildWaveFileOwnershipMap builds a per-wave file ownership index.
// implDocs maps slug -> *IMPLManifest.
// Returns: map[waveNum]map[qualifiedFile][]slug
func buildWaveFileOwnershipMap(implDocs map[string]*IMPLManifest) map[int]map[string][]string {
	result := make(map[int]map[string][]string)
	for slug, manifest := range implDocs {
		for _, fo := range manifest.FileOwnership {
			waveNum := fo.Wave
			if waveNum == 0 {
				waveNum = 1 // default
			}
			key := fo.File
			if fo.Repo != "" {
				key = fo.Repo + ":" + fo.File
			}
			if result[waveNum] == nil {
				result[waveNum] = make(map[string][]string)
			}
			// deduplicate slugs per file per wave
			found := false
			for _, existing := range result[waveNum][key] {
				if existing == slug {
					found = true
					break
				}
			}
			if !found {
				result[waveNum][key] = append(result[waveNum][key], slug)
			}
		}
	}
	return result
}

// CheckIMPLConflictsWaveLevel performs wave-aware cross-IMPL conflict detection.
// It replaces the coarse IMPL-level check used in GenerateProgramFromIMPLs.
// implSlugs: IMPL slugs to analyze
// repoPath: repository root
// Returns WaveConflictReport, which embeds ConflictReport for backwards compat.
func CheckIMPLConflictsWaveLevel(implSlugs []string, repoPath string) (*WaveConflictReport, error) {
	// Step 1: Run existing IMPL-level check for backwards compat
	implReport, err := CheckIMPLConflicts(implSlugs, repoPath)
	if err != nil {
		return nil, err
	}

	// Step 2: Load all IMPL docs
	implDocs := make(map[string]*IMPLManifest)
	for _, slug := range implSlugs {
		implPath, err := resolveIMPLPathOrAbs(repoPath, slug)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve IMPL %q: %w", slug, err)
		}
		manifest, err := Load(implPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load IMPL %q: %w", slug, err)
		}
		implDocs[slug] = manifest
	}

	// Step 3: Build per-wave ownership map
	waveOwnership := buildWaveFileOwnershipMap(implDocs)

	// Step 4: Detect wave-level conflicts
	var waveConflicts []WaveConflict
	// wave1ConflictMap is used for tier suggestion: only Wave 1 conflicts determine tier separation.
	// Later-wave conflicts are captured in SerialWaves — the IMPLs remain in the same tier.
	wave1ConflictMap := make(map[string]map[string]bool) // slug -> set of conflicting slugs (wave 1 only)

	// Sort wave numbers for deterministic output
	var waveNums []int
	for wn := range waveOwnership {
		waveNums = append(waveNums, wn)
	}
	sort.Ints(waveNums)

	for _, wn := range waveNums {
		fileMap := waveOwnership[wn]
		// Sort files for deterministic output
		var files []string
		for f := range fileMap {
			files = append(files, f)
		}
		sort.Strings(files)

		for _, file := range files {
			owners := fileMap[file]
			if len(owners) > 1 {
				sorted := make([]string, len(owners))
				copy(sorted, owners)
				sort.Strings(sorted)
				waveConflicts = append(waveConflicts, WaveConflict{
					File:    file,
					WaveNum: wn,
					Impls:   sorted,
				})
				// Only Wave 1 conflicts affect tier assignment.
				// Later-wave conflicts are serialized within a tier via SerialWaves.
				if wn == 1 {
					for _, a := range sorted {
						for _, b := range sorted {
							if a != b {
								if wave1ConflictMap[a] == nil {
									wave1ConflictMap[a] = make(map[string]bool)
								}
								wave1ConflictMap[a][b] = true
							}
						}
					}
				}
			}
		}
	}

	// Step 5: Build SerialWaves: for each slug, which wave numbers need serialization
	serialWaves := make(map[string][]int)
	for _, wc := range waveConflicts {
		for _, slug := range wc.Impls {
			// Check if wc.WaveNum already in serialWaves[slug]
			found := false
			for _, existing := range serialWaves[slug] {
				if existing == wc.WaveNum {
					found = true
					break
				}
			}
			if !found {
				serialWaves[slug] = append(serialWaves[slug], wc.WaveNum)
			}
		}
	}
	// Sort serial wave numbers for determinism
	for slug := range serialWaves {
		sort.Ints(serialWaves[slug])
	}

	// Step 6: Compute wave-level TierSuggestion (override IMPL-level suggestion)
	// Only Wave 1 conflicts drive tier separation. Two IMPLs whose first waves are
	// disjoint can share tier 1 — their later-wave conflicts are handled via SerialWaves.
	waveTierSuggestion := computeTierSuggestion(implSlugs, wave1ConflictMap)

	// Override TierSuggestion in the embedded ConflictReport with wave-level result
	implReport.TierSuggestion = waveTierSuggestion

	return &WaveConflictReport{
		ConflictReport: *implReport,
		WaveConflicts:  waveConflicts,
		SerialWaves:    serialWaves,
	}, nil
}

// CheckIMPLConflicts loads IMPL docs and performs cross-IMPL disjointness analysis.
// implRefs: list of IMPL slugs or absolute paths to analyze
// repoPath: repository root for resolving slug-based refs
func CheckIMPLConflicts(implRefs []string, repoPath string) (*ConflictReport, error) {
	if len(implRefs) == 0 {
		return &ConflictReport{
			Conflicts:      []IMPLFileConflict{},
			DisjointSets:   [][]string{},
			TierSuggestion: map[string]int{},
		}, nil
	}

	// Load all IMPL docs and build file -> []featureSlug map.
	// TierSuggestion keys are IMPL feature slugs (extracted from loaded docs),
	// not raw refs. This ensures compatibility with both slug and path refs.
	fileOwners := make(map[string][]string) // qualified-file -> feature slugs
	var featureSlugs []string               // ordered list of extracted feature slugs
	for _, ref := range implRefs {
		implPath, err := resolveIMPLPathOrAbs(repoPath, ref)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve IMPL ref %q: %w", ref, err)
		}
		manifest, err := Load(implPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load IMPL %q: %w", ref, err)
		}

		// Use the feature slug extracted from the loaded doc as the canonical key.
		slug := manifest.FeatureSlug
		if slug == "" {
			slug = ref // fallback for malformed docs
		}
		featureSlugs = append(featureSlugs, slug)

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

	// Compute DisjointSets using connected components (keyed by feature slug)
	disjointSets := computeDisjointSets(featureSlugs, overlaps)

	// Compute TierSuggestion using greedy graph coloring (keyed by feature slug)
	tierSuggestion := computeTierSuggestion(featureSlugs, overlaps)

	return &ConflictReport{
		Conflicts:      conflicts,
		DisjointSets:   disjointSets,
		TierSuggestion: tierSuggestion,
	}, nil
}

// resolveIMPLPathOrAbs resolves a single IMPL reference (slug or absolute path)
// to an absolute file path.
//
// Path detection rules:
//   - filepath.IsAbs(ref) → absolute path; verify exists, return as-is
//   - contains os.PathSeparator → relative path; join with repoPath, verify exists
//   - otherwise → slug; check docs/IMPL/IMPL-<slug>.yaml and docs/IMPL/complete/IMPL-<slug>.yaml
func resolveIMPLPathOrAbs(repoPath, ref string) (string, error) {
	if filepath.IsAbs(ref) {
		if _, err := os.Stat(ref); err != nil {
			return "", fmt.Errorf("IMPL path %q does not exist", ref)
		}
		return ref, nil
	}
	if strings.ContainsRune(ref, os.PathSeparator) {
		joined := filepath.Join(repoPath, ref)
		if _, err := os.Stat(joined); err != nil {
			return "", fmt.Errorf("IMPL path %q does not exist", joined)
		}
		return joined, nil
	}
	// Treat as slug
	return resolveIMPLPath(repoPath, ref)
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
