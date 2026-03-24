# Wiring Audit: Unwired Code in scout-and-wave-go

Last reviewed: 2026-03-24

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

### Protocol helper functions (exported API surface)
- `ValidateCompletionReportClaims` -- completion report cross-validation (tests only)
- `ValidateGateInputs` -- gate input validation (tests only)
- `LoadProjectMemory` / `SaveProjectMemory` / `AddCompletedFeature` -- project memory CRUD (tests only)
- `GenerateManifestSchema` / `ValidateManifestJSON` -- JSON Schema validation (tests only)
- `SetFreezeTimestamp` -- freeze timestamp setter (tests only)
- `IsSoloWave` / `IsWaveComplete` / `IsFinalWave` -- wave helper predicates (tests only)
- `LookupErrorCode` / `AllErrorCodes` / `MigrateErrorCode` -- error code utilities (tests only)
- `ShouldRetry` / `MaxRetriesWithReactions` / `ShouldRetryWithReactions` / `ValidFailureType` -- failure routing (tests only)
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
- `NewGateResultCache` -- gate result cache (tests only)
- `NewJournalIntegration` -- journal integration (tests only)
- `CleanupExpired` / `ListArchives` -- journal archive management (tests only)
- `WriteRequirementsFile` -- interview requirements writer (tests only)
- `ValidateRequiredField` / `HandleBackCommand` -- interview helpers (tests only)
- `SupportedLanguages` -- build diagnostics language list (tests only)
- `BuildWaveConstraints` / `BuildIntegratorConstraints` -- constraint builders (tests only)
