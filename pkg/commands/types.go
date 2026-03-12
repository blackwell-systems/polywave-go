package commands

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

// Parser interface for CI systems (GitHub Actions, GitLab CI, CircleCI)
type CIParser interface {
	ParseCI(repoRoot string) (*CommandSet, error)
	Priority() int
}

// Parser interface for build systems (Makefile, package.json, etc.)
type BuildSystemParser interface {
	ParseBuildSystem(repoRoot string) (*CommandSet, error)
	Priority() int
}
