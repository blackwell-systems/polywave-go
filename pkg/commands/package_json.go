package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PackageJSONParser extracts build/test/lint/format commands from npm/yarn scripts
type PackageJSONParser struct{}

// packageJSON represents the relevant fields from package.json
type packageJSON struct {
	Scripts    map[string]string `json:"scripts"`
	Workspaces []string          `json:"workspaces"`
}

// ParseBuildSystem reads package.json and extracts commands from the scripts section
func (p *PackageJSONParser) ParseBuildSystem(repoRoot string) (*CommandSet, error) {
	pkgPath := filepath.Join(repoRoot, "package.json")

	// Return nil (not error) when package.json doesn't exist
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("reading package.json: %w", err)
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package.json: %w", err)
	}

	// If no scripts section, nothing to extract
	if len(pkg.Scripts) == 0 {
		return nil, nil
	}

	cmdSet := &CommandSet{
		Toolchain:        "npm",
		DetectionSources: []string{"package.json"},
	}

	// Extract commands from scripts section
	cmdSet.Commands.Build = extractCommand(pkg.Scripts, []string{"build", "compile"})
	cmdSet.Commands.Test.Full = extractCommand(pkg.Scripts, []string{"test", "test:unit", "test:e2e"})
	cmdSet.Commands.Lint.Check = extractCommand(pkg.Scripts, []string{"lint", "eslint"})
	cmdSet.Commands.Format.Check = extractCommand(pkg.Scripts, []string{"format", "prettier"})

	// Handle monorepo workspaces (roadmap edge case #3)
	if len(pkg.Workspaces) > 0 {
		cmdSet.Commands.Test.FocusedPattern = buildWorkspacePattern(cmdSet.Commands.Test.Full, pkg.Workspaces)
	}

	return cmdSet, nil
}

// Priority returns 40 (lower than Makefile's 50, higher than language defaults)
func (p *PackageJSONParser) Priority() int {
	return 40
}

// extractCommand finds the first matching script name from the priority list
func extractCommand(scripts map[string]string, names []string) string {
	for _, name := range names {
		if _, exists := scripts[name]; exists {
			// npm scripts use "npm run <script>", not just "<script>"
			return fmt.Sprintf("npm run %s", name)
		}
	}
	return ""
}

// buildWorkspacePattern generates a focused test pattern for monorepos
// Example: "npm test" → "npm test --workspace=packages/*"
func buildWorkspacePattern(fullCommand string, workspaces []string) string {
	if fullCommand == "" || len(workspaces) == 0 {
		return ""
	}

	// Use the first workspace pattern as the focused target
	// Most monorepos use patterns like "packages/*" or "apps/*"
	workspace := workspaces[0]

	return fmt.Sprintf("%s --workspace=%s", fullCommand, workspace)
}
