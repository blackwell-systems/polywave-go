package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// MarkProgramCompleteOpts consolidates all inputs for MarkProgramComplete.
type MarkProgramCompleteOpts struct {
	ManifestPath string       // absolute path to PROGRAM manifest (required)
	RepoDir      string       // absolute path to repository root (required)
	Date         string       // YYYY-MM-DD; empty = today
	Logger       *slog.Logger // optional
}

// MarkProgramCompleteResult is the structured result of mark-program-complete.
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

// MarkProgramComplete verifies all tiers complete, writes the SAW:PROGRAM:COMPLETE
// marker, archives the manifest, updates CONTEXT.md, and commits. Returns a
// structured result.
func MarkProgramComplete(ctx context.Context, opts MarkProgramCompleteOpts) (MarkProgramCompleteResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// 1. Default date to today if empty
	date := opts.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 2. Parse manifest
	manifest, err := protocol.ParseProgramManifest(opts.ManifestPath)
	if err != nil {
		return MarkProgramCompleteResult{}, fmt.Errorf("mark-program-complete: parse error: %w", err)
	}

	// 3. Verify all tiers complete — hard error if not
	if err := verifyAllTiersComplete(manifest); err != nil {
		return MarkProgramCompleteResult{}, fmt.Errorf("mark-program-complete: %w", err)
	}

	// 4. Count tiers and impls
	tiersCount := len(manifest.Tiers)
	implsCount := len(manifest.Impls)

	// 5. Write SAW:PROGRAM:COMPLETE marker
	if err := writeProgramCompleteMarker(opts.ManifestPath, date); err != nil {
		return MarkProgramCompleteResult{}, fmt.Errorf("mark-program-complete: failed to update manifest: %w", err)
	}

	// 6. Archive to docs/PROGRAM/complete/ — non-fatal
	archivedPath, archiveErr := protocol.ArchiveProgram(opts.ManifestPath)
	if archiveErr != nil {
		logger.Warn("mark-program-complete: archive warning", "error", archiveErr)
		archivedPath = opts.ManifestPath // fall back to original path
	} else {
		logger.Info("mark-program-complete: archived", "path", archivedPath)
	}

	// 7. Update CONTEXT.md — non-fatal
	contextPath, ctxErr := updateContextForProgram(manifest, opts.RepoDir, date, tiersCount, implsCount)
	if ctxErr != nil {
		logger.Warn("mark-program-complete: failed to update CONTEXT.md", "error", ctxErr)
	}

	// 8. Commit — non-fatal
	commitSHA, commitErr := commitProgramComplete(opts.RepoDir, archivedPath, contextPath, manifest.ProgramSlug)
	if commitErr != nil {
		logger.Warn("mark-program-complete: failed to commit", "error", commitErr)
	}

	// 9. Return populated result
	return MarkProgramCompleteResult{
		Completed:      true,
		ProgramSlug:    manifest.ProgramSlug,
		Date:           date,
		ManifestPath:   opts.ManifestPath,
		ArchivedPath:   archivedPath,
		ContextUpdated: contextPath != "",
		ContextPath:    contextPath,
		CommitSHA:      commitSHA,
		TiersComplete:  tiersCount,
		ImplsComplete:  implsCount,
	}, nil
}

// verifyAllTiersComplete checks that all IMPLs in all tiers have status "complete".
func verifyAllTiersComplete(manifest *protocol.PROGRAMManifest) error {
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

// writeProgramCompleteMarker updates the manifest YAML file to set state: COMPLETE,
// completion_date, and appends the SAW:PROGRAM:COMPLETE marker.
func writeProgramCompleteMarker(manifestPath, date string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("cannot read manifest: %w", err)
	}

	lines := strings.Split(string(data), "\n")

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

	if !stateUpdated {
		lines = append([]string{"state: COMPLETE"}, lines...)
	}

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

	if strings.Contains(content, "## Features Completed") {
		idx := strings.Index(content, "## Features Completed")
		insertAfter := idx + len("## Features Completed")
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
func commitProgramComplete(repoDir, manifestPath, contextPath, programSlug string) (string, error) {
	if err := git.Add(repoDir, manifestPath); err != nil {
		return "", fmt.Errorf("git add manifest failed: %w", err)
	}

	// Stage deletions of tracked files under docs/ (non-fatal)
	_ = git.AddUpdate(repoDir, "docs/")

	if contextPath != "" {
		if err := git.Add(repoDir, contextPath); err != nil {
			return "", fmt.Errorf("git add context failed: %w", err)
		}
	}

	commitMsg := fmt.Sprintf("chore: mark PROGRAM %s complete", programSlug)
	if _, err := git.CommitWithMessage(repoDir, commitMsg); err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	sha, err := git.RevParse(repoDir, "HEAD")
	if err != nil {
		return "", nil // Non-fatal
	}
	return sha, nil
}
