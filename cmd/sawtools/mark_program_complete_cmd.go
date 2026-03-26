package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// MarkProgramCompleteResult is the JSON output of the mark-program-complete command.
type MarkProgramCompleteResult struct {
	Completed      bool   `json:"completed"`
	ProgramSlug    string `json:"program_slug"`
	Date           string `json:"date"`
	ManifestPath   string `json:"manifest_path"`
	ContextUpdated bool   `json:"context_updated"`
	ContextPath    string `json:"context_path,omitempty"`
	ArchivedPath   string `json:"archived_path,omitempty"`
	CommitSHA      string `json:"commit_sha,omitempty"`
	TiersComplete  int    `json:"tiers_complete"`
	ImplsComplete  int    `json:"impls_complete"`
}

func newMarkProgramCompleteCmd() *cobra.Command {
	var (
		date    string
		repoDir string
	)

	cmd := &cobra.Command{
		Use:   "mark-program-complete <program-manifest>",
		Short: "Mark a PROGRAM manifest as complete and update CONTEXT.md",
		Long: `Mark a PROGRAM manifest as complete.

Verifies all tiers are complete, updates the manifest state to PROGRAM_COMPLETE,
sets the completion_date, writes the SAW:PROGRAM:COMPLETE marker, updates CONTEXT.md,
and commits both files.

Exit codes:
  0 - Success
  1 - Not all tiers complete
  2 - Parse error or other failure`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Default date to today if not provided
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			// Default repoDir to current working directory
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("mark-program-complete: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("mark-program-complete: parse error: %w", err)
			}

			// Verify all tiers are complete (all IMPLs have status "complete")
			if err := verifyAllTiersComplete(manifest); err != nil {
				return fmt.Errorf("mark-program-complete: %w", err)
			}

			// Count tiers and impls for reporting
			tiersCount := len(manifest.Tiers)
			implsCount := len(manifest.Impls)

			// Update the manifest file: set state, completion_date, and marker
			if err := writeProgramCompleteMarker(manifestPath, date); err != nil {
				return fmt.Errorf("mark-program-complete: failed to update manifest: %w", err)
			}

			// Step 2: Archive to docs/PROGRAM/complete/
			archivedPath, archiveErr := protocol.ArchiveProgram(manifestPath)
			if archiveErr != nil {
				fmt.Fprintf(os.Stderr, "mark-program-complete: archive warning: %v\n", archiveErr)
				archivedPath = manifestPath // fall back to original path
			} else {
				fmt.Fprintf(os.Stderr, "mark-program-complete: archived to %s\n", archivedPath)
			}

			// Step 3: Update CONTEXT.md in repoDir/docs/
			contextPath, err := updateContextForProgram(manifest, repoDir, date, tiersCount, implsCount)
			if err != nil {
				// Non-fatal: log but continue
				fmt.Fprintf(os.Stderr, "mark-program-complete: warning: failed to update CONTEXT.md: %v\n", err)
			}

			// Commit files (use archived path if available)
			commitManifestPath := archivedPath
			commitSHA, err := commitProgramComplete(repoDir, commitManifestPath, contextPath, manifest.ProgramSlug)
			if err != nil {
				fmt.Fprintf(os.Stderr, "mark-program-complete: warning: failed to commit: %v\n", err)
			}

			result := MarkProgramCompleteResult{
				Completed:      true,
				ProgramSlug:    manifest.ProgramSlug,
				Date:           date,
				ManifestPath:   manifestPath,
				ArchivedPath:   archivedPath,
				ContextUpdated: contextPath != "",
				ContextPath:    contextPath,
				CommitSHA:      commitSHA,
				TiersComplete:  tiersCount,
				ImplsComplete:  implsCount,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository directory (default: current directory)")

	return cmd
}

// verifyAllTiersComplete checks that all IMPLs in all tiers have status "complete".
// Returns an error listing incomplete IMPLs if any are found.
func verifyAllTiersComplete(manifest *protocol.PROGRAMManifest) error {
	// Build a map of impl slug -> status
	implStatus := make(map[string]string)
	for _, impl := range manifest.Impls {
		implStatus[impl.Slug] = impl.Status
	}

	var incomplete []string
	for _, tier := range manifest.Tiers {
		for _, implSlug := range tier.Impls {
			status, ok := implStatus[implSlug]
			if !ok || status != "complete" {
				incomplete = append(incomplete, fmt.Sprintf("%s (status: %s)", implSlug, status))
			}
		}
	}

	if len(incomplete) > 0 {
		return fmt.Errorf("not all tiers complete — incomplete IMPLs: %s", strings.Join(incomplete, ", "))
	}
	return nil
}

// writeProgramCompleteMarker updates the manifest YAML file to set:
// - state: COMPLETE
// - completion_date: <date>
// - appends SAW:PROGRAM:COMPLETE marker at end
func writeProgramCompleteMarker(manifestPath, date string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("cannot read manifest: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Update or insert state field
	stateUpdated := false
	completionDateUpdated := false
	for i, line := range lines {
		if strings.HasPrefix(line, "state:") {
			lines[i] = "state: COMPLETE"
			stateUpdated = true
		}
		if strings.HasPrefix(line, "completion_date:") {
			lines[i] = fmt.Sprintf("completion_date: %q", date)
			completionDateUpdated = true
		}
	}

	// If state wasn't found, insert it near top
	if !stateUpdated {
		lines = append([]string{"state: COMPLETE"}, lines...)
	}

	// If completion_date wasn't found, insert after state
	if !completionDateUpdated {
		newLines := make([]string, 0, len(lines)+1)
		for _, line := range lines {
			newLines = append(newLines, line)
			if strings.HasPrefix(line, "state:") {
				newLines = append(newLines, fmt.Sprintf("completion_date: %q", date))
			}
		}
		lines = newLines
	}

	// Remove existing SAW:PROGRAM:COMPLETE marker if present, then re-append
	var filtered []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "SAW:PROGRAM:COMPLETE" {
			filtered = append(filtered, line)
		}
	}

	// Trim trailing blank lines, then add marker
	for len(filtered) > 0 && strings.TrimSpace(filtered[len(filtered)-1]) == "" {
		filtered = filtered[:len(filtered)-1]
	}
	filtered = append(filtered, "", "SAW:PROGRAM:COMPLETE", "")

	content := strings.Join(filtered, "\n")
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write manifest: %w", err)
	}
	return nil
}

// updateContextForProgram appends a program-level completion entry to CONTEXT.md.
// Entry format: "Program: <title> (<slug>) — <N> tiers, <M> IMPLs, <date>"
// Returns the path to the CONTEXT.md file, or empty string if update failed.
func updateContextForProgram(manifest *protocol.PROGRAMManifest, repoDir, date string, tiersCount, implsCount int) (string, error) {
	contextPath := protocol.ContextMDPath(repoDir)

	docsDir := filepath.Dir(contextPath)
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create docs directory: %w", err)
	}

	var content string
	data, err := os.ReadFile(contextPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to read CONTEXT.md: %w", err)
		}
		content = "# Project Context\n\n## Features Completed\n"
	} else {
		content = string(data)
	}

	entry := fmt.Sprintf("- Program: %s (%s) — %d tiers, %d IMPLs, %s\n",
		manifest.Title,
		manifest.ProgramSlug,
		tiersCount,
		implsCount,
		date,
	)

	// Append to features_completed section if it exists, otherwise just append
	if strings.Contains(content, "## Features Completed") {
		// Find insertion point after the section header
		idx := strings.Index(content, "## Features Completed")
		insertAfter := idx + len("## Features Completed")
		// Find end of that line
		if nl := strings.Index(content[insertAfter:], "\n"); nl != -1 {
			insertAfter += nl + 1
		}
		content = content[:insertAfter] + entry + content[insertAfter:]
	} else {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n## Features Completed\n" + entry
	}

	if err := os.WriteFile(contextPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write CONTEXT.md: %w", err)
	}

	return contextPath, nil
}

// commitProgramComplete stages and commits the manifest and context files.
// Returns the commit SHA on success.
func commitProgramComplete(repoDir, manifestPath, contextPath, programSlug string) (string, error) {
	// Stage the archived manifest (new location) and any deletion of old location
	// Use "git add -A docs/PROGRAM" to catch both the new file and the removal
	if err := git.Add(repoDir, manifestPath); err != nil {
		return "", fmt.Errorf("git add manifest failed: %w", err)
	}

	// Stage the deletion of the original location (if it was moved)
	// git add -u stages deletions of tracked files
	_ = git.AddUpdate(repoDir, "docs/") // Non-fatal — original may not have been tracked yet

	// Stage context if it was updated
	if contextPath != "" {
		if err := git.Add(repoDir, contextPath); err != nil {
			return "", fmt.Errorf("git add context failed: %w", err)
		}
	}

	// Commit
	commitMsg := fmt.Sprintf("chore: mark PROGRAM %s complete", programSlug)
	if _, err := git.CommitWithMessage(repoDir, commitMsg); err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	// Get commit SHA
	sha, err := git.RevParse(repoDir, "HEAD")
	if err != nil {
		return "", nil // Non-fatal
	}
	return sha, nil
}
