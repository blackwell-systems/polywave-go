// Package git provides git polling utilities for the SAW web UI.
// activity.go implements a Poller that snapshots branch and commit activity
// across all active agent worktrees.
package git

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	igit "github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

var agentBranchRe = regexp.MustCompile(`^wave(\d+)-agent-([a-z])$`)

// Commit is a single git commit summary.
type Commit struct {
	SHA          string    `json:"sha"`
	Message      string    `json:"message"`
	Author       string    `json:"author"`
	Timestamp    time.Time `json:"timestamp"`
	FilesChanged int       `json:"files_changed"`
}

// Branch describes a SAW agent branch and its commits.
type Branch struct {
	Name        string   `json:"name"`
	Agent       string   `json:"agent"`
	Wave        int      `json:"wave"`
	Commits     []Commit `json:"commits"`
	Merged      bool     `json:"merged"`
	MergeCommit string   `json:"merge_commit,omitempty"`
	Status      string   `json:"status"`
}

// ActivitySnapshot is a point-in-time view of git activity across all agent
// branches for a given slug/repo.
type ActivitySnapshot struct {
	Slug        string    `json:"slug"`
	MainCommits []Commit  `json:"main_commits"`
	Branches    []Branch  `json:"branches"`
	PolledAt    time.Time `json:"polled_at"`
}

// Poller polls a git repository for SAW agent branch activity.
type Poller struct {
	repoPath string
	slug     string
}

// NewPoller creates a Poller for the repository at repoPath tagged with slug.
func NewPoller(repoPath, slug string) *Poller {
	return &Poller{repoPath: repoPath, slug: slug}
}

// Snapshot returns a current ActivitySnapshot for the repository. If
// WorktreeList returns no non-main worktrees, an empty snapshot is returned
// with no error.
func (p *Poller) Snapshot() (ActivitySnapshot, error) {
	snap := ActivitySnapshot{
		Slug:        p.slug,
		PolledAt:    time.Now().UTC(),
		MainCommits: []Commit{},
		Branches:    []Branch{},
	}

	worktrees, err := igit.WorktreeList(p.repoPath)
	if err != nil {
		return snap, fmt.Errorf("activity snapshot: %w", err)
	}

	if len(worktrees) == 0 {
		return snap, nil
	}

	// Determine which branches have been merged into main.
	mergedSet := p.mergedBranches()

	// Get the latest SHA on main (used as merge commit ref for merged branches).
	latestMainSHA, _ := igit.RevParse(p.repoPath, "main")

	// Populate main commits.
	snap.MainCommits = p.logCommits([]string{"main", "-20"})

	// Populate agent branches.
	var branches []Branch
	for _, wt := range worktrees {
		branchName := wt[1]
		m := agentBranchRe.FindStringSubmatch(branchName)
		if m == nil {
			continue
		}
		wave, _ := strconv.Atoi(m[1])
		agent := strings.ToUpper(m[2])

		commits := p.logCommits([]string{branchName, "^main"})

		merged := mergedSet[branchName]
		mergeCommit := ""
		if merged {
			mergeCommit = strings.TrimSpace(latestMainSHA)
		}

		branches = append(branches, Branch{
			Name:        branchName,
			Agent:       agent,
			Wave:        wave,
			Commits:     commits,
			Merged:      merged,
			MergeCommit: mergeCommit,
			Status:      "running",
		})
	}

	if branches != nil {
		snap.Branches = branches
	}
	return snap, nil
}

// mergedBranches returns the set of branch names that have been merged into main.
func (p *Poller) mergedBranches() map[string]bool {
	out, err := igit.Run(p.repoPath, "branch", "--merged", "main")
	if err != nil {
		return map[string]bool{}
	}
	result := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "* "))
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// logCommits runs git log with the given extra args and returns parsed commits.
// On error, an empty (non-nil) slice is returned.
func (p *Poller) logCommits(args []string) []Commit {
	cmdArgs := append([]string{"log", "--format=%H%x00%an%x00%aI%x00%s"}, args...)
	out, err := igit.Run(p.repoPath, cmdArgs...)
	if err != nil {
		return []Commit{}
	}
	return parseCommits(out)
}

// parseCommits parses the output of git log with format %H%x00%an%x00%aI%x00%s.
func parseCommits(out string) []Commit {
	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		// Each line: <SHA>\x00<author>\x00<ISO timestamp>\x00<subject>
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) != 4 {
			continue
		}
		sha := parts[0]
		if len(sha) > 7 {
			sha = sha[:7]
		}
		ts, _ := time.Parse(time.RFC3339, parts[2])
		commits = append(commits, Commit{
			SHA:       sha,
			Author:    parts[1],
			Timestamp: ts,
			Message:   parts[3],
		})
	}
	if commits == nil {
		return []Commit{}
	}
	return commits
}
