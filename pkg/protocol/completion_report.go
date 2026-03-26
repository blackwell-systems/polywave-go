package protocol

import (
	"fmt"
	"strings"
	"time"
)

// CompletionReportBuilder constructs and validates a CompletionReport before
// writing it to a manifest. Not thread-safe; use one builder per goroutine.
// The consolidated lock is in WithCompletionReportLock (manifest.go); the builder
// itself does not acquire any lock.
type CompletionReportBuilder struct {
	report  CompletionReport
	agentID string
}

// NewCompletionReport creates a new builder for the given agent ID.
func NewCompletionReport(agentID string) *CompletionReportBuilder {
	return &CompletionReportBuilder{agentID: agentID}
}

func (b *CompletionReportBuilder) WithStatus(status string) *CompletionReportBuilder {
	b.report.Status = status
	return b
}

func (b *CompletionReportBuilder) WithCommit(sha string) *CompletionReportBuilder {
	b.report.Commit = sha
	return b
}

func (b *CompletionReportBuilder) WithFiles(changed, created []string) *CompletionReportBuilder {
	b.report.FilesChanged = changed
	b.report.FilesCreated = created
	return b
}

func (b *CompletionReportBuilder) WithVerification(result string) *CompletionReportBuilder {
	b.report.Verification = result
	return b
}

func (b *CompletionReportBuilder) WithFailureType(ft string) *CompletionReportBuilder {
	b.report.FailureType = ft
	return b
}

func (b *CompletionReportBuilder) WithWorktree(path string) *CompletionReportBuilder {
	b.report.Worktree = path
	return b
}

func (b *CompletionReportBuilder) WithBranch(name string) *CompletionReportBuilder {
	b.report.Branch = name
	return b
}

func (b *CompletionReportBuilder) WithRepo(path string) *CompletionReportBuilder {
	b.report.Repo = path
	return b
}

func (b *CompletionReportBuilder) WithTestsAdded(tests []string) *CompletionReportBuilder {
	b.report.TestsAdded = tests
	return b
}

func (b *CompletionReportBuilder) WithNotes(text string) *CompletionReportBuilder {
	b.report.Notes = text
	return b
}

func (b *CompletionReportBuilder) WithDedupStats(stats *DedupStats) *CompletionReportBuilder {
	b.report.DedupStats = stats
	return b
}

func (b *CompletionReportBuilder) WithInterfaceDeviations(devs []InterfaceDeviation) *CompletionReportBuilder {
	b.report.InterfaceDeviations = devs
	return b
}

// Validate checks semantic consistency rules before any manifest mutation.
// Rules enforced:
//   - status must be "complete", "partial", or "blocked"
//   - commit must be non-empty when status is "complete"
//   - failure_type must be set when status is "blocked" or "partial"
//   - failure_type must NOT be set when status is "complete"
//   - failure_type value must be a valid FailureTypeEnum (uses ValidFailureType)
func (b *CompletionReportBuilder) Validate() error {
	var errs []string

	switch b.report.Status {
	case "complete", "partial", "blocked":
		// valid
	case "":
		errs = append(errs, "status is required")
	default:
		errs = append(errs, fmt.Sprintf("status %q is invalid: must be complete, partial, or blocked", b.report.Status))
	}

	if b.report.Status == "complete" && b.report.Commit == "" {
		errs = append(errs, "commit is required when status is complete")
	}

	if (b.report.Status == "blocked" || b.report.Status == "partial") && b.report.FailureType == "" {
		errs = append(errs, fmt.Sprintf("failure_type is required when status is %q", b.report.Status))
	}

	if b.report.Status == "complete" && b.report.FailureType != "" {
		errs = append(errs, "failure_type must not be set when status is complete")
	}

	if b.report.FailureType != "" && !ValidFailureType(b.report.FailureType) {
		errs = append(errs, fmt.Sprintf("failure_type %q is invalid", b.report.FailureType))
	}

	if len(errs) > 0 {
		return fmt.Errorf("completion report validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// AppendToManifest validates the report, sets WrittenAt to time.Now(), and writes
// it into manifest.CompletionReports[agentID] in-memory.
// Does NOT call Save(); callers inside WithCompletionReportLock control persistence.
// Returns Validate() errors if validation fails.
// Returns ErrAgentNotFound if agentID is not in the manifest's wave list.
func (b *CompletionReportBuilder) AppendToManifest(manifest *IMPLManifest) error {
	if err := b.Validate(); err != nil {
		return err
	}
	now := time.Now()
	b.report.WrittenAt = &now
	return SetCompletionReport(manifest, b.agentID, b.report)
}
