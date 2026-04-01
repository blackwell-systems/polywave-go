### pkg/agent/backend/configfile.go Review Findings

#### Duplication Analysis
- **DUPLICATE**: `SAWProviders` struct (configfile.go:11-25) duplicates fields from `config.ProvidersConfig` (config.go:30-54)
- **Field comparison**:
  - `SAWProviders.Anthropic` (anonymous struct) vs `config.AnthropicProvider` (named type)
  - `SAWProviders.Bedrock` (anonymous struct) vs `config.BedrockProvider` (named type)
  - `SAWProviders.OpenAI` (anonymous struct) vs `config.OpenAIProvider` (named type)
  - Field names and JSON tags are **identical** — no drift detected
- **Recommendation**: **CONSOLIDATE**. `SAWProviders` should be eliminated. Callers should use `config.Load().GetData().Providers` instead of `backend.LoadProvidersFromConfig()`.

#### Usage Analysis
- **Called from**:
  1. `pkg/agent/backend/api/client.go:59` — Anthropic API client credential fallback
  2. `pkg/agent/backend/bedrock/client.go:65` — Bedrock client credential fallback
- **Purpose**: Provides a credential fallback mechanism when explicit API keys are not provided via config structs or environment variables. Both callers use it as a last resort to discover credentials from `saw.config.json`.
- **Why separate from config.Load()?**: Originally, `LoadProvidersFromConfig` had its own `findConfigFile()` implementation. After refactor (commit `e9ae569`), it now calls `config.FindConfigPath()` but still duplicates the parsing logic.

#### Result[T] Migration
- **Current behavior**: Returns zero-value `SAWProviders` on any error (file not found, invalid JSON, unmarshal failure)
- **Recommended**: **MIGRATE TO Result[T]** for consistency with `config.Load()`
- **Breaking change**: **YES**
  - Both call sites currently rely on zero-value-on-error semantics
  - `api/client.go:60` checks `if providers.Anthropic.APIKey != ""`
  - `bedrock/client.go:66-74` checks individual fields for emptiness
  - Callers would need to change from:
    ```go
    providers := backend.LoadProvidersFromConfig(cwd)
    if providers.Anthropic.APIKey != "" {
        apiKey = providers.Anthropic.APIKey
    }
    ```
    To:
    ```go
    r := backend.LoadProvidersFromConfig(cwd)
    if r.IsSuccess() {
        providers := r.GetData()
        if providers.Anthropic.APIKey != "" {
            apiKey = providers.Anthropic.APIKey
        }
    }
    ```

#### Simplification Opportunities
- **Can LoadProvidersFromConfig be eliminated?** **YES**
- **Can it delegate to config.Load()?** **YES**
- **Recommended pattern**:
  ```go
  // Replace:
  providers := backend.LoadProvidersFromConfig(cwd)

  // With:
  r := config.Load(cwd)
  if r.IsSuccess() {
      providers := r.GetData().Providers
      // ... use providers
  }
  ```
- **Benefits**:
  - Eliminates 46 lines of duplicate code (configfile.go)
  - Eliminates 73 lines of duplicate tests (configfile_test.go)
  - Consistent error handling across codebase (all config access uses Result[T])
  - Single config parsing implementation to maintain
- **Drawbacks**: None identified. The zero-value-on-error pattern was a design choice, but `config.Load()` provides better error visibility.

#### Error Handling
- **Silent failure risk**: **MODERATE**
  - Current implementation returns zero values on all errors (file not found, permission denied, invalid JSON, unmarshal failure)
  - Callers check for empty strings, so missing config or parse errors are silently ignored
  - **Scenario**: If `saw.config.json` exists but contains malformed JSON, callers will silently fall back to other credential sources without warning the user
- **Caller assumptions**:
  - Both callers correctly check for empty values before using credentials
  - No risk of nil pointer dereference
  - However, users might be confused if they set up `saw.config.json` incorrectly and credentials aren't used
- **Comparison to config.Load()**: `config.Load()` returns explicit errors with error codes (`N013_CONFIG_NOT_FOUND`, `N014_CONFIG_INVALID`), allowing callers to distinguish "no config" from "broken config"

#### Test Coverage
- **pkg/agent/backend coverage**: 52.9%
- **pkg/config coverage**: 83.1%
- **configfile_test.go tests**:
  1. `TestLoadProvidersFromConfig_NotFound` — missing config file
  2. `TestLoadProvidersFromConfig_Found` — valid config in directory
  3. `TestLoadProvidersFromConfig_WalksUp` — parent directory walk-up
- **Gaps compared to pkg/config tests**:
  - **No invalid JSON test** — `config_test.go:120-133` tests this; `configfile_test.go` does not
  - **No permission error test** — not covered in either test suite
  - **No test for other providers** — only Anthropic tested; Bedrock and OpenAI fields not covered
  - **No test for partial parse** — e.g., valid JSON but missing providers section
- **Overall**: Basic happy-path and walk-up behavior covered, but error handling paths undertested

#### Migration Path
Given that consolidation is strongly recommended, here is a detailed migration plan:

##### Phase 1: Deprecate (Non-Breaking)
1. Add deprecation comment to `LoadProvidersFromConfig`:
   ```go
   // Deprecated: Use config.Load(dir).GetData().Providers instead.
   // This function will be removed in v0.95.0.
   func LoadProvidersFromConfig(dir string) SAWProviders { ... }
   ```
2. Update CHANGELOG to announce deprecation
3. No behavior change in this phase

##### Phase 2: Migrate Callers (Non-Breaking)
1. Update `pkg/agent/backend/api/client.go:56-63`:
   ```go
   // Fall back to saw.config.json
   if apiKey == "" {
       cwd, _ := os.Getwd()
       r := config.Load(cwd)
       if r.IsSuccess() {
           if r.GetData().Providers.Anthropic.APIKey != "" {
               apiKey = r.GetData().Providers.Anthropic.APIKey
           }
       }
   }
   ```
2. Update `pkg/agent/backend/bedrock/client.go:62-76` similarly
3. Remove `backend` import if no longer needed (Bedrock still imports `backend.Config`)
4. Tests continue passing (behavior unchanged)

##### Phase 3: Remove Dead Code (Breaking)
1. Delete `pkg/agent/backend/configfile.go`
2. Delete `pkg/agent/backend/configfile_test.go`
3. Update CHANGELOG for breaking change
4. Release as part of next minor version (v0.94.0 or v0.95.0)

##### Timeline
- **Phase 1**: Immediate (add deprecation notice)
- **Phase 2**: Within 1-2 releases (migrate callers)
- **Phase 3**: Next minor version after migration (remove deprecated code)

##### Backward Compatibility
- Deprecation is non-breaking
- Caller migration is non-breaking (same behavior, different implementation)
- Only Phase 3 is breaking (removal of deprecated code)
- External users: None identified (this is an internal backend package, not exported in SDK)

#### Historical Context
- **Git history**:
  - `80f8cbd` (2024): `configfile.go` added with `findConfigFile()` implementation
  - `972de70` (later): `pkg/config` package created with unified config management
  - `e9ae569` (refactor): `configfile.go` switched from local `findConfigFile()` to `config.FindConfigPath()`
- **Intent**: `configfile.go` was created **before** the unified `pkg/config` package existed. It served as a temporary solution for credential discovery. After `pkg/config` was introduced, the `findConfigFile()` duplication was removed, but the rest of the duplication remained.
- **Conclusion**: This is **accidental duplication**, not intentional. The code should have been fully migrated to `pkg/config` during the `e9ae569` refactor, but only the path-finding logic was updated.

#### Summary Recommendation
**CONSOLIDATE immediately.** The duplication is accidental, not intentional. Both callers can migrate to `config.Load()` without behavior change. The migration path is clear and low-risk.
