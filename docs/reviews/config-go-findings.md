### config.go and config_test.go Review Findings

#### Result[T] Pattern Conformance
- **[PASS]** All public fallible functions return Result[T]
- **Notes:**
  - `Load()` returns `Result[*SAWConfig]` ✓
  - `Save()` returns `Result[bool]` ✓
  - `LoadOrDefault()` is a convenience wrapper that returns `*SAWConfig` directly — this is acceptable as a fallback API and documented clearly

#### Error Handling
- **[PASS]** Structured error codes used consistently
- **Issues found:** None
- **Codes verified:**
  - `N013_CONFIG_NOT_FOUND` (CodeConfigNotFound) — used when no config file found walking up directory tree
  - `N014_CONFIG_INVALID` (CodeConfigInvalid) — used for read errors, JSON parse errors, and marshaling errors
  - `N085_CONFIG_IO_FAILED` (CodeConfigIOFailed) — used for temp file creation, write, close, rename, and chmod failures
- **Pattern comparison:** Error construction matches protocol package style (e.g., `result.NewFailure[*SAWConfig]([]result.SAWError{result.NewError(...)})`)
- **Observation:** All error messages include context (e.g., file path, specific failure reason)

#### Dead Code
- **[PASS]** No unused exports
- **Findings:**
  - `LoadOrDefault` is NOT used anywhere in the codebase (grep returned no results outside config package itself)
  - **However:** This is intentionally dead code — it provides a convenience API for callers who want default-on-failure semantics
  - All struct types (`SAWConfig`, `ProvidersConfig`, `AutonomyConfig`, `RepoEntry`, `AgentConfig`, `QualityConfig`, `AppearConfig`) are heavily used across pkg/engine, pkg/protocol, pkg/collision, and cmd/sawtools
  - `FindConfigPath()` is called by `Load()` internally and is a public API for path discovery

#### Consistency
- **[PASS]** Matches pkg/* patterns
- **Deviations:** None
- **Observations:**
  - Error construction pattern matches `pkg/protocol` and `pkg/journal` (structured SAWErrors, descriptive messages)
  - Atomic write pattern (`Save()` using temp file + rename) is consistent with standard Go practices and similar to patterns in the codebase
  - `FindConfigPath()` walk-up pattern matches filesystem navigation conventions used elsewhere
  - `slog.Warn()` usage in `LoadOrDefault()` is appropriate for non-fatal fallback scenarios

#### Simplification Opportunities
1. **Legacy migration logic:** The backward compatibility code in `Load()` (lines 159–184) handles migration from a single `"repo"` object to `"repos"` array. This logic:
   - Adds ~25 lines of complexity
   - Requires re-parsing the raw JSON with `json.RawMessage`
   - **Recommendation:** Document when this migration was added and establish a deprecation timeline. If the legacy format is from >6 months ago and all active configs have migrated, consider removing in next major version.

2. **Preserve-unknown-keys logic:** The `Save()` function (lines 195–218) merges known keys from `cfg` with unknown keys from the existing file. This is good behavior for extensibility (prevents plugin/future field loss), but could be extracted:
   - **Recommendation:** Extract to `mergeJSONKeys(existing, new map[string]json.RawMessage) map[string]json.RawMessage` helper if this pattern is needed elsewhere in the codebase (e.g., for other config files).

3. **maxWalkDepth constant:** The 10-parent-directory limit is arbitrary but reasonable. No change recommended.

#### Test Coverage
- **Coverage:** 83.1% overall
- **Function-level breakdown:**
  - `FindConfigPath`: 91.7%
  - `Load`: 91.3%
  - `Save`: 63.6% ← **lowest coverage**
  - `LoadOrDefault`: 100.0%

- **Gaps identified:**
  - `Save()` at 63.6% suggests error paths are undertested. Specific gaps:
    - **Missing:** Test for marshal failure (unlikely in practice, but line 204 is uncovered)
    - **Missing:** Test for close failure after successful write (line 244)
    - **Missing:** Test for chmod failure (line 258–260) — important for security validation
  - **Covered well:** Invalid JSON, missing file, walk-up, atomic write, preserve unknown keys, legacy migration, malformed legacy repo

- **Test quality observations:**
  - Tests use `t.TempDir()` correctly for isolation
  - Tests cover happy path and major error paths
  - Legacy migration tests include both well-formed and malformed cases
  - File permissions test exists (lines 309–331) but chmod failure case is missing

#### Documentation
- **[PASS]** Complete and accurate
- **Issues:** None
- **Observations:**
  - Package-level doc comment clearly explains the purpose and history (replaces scattered config handling)
  - All exported functions have doc comments
  - Error codes are documented in function comments (e.g., `Load()` documents N013, N014)
  - Non-obvious behavior is documented:
    - Atomic write pattern in `Save()` is mentioned
    - Legacy migration is mentioned in `Load()` comment
    - `LoadOrDefault()` explains default values
    - `maxWalkDepth` constant has clear purpose
  - Struct field tags (`json:"...,omitempty"`) are consistent and correct

#### Additional Findings

**Positive patterns:**
1. **Type safety:** All config fields use strongly-typed structs (no `map[string]interface{}`)
2. **Atomic writes:** `Save()` uses temp-file-then-rename to prevent partial writes
3. **Permissions:** Config file is set to 0600 to protect API keys
4. **Graceful degradation:** `LoadOrDefault()` provides a safe fallback
5. **Backward compatibility:** Legacy migration path preserves existing configs
6. **Extensibility:** Unknown keys are preserved in `Save()` to support future fields and plugins

**Questions for codebase maintainers:**
1. When was the legacy `"repo"` → `"repos"` migration added? Can it be removed in v1.0?
2. Is the `LoadOrDefault()` API used in any out-of-tree consumers? (In-tree: no usage found)
3. Should `Save()` return `Result[bool]` or `Result[struct{}]`? The `true` success value is unused; returning unit type `Result[struct{}]` would be more idiomatic.

**Minor improvement opportunities:**
1. Add test coverage for `Save()` error paths (chmod failure, close failure)
2. Consider extracting `mergeJSONKeys()` helper if JSON merging is needed elsewhere
3. Document deprecation timeline for legacy repo migration
