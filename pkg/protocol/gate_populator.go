package protocol

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// PopulateVerificationGates populates verification gate blocks for all agents
// in the manifest that don't already have them. Supports multi-repo IMPLs by
// accepting a map of repo paths to command sets. The repoMap parameter provides
// a mapping from relative repo names (as specified in file_ownership) to absolute
// paths (as used in commandSets keys). Returns a new manifest copy without
// modifying the input.
func PopulateVerificationGates(ctx context.Context, m *IMPLManifest, commandSets map[string]*commands.CommandSet, repoMap map[string]string) (*IMPLManifest, error) {
	if len(commandSets) == 0 {
		return nil, fmt.Errorf("H2 data unavailable - run extract-commands first")
	}

	// Build agent-to-repo mapping from file_ownership
	agentRepos := buildAgentRepoMap(m)

	// Create deep copy of manifest
	result := *m
	result.Waves = make([]Wave, len(m.Waves))
	copy(result.Waves, m.Waves)

	// Process each wave's agents
	for waveIdx := range result.Waves {
		wave := &result.Waves[waveIdx]
		wave.Agents = make([]Agent, len(m.Waves[waveIdx].Agents))
		copy(wave.Agents, m.Waves[waveIdx].Agents)

		for agentIdx := range wave.Agents {
			agent := &wave.Agents[agentIdx]

			// Skip if agent already has verification gate
			if strings.Contains(agent.Task, "## Verification Gate") {
				continue
			}

			// Determine which repo this agent belongs to
			repoKey := agentRepos[agent.ID]
			if repoKey == "" {
				repoKey = "." // Default to current directory for single-repo IMPLs
			}

			// Resolve relative repo path to absolute path
			absRepoPath := repoMap[repoKey]
			if absRepoPath == "" {
				absRepoPath = repoKey // Fallback if not in map
			}

			// Look up command set for this repo
			commandSet, ok := commandSets[absRepoPath]
			if !ok {
				// Skip this agent if we don't have H2 data for its repo
				continue
			}

			// Determine focused test pattern
			focusedTest := DetermineFocusedTestPattern(agent.Files, commandSet)

			// Format verification block
			verificationBlock := FormatVerificationBlock(
				commandSet.Commands.Build,
				commandSet.Commands.Lint.Check,
				focusedTest,
			)

			// Append to agent's task (preserve existing content)
			if agent.Task != "" && !strings.HasSuffix(agent.Task, "\n") {
				agent.Task += "\n"
			}
			agent.Task += verificationBlock
		}
	}

	return &result, nil
}

// buildAgentRepoMap creates a mapping from agent ID to repository path.
// Uses the file_ownership table to determine which repo each agent's files belong to.
// Repository paths are stored as-is (may be relative or absolute).
func buildAgentRepoMap(m *IMPLManifest) map[string]string {
	agentRepos := make(map[string]string)

	for _, ownership := range m.FileOwnership {
		// If this file has a repo specified, map the agent to that repo
		if ownership.Repo != "" {
			agentRepos[ownership.Agent] = ownership.Repo
		}
	}

	return agentRepos
}

// extractUniqueRepos returns a list of unique repository paths from the
// file_ownership table. If no repos are specified (single-repo IMPL), returns
// a list containing only the provided repoRoot. Resolves relative repo paths
// to absolute paths by looking in the parent directory of defaultRepo.
func extractUniqueRepos(m *IMPLManifest, defaultRepo string) []string {
	repoSet := make(map[string]bool)

	// Scan file_ownership for unique repo values
	for _, ownership := range m.FileOwnership {
		if ownership.Repo != "" {
			repoSet[ownership.Repo] = true
		}
	}

	// If no repos specified, use default (single-repo IMPL)
	if len(repoSet) == 0 {
		return []string{defaultRepo}
	}

	// Resolve relative paths to absolute paths
	// For multi-repo IMPLs, repos are typically siblings in a common parent directory
	parentDir := filepath.Dir(defaultRepo)

	repos := make([]string, 0, len(repoSet))
	for repo := range repoSet {
		absPath := repo
		if !filepath.IsAbs(repo) {
			// Try to resolve relative path by checking sibling directory
			candidatePath := filepath.Join(parentDir, repo)
			if _, err := filepath.Abs(candidatePath); err == nil {
				absPath = candidatePath
			}
		}
		repos = append(repos, absPath)
	}

	return repos
}

// DetermineFocusedTestPattern infers a focused test command from the agent's
// file list. Returns empty string if focused pattern cannot be determined
// (multiple packages, no files, etc).
func DetermineFocusedTestPattern(files []string, commandSet *commands.CommandSet) string {
	if len(files) == 0 {
		return commandSet.Commands.Test.Full
	}

	// Extract unique package paths from file list
	packages := extractPackages(files)

	// If multiple packages, cannot focus - use full suite
	if len(packages) > 1 {
		return commandSet.Commands.Test.Full
	}

	// If no focused pattern template available, use full suite
	if commandSet.Commands.Test.FocusedPattern == "" {
		return commandSet.Commands.Test.Full
	}

	// Single package - generate focused test command
	pkgPath := packages[0]

	// Apply focused pattern template based on toolchain
	switch commandSet.Toolchain {
	case "go":
		// Extract package name from path (e.g., "pkg/auth" -> "auth")
		pkgName := filepath.Base(pkgPath)
		// Capitalize first letter for test prefix
		testPrefix := "Test" + cases.Title(language.Und).String(pkgName)
		// Replace template placeholders
		pattern := strings.ReplaceAll(commandSet.Commands.Test.FocusedPattern, "{package}", pkgPath)
		pattern = strings.ReplaceAll(pattern, "{TestPrefix}", testPrefix)
		return pattern

	case "rust":
		// Extract module name from path
		moduleName := filepath.Base(pkgPath)
		return strings.ReplaceAll(commandSet.Commands.Test.FocusedPattern, "{module_name}", moduleName)

	case "javascript", "typescript":
		// Extract module name from path
		moduleName := filepath.Base(pkgPath)
		return strings.ReplaceAll(commandSet.Commands.Test.FocusedPattern, "{module}", moduleName)

	case "python":
		// Use full path for pytest
		return strings.ReplaceAll(commandSet.Commands.Test.FocusedPattern, "{path}", pkgPath)

	default:
		// Unknown toolchain - use full suite
		return commandSet.Commands.Test.Full
	}
}

// extractPackages extracts unique package/module paths from a list of file paths.
// For Go: "pkg/auth/handler.go" -> "pkg/auth"
// For Rust: "src/auth/mod.rs" -> "src/auth"
func extractPackages(files []string) []string {
	pkgMap := make(map[string]bool)

	for _, file := range files {
		// Get directory of file (package path)
		dir := filepath.Dir(file)
		if dir == "." {
			dir = ""
		}
		pkgMap[dir] = true
	}

	// Convert map to sorted slice for determinism
	packages := make([]string, 0, len(pkgMap))
	for pkg := range pkgMap {
		packages = append(packages, pkg)
	}

	return packages
}

// FormatVerificationBlock formats build/lint/test commands into a markdown
// verification block string that will be appended to an agent's task field.
func FormatVerificationBlock(buildCmd, lintCmd, testCmd string) string {
	var sb strings.Builder

	sb.WriteString("\n## Verification Gate\n\n")
	sb.WriteString("Run these commands to verify your implementation:\n\n")
	sb.WriteString("```bash\n")

	if buildCmd != "" {
		sb.WriteString(buildCmd)
		sb.WriteString("\n")
	}

	if lintCmd != "" {
		sb.WriteString(lintCmd)
		sb.WriteString("\n")
	}

	if testCmd != "" {
		sb.WriteString(testCmd)
		sb.WriteString("\n")
	}

	sb.WriteString("```\n")

	return sb.String()
}

// FinalizeIMPLData contains the results of the FinalizeIMPL operation.
type FinalizeIMPLData struct {
	Validation          ValidateResult           `json:"validation"`
	GatePopulation      GatePopulationStats      `json:"gate_population"`
	ChecklistPopulation ChecklistPopulationStats `json:"checklist_population"`
	FinalValidation     ValidateResult           `json:"final_validation"`
}

// ChecklistPopulationStats contains statistics about integration checklist population.
type ChecklistPopulationStats struct {
	GroupsAdded int `json:"groups_added"`
	ItemsAdded  int `json:"items_added"`
}

// ValidateResult contains validation pass/fail status and error list.
type ValidateResult struct {
	Passed bool             `json:"passed"`
	Errors []result.SAWError `json:"errors"`
}

// GatePopulationStats contains statistics about verification gate population.
type GatePopulationStats struct {
	AgentsUpdated   int    `json:"agents_updated"`
	Toolchain       string `json:"toolchain"`
	H2DataAvailable bool   `json:"h2_data_available"`
}

// FinalizeIMPL is a batching command that combines:
// 1. Validate (E16 structure check)
// 2. Populate verification gates (M4 gate generation)
// 3. Validate again (confirm gates valid)
// Atomic operation with rollback on failure.
func FinalizeIMPL(implPath, repoRoot string) result.Result[FinalizeIMPLData] {
	data := FinalizeIMPLData{}

	// Step 1: Load manifest
	manifest, err := Load(context.TODO(), implPath)
	if err != nil {
		return result.NewFailure[FinalizeIMPLData]([]result.SAWError{
			{
				Code:     result.CodeIMPLParseFailed,
				Message:  fmt.Sprintf("failed to load IMPL manifest: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Step 2: Initial validation (E16)
	validationErrors := Validate(manifest)
	data.Validation = ValidateResult{
		Passed: len(validationErrors) == 0,
		Errors: validationErrors,
	}

	if !data.Validation.Passed {
		return result.NewFailure[FinalizeIMPLData]([]result.SAWError{
			{
				Code:     result.CodeManifestInvalid,
				Message:  "initial validation failed",
				Severity: "fatal",
			},
		})
	}

	// Step 3: Extract H2 command data from all repos
	// Build list of unique repos from file_ownership
	repos := extractUniqueRepos(manifest, repoRoot)

	// Build repo mapping: relative name -> absolute path
	// This allows PopulateVerificationGates to resolve agent repos
	repoMap := make(map[string]string)
	parentDir := filepath.Dir(repoRoot)

	for _, ownership := range manifest.FileOwnership {
		if ownership.Repo != "" && !filepath.IsAbs(ownership.Repo) {
			// Map relative repo name to absolute path
			absPath := filepath.Join(parentDir, ownership.Repo)
			repoMap[ownership.Repo] = absPath
		} else if filepath.IsAbs(ownership.Repo) {
			// Already absolute - identity mapping
			repoMap[ownership.Repo] = ownership.Repo
		}
	}
	// Add default repo mapping for single-repo IMPLs
	repoMap["."] = repoRoot

	// Extract command set for each repo
	commandSets := make(map[string]*commands.CommandSet)
	toolchains := make(map[string]string)

	extractor := commands.New()
	// Register default parsers
	extractor.RegisterCIParser(&commands.GithubActionsParser{})
	extractor.RegisterBuildSystemParser(&commands.MakefileParser{})
	extractor.RegisterBuildSystemParser(&commands.PackageJSONParser{})

	for _, repo := range repos {
		commandSet, err := extractor.Extract(context.TODO(), repo)
		if err != nil || commandSet == nil {
			// Skip repos without H2 data - agents may have manually-specified gates
			continue
		}
		commandSets[repo] = commandSet
		toolchains[repo] = commandSet.Toolchain
	}

	if len(commandSets) == 0 {
		data.GatePopulation.H2DataAvailable = false
		return result.NewFailure[FinalizeIMPLData]([]result.SAWError{
			{
				Code:     result.CodeToolNotFound,
				Message:  "H2 data unavailable - run extract-commands first: no valid toolchains found in any repo",
				Severity: "fatal",
			},
		})
	}

	data.GatePopulation.H2DataAvailable = true
	// For multi-repo, report comma-separated toolchains
	toolchainList := make([]string, 0, len(toolchains))
	for _, tc := range toolchains {
		toolchainList = append(toolchainList, tc)
	}
	data.GatePopulation.Toolchain = strings.Join(toolchainList, ", ")

	// Step 4: Populate verification gates
	updatedManifest, err := PopulateVerificationGates(context.TODO(), manifest, commandSets, repoMap)
	if err != nil {
		return result.NewFailure[FinalizeIMPLData]([]result.SAWError{
			{
				Code:     result.CodeFinalizeWaveFailed,
				Message:  fmt.Sprintf("failed to populate verification gates: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Count how many agents were updated
	agentsUpdated := 0
	for waveIdx, wave := range updatedManifest.Waves {
		for agentIdx, agent := range wave.Agents {
			originalAgent := manifest.Waves[waveIdx].Agents[agentIdx]
			if agent.Task != originalAgent.Task {
				agentsUpdated++
			}
		}
	}
	data.GatePopulation.AgentsUpdated = agentsUpdated

	// Step 4.5: Populate integration checklist (M5)
	updatedManifest, err = PopulateIntegrationChecklist(updatedManifest)
	if err != nil {
		return result.NewFailure[FinalizeIMPLData]([]result.SAWError{
			{
				Code:     result.CodeFinalizeWaveFailed,
				Message:  fmt.Sprintf("failed to populate integration checklist: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Count groups and items added
	groupsAdded := 0
	itemsAdded := 0
	if updatedManifest.PostMergeChecklist != nil {
		groupsAdded = len(updatedManifest.PostMergeChecklist.Groups)
		for _, group := range updatedManifest.PostMergeChecklist.Groups {
			itemsAdded += len(group.Items)
		}
	}
	data.ChecklistPopulation = ChecklistPopulationStats{
		GroupsAdded: groupsAdded,
		ItemsAdded:  itemsAdded,
	}

	// Step 5: Final validation
	finalValidationErrors := Validate(updatedManifest)
	data.FinalValidation = ValidateResult{
		Passed: len(finalValidationErrors) == 0,
		Errors: finalValidationErrors,
	}

	if !data.FinalValidation.Passed {
		return result.NewFailure[FinalizeIMPLData]([]result.SAWError{
			{
				Code:     result.CodeManifestInvalid,
				Message:  "final validation failed after gate population",
				Severity: "fatal",
			},
		})
	}

	// Step 6: Save updated manifest (atomic write)
	if saveRes := Save(context.TODO(), updatedManifest, implPath); saveRes.IsFatal() {
		return result.NewFailure[FinalizeIMPLData](saveRes.Errors)
	}

	return result.NewSuccess(data)
}
