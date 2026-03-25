// Package result — error code constants for SAWError.Code field.
//
// Code ranges by domain:
//
//	V001-V099: Validation errors (IMPL doc structure, schema, fields)
//	B001-B099: Build and gate errors (compilation, test, lint failures)
//	G001-G099: Git errors (merge conflicts, worktree, commit issues)
//	A001-A099: Agent errors (launch, timeout, brief extraction)
//	N001-N099: Engine errors (orchestration, state machine)
//	P001-P099: Protocol errors (invariant violations, execution rules)
//	T001-T099: Tool/parse errors (errparse output, tool runner failures)
package result

// Validation error codes (V001-V099)
const (
	CodeManifestInvalid         = "V001_MANIFEST_INVALID"
	CodeDisjointOwnership       = "V002_DISJOINT_OWNERSHIP"
	CodeSameWaveDependency      = "V003_SAME_WAVE_DEPENDENCY"
	CodeWaveNotOneIndexed       = "V004_WAVE_NOT_1INDEXED"
	CodeRequiredFieldsMissing   = "V005_REQUIRED_FIELDS_MISSING"
	CodeFileOwnershipIncomplete = "V006_FILE_OWNERSHIP_INCOMPLETE"
	CodeDependencyCycle         = "V007_DEPENDENCY_CYCLE"
	CodeInvalidState            = "V008_INVALID_STATE"
	CodeInvalidAgentID          = "V009_INVALID_AGENT_ID"
	CodeInvalidGateType         = "V010_INVALID_GATE_TYPE"
	CodeInvalidActionEnum       = "V011_INVALID_ACTION_ENUM"
	CodeDuplicateKey            = "V012_DUPLICATE_KEY"
	CodeUnknownKey              = "V013_UNKNOWN_KEY"
	CodeInvalidScaffoldStatus   = "V014_INVALID_SCAFFOLD_STATUS"
	CodeInvalidPreMortemRisk    = "V015_INVALID_PRE_MORTEM_RISK"
	CodeJSONSchemaFailed        = "V016_JSONSCHEMA_FAILED"
	CodeSlugMismatch            = "V017_SLUG_MISMATCH"
	CodeInvalidSlugFormat       = "V018_INVALID_SLUG_FORMAT"
	CodeOrphanFile              = "V019_ORPHAN_FILE"
	CodeInconsistentRepo        = "V020_INCONSISTENT_REPO"
	CodeKnownIssueMissingTitle  = "V021_KNOWN_ISSUE_MISSING_TITLE"
	CodeInvalidFailureType      = "V022_INVALID_FAILURE_TYPE"
	CodeInvalidMergeState       = "V023_INVALID_MERGE_STATE"
	CodeProgramInvalid          = "V024_PROGRAM_INVALID"
	CodeTierMismatch            = "V025_TIER_MISMATCH"
	CodeTierOrderViolation      = "V026_TIER_ORDER_VIOLATION"
	CodeInvalidConsumer         = "V027_INVALID_CONSUMER"
	CodeInvalidDependency       = "V028_INVALID_DEPENDENCY"
	CodeP1FileOverlap           = "V029_P1_FILE_OVERLAP"
	CodeP2ContractRedefinition  = "V030_P2_CONTRACT_REDEFINITION"
	CodeIMPLFileMissing         = "V031_IMPL_FILE_MISSING"
	CodeIMPLStateMismatch       = "V032_IMPL_STATE_MISMATCH"
	CodeCompletionBounds        = "V033_COMPLETION_BOUNDS"
	CodeImplsTotalMismatch      = "V034_IMPLS_TOTAL_MISMATCH"
	CodeP1Violation             = "V035_P1_VIOLATION"
)

// Build and gate error codes (B001-B099)
const (
	CodeBuildFailed        = "B001_BUILD_FAILED"
	CodeTestFailed         = "B002_TEST_FAILED"
	CodeLintFailed         = "B003_LINT_FAILED"
	CodeFormatCheckFailed  = "B004_FORMAT_CHECK_FAILED"
	CodeGateTimeout        = "B005_GATE_TIMEOUT"
	CodeGateCommandMissing = "B006_GATE_COMMAND_MISSING"
	CodeStubDetected       = "B007_STUB_DETECTED"
)

// Git error codes (G001-G099)
const (
	CodeWorktreeCreateFailed = "G001_WORKTREE_CREATE_FAILED"
	CodeMergeConflict        = "G002_MERGE_CONFLICT"
	CodeCommitMissing        = "G003_COMMIT_MISSING"
	CodeBranchExists         = "G004_BRANCH_EXISTS"
	CodeDirtyWorktree        = "G005_DIRTY_WORKTREE"
	CodeHookInstallFailed    = "G006_HOOK_INSTALL_FAILED"
	CodeWorktreeCleanup      = "G007_WORKTREE_CLEANUP"
)

// Agent error codes (A001-A099)
const (
	CodeAgentTimeout            = "A001_AGENT_TIMEOUT"
	CodeAgentStubDetected       = "A002_STUB_DETECTED"
	CodeCompletionReportMissing = "A003_COMPLETION_REPORT_MISSING"
	CodeVerificationFailed      = "A004_VERIFICATION_FAILED"
	CodeAgentLaunchFailed       = "A005_AGENT_LAUNCH_FAILED"
	CodeBriefExtractFail        = "A006_BRIEF_EXTRACT_FAIL"
	CodeJournalInitFail         = "A007_JOURNAL_INIT_FAIL"
)

// Engine error codes (N001-N099)
const (
	CodePrepareWaveFailed     = "N001_PREPARE_WAVE_FAILED"
	CodeFinalizeWaveFailed    = "N002_FINALIZE_WAVE_FAILED"
	CodeScoutFailed           = "N003_SCOUT_FAILED"
	CodeIsolationVerifyFailed = "N004_ISOLATION_VERIFY_FAILED"
	CodeIMPLNotFound          = "N005_IMPL_NOT_FOUND"
	CodeIMPLParseFailed       = "N006_IMPL_PARSE_FAILED"
	CodeWaveNotReady          = "N007_WAVE_NOT_READY"
	CodeStateTransition       = "N008_STATE_TRANSITION"
	CodeContextError          = "N009_CONTEXT_ERROR"
	CodeBaselineError         = "N010_BASELINE_ERROR"
	CodeStaleWorktree         = "N011_STALE_WORKTREE"
	CodeFreezeError           = "N012_FREEZE_ERROR"
	CodeConfigNotFound        = "N013_CONFIG_NOT_FOUND"
	CodeConfigInvalid         = "N014_CONFIG_INVALID"
)

// Protocol error codes (P001-P099)
const (
	CodeStateTransitionInvalid  = "P001_STATE_TRANSITION_INVALID"
	CodeProgramValidationFailed = "P002_PROGRAM_VALIDATION_FAILED"
	CodeMigrationBoundaryUnsafe = "P003_MIGRATION_BOUNDARY_UNSAFE"
	CodeDepsNotMet              = "P004_DEPS_NOT_MET"
	CodeInvariantViolation      = "P005_INVARIANT_VIOLATION"
	CodeExecutionRule           = "P006_EXECUTION_RULE"
)

// Tool/parse error codes (T001-T099)
const (
	CodeToolError    = "T001_TOOL_ERROR"
	CodeParsePanic   = "T002_PARSE_PANIC"
	CodeToolNotFound = "T003_TOOL_NOT_FOUND"
	CodeToolTimeout  = "T004_TOOL_TIMEOUT"
)

