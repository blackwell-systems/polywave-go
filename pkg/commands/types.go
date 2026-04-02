package commands

import "github.com/blackwell-systems/scout-and-wave-go/pkg/result"

// CommandSet represents extracted build/test/lint/format commands
type CommandSet struct {
	Toolchain        string
	Commands         Commands
	DetectionSources []string
	ModuleMap        []Module
}

// Commands holds the actual command strings
type Commands struct {
	Build  string
	Test   TestCommands
	Lint   LintCommands
	Format FormatCommands
}

// TestCommands separates full suite vs focused test patterns
type TestCommands struct {
	Full           string
	FocusedPattern string
}

// LintCommands separates check mode from auto-fix mode
type LintCommands struct {
	Check string
	Fix   string
}

// FormatCommands separates check mode from auto-fix mode
type FormatCommands struct {
	Check string
	Fix   string
}

// Module represents a package/module with test count metadata
type Module struct {
	Package            string
	TestCount          int
	FocusedRecommended bool
}

// ParseCIData wraps the result of ParseCI operation
type ParseCIData struct {
	CommandSet *CommandSet
}

// Parser interface for CI systems (GitHub Actions, GitLab CI, CircleCI)
type CIParser interface {
	ParseCI(repoRoot string) result.Result[ParseCIData]
	Priority() int
}

// ParseBuildSystemData wraps the result of ParseBuildSystem operation
type ParseBuildSystemData struct {
	CommandSet *CommandSet
}

// Parser interface for build systems (Makefile, package.json, etc.)
type BuildSystemParser interface {
	ParseBuildSystem(repoRoot string) result.Result[ParseBuildSystemData]
	Priority() int
}
