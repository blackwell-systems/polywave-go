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
//	S001-S099: Suitability error codes
//	C001-C099: Collision detection errors
//	K001-K099: Cache errors (gatecache operations)
//	I001-I099: Agent ID generation errors
//	D001-D099: Dependency check errors (lock file parsing, missing deps)
//	E001-E099: Command extraction errors (workflow/package parsing, toolchain detection)
//	X001-X099: Scout automation static analysis errors (check-callers, check-test-cascade, suggest-wave-structure)
//	Q001-Q099: Queue errors
//	R001-R099: Retry errors
//	J001-J099: Journal errors (archive, checkpoint, observer operations)
//	B009: gate validation failed (closed-loop gate retry input validation)
//	N091-N098: engine init, pipeline, and step failures
//	P008-P013: type collision fatal, critic gate failed, tier conflict detected, wave not found, unknown agent in ownership, amend blocked
//	K007: cache build key cancelled
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
	// B009: gate validation failed (closed-loop gate retry input validation)
	CodeGateValidationFailed = "B009_GATE_VALIDATION_FAILED"
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
	CodeWorktreeRemoveFailed = "G008_WORKTREE_REMOVE_FAILED"
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

	// N018-N087: Engine operation codes (renamed to follow Nxxx_DESCRIPTION pattern)

	// CodeContextCancelled is emitted when an operation is cancelled via context.
	CodeContextCancelled = "N018_CONTEXT_CANCELLED"
	// CodeDispatchNoAdapters is emitted by Dispatcher.Dispatch when no adapters are registered.
	CodeDispatchNoAdapters = "N086_DISPATCH_NO_ADAPTERS"
	// CodeDispatchAllFailed is emitted by Dispatcher.Dispatch when all adapters fail.
	CodeDispatchAllFailed = "N087_DISPATCH_ALL_FAILED"

	// Scout operation codes
	CodeScoutInvalidOpts         = "N019_SCOUT_INVALID_OPTS"
	CodeScoutRunFailed           = "N020_SCOUT_RUN_FAILED"
	CodeScoutBoundaryViolation   = "N021_SCOUT_BOUNDARY_VIOLATION"
	CodePlannerInvalidOpts       = "N022_PLANNER_INVALID_OPTS"
	CodePlannerFailed            = "N023_PLANNER_FAILED"
	CodeWaveInvalidOpts          = "N024_WAVE_INVALID_OPTS"
	CodeWaveFailed               = "N025_WAVE_FAILED"
	CodeWaveSequencingFailed     = "N026_WAVE_SEQUENCING_FAILED"
	CodeHookVerifyFailed         = "N027_HOOK_VERIFY_FAILED"
	CodeScaffoldRunFailed        = "N028_SCAFFOLD_RUN_FAILED"
	CodeAgentRunFailed           = "N029_AGENT_RUN_FAILED"
	CodeAgentRunInvalidOpts      = "N030_AGENT_RUN_INVALID_OPTS"
	CodeMergeWaveFailed          = "N031_MERGE_WAVE_FAILED"
	CodeMergeWaveInvalidOpts     = "N032_MERGE_WAVE_INVALID_OPTS"
	CodeEngineVerificationFailed = "N033_ENGINE_VERIFICATION_FAILED"
	CodeUpdateStatusFailed       = "N034_UPDATE_STATUS_FAILED"
	CodeValidateFailed           = "N035_VALIDATE_FAILED"
	CodeJournalArchiveFailed     = "N036_JOURNAL_ARCHIVE_FAILED"
	CodeMarkCompleteFailed       = "N037_MARK_COMPLETE_FAILED"
	CodeMarkCompleteInvalidOpts  = "N038_MARK_COMPLETE_INVALID_OPTS"
	CodeVerifyTiersIncomplete    = "N039_VERIFY_TIERS_INCOMPLETE"
	CodeMarkerReadFailed         = "N040_MARKER_READ_FAILED"
	CodeMarkerWriteFailed        = "N041_MARKER_WRITE_FAILED"
	CodeUpdateProgParseFailed    = "N042_UPDATE_PROG_PARSE_FAILED"
	CodeUpdateProgSlugNotFound   = "N043_UPDATE_PROG_SLUG_NOT_FOUND"
	CodeSyncParseFailed          = "N044_SYNC_PARSE_FAILED"
	CodeSyncStatusFailed         = "N045_SYNC_STATUS_FAILED"
	CodeWriteManifestFailed      = "N046_WRITE_MANIFEST_FAILED"
	CodeRestoreLoadFailed        = "N047_RESTORE_LOAD_FAILED"
	CodeRestoreSaveFailed        = "N048_RESTORE_SAVE_FAILED"
	CodeTestLoadFailed           = "N049_TEST_LOAD_FAILED"
	CodeTestNoCommand            = "N050_TEST_NO_COMMAND"
	CodeTestPipeFailed           = "N051_TEST_PIPE_FAILED"
	CodeTestStartFailed          = "N052_TEST_START_FAILED"
	CodeTestCommandFailed        = "N053_TEST_COMMAND_FAILED"
	CodeScoutRunnerFailed        = "N054_SCOUT_RUNNER_FAILED"
	CodeScoutValidationFailed    = "N055_SCOUT_VALIDATION_FAILED"
	CodeScoutCorrectionExhausted = "N056_SCOUT_CORRECTION_EXHAUSTED"
	CodeSetBlockedLoadFailed     = "N057_SET_BLOCKED_LOAD_FAILED"
	CodeSetBlockedSaveFailed     = "N058_SET_BLOCKED_SAVE_FAILED"
	CodeFixBuildInvalidOpts      = "N059_FIX_BUILD_INVALID_OPTS"
	CodeFixBuildFailed           = "N060_FIX_BUILD_FAILED"
	CodeGomodFixupFailed         = "N061_GOMOD_FIXUP_FAILED"
	CodeCleanupFailed            = "N062_CLEANUP_FAILED"
	CodeResolveInvalidOpts       = "N063_RESOLVE_INVALID_OPTS"
	CodeResolveLoadFailed        = "N064_RESOLVE_LOAD_FAILED"
	CodeResolveGitFailed         = "N065_RESOLVE_GIT_FAILED"
	CodeResolveNoConflicts       = "N066_RESOLVE_NO_CONFLICTS"
	CodeResolveBackendFailed     = "N067_RESOLVE_BACKEND_FAILED"
	CodeResolveFileFailed        = "N068_RESOLVE_FILE_FAILED"
	CodeResolveCommitFailed      = "N069_RESOLVE_COMMIT_FAILED"
	CodeResolveFileReadFailed    = "N070_RESOLVE_FILE_READ_FAILED"
	CodeResolveBackendCallFailed = "N071_RESOLVE_BACKEND_CALL_FAILED"
	CodeResolveFileWriteFailed   = "N072_RESOLVE_FILE_WRITE_FAILED"
	CodeResolveGitAddFailed      = "N073_RESOLVE_GIT_ADD_FAILED"
	CodeExportFileExists         = "N074_EXPORT_FILE_EXISTS"
	CodeExportNoEntries          = "N075_EXPORT_NO_ENTRIES"
	CodeExportWriteFailed        = "N076_EXPORT_WRITE_FAILED"
	CodeIntegrationInvalidOpts   = "N077_INTEGRATION_INVALID_OPTS"
	CodeIntegrationLoadFailed    = "N078_INTEGRATION_LOAD_FAILED"
	CodeIntegrationNoConnectors  = "N079_INTEGRATION_NO_CONNECTORS"
	CodeIntegrationPromptFailed  = "N080_INTEGRATION_PROMPT_FAILED"
	CodeIntegrationBackendFailed = "N081_INTEGRATION_BACKEND_FAILED"
	CodeIntegrationAgentFailed   = "N082_INTEGRATION_AGENT_FAILED"
	CodeChatInvalidOpts          = "N083_CHAT_INVALID_OPTS"
	CodeChatFailed               = "N084_CHAT_FAILED"

	// N085: config file I/O failed (permission set, write, rename)
	CodeConfigIOFailed = "N085_CONFIG_IO_FAILED"

	// N088: session save/load failed in pkg/resume (SaveAgentSession, LoadAgentSessions, DetectWithConfig)
	CodeSessionSaveFailed = "N088_SESSION_SAVE_FAILED"
	// N089: interview save failed (replaces inline "INTERVIEW_SAVE_FAILED" in pkg/interview)
	CodeInterviewSaveFailed = "N089_INTERVIEW_SAVE_FAILED"
	// N090: requirements write failed (replaces inline "REQUIREMENTS_WRITE_FAILED" in pkg/interview)
	CodeRequirementsWriteFailed = "N090_REQUIREMENTS_WRITE_FAILED"
	// N091: engine init failed
	CodeEngineInitFailed = "N091_ENGINE_INIT_FAILED"
	// N092: engine already initialized
	CodeEngineAlreadyInitialized = "N092_ENGINE_ALREADY_INITIALIZED"
	// N093: finalize step failed
	CodeFinalizeStepFailed = "N093_FINALIZE_STEP_FAILED"
	// N094: manifest save failed
	CodeManifestSaveFailed = "N094_MANIFEST_SAVE_FAILED"
	// N095: completion report set failed
	CodeReportSetFailed = "N095_REPORT_SET_FAILED"
	// N096: pipeline Run aborted (step failure or context cancellation)
	CodePipelineRunFailed = "N096_PIPELINE_RUN_FAILED"
	// N097: pipeline step failed (nil Func, context cancel, retry exhausted)
	CodeStepExecutionFailed = "N097_STEP_EXECUTION_FAILED"
	// N098: required pipeline state key absent or nil
	CodeRequiredKeyMissing = "N098_REQUIRED_KEY_MISSING"
)

// Queue error codes (Q001-Q099)
const (
	CodeQueueAddFailed           = "Q001_QUEUE_ADD_FAILED"
	CodeQueueListFailed          = "Q002_QUEUE_LIST_FAILED"
	CodeQueueEmpty               = "Q003_QUEUE_EMPTY"
	CodeQueueStatusUpdateFailed  = "Q004_QUEUE_STATUS_UPDATE_FAILED"
	CodeQueueCompletedScanFailed = "Q005_QUEUE_COMPLETED_SCAN_FAILED"
	CodeQueueCorruptedFile       = "Q006_QUEUE_CORRUPTED_FILE"
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
	// P008: E41 type collision fatal — fatal DetectCollisions infrastructure failure
	CodeTypeCollisionFatal = "P008_TYPE_COLLISION_FATAL"
	// P009: E37 critic gate failed
	CodeCriticGateFailed = "P009_CRITIC_GATE_FAILED"
	// P010: P1+ tier conflict detected
	CodeTierConflictDetected = "P010_TIER_CONFLICT_DETECTED"
	// P011: wave not found during merge
	CodeWaveNotFound = "P011_WAVE_NOT_FOUND"
	// P012: unknown agent referenced in file ownership
	CodeUnknownAgentInOwnership = "P012_UNKNOWN_AGENT_IN_OWNERSHIP"
	// P013: amend blocked due to state
	CodeAmendBlocked = "P013_AMEND_BLOCKED"
)

// Tool/parse error codes (T001-T099)
const (
	CodeToolError    = "T001_TOOL_ERROR"
	CodeParsePanic   = "T002_PARSE_PANIC"
	CodeToolNotFound = "T003_TOOL_NOT_FOUND"
	CodeToolTimeout  = "T004_TOOL_TIMEOUT"
	// CodeToolAlreadyRegistered is emitted when Workshop.Register receives
	// a tool name that is already registered.
	CodeToolAlreadyRegistered = "T005_TOOL_ALREADY_REGISTERED"
)

// Suitability error codes (S001-S099)
const (
	CodeSuitabilityRepoRootEmpty     = "S001_REPO_ROOT_EMPTY"
	CodeSuitabilityClassifyFailed    = "S002_CLASSIFY_FAILED"
	CodeSuitabilityFileStatFailed    = "S003_FILE_STAT_FAILED"
	CodeSuitabilityFileReadFailed    = "S004_FILE_READ_FAILED"
	CodeSuitabilityRequirementsRead  = "S005_REQUIREMENTS_READ"
	CodeSuitabilityRequirementsParse = "S006_REQUIREMENTS_PARSE"
)

// Collision error codes (C001-C099)
const (
	CodeCollisionLoadManifestFailed = "C001_LOAD_MANIFEST_FAILED"
	CodeCollisionInvalidWave        = "C002_INVALID_WAVE"
	CodeCollisionGetFilesFailed     = "C003_GET_FILES_FAILED"
	CodeCollisionExtractTypesFailed = "C004_EXTRACT_TYPES_FAILED"
	CodeCollisionGitDiffFailed      = "C005_GIT_DIFF_FAILED"
	CodeCollisionParseFailed        = "C006_PARSE_FAILED"
	CodeCollisionKeyParseFailed     = "C007_KEY_PARSE_FAILED"
	CodeCollisionGitShowFailed      = "C008_GIT_SHOW_FAILED"
	CodeCollisionContextCancelled   = "C009_CONTEXT_CANCELLED"
	CodeCollisionBranchNotFound     = "C010_BRANCH_NOT_FOUND"
	CodeCollisionInvalidInput       = "C011_INVALID_INPUT"
)

// Cache error codes (K001-K099)
const (
	// CodeCacheMiss is emitted when Get finds no entry or entry is expired
	CodeCacheMiss = "K001_CACHE_MISS"
	// CodeCacheBuildKeyFailed is emitted when BuildKey/BuildKeyForGate fails
	CodeCacheBuildKeyFailed = "K002_CACHE_BUILD_KEY_FAILED"
	// CodeCachePutFailed is emitted when Put fails to save cache to disk
	CodeCachePutFailed = "K003_CACHE_PUT_FAILED"
	// CodeCacheInvalidateFailed is emitted when Invalidate fails to remove cache file
	CodeCacheInvalidateFailed = "K004_CACHE_INVALIDATE_FAILED"
	// CodeCachePutCancelled is emitted when Put is cancelled via context
	CodeCachePutCancelled = "K005_CACHE_PUT_CANCELLED"
	// CodeCacheInvalidateCancelled is emitted when Invalidate is cancelled
	CodeCacheInvalidateCancelled = "K006_CACHE_INVALIDATE_CANCELLED"
	// CodeCacheBuildKeyCancelled is emitted when BuildKey/BuildKeyForGate is cancelled via context
	CodeCacheBuildKeyCancelled = "K007_CACHE_BUILD_KEY_CANCELLED"
)

// Agent ID generation error codes (I001-I099)
const (
	CodeAgentCountInvalid        = "I001_AGENT_COUNT_INVALID"
	CodeAgentCountMismatch       = "I002_AGENT_COUNT_MISMATCH"
	CodeAgentLimitExceeded       = "I003_AGENT_LIMIT_EXCEEDED"
	CodeCategoryLimitExceeded    = "I004_CATEGORY_LIMIT_EXCEEDED"
	CodeCategoryCountExceeded    = "I005_CATEGORY_COUNT_EXCEEDED"
	CodeInvalidAgentIDGenerated  = "I006_INVALID_AGENT_ID_GENERATED"
)

// Dependency check error codes (D001-D099)
const (
	CodeDepLockFileOpen       = "D001_LOCK_FILE_OPEN"
	CodeDepLockFileParse      = "D002_LOCK_FILE_PARSE"
	CodeDepMissingDeps        = "D003_MISSING_DEPS"
	CodeDepVersionConflict    = "D004_VERSION_CONFLICT"
	CodeDepInvalidToml        = "D005_INVALID_TOML"
	CodeDepMalformedPackage   = "D006_MALFORMED_PACKAGE"
	CodeDepUnsupportedVersion = "D007_UNSUPPORTED_VERSION"
	CodeDepEmptyPackage       = "D008_EMPTY_PACKAGE"
	CodeDepGoModRead          = "D009_GOMOD_READ"
	CodeDepGoModParse         = "D010_GOMOD_PARSE"
	CodeDepRepoRootInvalid    = "D011_REPO_ROOT_INVALID"
)

// Command extraction error codes (E001-E099)
const (
	CodeCommandExtractWorkflowRead  = "E001_WORKFLOW_READ"
	CodeCommandExtractWorkflowParse = "E002_WORKFLOW_PARSE"
	CodeCommandExtractPackageRead   = "E003_PACKAGE_READ"
	CodeCommandExtractPackageParse  = "E004_PACKAGE_PARSE"
	CodeCommandExtractNoToolchain   = "E005_NO_TOOLCHAIN"
	CodeCommandExtractCancelled     = "E006_EXTRACT_CANCELLED"
)

// Retry error codes (R001-R099)
const (
	CodeRetryLoadManifestFailed  = "R001_LOAD_MANIFEST_FAILED"   // protocol.Load failed
	CodeRetryReportMissing       = "R002_REPORT_MISSING"         // agent has no completion report
	CodeRetrySaveIMPLFailed      = "R003_SAVE_IMPL_FAILED"       // failed to write retry IMPL to disk
	CodeRetryIMPLDirCreateFailed = "R004_IMPL_DIR_CREATE_FAILED" // os.MkdirAll failed
)

// Scout automation error codes (X001-X099)
const (
	// CodeCheckCallerInvalidInput is emitted when check-callers receives invalid input
	CodeCheckCallerInvalidInput = "X001_INVALID_INPUT"
	// CodeCheckCallerFileRead is emitted when a file cannot be read during caller scan
	CodeCheckCallerFileRead = "X002_FILE_READ"
	// CodeTestCascadeOrphan is emitted when a test file calls a changed symbol but is not assigned to any agent
	CodeTestCascadeOrphan = "X003_TEST_CASCADE_ORPHAN"
	// CodeWaveStructureCallerBefore is emitted when callers of a changed symbol are in an earlier wave
	CodeWaveStructureCallerBefore = "X004_WAVE_STRUCTURE_CALLER_BEFORE"
	// CodeWaveStructureMissingDep is emitted when callers in a later wave lack depends_on for the changing agent
	CodeWaveStructureMissingDep = "X005_WAVE_STRUCTURE_MISSING_DEP"
)

// Journal error codes (J001-J099)
const (
	CodeJournalArchiveCreateFailed  = "J001_ARCHIVE_CREATE_FAILED"
	CodeJournalArchiveMetaFailed    = "J002_ARCHIVE_META_FAILED"
	CodeJournalArchiveExtractFailed = "J003_ARCHIVE_EXTRACT_FAILED"
	CodeJournalArchiveListFailed    = "J004_ARCHIVE_LIST_FAILED"
	CodeJournalArchiveCleanupFailed = "J005_ARCHIVE_CLEANUP_FAILED"
	CodeJournalCheckpointInvalidName   = "J006_CHECKPOINT_INVALID_NAME"
	CodeJournalCheckpointCreateFailed  = "J007_CHECKPOINT_CREATE_FAILED"
	CodeJournalCheckpointAlreadyExists = "J008_CHECKPOINT_ALREADY_EXISTS"
	CodeJournalCheckpointNotFound      = "J009_CHECKPOINT_NOT_FOUND"
	CodeJournalCheckpointCopyFailed    = "J010_CHECKPOINT_COPY_FAILED"
	CodeJournalCheckpointDeleteFailed  = "J011_CHECKPOINT_DELETE_FAILED"
	CodeJournalObserverCursorFailed    = "J012_OBSERVER_CURSOR_FAILED"
	CodeJournalObserverAppendFailed    = "J013_OBSERVER_APPEND_FAILED"
	CodeJournalObserverUpdateFailed    = "J014_OBSERVER_UPDATE_FAILED"
)

