package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// GenerateProgramOpts configures automatic PROGRAM generation.
type GenerateProgramOpts struct {
	ImplRefs    []string // IMPL slugs or absolute paths to include
	RepoPath    string   // repository root (used for slug resolution only)
	ProgramSlug string   // slug for the generated PROGRAM (auto-derived if empty)
	Title       string   // title for the generated PROGRAM (auto-derived if empty)
}

// GenerateProgramData is the output of automatic PROGRAM generation.
type GenerateProgramData struct {
	ManifestPath       string             `json:"manifest_path"`
	ConflictReport     *ConflictReport    `json:"conflict_report"`
	WaveConflictReport *WaveConflictReport `json:"wave_conflict_report,omitempty"`
	TierAssignments    map[string]int     `json:"tier_assignments"`
	Manifest           *PROGRAMManifest   `json:"manifest"`
	ValidationErrors   []result.SAWError  `json:"validation_errors,omitempty"`
}

// GenerateProgramFromIMPLs creates a PROGRAM manifest from existing IMPL docs.
// It loads each IMPL by slug or absolute path, runs conflict detection for tier
// assignment, and writes a complete PROGRAMManifest YAML file to disk.
func GenerateProgramFromIMPLs(opts GenerateProgramOpts) result.Result[GenerateProgramData] {
	if len(opts.ImplRefs) == 0 {
		return result.NewFailure[GenerateProgramData]([]result.SAWError{{
			Code:     result.CodeRequiredFieldsMissing,
			Message:  "generate-program: at least one IMPL slug is required",
			Severity: "fatal",
		}})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[GenerateProgramData]([]result.SAWError{{
			Code:     result.CodeInvalidSlugFormat,
			Message:  "generate-program: RepoPath is required",
			Severity: "fatal",
		}})
	}

	// Step 1: Run wave-level conflict detection for tier assignments.
	// TierSuggestion keys are IMPL feature slugs (extracted from loaded docs),
	// not raw refs. This ensures compatibility with both slug and path refs.
	waveConflictReport, err := CheckIMPLConflictsWaveLevel(opts.ImplRefs, opts.RepoPath)
	if err != nil {
		return result.NewFailure[GenerateProgramData]([]result.SAWError{{
			Code:     result.CodeManifestInvalid,
			Message:  fmt.Sprintf("generate-program: conflict check failed: %v", err),
			Severity: "fatal",
		}})
	}
	// Downcast for backwards-compat fields used later.
	conflictReport := &waveConflictReport.ConflictReport

	// Step 2: Load each IMPL doc and build ProgramIMPL entries.
	var impls []ProgramIMPL
	var titles []string
	var slugList []string // feature slugs extracted from loaded docs (for auto-derivation)

	for _, ref := range opts.ImplRefs {
		implPath, resolveErr := resolveIMPLPathOrAbs(opts.RepoPath, ref)
		if resolveErr != nil {
			return result.NewFailure[GenerateProgramData]([]result.SAWError{{
				Code:     result.CodeInvalidAgentID,
				Message:  fmt.Sprintf("generate-program: %v", resolveErr),
				Severity: "fatal",
			}})
		}

		implDoc, loadErr := Load(implPath)
		if loadErr != nil {
			return result.NewFailure[GenerateProgramData]([]result.SAWError{{
				Code:     result.CodeDisjointOwnership,
				Message:  fmt.Sprintf("generate-program: failed to load IMPL %q: %v", ref, loadErr),
				Severity: "fatal",
			}})
		}

		// Extract canonical slug from the loaded doc.
		slug := implDoc.FeatureSlug
		if slug == "" {
			// Derive slug from filename: strip "IMPL-" prefix and ".yaml" suffix.
			// filepath.Base alone gives "IMPL-feature-a.yaml" which fails kebab-case validation.
			base := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(implPath), "IMPL-"), ".yaml")
			if base != "" {
				slug = base
			} else {
				slug = ref
			}
		}

		// Record absolute path for cross-repo refs.
		var absPath string
		if strings.HasPrefix(ref, "/") || strings.ContainsRune(ref, os.PathSeparator) {
			absPath = implPath
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

		// TierSuggestion keys are IMPL feature slugs (extracted from loaded docs),
		// not raw refs. This ensures compatibility with both slug and path refs.
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
		slugList = append(slugList, slug)

		// Populate SerialWaves from wave conflict report.
		var serialWaves []int
		if waveConflictReport.SerialWaves != nil {
			serialWaves = waveConflictReport.SerialWaves[slug]
		}

		impls = append(impls, ProgramIMPL{
			Slug:            slug,
			AbsPath:         absPath,
			Title:           title,
			Tier:            tier,
			DependsOn:       dependsOn,
			EstimatedAgents: agentCount,
			EstimatedWaves:  waveCount,
			KeyOutputs:      keyOutputs,
			Status:          status,
			SerialWaves:     serialWaves,
		})
	}

	// Step 3: Auto-derive ProgramSlug if empty.
	// Use slugList (feature slugs from loaded docs) instead of raw refs,
	// so path-based refs produce meaningful slugs.
	programSlug := opts.ProgramSlug
	if programSlug == "" {
		if len(slugList) <= 3 {
			programSlug = strings.Join(slugList, "-and-")
		} else {
			programSlug = "auto-program-" + slugList[0]
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
	outputPath := filepath.Join(opts.RepoPath, "docs", "PROGRAM", fmt.Sprintf("PROGRAM-%s.yaml", manifest.ProgramSlug))

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return result.NewFailure[GenerateProgramData]([]result.SAWError{{
			Code:     result.CodeDependencyCycle,
			Message:  fmt.Sprintf("generate-program: failed to create output directory: %v", err),
			Severity: "fatal",
		}})
	}

	if err := SaveYAML(outputPath, manifest); err != nil {
		return result.NewFailure[GenerateProgramData]([]result.SAWError{{
			Code:     result.CodeOrphanFile,
			Message:  fmt.Sprintf("generate-program: failed to write manifest: %v", err),
			Severity: "fatal",
		}})
	}

	// Step 9: Validate (non-fatal).
	validationErrors := ValidateProgram(manifest)

	data_ := GenerateProgramData{
		ManifestPath:       outputPath,
		ConflictReport:     conflictReport,
		WaveConflictReport: waveConflictReport,
		TierAssignments:    tierAssignments,
		Manifest:           manifest,
		ValidationErrors:   validationErrors,
	}

	if len(validationErrors) > 0 {
		var warnings []result.SAWError
		for _, ve := range validationErrors {
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeOrphanFile,
				Message:  ve.Message,
				Severity: "warning",
				Field:    ve.Field,
			})
		}
		return result.NewPartial(data_, warnings)
	}

	return result.NewSuccess(data_)
}

// ResolveIMPLPath is defined in program_conflict.go (canonical location).

// implStateToStatus is a backwards-compat wrapper around IMPLStateToStatus.
func implStateToStatus(state ProtocolState) string {
	return IMPLStateToStatus(state)
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
