package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// GenerateProgramOpts configures automatic PROGRAM generation.
type GenerateProgramOpts struct {
	ImplSlugs   []string // IMPL slugs to include
	RepoPath    string   // repository root
	ProgramSlug string   // slug for the generated PROGRAM (auto-derived if empty)
	Title       string   // title for the generated PROGRAM (auto-derived if empty)
}

// GenerateProgramResult is the output of automatic PROGRAM generation.
type GenerateProgramResult struct {
	ManifestPath     string            `json:"manifest_path"`
	ConflictReport   *ConflictReport   `json:"conflict_report"`
	TierAssignments  map[string]int    `json:"tier_assignments"`
	Manifest         *PROGRAMManifest  `json:"manifest"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

// GenerateProgramFromIMPLs creates a PROGRAM manifest from existing IMPL docs.
// It loads each IMPL by slug, runs conflict detection for tier assignment,
// and writes a complete PROGRAMManifest YAML file to disk.
func GenerateProgramFromIMPLs(opts GenerateProgramOpts) (*GenerateProgramResult, error) {
	if len(opts.ImplSlugs) == 0 {
		return nil, fmt.Errorf("generate-program: at least one IMPL slug is required")
	}
	if opts.RepoPath == "" {
		return nil, fmt.Errorf("generate-program: RepoPath is required")
	}

	// Step 1: Run conflict detection for tier assignments.
	conflictReport, err := CheckIMPLConflicts(opts.ImplSlugs, opts.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("generate-program: conflict check failed: %w", err)
	}

	// Step 2: Load each IMPL doc and build ProgramIMPL entries.
	var impls []ProgramIMPL
	var titles []string

	for _, slug := range opts.ImplSlugs {
		implPath := resolveIMPLPath(slug, opts.RepoPath)
		if implPath == "" {
			return nil, fmt.Errorf("generate-program: IMPL doc not found for slug %q", slug)
		}

		implDoc, loadErr := Load(implPath)
		if loadErr != nil {
			return nil, fmt.Errorf("generate-program: failed to load IMPL %q: %w", slug, loadErr)
		}

		status := implStateToStatus(implDoc.State)

		// Count agents and waves
		agentCount := 0
		for _, w := range implDoc.Waves {
			agentCount += len(w.Agents)
		}
		waveCount := len(implDoc.Waves)

		// Collect key_outputs from interface contract names
		var keyOutputs []string
		for _, ic := range implDoc.InterfaceContracts {
			keyOutputs = append(keyOutputs, ic.Name)
		}

		tier := 1
		if t, ok := conflictReport.TierSuggestion[slug]; ok {
			tier = t
		}

		// Build DependsOn for tier > 1: depend on all tier-1 slugs that overlap
		var dependsOn []string
		if tier > 1 {
			for _, c := range conflictReport.Conflicts {
				for _, conflictImpl := range c.Impls {
					if conflictImpl != slug {
						if t, ok := conflictReport.TierSuggestion[conflictImpl]; ok && t < tier {
							dependsOn = appendUnique(dependsOn, conflictImpl)
						}
					}
				}
			}
		}

		title := implDoc.Title
		if title == "" {
			title = slug
		}
		titles = append(titles, title)

		impls = append(impls, ProgramIMPL{
			Slug:            slug,
			Title:           title,
			Tier:            tier,
			DependsOn:       dependsOn,
			EstimatedAgents: agentCount,
			EstimatedWaves:  waveCount,
			KeyOutputs:      keyOutputs,
			Status:          status,
		})
	}

	// Step 3: Auto-derive ProgramSlug if empty.
	programSlug := opts.ProgramSlug
	if programSlug == "" {
		if len(opts.ImplSlugs) <= 3 {
			programSlug = strings.Join(opts.ImplSlugs, "-and-")
		} else {
			programSlug = "auto-program-" + opts.ImplSlugs[0]
		}
	}

	// Step 4: Auto-derive Title if empty.
	title := opts.Title
	if title == "" {
		title = "Auto-generated PROGRAM: " + strings.Join(titles, ", ")
	}

	// Step 5: Build tiers from IMPL tier assignments.
	tierMap := make(map[int][]string)
	maxTier := 0
	for _, impl := range impls {
		tierMap[impl.Tier] = append(tierMap[impl.Tier], impl.Slug)
		if impl.Tier > maxTier {
			maxTier = impl.Tier
		}
	}
	var tiers []ProgramTier
	for t := 1; t <= maxTier; t++ {
		if slugs, ok := tierMap[t]; ok {
			tiers = append(tiers, ProgramTier{
				Number: t,
				Impls:  slugs,
			})
		}
	}

	// Step 6: Compute completion.
	completeCount := 0
	totalAgents := 0
	totalWaves := 0
	for _, impl := range impls {
		if impl.Status == "complete" {
			completeCount++
		}
		totalAgents += impl.EstimatedAgents
		totalWaves += impl.EstimatedWaves
	}

	now := time.Now().UTC().Format(time.RFC3339)

	manifest := &PROGRAMManifest{
		Title:       title,
		ProgramSlug: programSlug,
		State:       ProgramStateReviewed,
		Created:     now,
		Updated:     now,
		Impls:       impls,
		Tiers:       tiers,
		Completion: ProgramCompletion{
			TiersComplete: 0,
			TiersTotal:    len(tiers),
			ImplsComplete: completeCount,
			ImplsTotal:    len(impls),
			TotalAgents:   totalAgents,
			TotalWaves:    totalWaves,
		},
	}

	// Step 7: Build tier assignments map for result.
	tierAssignments := make(map[string]int)
	for _, impl := range impls {
		tierAssignments[impl.Slug] = impl.Tier
	}

	// Step 8: Marshal and write to disk.
	outputPath := filepath.Join(opts.RepoPath, "docs", fmt.Sprintf("PROGRAM-%s.yaml", manifest.ProgramSlug))

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, fmt.Errorf("generate-program: failed to create output directory: %w", err)
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("generate-program: failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return nil, fmt.Errorf("generate-program: failed to write manifest: %w", err)
	}

	// Step 9: Validate (non-fatal).
	validationErrors := ValidateProgram(manifest)

	return &GenerateProgramResult{
		ManifestPath:     outputPath,
		ConflictReport:   conflictReport,
		TierAssignments:  tierAssignments,
		Manifest:         manifest,
		ValidationErrors: validationErrors,
	}, nil
}

// resolveIMPLPath finds the IMPL doc for a given slug by checking standard locations.
func resolveIMPLPath(slug, repoPath string) string {
	filename := fmt.Sprintf("IMPL-%s.yaml", slug)
	candidates := []string{
		filepath.Join(repoPath, "docs", "IMPL", filename),
		filepath.Join(repoPath, "docs", "IMPL", "complete", filename),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// implStateToStatus maps an IMPL doc ProtocolState to a program IMPL status string.
// This mirrors the logic in cmd/saw/import_impls_cmd.go but lives in pkg/protocol
// so it can be used by the generator without importing cmd/.
func implStateToStatus(state ProtocolState) string {
	switch state {
	case StateComplete:
		return "complete"
	case StateReviewed, StateScaffoldPending,
		StateWavePending, StateWaveExecuting,
		StateWaveMerging, StateWaveVerified:
		return "reviewed"
	case StateScoutPending, StateScoutValidating:
		return "pending"
	default:
		return "pending"
	}
}

// appendUnique appends s to slice only if not already present.
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
