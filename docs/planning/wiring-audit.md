# Wiring Audit: Unwired Code in scout-and-wave-go

Last reviewed: 2026-03-25

**Scope:** All exported functions, types, and patterns in `pkg/` and `cmd/sawtools/`

## Summary

All critical gaps (C1-C10) previously identified have been resolved.
The items below are advisory: exported and tested, but with no production
caller in either `scout-and-wave-go` or `scout-and-wave-web`. They may be
intentionally exported as SDK surface for future consumers.

---

## Advisory Gaps (exported, tested, but no production caller)

### Observability query/rollup functions
- `GetAgentHistory` -- `pkg/observability/query.go`
- `GetFailurePatterns` -- `pkg/observability/query.go`
- `ComputeTrend` -- `pkg/observability/rollups.go`

> `ComputeCostRollup`, `ComputeSuccessRateRollup`, and `ComputeRetryRollup`
> are now wired via `scout-and-wave-web/pkg/api/observability.go` (resolved 2026-03-24).
> `cmd/sawtools/observability_query.go` exposes `query events` but uses
> `store.QueryEvents` directly; the higher-level rollup functions above remain
> uncalled in production.

### Protocol helper functions (exported API surface)
- `ValidateCompletionReportClaims` -- completion report cross-validation (tests only)
- `ValidateGateInputs` -- gate input validation (tests only)
- `LoadProjectMemory` / `SaveProjectMemory` / `AddCompletedFeature` -- project memory CRUD (tests only)
- `GenerateManifestSchema` / `ValidateManifestJSON` -- JSON Schema validation (tests only)
- `SetFreezeTimestamp` -- freeze timestamp setter (tests only)
- `IsSoloWave` / `IsWaveComplete` / `IsFinalWave` -- wave helper predicates (tests only; defined in `pkg/protocol/helpers.go`, no caller in engine or cmd)
- `LookupErrorCode` / `AllErrorCodes` / `MigrateErrorCode` -- error code utilities (tests only)
- `ShouldRetry` / `MaxRetriesWithReactions` / `ShouldRetryWithReactions` / `ValidFailureType` -- failure routing (tests only; no caller in engine or cmd)
- `ValidateBytes` -- raw YAML validation entry point (tests only)

### Orchestrator functions
- `RouteFailureWithReactions` / `MaxAttemptsFor` -- failure routing with reaction support (tests only)
- `WriteJournalEntry` -- journal integration helper (tests only)

### Tools package (SDK surface for external consumers)
- `NewAnthropicAdapter` / `NewOpenAIAdapter` / `NewBedrockAdapter` -- tool format adapters
- `PermissionMiddleware` / `WithPermissions` -- tool permission middleware
- `ConstrainedTools` -- constrained tool workshop (tests only)

### Pipeline package
- `DefaultRegistry` / `WavePipeline` -- pipeline registry and wave pipeline definition (tests only)

### Other
- `NewGateResultCache` -- gate result cache (defined in `pkg/engine/gate_cache.go`, tests only; no production caller in engine, cmd, or web)
- `NewJournalIntegration` -- journal integration (defined in `pkg/engine/runner.go`, tests only; no caller in cmd or web)
- `CleanupExpired` / `ListArchives` -- journal archive management (tests only)
- `WriteRequirementsFile` -- interview requirements writer (exported from `pkg/interview/compiler.go`; the `interview` cmd uses `mgr.Compile` which internally delegates via `CompileToRequirements`; `WriteRequirementsFile` itself has no production caller)
- `ValidateRequiredField` / `HandleBackCommand` -- interview helpers (called within `pkg/interview/deterministic.go` production code, not test-only; exported but no external caller outside the package)
- `SupportedLanguages` -- build diagnostics language list (`pkg/builddiag/diagnose.go`; no caller in cmd or web)
- `BuildWaveConstraints` / `BuildIntegratorConstraints` -- constraint builders (defined in `pkg/engine/constraints.go`; no caller in cmd or web)
