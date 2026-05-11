package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// MakefileParser extracts commands from Makefile targets
type MakefileParser struct{}

// ParseBuildSystem parses a Makefile and extracts build/test/lint/format commands
func (p *MakefileParser) ParseBuildSystem(repoRoot string) result.Result[ParseBuildSystemData] {
	makefilePath := filepath.Join(repoRoot, "Makefile")

	// Return nil (not error) when Makefile doesn't exist
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		return result.NewSuccess(ParseBuildSystemData{CommandSet: nil})
	}

	file, err := os.Open(makefilePath)
	if err != nil {
		return result.NewFailure[ParseBuildSystemData]([]result.PolywaveError{
			result.NewFatal(result.CodeCommandExtractMakefileRead, fmt.Sprintf("reading Makefile: %v", err)),
		})
	}
	defer file.Close()

	targets, targetOrder := parseMakefileTargets(file)
	if len(targets) == 0 {
		return result.NewSuccess(ParseBuildSystemData{CommandSet: nil})
	}

	// Resolve target chains to find leaf targets
	_, leafOrder := resolveTargetChains(targets, targetOrder)

	// Classify and extract commands
	cmdSet := &CommandSet{
		Toolchain:        "make",
		DetectionSources: []string{"Makefile"},
	}

	for _, targetName := range leafOrder {
		cmd := buildMakeCommand(targetName)
		classifyAndAssignCommand(targetName, cmd, cmdSet)
	}

	return result.NewSuccess(ParseBuildSystemData{CommandSet: cmdSet})
}

// Priority returns 50 (lower than CI parsers, higher than package.json)
func (p *MakefileParser) Priority() int {
	return 50
}

// makeTarget represents a parsed Makefile target
type makeTarget struct {
	name         string
	dependencies []string
	commands     []string
}

// parseMakefileTargets extracts all targets from a Makefile
func parseMakefileTargets(file *os.File) (map[string]*makeTarget, []string) {
	targets := make(map[string]*makeTarget)
	targetOrder := []string{}
	scanner := bufio.NewScanner(file)

	var currentTarget *makeTarget

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		// Check if this is a target line (contains ':')
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "\t") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				targetName := strings.TrimSpace(parts[0])

				// Skip variable assignments and special targets
				if strings.Contains(targetName, "=") || strings.HasPrefix(targetName, ".") {
					currentTarget = nil
					continue
				}

				currentTarget = &makeTarget{
					name:     targetName,
					commands: []string{},
				}

				// Parse dependencies
				depPart := strings.TrimSpace(parts[1])
				if depPart != "" {
					deps := strings.Fields(depPart)
					currentTarget.dependencies = deps
				}

				targets[targetName] = currentTarget
				targetOrder = append(targetOrder, targetName)
			}
		} else if currentTarget != nil && strings.HasPrefix(line, "\t") {
			// This is a command line for the current target
			cmd := strings.TrimSpace(line)
			if cmd != "" && !strings.HasPrefix(cmd, "@") && !strings.HasPrefix(cmd, "-") {
				currentTarget.commands = append(currentTarget.commands, cmd)
			} else if strings.HasPrefix(cmd, "@") || strings.HasPrefix(cmd, "-") {
				// Strip @ and - prefixes
				cmd = strings.TrimPrefix(cmd, "@")
				cmd = strings.TrimPrefix(cmd, "-")
				cmd = strings.TrimSpace(cmd)
				if cmd != "" {
					currentTarget.commands = append(currentTarget.commands, cmd)
				}
			}
		}
	}

	return targets, targetOrder
}

// collectLeaves recursively collects leaf targets (targets with commands) reachable
// from name, using a visited set to prevent infinite loops in circular dependencies.
func collectLeaves(name string, targets map[string]*makeTarget, visited map[string]bool) []*makeTarget {
	if _, exists := targets[name]; !exists {
		return nil
	}
	if visited[name] {
		return nil
	}
	visited[name] = true

	target := targets[name]
	if len(target.commands) > 0 {
		return []*makeTarget{target}
	}

	var leaves []*makeTarget
	for _, dep := range target.dependencies {
		leaves = append(leaves, collectLeaves(dep, targets, visited)...)
	}
	return leaves
}

// resolveTargetChains finds leaf targets by recursively resolving transitive
// dependencies. A leaf target is one that has actual commands (not just deps).
func resolveTargetChains(targets map[string]*makeTarget, targetOrder []string) (map[string]*makeTarget, []string) {
	leafTargets := make(map[string]*makeTarget)
	leafOrder := []string{}

	for _, name := range targetOrder {
		visited := make(map[string]bool)
		leaves := collectLeaves(name, targets, visited)
		for _, leaf := range leaves {
			if _, exists := leafTargets[leaf.name]; !exists {
				leafTargets[leaf.name] = leaf
				leafOrder = append(leafOrder, leaf.name)
			}
		}
	}

	return leafTargets, leafOrder
}

// buildMakeCommand constructs the make command for a target.
func buildMakeCommand(targetName string) string {
	return "make " + targetName
}

// classifyAndAssignCommand classifies a target by name and assigns to CommandSet
func classifyAndAssignCommand(targetName, cmd string, cmdSet *CommandSet) {
	lowerName := strings.ToLower(targetName)

	// Build commands
	if strings.Contains(lowerName, "build") || strings.Contains(lowerName, "compile") || lowerName == "all" {
		if cmdSet.Commands.Build == "" {
			cmdSet.Commands.Build = cmd
		}
		return
	}

	// Test commands
	if strings.Contains(lowerName, "test") || lowerName == "check" {
		if cmdSet.Commands.Test.Full == "" {
			cmdSet.Commands.Test.Full = cmd
		}
		return
	}

	// Lint commands
	if strings.Contains(lowerName, "lint") || strings.Contains(lowerName, "vet") {
		if cmdSet.Commands.Lint.Check == "" {
			cmdSet.Commands.Lint.Check = cmd
		}
		return
	}

	// Format commands
	if strings.Contains(lowerName, "fmt") || strings.Contains(lowerName, "format") {
		// Heuristic: if target name contains "check", it's check mode
		if strings.Contains(lowerName, "check") {
			if cmdSet.Commands.Format.Check == "" {
				cmdSet.Commands.Format.Check = cmd
			}
		} else {
			// Otherwise assume it's fix mode
			if cmdSet.Commands.Format.Fix == "" {
				cmdSet.Commands.Format.Fix = cmd
			}
		}
		return
	}
}
