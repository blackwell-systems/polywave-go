package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newImportImplsCmd() *cobra.Command {
	var (
		programPath string
		fromImpls   []string
		discover    bool
		repoDir     string
	)

	cmd := &cobra.Command{
		Use:   "import-impls",
		Short: "Import existing IMPL docs into a PROGRAM manifest",
		Long: `Import existing IMPL documents into a PROGRAM manifest for tiered execution.

Examples:
  # Discover all IMPL docs in the repo
  sawtools import-impls --program PROGRAM-my-feature.yaml --discover

  # Import specific IMPL docs
  sawtools import-impls --program PROGRAM-my-feature.yaml --from-impls IMPL-a.yaml IMPL-b.yaml

  # Discover from a specific repo directory
  sawtools import-impls --program PROGRAM-my-feature.yaml --discover --repo-dir /path/to/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if programPath == "" {
				return fmt.Errorf("import-impls: --program flag is required")
			}
			if !discover && len(fromImpls) == 0 {
				return fmt.Errorf("import-impls: must specify --discover or --from-impls")
			}

			// Resolve repo dir
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("import-impls: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			// Discover IMPL files if requested
			var implPaths []string
			var discoveredPaths []string
			if discover {
				patterns := []string{
					filepath.Join(repoDir, "docs", "IMPL", "IMPL-*.yaml"),
					filepath.Join(repoDir, "docs", "IMPL", "complete", "IMPL-*.yaml"),
				}
				for _, pattern := range patterns {
					matches, err := filepath.Glob(pattern)
					if err != nil {
						return fmt.Errorf("import-impls: glob error: %w", err)
					}
					implPaths = append(implPaths, matches...)
				}
				for _, p := range implPaths {
					discoveredPaths = append(discoveredPaths, p)
				}
			} else {
				implPaths = fromImpls
			}

			if len(implPaths) == 0 {
				return fmt.Errorf("import-impls: no IMPL docs found")
			}

			// Parse or create program manifest
			var manifest *protocol.PROGRAMManifest
			var created bool
			manifest, err := protocol.ParseProgramManifest(programPath)
			if err != nil {
				// Create a minimal manifest if the file doesn't exist
				if os.IsNotExist(unwrapPathError(err)) {
					slug := slugFromProgramPath(programPath)
					manifest = &protocol.PROGRAMManifest{
						Title:       slug,
						ProgramSlug: slug,
						State:       protocol.ProgramStatePlanning,
					}
					created = true
				} else {
					return fmt.Errorf("import-impls: failed to parse program manifest: %w", err)
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
				implDoc, loadErr := protocol.Load(implPath)
				if loadErr != nil {
					fmt.Fprintf(os.Stderr, "import-impls: warning: failed to load %s: %v\n", implPath, loadErr)
					continue
				}

				slug := implDoc.FeatureSlug
				if slug == "" {
					// Derive slug from filename: IMPL-<slug>.yaml
					base := filepath.Base(implPath)
					slug = strings.TrimPrefix(base, "IMPL-")
					slug = strings.TrimSuffix(slug, ".yaml")
				}

				// Skip if already in manifest
				if existingSlugs[slug] {
					continue
				}

				// Map IMPL state to program status
				status := mapIMPLStateToStatus(implDoc.State)

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

			// Compute tier assignments based on file ownership overlap
			// IMPLs that share files with other IMPLs get assigned to later tiers
			tierAssignments := computeTierAssignments(imported, fileOwners)

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
			importErrs := protocol.ValidateProgramImportMode(manifest, repoDir)
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
			data, err := yaml.Marshal(manifest)
			if err != nil {
				return fmt.Errorf("import-impls: failed to marshal manifest: %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(programPath), 0755); err != nil {
				return fmt.Errorf("import-impls: failed to create directory: %w", err)
			}
			if err := os.WriteFile(programPath, data, 0644); err != nil {
				return fmt.Errorf("import-impls: failed to write manifest: %w", err)
			}

			// Build result
			result := protocol.ImportIMPLsData{
				ManifestPath:    programPath,
				ImplsImported:   imported,
				TierAssignments: tierAssignments,
				P1Conflicts:     p1Conflicts,
				P2Conflicts:     p2Conflicts,
				Created:         created,
				Updated:         !created,
			}
			if discover {
				result.ImplsDiscovered = discoveredPaths
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	cmd.Flags().StringVar(&programPath, "program", "", "Path to PROGRAM manifest (created if missing)")
	cmd.Flags().StringSliceVar(&fromImpls, "from-impls", nil, "Explicit IMPL doc paths to import")
	cmd.Flags().BoolVar(&discover, "discover", false, "Auto-discover IMPL docs in docs/IMPL/")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository root directory (default: cwd)")

	return cmd
}

// mapIMPLStateToStatus maps an IMPL doc ProtocolState to a program IMPL status.
func mapIMPLStateToStatus(state protocol.ProtocolState) string {
	switch state {
	case protocol.StateComplete:
		return "complete"
	case protocol.StateReviewed, protocol.StateScaffoldPending,
		protocol.StateWavePending, protocol.StateWaveExecuting,
		protocol.StateWaveMerging, protocol.StateWaveVerified:
		return "reviewed"
	case protocol.StateScoutPending, protocol.StateScoutValidating:
		return "pending"
	default:
		return "pending"
	}
}

// computeTierAssignments analyzes file ownership overlap to suggest tier assignments.
// IMPLs with no overlap go to tier 1. IMPLs that overlap with tier-1 IMPLs go to tier 2, etc.
func computeTierAssignments(imported []protocol.ImportedIMPL, fileOwners map[string][]string) map[string]int {
	assignments := make(map[string]int)

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

	// Simple greedy tier assignment: assign each IMPL to the earliest tier
	// where it doesn't overlap with any already-assigned IMPL in that tier.
	tierMembers := make(map[int][]string) // tier -> slugs assigned to it

	for _, imp := range imported {
		slug := imp.Slug
		assigned := false
		for tier := 1; tier <= len(imported); tier++ {
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
