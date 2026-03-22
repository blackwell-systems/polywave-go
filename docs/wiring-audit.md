# Wiring Audit: Unwired Code in scout-and-wave-go

Last validated: 2026-03-22 — all critical gaps resolved.

**Date:** 2026-03-22 (last validated: 2026-03-22)
**Scope:** All exported functions, types, and patterns in `pkg/` and `cmd/saw/`

## Summary

| Pattern | Gaps Found |
|---|---|
| Engine functions with no production callers | 0 |
| Observability Store has no concrete impl | 0 |

---

## Advisory Gaps (exported, tested, but no production caller in this repo)

These may be intentionally exported for `scout-and-wave-web` or future use.

### Observability query/rollup functions
- `GetAgentHistory` -- `pkg/observability/query.go`
- `GetFailurePatterns` -- `pkg/observability/query.go`
- `ComputeCostRollup` -- `pkg/observability/rollups.go`
- `ComputeSuccessRateRollup` -- `pkg/observability/rollups.go`
- `ComputeRetryRollup` -- `pkg/observability/rollups.go`
- `ComputeTrend` -- `pkg/observability/rollups.go`

### Protocol helper functions (exported API surface)
- `ValidateCompletionReportClaims` -- completion report cross-validation (15 tests, 0 production callers)
- `ValidateGateInputs` -- gate input validation
- `LoadProjectMemory` / `SaveProjectMemory` / `AddCompletedFeature` -- project memory CRUD
- `GenerateManifestSchema` / `ValidateManifestJSON` -- JSON Schema validation
- `SetFreezeTimestamp` -- freeze timestamp setter
- `IsSoloWave` / `IsWaveComplete` / `IsFinalWave` -- wave helper predicates
- `LookupErrorCode` / `AllErrorCodes` / `MigrateErrorCode` -- error code utilities
- `ShouldRetry` / `MaxRetriesWithReactions` / `ShouldRetryWithReactions` / `ValidFailureType` -- failure routing
- `ValidateBytes` -- raw YAML validation entry point

### Orchestrator functions
- `RouteFailureWithReactions` / `MaxAttemptsFor` -- failure routing with reaction support
- `WriteJournalEntry` -- journal integration helper

### Tools package (SDK surface for external consumers)
- `NewAnthropicAdapter` / `NewOpenAIAdapter` / `NewBedrockAdapter` -- tool format adapters
- `PermissionMiddleware` / `WithPermissions` -- tool permission middleware
- `ConstrainedTools` -- constrained tool workshop

### Pipeline package
- `DefaultRegistry` / `WavePipeline` -- pipeline registry and wave pipeline definition

### Other
- `NewGateResultCache` -- gate result cache (tests only)
- `NewJournalIntegration` -- journal integration (tests only)
- `CleanupExpired` / `ListArchives` -- journal archive management
- `WriteRequirementsFile` -- interview requirements writer
- `ValidateRequiredField` / `HandleBackCommand` -- interview helpers
- `SupportedLanguages` -- build diagnostics language list
- `BuildWaveConstraints` / `BuildIntegratorConstraints` -- constraint builders (tests only)

---

## Resolved Items (validated 2026-03-22)

These were previously listed as gaps and have been confirmed wired:

- **C1** `SetPrioritizeAgentsFunc` -- wired in `pkg/engine/engine.go:29`
- **C2** `ClosedLoopGateRetry` -- called from `cmd/saw/finalize_wave.go:223`
- **C4** `AdvanceTierAutomatically` -- called from `pkg/engine/program_tier_loop.go:256`
- **C5** `AutoTriggerReplan` -- called from `pkg/engine/program_tier_loop.go:216`
- **C6** `SyncProgramStatusFromDisk` -- called from `cmd/saw/program_status_cmd.go:50`
- **C7** All 4 CLI commands registered in `cmd/saw/main.go:81-84`
- **C3** `FinalizeIMPLEngine` -- called from scout-and-wave-web
  pkg/api/bootstrap_handler.go:160 (verified 2026-03-22)
- **C8** `Store` interface -- SQLite implementation added in
  pkg/observability/sqlite/sqlite.go, wired into CLI via
  openStore() in cmd/saw/observability_metrics.go (resolved 2026-03-22)
- **C9** `ScoutCorrectionLoop` -- called from `cmd/saw/run_scout_cmd.go:137`
- **C10** `RunSingleWave` -- called from scout-and-wave-web
  pkg/api/wave_runner.go:236 (verified 2026-03-22)
- **C10** `RunSingleAgent` -- called from scout-and-wave-web
  pkg/api/resume_action.go:206 (verified 2026-03-22)
