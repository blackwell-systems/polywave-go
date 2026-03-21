package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCreateProgramCmd() *cobra.Command {
	var (
		fromImpls []string
		slug      string
		title     string
	)

	cmd := &cobra.Command{
		Use:   "create-program",
		Short: "Auto-generate a PROGRAM manifest from existing IMPL docs",
		Long: `Auto-generate a PROGRAM manifest from existing IMPL docs.

Uses cross-IMPL conflict detection to automatically assign tiers so that
IMPLs with overlapping file ownership are placed in separate tiers.

Examples:
  # Generate a PROGRAM from two IMPL slugs
  sawtools create-program --from-impls feature-a --from-impls feature-b

  # With custom slug and title
  sawtools create-program --from-impls feature-a --from-impls feature-b --slug my-program --title "My Program"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(fromImpls) == 0 {
				return fmt.Errorf("create-program: --from-impls is required")
			}
			if len(fromImpls) < 2 {
				return fmt.Errorf("create-program: need at least 2 IMPL slugs (single IMPL doesn't need a PROGRAM)")
			}

			result, err := protocol.GenerateProgramFromIMPLs(protocol.GenerateProgramOpts{
				ImplSlugs:   fromImpls,
				RepoPath:    repoDir,
				ProgramSlug: slug,
				Title:       title,
			})
			if err != nil {
				return fmt.Errorf("create-program: %w", err)
			}

			// Print validation errors to stderr (non-fatal)
			for _, ve := range result.ValidationErrors {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", ve.Message)
			}

			// Print conflict summary to stderr if conflicts found
			if result.ConflictReport != nil && len(result.ConflictReport.Conflicts) > 0 {
				fmt.Fprintf(os.Stderr, "Note: %d file conflicts detected across IMPLs. IMPLs assigned to separate tiers.\n",
					len(result.ConflictReport.Conflicts))
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&fromImpls, "from-impls", nil, "IMPL slugs to include (required, at least 2)")
	cmd.Flags().StringVar(&slug, "slug", "", "Override program slug (auto-derived if empty)")
	cmd.Flags().StringVar(&title, "title", "", "Override program title (auto-derived if empty)")

	return cmd
}

func newCheckIMPLConflictsCmd() *cobra.Command {
	var impls []string

	cmd := &cobra.Command{
		Use:   "check-impl-conflicts",
		Short: "Check for file ownership conflicts across IMPL docs",
		Long: `Check for file ownership conflicts across IMPL docs.

Loads IMPL docs by slug, extracts file_ownership entries, and detects
overlapping files across IMPLs. Returns a structured JSON report.

Exit code 1 if conflicts found, 0 if all disjoint.

Examples:
  sawtools check-impl-conflicts --impls feature-a --impls feature-b
  sawtools check-impl-conflicts --impls feature-a --impls feature-b --repo-dir /path/to/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(impls) == 0 {
				return fmt.Errorf("check-impl-conflicts: --impls is required")
			}

			report, err := protocol.CheckIMPLConflicts(impls, repoDir)
			if err != nil {
				return fmt.Errorf("check-impl-conflicts: %w", err)
			}

			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if len(report.Conflicts) > 0 {
				// Print to stderr and return error so cobra sets exit code 1
				fmt.Fprintf(os.Stderr, "%d file ownership conflicts detected\n", len(report.Conflicts))
				return fmt.Errorf("conflicts detected")
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&impls, "impls", nil, "IMPL slugs to check for conflicts (required)")

	return cmd
}
