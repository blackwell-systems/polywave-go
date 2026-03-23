// Package result — error code constants for SAWError.Code field.
//
// Code ranges by domain:
//   V001-V099: Validation errors (IMPL doc structure, schema, fields)
//   B001-B099: Build and gate errors (compilation, test, lint failures)
//   G001-G099: Git errors (merge conflicts, worktree, commit issues)
//   A001-A099: Agent errors (launch, timeout, brief extraction)
//   N001-N099: Engine errors (orchestration, state machine)
//   P001-P099: Protocol errors (invariant violations, execution rules)
//   T001-T099: Tool/parse errors (errparse output, tool runner failures)
package result

// Validation error codes (V001-V099)
const (
	CodeManifestInvalid     = "V001_MANIFEST_INVALID"
	CodeFieldMissing        = "V002_FIELD_MISSING"
	CodeFieldInvalid        = "V003_FIELD_INVALID"
	CodeSchemaViolation     = "V004_SCHEMA_VIOLATION"
	CodeSlugMismatch        = "V005_SLUG_MISMATCH"
	CodeWaveOrderInvalid    = "V006_WAVE_ORDER_INVALID"
	CodeDuplicateAgent      = "V007_DUPLICATE_AGENT"
	CodeDependencyCycle     = "V008_DEPENDENCY_CYCLE"
	CodeFileOwnershipGap    = "V009_FILE_OWNERSHIP_GAP"
	CodeFileOwnershipConfl  = "V010_FILE_OWNERSHIP_CONFLICT"
	CodeScaffoldMissing     = "V011_SCAFFOLD_MISSING"
	CodeGateMissing         = "V012_GATE_MISSING"
	CodeEnumInvalid         = "V013_ENUM_INVALID"
	CodeTestCommandMissing  = "V014_TEST_COMMAND_MISSING"
	CodeProgramInvalid      = "V015_PROGRAM_INVALID"
	CodeJSONSchemaFailed    = "V016_JSONSCHEMA_FAILED"
)

// Build and gate error codes (B001-B099)
const (
	CodeBuildFailed   = "B001_BUILD_FAILED"
	CodeTestFailed    = "B002_TEST_FAILED"
	CodeLintFailed    = "B003_LINT_FAILED"
	CodeGateFailed    = "B004_GATE_FAILED"
	CodeStubRemaining = "B005_STUB_REMAINING"
)

// Git error codes (G001-G099)
const (
	CodeMergeConflict     = "G001_MERGE_CONFLICT"
	CodeWorktreeCreate    = "G002_WORKTREE_CREATE"
	CodeWorktreeCleanup   = "G003_WORKTREE_CLEANUP"
	CodeCommitMissing     = "G004_COMMIT_MISSING"
	CodeBranchExists      = "G005_BRANCH_EXISTS"
	CodeHookInstallFailed = "G006_HOOK_INSTALL_FAILED"
)

// Agent error codes (A001-A099)
const (
	CodeAgentLaunchFailed = "A001_AGENT_LAUNCH_FAILED"
	CodeAgentTimeout      = "A002_AGENT_TIMEOUT"
	CodeBriefExtractFail  = "A003_BRIEF_EXTRACT_FAIL"
	CodeJournalInitFail   = "A004_JOURNAL_INIT_FAIL"
)

// Engine error codes (N001-N099)
const (
	CodeStateTransition = "N001_STATE_TRANSITION"
	CodeIMPLNotFound    = "N002_IMPL_NOT_FOUND"
	CodeIMPLParseFailed = "N003_IMPL_PARSE_FAILED"
	CodeWaveNotReady    = "N004_WAVE_NOT_READY"
)

// Protocol error codes (P001-P099)
const (
	CodeInvariantViolation = "P001_INVARIANT_VIOLATION"
	CodeExecutionRule      = "P002_EXECUTION_RULE"
	CodeDepsNotMet         = "P003_DEPS_NOT_MET"
)

// Tool/parse error codes (T001-T099)
const (
	CodeToolError    = "T001_TOOL_ERROR"
	CodeParseFailure = "T002_PARSE_FAILURE"
	CodeToolTimeout  = "T003_TOOL_TIMEOUT"
)
