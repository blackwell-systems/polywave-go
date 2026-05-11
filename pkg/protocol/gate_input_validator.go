package protocol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// GateInputValidationData captures whether the reported files in completion
// reports match the actual git-diff files for each agent in the wave.
type GateInputValidationData struct {
	Valid             bool     `json:"valid"`
	ReportedFiles     []string `json:"reported_files"`
	ActualFiles       []string `json:"actual_files"`
	MissingFromReport []string `json:"missing_from_report,omitempty"`
	ExtraInReport     []string `json:"extra_in_report,omitempty"`
}


// ValidateGateInputs verifies that the set of files reported by each agent's
// completion report matches the files actually changed in their worktree branch
// (via git diff). It collects reported files across all agents in the wave,
// retrieves the actual changed files via git diff, and flags discrepancies.
//
// For each agent in waveNum:
//   - Reported files are gathered from the CompletionReport (FilesChanged union FilesCreated).
//   - Actual files are obtained via `git diff --name-only <baseCommit>..<branch>`.
//   - If wave.BaseCommit is set it is used as the base; otherwise HEAD is used.
//
// Returns a fatal result if waveNum is not found in the manifest.
func ValidateGateInputs(manifest *IMPLManifest, waveNum int, repoDir string) result.Result[*GateInputValidationData] {
	// Find target wave
	targetWave := manifest.FindWave(waveNum)
	if targetWave == nil {
		return result.NewFailure[*GateInputValidationData]([]result.PolywaveError{{
			Code:     result.CodeGateInputInvalid,
			Message:  fmt.Sprintf("wave %d not found in manifest", waveNum),
			Severity: "fatal",
		}})
	}

	reportedSet := map[string]struct{}{}
	actualSet := map[string]struct{}{}

	for _, agent := range targetWave.Agents {
		// Collect reported files from completion report
		if report, ok := manifest.CompletionReports[agent.ID]; ok {
			for _, f := range report.FilesChanged {
				if f != "" {
					reportedSet[f] = struct{}{}
				}
			}
			for _, f := range report.FilesCreated {
				if f != "" {
					reportedSet[f] = struct{}{}
				}
			}
		}

		// Determine base reference
		baseRef := targetWave.BaseCommit
		if baseRef == "" {
			baseRef = "HEAD"
		}

		// Determine agent branch
		branchName := BranchName(manifest.FeatureSlug, waveNum, agent.ID)

		// Run git diff to get actual changed files
		actual, err := git.DiffNameOnly(repoDir, baseRef, branchName)
		if err != nil {
			// If the branch doesn't exist yet or diff fails, treat as empty
			actual = []string{}
		}
		for _, f := range actual {
			if f != "" {
				actualSet[f] = struct{}{}
			}
		}
	}

	// Flatten sets to sorted slices
	reported := setToSortedSlice(reportedSet)
	actual := setToSortedSlice(actualSet)

	// Compute discrepancies
	missingFromReport := []string{} // in actual but not in reported
	for f := range actualSet {
		if _, ok := reportedSet[f]; !ok {
			missingFromReport = append(missingFromReport, f)
		}
	}
	sort.Strings(missingFromReport)

	extraInReport := []string{} // in reported but not in actual
	for f := range reportedSet {
		if _, ok := actualSet[f]; !ok {
			extraInReport = append(extraInReport, f)
		}
	}
	sort.Strings(extraInReport)

	valid := len(missingFromReport) == 0 && len(extraInReport) == 0

	data := &GateInputValidationData{
		Valid:         valid,
		ReportedFiles: reported,
		ActualFiles:   actual,
	}
	if len(missingFromReport) > 0 {
		data.MissingFromReport = missingFromReport
	}
	if len(extraInReport) > 0 {
		data.ExtraInReport = extraInReport
	}

	return result.NewSuccess(data)
}

// setToSortedSlice converts a string set (map[string]struct{}) to a sorted
// slice, filtering out any whitespace-only entries.
func setToSortedSlice(s map[string]struct{}) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		if strings.TrimSpace(k) != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
