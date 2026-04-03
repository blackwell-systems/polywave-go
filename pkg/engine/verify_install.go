package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// InstallCheck represents a single prerequisite check result.
type InstallCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "fail", "warn", "skip"
	Detail string `json:"detail"`
}

// InstallResult is the top-level result for verify-install.
type InstallResult struct {
	Checks  []InstallCheck `json:"checks"`
	Verdict string         `json:"verdict"` // "PASS", "PARTIAL", "FAIL"
	Summary string         `json:"summary"`
}

// VerifyInstallOpts holds options for RunVerifyInstall.
// RepoPath is used to find saw.config.json; empty means use cwd.
type VerifyInstallOpts struct {
	RepoPath string // absolute path (for saw.config.json lookup); empty = cwd
}

// RunVerifyInstall runs all prerequisite checks and returns a structured result.
// It checks: sawtools binary, git version, skill directory, skill files,
// config file, and configured repos.
func RunVerifyInstall(opts VerifyInstallOpts) InstallResult {
	var checks []InstallCheck

	// 1. sawtools binary
	checks = append(checks, checkSawtoolsBinary())

	// 2. Git version
	checks = append(checks, checkGitVersion())

	// 3. Skill directory
	skillDirCheck := checkSkillDirectory()
	checks = append(checks, skillDirCheck)

	// 4. Skill files (skip if directory missing)
	if skillDirCheck.Status == "fail" {
		checks = append(checks, InstallCheck{
			Name:   "skill_files",
			Status: "skip",
			Detail: "skipped: skill directory missing",
		})
	} else {
		checks = append(checks, checkSkillFiles())
	}

	// 5. Config file
	configCheck, configPath := checkConfigFile(opts.RepoPath)
	checks = append(checks, configCheck)

	// 6. Configured repos (skip if no config)
	if configCheck.Status == "fail" || configCheck.Status == "skip" {
		checks = append(checks, InstallCheck{
			Name:   "configured_repos",
			Status: "skip",
			Detail: "skipped: no config file found",
		})
	} else {
		checks = append(checks, checkConfiguredRepos(configPath))
	}

	// Compute verdict
	var passed, failed, warned, skipped int
	for _, c := range checks {
		switch c.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "warn":
			warned++
		case "skip":
			skipped++
		}
	}

	verdict := "PASS"
	if failed > 0 {
		verdict = "FAIL"
	} else if warned > 0 {
		verdict = "PARTIAL"
	}

	parts := []string{}
	if passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", passed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if warned > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", warned))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	summary := strings.Join(parts, ", ")
	if failed > 0 {
		summary += ". Run install.sh to fix."
	}

	return InstallResult{
		Checks:  checks,
		Verdict: verdict,
		Summary: summary,
	}
}

func checkSawtoolsBinary() InstallCheck {
	path, err := os.Executable()
	if err != nil {
		return InstallCheck{
			Name:   "sawtools_binary",
			Status: "pass",
			Detail: "running (path unknown)",
		}
	}
	// Resolve symlinks for cleaner output
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}
	return InstallCheck{
		Name:   "sawtools_binary",
		Status: "pass",
		Detail: fmt.Sprintf("at %s", resolved),
	}
}

func checkGitVersion() InstallCheck {
	versionStr, err := git.Version()
	if err != nil {
		return InstallCheck{
			Name:   "git_version",
			Status: "fail",
			Detail: "git not found on PATH",
		}
	}
	// Parse "git version X.Y.Z ..." format
	re := regexp.MustCompile(`(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) < 3 {
		return InstallCheck{
			Name:   "git_version",
			Status: "warn",
			Detail: fmt.Sprintf("could not parse version from: %s", versionStr),
		}
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])

	if major > 2 || (major == 2 && minor >= 20) {
		return InstallCheck{
			Name:   "git_version",
			Status: "pass",
			Detail: fmt.Sprintf("%d.%d >= 2.20", major, minor),
		}
	}

	return InstallCheck{
		Name:   "git_version",
		Status: "fail",
		Detail: fmt.Sprintf("%d.%d < 2.20 (worktree support required)", major, minor),
	}
}

func checkSkillDirectory() InstallCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return InstallCheck{
			Name:   "skill_directory",
			Status: "fail",
			Detail: "cannot determine home directory",
		}
	}

	skillDir := filepath.Join(home, ".claude", "skills", "saw")
	if info, err := os.Stat(skillDir); err != nil || !info.IsDir() {
		return InstallCheck{
			Name:   "skill_directory",
			Status: "fail",
			Detail: fmt.Sprintf("%s not found", skillDir),
		}
	}

	return InstallCheck{
		Name:   "skill_directory",
		Status: "pass",
		Detail: fmt.Sprintf("%s exists", skillDir),
	}
}

func checkSkillFiles() InstallCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return InstallCheck{
			Name:   "skill_files",
			Status: "fail",
			Detail: "cannot determine home directory",
		}
	}

	skillDir := filepath.Join(home, ".claude", "skills", "saw")
	// Update this list when skill files are added or renamed.
	required := []string{"SKILL.md", "agent-template.md", "saw-bootstrap.md"}
	var missing []string

	for _, f := range required {
		p := filepath.Join(skillDir, f)
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return InstallCheck{
			Name:   "skill_files",
			Status: "pass",
			Detail: fmt.Sprintf("all %d skill files present", len(required)),
		}
	}

	return InstallCheck{
		Name:   "skill_files",
		Status: "fail",
		Detail: fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
	}
}

// checkConfigFile looks for saw.config.json in repoPath (or cwd if empty), then ~/.claude/.
// Returns the check result and the path to the found config (empty if not found).
func checkConfigFile(repoPath string) (InstallCheck, string) {
	// Determine base directory
	baseDir := repoPath
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}

	candidates := []string{
		filepath.Join(baseDir, "saw.config.json"),
	}

	// Then check ~/.claude/
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".claude", "saw.config.json"))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return InstallCheck{
				Name:   "config_file",
				Status: "pass",
				Detail: fmt.Sprintf("found at %s", p),
			}, p
		}
	}

	return InstallCheck{
		Name:   "config_file",
		Status: "warn",
		Detail: "saw.config.json not found (checked project root and ~/.claude/)",
	}, ""
}

// checkConfiguredRepos reads saw.config.json and verifies each repo path exists.
func checkConfiguredRepos(configPath string) InstallCheck {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return InstallCheck{
			Name:   "configured_repos",
			Status: "warn",
			Detail: fmt.Sprintf("could not read %s: %v", configPath, err),
		}
	}

	// Parse the config to find repo paths
	var config struct {
		Repos []struct {
			Path string `json:"path"`
		} `json:"repos"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return InstallCheck{
			Name:   "configured_repos",
			Status: "warn",
			Detail: fmt.Sprintf("could not parse %s: %v", configPath, err),
		}
	}

	if len(config.Repos) == 0 {
		return InstallCheck{
			Name:   "configured_repos",
			Status: "warn",
			Detail: "no repos configured in saw.config.json",
		}
	}

	var found, total int
	var missingRepos []string
	for _, r := range config.Repos {
		total++
		expanded := expandHome(r.Path)
		if _, err := os.Stat(expanded); err == nil {
			found++
		} else {
			missingRepos = append(missingRepos, r.Path)
		}
	}

	if found == total {
		return InstallCheck{
			Name:   "configured_repos",
			Status: "pass",
			Detail: fmt.Sprintf("all %d repos found on disk", total),
		}
	}

	if found == 0 {
		return InstallCheck{
			Name:   "configured_repos",
			Status: "fail",
			Detail: fmt.Sprintf("0/%d repos found on disk: %s", total, strings.Join(missingRepos, ", ")),
		}
	}

	return InstallCheck{
		Name:   "configured_repos",
		Status: "warn",
		Detail: fmt.Sprintf("%d/%d repos found on disk", found, total),
	}
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
