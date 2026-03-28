package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// ResolveImplData is the structured JSON output for resolve-impl command.
// Used by orchestrator to parse IMPL targeting flags and auto-select logic.
type ResolveImplData struct {
	ImplPath         string `json:"impl_path"`          // Absolute path to IMPL doc
	Slug             string `json:"slug"`               // Feature slug
	ResolutionMethod string `json:"resolution_method"`  // "auto-select" | "explicit-slug" | "explicit-filename" | "explicit-path"
	PendingCount     int    `json:"pending_count"`      // Number of pending IMPLs found (for diagnostics)
}

// newResolveImplCmd returns a cobra.Command that implements IMPL doc resolution logic.
// Parses --impl flag (slug, filename, or path) and auto-selects if exactly 1 pending IMPL exists.
// Always returns JSON (ResolveImplData) on success.
func newResolveImplCmd() *cobra.Command {
	var implFlag string

	cmd := &cobra.Command{
		Use:   "resolve-impl",
		Short: "Resolve IMPL doc path from slug, filename, or auto-select",
		Long: `Resolves --impl flag value to an absolute IMPL doc path.

Resolution order:
  1. If --impl is absolute path and file exists → use directly
  2. If --impl is relative path → resolve from cwd, verify exists
  3. If --impl is filename (IMPL-*.yaml) → resolve to <repo-dir>/docs/IMPL/<filename>
  4. If --impl is slug → scan for matching feature_slug in pending IMPLs
  5. If --impl omitted → auto-select if exactly 1 pending IMPL exists

Returns JSON (ResolveImplData) on success, exits 1 with error message on failure.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var result ResolveImplData

			// Determine effective repo directory for scanning
			effectiveRepoDir := repoDir
			if effectiveRepoDir == "" {
				effectiveRepoDir = "."
			}

			implDocsDir := filepath.Join(effectiveRepoDir, "docs", "IMPL")

			// Case 1: --impl flag omitted → auto-select
			if implFlag == "" {
				return autoSelectIMPL(cmd, implDocsDir)
			}

			// Case 2: Absolute path
			if filepath.IsAbs(implFlag) {
				if _, err := os.Stat(implFlag); err == nil {
					// Load to extract slug
					manifest, err := protocol.Load(implFlag)
					if err != nil {
						return fmt.Errorf("resolve-impl: failed to load IMPL at absolute path %s: %w", implFlag, err)
					}
					result = ResolveImplData{
						ImplPath:         implFlag,
						Slug:             manifest.FeatureSlug,
						ResolutionMethod: "explicit-path",
						PendingCount:     0, // Not applicable for explicit path
					}
					return outputResolveJSON(cmd, result)
				}
				return fmt.Errorf("resolve-impl: absolute path does not exist: %s", implFlag)
			}

			// Case 3: Relative path (contains path separators)
			if filepath.Dir(implFlag) != "." {
				absPath, err := filepath.Abs(implFlag)
				if err != nil {
					return fmt.Errorf("resolve-impl: failed to resolve relative path %s: %w", implFlag, err)
				}
				if _, err := os.Stat(absPath); err == nil {
					// Load to extract slug
					manifest, err := protocol.Load(absPath)
					if err != nil {
						return fmt.Errorf("resolve-impl: failed to load IMPL at relative path %s: %w", implFlag, err)
					}
					result = ResolveImplData{
						ImplPath:         absPath,
						Slug:             manifest.FeatureSlug,
						ResolutionMethod: "explicit-path",
						PendingCount:     0, // Not applicable for explicit path
					}
					return outputResolveJSON(cmd, result)
				}
				return fmt.Errorf("resolve-impl: relative path does not exist: %s (resolved to %s)", implFlag, absPath)
			}

			// Case 4: Filename (IMPL-*.yaml)
			if filepath.Ext(implFlag) == ".yaml" || filepath.Ext(implFlag) == ".yml" {
				candidatePath := filepath.Join(implDocsDir, implFlag)
				if _, err := os.Stat(candidatePath); err == nil {
					// Load to extract slug
					manifest, err := protocol.Load(candidatePath)
					if err != nil {
						return fmt.Errorf("resolve-impl: failed to load IMPL at filename %s: %w", implFlag, err)
					}
					result = ResolveImplData{
						ImplPath:         candidatePath,
						Slug:             manifest.FeatureSlug,
						ResolutionMethod: "explicit-filename",
						PendingCount:     0, // Not applicable for explicit filename
					}
					return outputResolveJSON(cmd, result)
				}
				return fmt.Errorf("resolve-impl: filename not found in %s: %s", implDocsDir, implFlag)
			}

			// Case 5: Slug → scan pending IMPLs
			return resolveBySlug(cmd, implDocsDir, implFlag)
		},
	}

	cmd.Flags().StringVar(&implFlag, "impl", "", "IMPL slug, filename, or path (omit to auto-select)")

	return cmd
}

// autoSelectIMPL scans for pending IMPLs and auto-selects if exactly 1 exists.
func autoSelectIMPL(cmd *cobra.Command, implDocsDir string) error {
	res := protocol.ListIMPLs(implDocsDir, protocol.ListIMPLsOpts{
		IncludeComplete: false,
	})
	if res.IsFatal() {
		return fmt.Errorf("resolve-impl: failed to list IMPLs: %s", res.Errors[0].Message)
	}

	data := res.GetData()
	pending := data.IMPLs

	if len(pending) == 0 {
		return fmt.Errorf("resolve-impl: no pending IMPLs found (auto-select requires exactly 1)")
	}
	if len(pending) > 1 {
		return fmt.Errorf("resolve-impl: multiple pending IMPLs found (%d), cannot auto-select. Use --impl to specify.", len(pending))
	}

	// Exactly 1 pending IMPL → auto-select
	impl := pending[0]
	result := ResolveImplData{
		ImplPath:         impl.Path,
		Slug:             impl.FeatureSlug,
		ResolutionMethod: "auto-select",
		PendingCount:     1,
	}
	return outputResolveJSON(cmd, result)
}

// resolveBySlug scans pending IMPLs and matches by feature_slug.
func resolveBySlug(cmd *cobra.Command, implDocsDir, slug string) error {
	res := protocol.ListIMPLs(implDocsDir, protocol.ListIMPLsOpts{
		IncludeComplete: false,
	})
	if res.IsFatal() {
		return fmt.Errorf("resolve-impl: failed to list IMPLs: %s", res.Errors[0].Message)
	}

	data := res.GetData()
	pending := data.IMPLs

	// Find matching slug
	for _, impl := range pending {
		if impl.FeatureSlug == slug {
			result := ResolveImplData{
				ImplPath:         impl.Path,
				Slug:             impl.FeatureSlug,
				ResolutionMethod: "explicit-slug",
				PendingCount:     len(pending),
			}
			return outputResolveJSON(cmd, result)
		}
	}

	// No match found
	return fmt.Errorf("resolve-impl: no pending IMPL found with slug '%s' (searched %d pending IMPLs)", slug, len(pending))
}

// outputResolveJSON marshals result to JSON and prints to stdout.
// cmd parameter allows writing to test buffers via SetOut.
func outputResolveJSON(cmd *cobra.Command, data ResolveImplData) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("resolve-impl: failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}
