// Package result — error code constants for SAWError.Code field.
//
// Code ranges by domain:
//
//	V001-V099: Validation errors (IMPL doc structure, schema, fields)
//	W001-W099: Warning codes (advisory only, never block execution)
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
	// CodeInvalidEnum is emitted for invalid enum field values (replaces SV01_INVALID_ENUM, DC02_INVALID_STATUS)
	CodeInvalidEnum = "V036_INVALID_ENUM"
	// CodeInvalidPath is emitted for invalid path format (replaces SV01_INVALID_PATH)
	CodeInvalidPath = "V037_INVALID_PATH"
	// CodeCrossField is emitted for cross-field consistency violations (replaces SV01_CROSS_FIELD)
	CodeCrossField = "V038_CROSS_FIELD"
	// CodeInvalidFieldValue is emitted for invalid field values (replaces I4_INVALID_VALUE, I4_INVALID_FORMAT)
	CodeInvalidFieldValue = "V039_INVALID_FIELD_VALUE"
	// CodeUnscopedGate is emitted for multi-repo gates missing a repo: scope (replaces MR02_UNSCOPED_GATE)
	CodeUnscopedGate = "V040_UNSCOPED_GATE"
	// CodeFileMissing is emitted when an action=modify file does not exist (replaces E16_FILE_NOT_FOUND)
	CodeFileMissing = "V041_FILE_MISSING"
	// CodeInvalidWorktreeName is emitted for invalid worktree branch or path naming (replaces E5_INVALID_WORKTREE_*)
	CodeInvalidWorktreeName = "V042_INVALID_WORKTREE_NAME"
	// CodeInvalidVerification is emitted for invalid verification field format (replaces E10_INVALID_VERIFICATION)
	CodeInvalidVerification = "V043_INVALID_VERIFICATION"
	// CodeMissingChecklist is emitted when new handlers lack a post_merge_checklist (replaces E16_MISSING_CHECKLIST)
	CodeMissingChecklist = "V044_MISSING_CHECKLIST"
	// CodeRepoMismatch is emitted when all action=modify files are missing, indicating wrong repo (replaces E16_REPO_MISMATCH_SUSPECTED)
	CodeRepoMismatch = "V045_REPO_MISMATCH"
	// CodeParseError is emitted for YAML parse failures (replaces E16_PARSE_ERROR, P000_PARSE_ERROR)
	CodeParseError = "V046_PARSE_ERROR"
	// CodeTrivialScope is emitted when an IMPL is SUITABLE but has only 1 agent owning 1 file.
	// SAW adds no parallelization value at this scope — the change should be made directly.
	CodeTrivialScope = "V047_TRIVIAL_SCOPE"
)

// Warning codes (W001-W099) — advisory only, never block execution
const (
	// CodeAgentScopeLarge is emitted when an agent owns more than 8 files
	// or creates more than 5 new files. Severity is always "warning".
	CodeAgentScopeLarge = "W001_AGENT_SCOPE_LARGE"
	// CodeCompletionVerificationWarning is emitted when an agent has no commits on its wave branch (replaces W101)
	CodeCompletionVerificationWarning = "W002_COMPLETION_VERIFY"
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
	// B008: gate input validation failed (replaces "E_GATE_INPUT" in gate_input_validator.go)
	CodeGateInputInvalid = "B008_GATE_INPUT_INVALID"
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
	// N015: status update mutation failed (replaces "E_STATUS" in status_update.go)
	CodeStatusUpdateFailed = "N015_STATUS_UPDATE_FAILED"
	// N016: tier gate evaluation failed (replaces "E_TIER_GATE" in program_tier_gate.go)
	CodeTierGateFailed = "N016_TIER_GATE_FAILED"
	// N017: program status computation failed (replaces "E_PROGRAM_STATUS" in program_status.go)
	CodeProgramStatusFailed = "N017_PROGRAM_STATUS_FAILED"

	// N018-N084: Engine operation codes — ad-hoc strings migrated from pkg/engine/ and pkg/notify/

	// CodeContextCancelled is emitted when an operation is cancelled via context.
	CodeContextCancelled = "CONTEXT_CANCELLED" // N018

	// Scout operation codes
	CodeScoutInvalidOpts         = "ENGINE_SCOUT_INVALID_OPTS"         // N019
	CodeScoutRunFailed           = "ENGINE_SCOUT_FAILED"               // N020
	CodeScoutBoundaryViolation   = "ENGINE_SCOUT_BOUNDARY_VIOLATION"   // N021
	CodePlannerInvalidOpts       = "ENGINE_PLANNER_INVALID_OPTS"       // N022
	CodePlannerFailed            = "ENGINE_PLANNER_FAILED"             // N023
	CodeWaveInvalidOpts          = "ENGINE_WAVE_INVALID_OPTS"          // N024
	CodeWaveFailed               = "ENGINE_WAVE_FAILED"                // N025
	CodeWaveSequencingFailed     = "ENGINE_WAVE_SEQUENCING_FAILED"     // N026
	CodeHookVerifyFailed         = "ENGINE_HOOK_VERIFY_FAILED"         // N027
	CodeScaffoldRunFailed        = "ENGINE_SCAFFOLD_FAILED"            // N028
	CodeAgentRunFailed           = "ENGINE_AGENT_FAILED"               // N029
	CodeAgentRunInvalidOpts      = "ENGINE_AGENT_INVALID_OPTS"         // N030
	CodeMergeWaveFailed          = "ENGINE_MERGE_FAILED"               // N031
	CodeMergeWaveInvalidOpts     = "ENGINE_MERGE_INVALID_OPTS"         // N032
	CodeEngineVerificationFailed = "ENGINE_VERIFICATION_FAILED"        // N033
	CodeUpdateStatusFailed       = "ENGINE_UPDATE_STATUS_FAILED"       // N034
	CodeValidateFailed           = "ENGINE_VALIDATE_FAILED"            // N035
	CodeJournalArchiveFailed     = "ENGINE_JOURNAL_ARCHIVE_FAILED"     // N036
	CodeMarkCompleteFailed       = "ENGINE_MARK_COMPLETE_FAILED"       // N037
	CodeMarkCompleteInvalidOpts  = "ENGINE_MARK_COMPLETE_INVALID_OPTS" // N038
	CodeVerifyTiersIncomplete    = "ENGINE_VERIFY_TIERS_INCOMPLETE"    // N039
	CodeMarkerReadFailed         = "ENGINE_MARKER_READ_FAILED"         // N040
	CodeMarkerWriteFailed        = "ENGINE_MARKER_WRITE_FAILED"        // N041
	CodeUpdateProgParseFailed    = "ENGINE_UPDATE_PROG_PARSE_FAILED"   // N042
	CodeUpdateProgSlugNotFound   = "ENGINE_UPDATE_PROG_SLUG_NOT_FOUND" // N043
	CodeSyncParseFailed          = "ENGINE_SYNC_PARSE_FAILED"          // N044
	CodeSyncStatusFailed         = "ENGINE_SYNC_STATUS_FAILED"         // N045
	CodeWriteManifestFailed      = "ENGINE_WRITE_MANIFEST_FAILED"      // N046
	CodeRestoreLoadFailed        = "ENGINE_RESTORE_LOAD_FAILED"        // N047
	CodeRestoreSaveFailed        = "ENGINE_RESTORE_SAVE_FAILED"        // N048
	CodeTestLoadFailed           = "ENGINE_TEST_LOAD_FAILED"           // N049
	CodeTestNoCommand            = "ENGINE_TEST_NO_COMMAND"            // N050
	CodeTestPipeFailed           = "ENGINE_TEST_PIPE_FAILED"           // N051
	CodeTestStartFailed          = "ENGINE_TEST_START_FAILED"          // N052
	CodeTestCommandFailed        = "ENGINE_TEST_COMMAND_FAILED"        // N053
	CodeScoutRunnerFailed        = "ENGINE_SCOUT_RUN_FAILED"           // N054
	CodeScoutValidationFailed    = "ENGINE_SCOUT_VALIDATION_FAILED"    // N055
	CodeScoutCorrectionExhausted = "ENGINE_SCOUT_CORRECTION_EXHAUSTED" // N056
	CodeSetBlockedLoadFailed     = "ENGINE_SET_BLOCKED_LOAD_FAILED"    // N057
	CodeSetBlockedSaveFailed     = "ENGINE_SET_BLOCKED_SAVE_FAILED"    // N058
	CodeFixBuildInvalidOpts      = "ENGINE_FIX_BUILD_INVALID_OPTS"     // N059
	CodeFixBuildFailed           = "ENGINE_FIX_BUILD_FAILED"           // N060
	CodeGomodFixupFailed         = "ENGINE_GOMOD_FIXUP_FAILED"         // N061
	CodeCleanupFailed            = "ENGINE_CLEANUP_FAILED"             // N062
	CodeResolveInvalidOpts       = "ENGINE_RESOLVE_INVALID_OPTS"       // N063
	CodeResolveLoadFailed        = "ENGINE_RESOLVE_LOAD_FAILED"        // N064
	CodeResolveGitFailed         = "ENGINE_RESOLVE_GIT_FAILED"         // N065
	CodeResolveNoConflicts       = "ENGINE_RESOLVE_NO_CONFLICTS"       // N066
	CodeResolveBackendFailed     = "ENGINE_RESOLVE_BACKEND_FAILED"     // N067
	CodeResolveFileFailed        = "ENGINE_RESOLVE_FILE_FAILED"        // N068
	CodeResolveCommitFailed      = "ENGINE_RESOLVE_COMMIT_FAILED"      // N069
	CodeResolveFileReadFailed    = "ENGINE_RESOLVE_FILE_READ_FAILED"   // N070
	CodeResolveBackendCallFailed = "ENGINE_RESOLVE_BACKEND_CALL_FAILED" // N071
	CodeResolveFileWriteFailed   = "ENGINE_RESOLVE_FILE_WRITE_FAILED"  // N072
	CodeResolveGitAddFailed      = "ENGINE_RESOLVE_GIT_ADD_FAILED"     // N073
	CodeExportFileExists         = "ENGINE_EXPORT_FILE_EXISTS"         // N074
	CodeExportNoEntries          = "ENGINE_EXPORT_NO_ENTRIES"          // N075
	CodeExportWriteFailed        = "ENGINE_EXPORT_WRITE_FAILED"        // N076
	CodeIntegrationInvalidOpts   = "ENGINE_INTEGRATION_INVALID_OPTS"   // N077
	CodeIntegrationLoadFailed    = "ENGINE_INTEGRATION_LOAD_FAILED"    // N078
	CodeIntegrationNoConnectors  = "ENGINE_INTEGRATION_NO_CONNECTORS"  // N079
	CodeIntegrationPromptFailed  = "ENGINE_INTEGRATION_PROMPT_FAILED"  // N080
	CodeIntegrationBackendFailed = "ENGINE_INTEGRATION_BACKEND_FAILED" // N081
	CodeIntegrationAgentFailed   = "ENGINE_INTEGRATION_AGENT_FAILED"   // N082
	CodeChatInvalidOpts          = "ENGINE_CHAT_INVALID_OPTS"          // N083
	CodeChatFailed               = "ENGINE_CHAT_FAILED"                // N084
)

// Protocol error codes (P001-P099)
const (
	CodeStateTransitionInvalid  = "P001_STATE_TRANSITION_INVALID"
	CodeProgramValidationFailed = "P002_PROGRAM_VALIDATION_FAILED"
	CodeMigrationBoundaryUnsafe = "P003_MIGRATION_BOUNDARY_UNSAFE"
	CodeDepsNotMet              = "P004_DEPS_NOT_MET"
	CodeInvariantViolation      = "P005_INVARIANT_VIOLATION"
	CodeExecutionRule           = "P006_EXECUTION_RULE"
	// P007: wiring gap detected (replaces "E_WIRING" in wiring_validation.go)
	CodeWiringGap = "P007_WIRING_GAP"
)

// Tool/parse error codes (T001-T099)
const (
	CodeToolError    = "T001_TOOL_ERROR"
	CodeParsePanic   = "T002_PARSE_PANIC"
	CodeToolNotFound = "T003_TOOL_NOT_FOUND"
	CodeToolTimeout  = "T004_TOOL_TIMEOUT"
)

