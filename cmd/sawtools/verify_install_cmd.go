package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"

	"github.com/spf13/cobra"
)

// installCheck represents a single prerequisite check result.
type installCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "fail", "warn", "skip"
	Detail string `json:"detail"`
}

// installResult is the top-level JSON output for verify-install.
type installResult struct {
	Checks  []installCheck `json:"checks"`
	Verdict string         `json:"verdict"` // "PASS", "PARTIAL", "FAIL"
	Summary string         `json:"summary"`
}

// newVerifyInstallCmd returns a cobra.Command for "sawtools verify-install".
// Checks:
//  1. sawtools binary is on PATH and executable
//  2. Git version >= 2.20 (worktree support)
//  3. ~/.claude/skills/saw/ directory exists with expected symlinks
//  4. saw.config.json exists and has valid repos entries
//  5. All configured repo paths exist on disk
//
// Output: JSON with per-check pass/fail + overall verdict.
func newVerifyInstallCmd() *cobra.Command {
	var humanFlag bool

	cmd := &cobra.Command{
		Use:   "verify-install",
		Short: "Check that all SAW prerequisites are met",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := runInstallChecks()

			if humanFlag {
				printHumanOutput(cmd, result)
				return nil
			}

			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("verify-install: marshal: %w", err)
			}
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVar(&humanFlag, "human", false, "Print human-readable output instead of JSON")

	return cmd
}

// runInstallChecks executes all prerequisite checks and returns the result.
func runInstallChecks() installResult {
	var checks []installCheck

	// 1. sawtools binary
	checks = append(checks, checkSawtoolsBinary())

	// 2. Git version
	checks = append(checks, checkGitVersion())

	// 3. Skill directory
	skillDirCheck := checkSkillDirectory()
	checks = append(checks, skillDirCheck)

	// 4. Skill files (skip if directory missing)
	if skillDirCheck.Status == "fail" {
		checks = append(checks, installCheck{
			Name:   "skill_files",
			Status: "skip",
			Detail: "skipped: skill directory missing",
		})
	} else {
		checks = append(checks, checkSkillFiles())
	}

	// 5. Config file
	configCheck, configPath := checkConfigFile()
	checks = append(checks, configCheck)

	// 6. Configured repos (skip if no config)
	if configCheck.Status == "fail" || configCheck.Status == "skip" {
		checks = append(checks, installCheck{
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

	return installResult{
		Checks:  checks,
		Verdict: verdict,
		Summary: summary,
	}
}

func checkSawtoolsBinary() installCheck {
	path, err := os.Executable()
	if err != nil {
		return installCheck{
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
	return installCheck{
		Name:   "sawtools_binary",
		Status: "pass",
		Detail: fmt.Sprintf("at %s", resolved),
	}
}

func checkGitVersion() installCheck {
	versionStr, err := git.Version()
	if err != nil {
		return installCheck{
			Name:   "git_version",
			Status: "fail",
			Detail: "git not found on PATH",
		}
	}
	// Parse "git version X.Y.Z ..." format
	re := regexp.MustCompile(`(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) < 3 {
		return installCheck{
			Name:   "git_version",
			Status: "warn",
			Detail: fmt.Sprintf("could not parse version from: %s", versionStr),
		}
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])

	if major > 2 || (major == 2 && minor >= 20) {
		return installCheck{
			Name:   "git_version",
			Status: "pass",
			Detail: fmt.Sprintf("%d.%d >= 2.20", major, minor),
		}
	}

	return installCheck{
		Name:   "git_version",
		Status: "fail",
		Detail: fmt.Sprintf("%d.%d < 2.20 (worktree support required)", major, minor),
		}
}

func checkSkillDirectory() installCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return installCheck{
			Name:   "skill_directory",
			Status: "fail",
			Detail: "cannot determine home directory",
		}
	}

	skillDir := filepath.Join(home, ".claude", "skills", "saw")
	if info, err := os.Stat(skillDir); err != nil || !info.IsDir() {
		return installCheck{
			Name:   "skill_directory",
			Status: "fail",
			Detail: fmt.Sprintf("%s not found", skillDir),
		}
	}

	return installCheck{
		Name:   "skill_directory",
		Status: "pass",
		Detail: fmt.Sprintf("%s exists", skillDir),
	}
}

func checkSkillFiles() installCheck {
	home, err := os.UserHomeDir()
	if err != nil {
		return installCheck{
			Name:   "skill_files",
			Status: "fail",
			Detail: "cannot determine home directory",
		}
	}

	skillDir := filepath.Join(home, ".claude", "skills", "saw")
	required := []string{"SKILL.md", "agent-template.md", "saw-bootstrap.md"}
	var missing []string

	for _, f := range required {
		p := filepath.Join(skillDir, f)
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return installCheck{
			Name:   "skill_files",
			Status: "pass",
			Detail: fmt.Sprintf("all %d skill files present", len(required)),
		}
	}

	return installCheck{
		Name:   "skill_files",
		Status: "fail",
		Detail: fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
	}
}

// checkConfigFile looks for saw.config.json in the current directory then ~/.claude/.
// Returns the check result and the path to the found config (empty if not found).
func checkConfigFile() (installCheck, string) {
	// Check current working directory first
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "saw.config.json"),
	}

	// Then check ~/.claude/
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".claude", "saw.config.json"))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return installCheck{
				Name:   "config_file",
				Status: "pass",
				Detail: fmt.Sprintf("found at %s", p),
			}, p
		}
	}

	return installCheck{
		Name:   "config_file",
		Status: "warn",
		Detail: "saw.config.json not found (checked project root and ~/.claude/)",
	}, ""
}

// checkConfiguredRepos reads saw.config.json and verifies each repo path exists.
func checkConfiguredRepos(configPath string) installCheck {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return installCheck{
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
		return installCheck{
			Name:   "configured_repos",
			Status: "warn",
			Detail: fmt.Sprintf("could not parse %s: %v", configPath, err),
		}
	}

	if len(config.Repos) == 0 {
		return installCheck{
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
		return installCheck{
			Name:   "configured_repos",
			Status: "pass",
			Detail: fmt.Sprintf("all %d repos found on disk", total),
		}
	}

	if found == 0 {
		return installCheck{
			Name:   "configured_repos",
			Status: "fail",
			Detail: fmt.Sprintf("0/%d repos found on disk: %s", total, strings.Join(missingRepos, ", ")),
		}
	}

	return installCheck{
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

// printHumanOutput renders the install result in a human-readable format.
func printHumanOutput(cmd *cobra.Command, result installResult) {
	for _, c := range result.Checks {
		var icon string
		switch c.Status {
		case "pass":
			icon = "[OK]"
		case "fail":
			icon = "[FAIL]"
		case "warn":
			icon = "[WARN]"
		case "skip":
			icon = "[SKIP]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s: %s\n", icon, c.Name, c.Detail)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nVerdict: %s\n", result.Verdict)
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", result.Summary)
}
