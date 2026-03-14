package protocol

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
)

// PopulateVerificationGates populates verification gate blocks for all agents
// in the manifest that don't already have them. Returns a new manifest copy
// without modifying the input.
func PopulateVerificationGates(m *IMPLManifest, commandSet *commands.CommandSet) (*IMPLManifest, error) {
	if commandSet == nil {
		return nil, fmt.Errorf("H2 data unavailable - run extract-commands first")
	}

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
		testPrefix := "Test" + strings.Title(pkgName)
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

// FinalizeIMPLResult contains the results of the FinalizeIMPL operation.
type FinalizeIMPLResult struct {
	Success         bool                `json:"success"`
	Validation      ValidateResult      `json:"validation"`
	GatePopulation  GatePopulationStats `json:"gate_population"`
	FinalValidation ValidateResult      `json:"final_validation"`
}

// ValidateResult contains validation pass/fail status and error list.
type ValidateResult struct {
	Passed bool              `json:"passed"`
	Errors []ValidationError `json:"errors"`
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
func FinalizeIMPL(implPath, repoRoot string) (*FinalizeIMPLResult, error) {
	result := &FinalizeIMPLResult{}

	// Step 1: Load manifest
	manifest, err := Load(implPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load IMPL manifest: %w", err)
	}

	// Step 2: Initial validation (E16)
	validationErrors := Validate(manifest)
	result.Validation = ValidateResult{
		Passed: len(validationErrors) == 0,
		Errors: validationErrors,
	}

	if !result.Validation.Passed {
		result.Success = false
		return result, nil
	}

	// Step 3: Extract H2 command data
	extractor := commands.New()
	// Register default parsers
	extractor.RegisterCIParser(&commands.GithubActionsParser{})
	extractor.RegisterBuildSystemParser(&commands.MakefileParser{})
	extractor.RegisterBuildSystemParser(&commands.PackageJSONParser{})

	commandSet, err := extractor.Extract(repoRoot)
	if err != nil || commandSet == nil {
		result.Success = false
		result.GatePopulation.H2DataAvailable = false
		return result, fmt.Errorf("H2 data unavailable - run extract-commands first: %w", err)
	}

	result.GatePopulation.H2DataAvailable = true
	result.GatePopulation.Toolchain = commandSet.Toolchain

	// Step 4: Populate verification gates
	updatedManifest, err := PopulateVerificationGates(manifest, commandSet)
	if err != nil {
		result.Success = false
		return result, fmt.Errorf("failed to populate verification gates: %w", err)
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
	result.GatePopulation.AgentsUpdated = agentsUpdated

	// Step 5: Final validation
	finalValidationErrors := Validate(updatedManifest)
	result.FinalValidation = ValidateResult{
		Passed: len(finalValidationErrors) == 0,
		Errors: finalValidationErrors,
	}

	if !result.FinalValidation.Passed {
		result.Success = false
		return result, nil
	}

	// Step 6: Save updated manifest (atomic write)
	if err := Save(updatedManifest, implPath); err != nil {
		result.Success = false
		return result, fmt.Errorf("failed to save updated manifest: %w", err)
	}

	result.Success = true
	return result, nil
}
