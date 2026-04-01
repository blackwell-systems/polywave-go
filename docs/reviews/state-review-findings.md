### state.go and state_test.go Review Findings

#### Result[T] Pattern Conformance

✅ **PASS** — Both public functions conform to Result[T] pattern:
- `GetWaveState()` returns `result.Result[*WaveState]`
- `GetAllWaveStates()` returns `result.Result[[]WaveState]`

All error paths use `result.NewFailure()` with proper `result.SAWError` construction.

#### Error Handling

🟡 **ISSUE FOUND** — Error codes are mostly correct but have consistency issues:

1. **CodeWaveNotReady vs CodeConfigInvalid usage**
   - `CodeConfigInvalid` (N014) is used for nil manifest (correct)
   - `CodeWaveNotReady` (N007) is used for "wave not found in manifest" (line 41)
   - **Analysis**: `CodeWaveNotReady` is defined as "Wave is not ready for execution" but is being used here for "wave doesn't exist in manifest". These are different semantic conditions.
   - **Recommendation**: Create a more specific code like `CodeWaveNotFound` or use `CodeManifestInvalid` (V001) for structural issues with the manifest data.

2. **Error wrapping in GetAllWaveStates**
   - Lines 111-115: Error wrapping logic creates new SAWError structs and wraps the message with `fmt.Sprintf("wave %d: %s", w.Number, e.Message)`
   - **Issue**: This is good practice for debugging but loses the original error context beyond the message string
   - **Recommendation**: Consider using `SAWError.WithContext()` method if available, or add wave number as structured context rather than string interpolation

#### Dead Code

🔴 **CRITICAL ISSUE** — This code appears to be **completely unused or broken**:

**Key format inconsistency**:
- `state.go` line 55: Uses `reportKey := fmt.Sprintf("wave%d-%s", waveNum, id)` to create keys like `"wave1-A"`
- `pkg/protocol` everywhere else: Uses `manifest.CompletionReports[agent.ID]` directly (just `"A"`, not `"wave1-A"`)

**Evidence**:
- Searched entire codebase: `config.WaveState`, `config.GetWaveState`, `config.GetAllWaveStates` — **NO external callers found**
- Only references are in `pkg/config/state_test.go` and historical IMPL docs
- The IMPL doc `IMPL-config-hardening.yaml` lists Issue 6: "remove dead GateResults field" and Issue 2: "nil manifest guard" for WaveState

**Conclusion**: This code was created as part of `IMPL-config-hardening.yaml` but appears to have never been integrated into the actual orchestration flow. The key format mismatch (`wave1-A` vs `A`) means it would never successfully read completion reports from manifests written by the rest of the system.

**Impact**: Either this is:
1. Future planned functionality that needs the key format fixed before integration, OR
2. Dead code that should be removed

#### Consistency

🔴 **FAIL** — Multiple consistency issues:

1. **Status classification logic mismatch with pkg/protocol**
   - Lines 61-71: status classification switch statement
   - `StatusBlocked` → failed ✅
   - `StatusPartial` → failed ✅
   - `StatusComplete` → completed ✅
   - `default` → pending ✅
   - **Finding**: Logic matches `pkg/protocol/types.go` definitions correctly
   - **BUT**: See key format issue above — this never finds real reports

2. **IsComplete calculation**
   - Line 85: `isComplete := len(completed) == len(agentIDs) && len(failed) == 0`
   - **Analysis**: Correct — requires ALL agents complete AND zero failures
   - **Edge case**: Treats unknown status as pending (not a blocker), which is reasonable

3. **MergeState field inclusion**
   - Line 94: `MergeState: string(manifest.MergeState)`
   - Searched usage: `MergeState` is used in `pkg/protocol` for tracking post-wave merge status
   - **Finding**: Including it in WaveState is reasonable for status queries, though it's manifest-level metadata, not wave-level

4. **Key format breaks all integrations**
   - `state.go` creates keys like `"wave1-A"` but protocol uses `"A"`
   - This is a **showstopper bug** that prevents this code from ever working with real manifests
   - The test suite passes because tests use the same wrong key format

#### Simplification Opportunities

1. **Extract agent classification to helper function**
   - Lines 53-72: agent classification loop could be `classifyAgents(agentIDs, reports, waveNum)`
   - Benefit: Testable in isolation, reusable if needed elsewhere
   - Priority: LOW (code is clear as-is)

2. **Nil-slice initialization unnecessary**
   - Lines 74-83: Explicit nil-to-empty-slice conversion
   - `json.Marshal` already handles nil slices as `[]` in JSON output
   - Could be removed without changing behavior
   - **Counter-argument**: Explicit initialization makes behavior predictable and documents intent
   - Priority: LOW (acceptable either way)

3. **Move WaveState to pkg/protocol**
   - WaveState calculation logic is tightly coupled to IMPLManifest structure
   - `pkg/protocol` already has `CurrentWave()` function doing similar iteration
   - Co-locating related logic would improve discoverability
   - **Trade-off**: Would increase pkg/protocol surface area
   - Priority: MEDIUM

4. **Fix key format OR remove this code**
   - **Option A**: Change line 55 to `reportKey := id` (remove wave number prefix)
   - **Option B**: Change protocol package to use `wave{N}-{ID}` format everywhere (breaking change)
   - **Option C**: Remove this code entirely if it's not used
   - Priority: **CRITICAL** (must be resolved before any integration)

#### Test Coverage

📊 **Coverage: 83.1%** (from `go test -cover`)

**Coverage by scenario**:
- ✅ Valid wave lookup
- ✅ Wave not found
- ✅ All agents complete
- ✅ Partial completion
- ✅ Agent failures (blocked + partial)
- ✅ Nil manifest error paths
- ✅ Multi-wave aggregation
- ✅ Error wrapping behavior documented

**Missing coverage**:
1. **Empty waves** (wave with zero agents) — not tested
2. **Unknown status handling** (line 70 default case) — covered indirectly but no explicit test
3. **Edge case**: Wave exists but manifest.CompletionReports is nil — covered by initialization logic but not explicitly tested

**Test quality notes**:
- Tests use inline test helpers (`makeManifest`, `twoAgentWave`) — good
- `TestGetAllWaveStates_ErrorWrapping_NilManifestMessage` has excellent documentation explaining why it tests nil-manifest path instead of per-wave errors
- Test coverage is solid for the functionality, but the functionality itself has a critical bug (key format)

#### Naming Consistency

🟡 **MINOR ISSUES**:

1. **WaveState vs WaveStatus vs WaveProgress**
   - Current: `WaveState`
   - Compare: `protocol.Wave`, `protocol.CompletionReport`, `protocol.CompletionStatus`
   - **Analysis**: `WaveState` is consistent with "state" terminology used elsewhere (`manifest.State`, `MergeState`)
   - **Alternative**: `WaveProgress` might be more descriptive since it's specifically about completion tracking
   - **Verdict**: Acceptable as-is

2. **GetWaveState vs GetAllWaveStates**
   - Follows Go convention: singular `Get` + plural `GetAll`
   - ✅ Consistent with codebase patterns

3. **WaveState struct field names**
   - `CompletedAgents`, `FailedAgents`, `PendingAgents` — clear and parallel
   - `IsComplete` — boolean flag follows Go convention
   - `MergeState` — matches `protocol.MergeState` type name
   - ✅ Good naming

#### Summary

**Critical Issues**:
1. 🔴 **Key format bug**: `"wave1-A"` vs `"A"` makes this code incompatible with the rest of the system
2. 🔴 **Dead code**: No callers found outside test suite
3. 🔴 **Integration needed**: If this is intended for use, it must be wired into the orchestration layer

**Recommendation**:
- **If keeping this code**: Fix key format to use `agent.ID` directly (remove `wave%d-` prefix), then integrate into orchestrator/engine status query paths
- **If not keeping**: Remove `state.go` and `state_test.go` entirely to avoid confusion
- **Check with team**: Was Issue 6 from IMPL-config-hardening.yaml about removing `GateResults` or the entire `WaveState` struct?

**Test quality**: ✅ Excellent (83.1% coverage, edge cases well-documented)
**Code quality**: ✅ Good (Result[T] pattern, error handling, nil checks)
**Integration status**: 🔴 Not integrated, possibly abandoned
