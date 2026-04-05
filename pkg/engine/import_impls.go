package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ImportImplsOpts configures an ImportImpls run.
type ImportImplsOpts struct {
	ProgramPath string       // absolute path to PROGRAM manifest (required)
	FromImpls   []string     // explicit IMPL paths; mutually exclusive with Discover
	Discover    bool         // auto-discover IMPL docs in docs/IMPL/ and docs/IMPL/complete/
	RepoDir     string       // absolute repo root for discovery (required when Discover=true)
	Logger      *slog.Logger // optional
}

// ImportImpls handles IMPL discovery, manifest parsing/creation, tier
// assignment, and PROGRAM manifest save. It returns result.Result[protocol.ImportIMPLsData].
func ImportImpls(ctx context.Context, opts ImportImplsOpts) result.Result[protocol.ImportIMPLsData] {
	// Discover IMPL files if requested
	var implPaths []string
	var discoveredPaths []string
	if opts.Discover {
		patterns := []string{
			filepath.Join(protocol.IMPLDir(opts.RepoDir), "IMPL-*.yaml"),
			filepath.Join(protocol.IMPLCompleteDir(opts.RepoDir), "IMPL-*.yaml"),
		}
		for _, pattern := range patterns {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return result.NewFailure[protocol.ImportIMPLsData]([]result.SAWError{
					result.NewFatal(result.CodePrepareWaveFailed, fmt.Sprintf("import-impls: glob error: %v", err)),
				})
			}
			implPaths = append(implPaths, matches...)
		}
		discoveredPaths = append(discoveredPaths, implPaths...)
	} else {
		implPaths = opts.FromImpls
	}

	if len(implPaths) == 0 {
		return result.NewFailure[protocol.ImportIMPLsData]([]result.SAWError{
			result.NewFatal(result.CodeIMPLNotFound, "import-impls: no IMPL docs found"),
		})
	}

	// Parse or create program manifest
	var manifest *protocol.PROGRAMManifest
	var created bool
	manifest, err := protocol.ParseProgramManifest(opts.ProgramPath)
	if err != nil {
		// Create a minimal manifest if the file doesn't exist
		if os.IsNotExist(unwrapPathError(err)) {
			slug := slugFromProgramPath(opts.ProgramPath)
			manifest = &protocol.PROGRAMManifest{
				Title:       slug,
				ProgramSlug: slug,
				State:       protocol.ProgramStatePlanning,
			}
			created = true
		} else {
			return result.NewFailure[protocol.ImportIMPLsData]([]result.SAWError{
				result.NewFatal(result.CodeIMPLParseFailed, fmt.Sprintf("import-impls: failed to parse program manifest: %v", err)),
			})
		}
	}

	// Track existing IMPL slugs in manifest
	existingSlugs := make(map[string]bool)
	for _, impl := range manifest.Impls {
		existingSlugs[impl.Slug] = true
	}

	// Track file ownership across all IMPLs for tier assignment
	// file -> list of IMPL slugs that own it
	fileOwners := make(map[string][]string)

	// Parse each IMPL doc and build imported entries
	var imported []protocol.ImportedIMPL
	for _, implPath := range implPaths {
		implDoc, loadErr := protocol.Load(ctx, implPath)
		if loadErr != nil {
			slog.WarnContext(ctx, "import-impls: failed to load IMPL doc", "path", implPath, "err", loadErr)
			continue
		}

		slug := protocol.ExtractIMPLSlug(implPath, implDoc)

		// Skip if already in manifest
		if existingSlugs[slug] {
			continue
		}

		// Map IMPL state to program status
		status := protocol.IMPLStateToStatus(implDoc.State)

		// Count agents and waves
		agentCount := 0
		for _, w := range implDoc.Waves {
			agentCount += len(w.Agents)
		}
		waveCount := len(implDoc.Waves)

		// Track file ownership for tier assignment
		for _, fo := range implDoc.FileOwnership {
			fileOwners[fo.File] = append(fileOwners[fo.File], slug)
		}

		imp := protocol.ImportedIMPL{
			Slug:         slug,
			Title:        implDoc.Title,
			Status:       status,
			AssignedTier: 1, // default; adjusted below
			AgentCount:   agentCount,
			WaveCount:    waveCount,
		}
		imported = append(imported, imp)
		existingSlugs[slug] = true
	}

	// Compute tier assignments based on file ownership overlap.
	// IMPLs that share files with other IMPLs get assigned to later tiers.
	var slugList []string
	for _, imp := range imported {
		slugList = append(slugList, imp.Slug)
	}
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
	tierAssignments := protocol.ComputeTierAssignments(slugList, overlaps)

	// Apply tier assignments and add to manifest
	for i := range imported {
		if tier, ok := tierAssignments[imported[i].Slug]; ok {
			imported[i].AssignedTier = tier
		}

		manifest.Impls = append(manifest.Impls, protocol.ProgramIMPL{
			Slug:            imported[i].Slug,
			Title:           imported[i].Title,
			Tier:            imported[i].AssignedTier,
			Status:          imported[i].Status,
			EstimatedAgents: imported[i].AgentCount,
			EstimatedWaves:  imported[i].WaveCount,
		})
	}

	// Rebuild tiers from IMPL tier assignments
	rebuildTiers(manifest)

	// Update completion counts
	manifest.Completion.ImplsTotal = len(manifest.Impls)
	manifest.Completion.TiersTotal = len(manifest.Tiers)
	completeCount := 0
	for _, impl := range manifest.Impls {
		if impl.Status == "complete" {
			completeCount++
		}
	}
	manifest.Completion.ImplsComplete = completeCount

	// Run import-mode validation for P1/P2 conflict detection
	importErrs := protocol.ValidateProgramImportMode(ctx, manifest, opts.RepoDir)
	var p1Conflicts, p2Conflicts []string
	for _, ve := range importErrs {
		switch ve.Code {
		case "P1_FILE_OVERLAP":
			p1Conflicts = append(p1Conflicts, ve.Message)
		case "P2_CONTRACT_REDEFINITION":
			p2Conflicts = append(p2Conflicts, ve.Message)
		}
	}

	// Write updated manifest
	if err := os.MkdirAll(filepath.Dir(opts.ProgramPath), 0755); err != nil {
		return result.NewFailure[protocol.ImportIMPLsData]([]result.SAWError{
			result.NewFatal(result.CodeWriteManifestFailed, fmt.Sprintf("import-impls: failed to create directory: %v", err)),
		})
	}
	if err := protocol.SaveYAML(opts.ProgramPath, manifest); err != nil {
		return result.NewFailure[protocol.ImportIMPLsData]([]result.SAWError{
			result.NewFatal(result.CodeWriteManifestFailed, fmt.Sprintf("import-impls: failed to save manifest: %v", err)),
		})
	}

	// Build data
	data := protocol.ImportIMPLsData{
		ManifestPath:    opts.ProgramPath,
		ImplsImported:   imported,
		TierAssignments: tierAssignments,
		P1Conflicts:     p1Conflicts,
		P2Conflicts:     p2Conflicts,
		Created:         created,
		Updated:         !created,
	}
	if opts.Discover {
		data.ImplsDiscovered = discoveredPaths
	}

	return result.NewSuccess(data)
}

// rebuildTiers rebuilds the Tiers slice from IMPL tier assignments.
func rebuildTiers(manifest *protocol.PROGRAMManifest) {
	tierMap := make(map[int][]string)
	maxTier := 0
	for _, impl := range manifest.Impls {
		tierMap[impl.Tier] = append(tierMap[impl.Tier], impl.Slug)
		if impl.Tier > maxTier {
			maxTier = impl.Tier
		}
	}

	manifest.Tiers = nil
	for t := 1; t <= maxTier; t++ {
		if slugs, ok := tierMap[t]; ok {
			manifest.Tiers = append(manifest.Tiers, protocol.ProgramTier{
				Number: t,
				Impls:  slugs,
			})
		}
	}
}

// slugFromProgramPath extracts a slug from a PROGRAM manifest filename.
func slugFromProgramPath(path string) string {
	base := filepath.Base(path)
	slug := strings.TrimPrefix(base, "PROGRAM-")
	slug = strings.TrimSuffix(slug, ".yaml")
	slug = strings.TrimSuffix(slug, ".yml")
	return slug
}

// unwrapPathError extracts the underlying error from an os.PathError wrapper.
func unwrapPathError(err error) error {
	if err == nil {
		return nil
	}
	// Check if the inner error from ParseProgramManifest wraps an os.PathError
	for e := err; e != nil; {
		if pe, ok := e.(*os.PathError); ok {
			return pe
		}
		if uw, ok := e.(interface{ Unwrap() error }); ok {
			e = uw.Unwrap()
		} else {
			break
		}
	}
	return err
}
