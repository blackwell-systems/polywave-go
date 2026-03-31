# Item 3 Verification Report: %w Sweep

**Date:** 2026-03-31
**Agent:** B (Wave 1, final-cleanup)
**Scope:** Verify whether any `fmt.Errorf` calls use `%v` to wrap an `error` value
instead of `%w`, and determine if code changes are needed.

---

## Summary

**No code changes are needed.** The claim that 229 `fmt.Errorf` calls incorrectly use
`%v` for error wrapping is **not supported by the codebase**. All `%v` usages in
`fmt.Errorf` format non-error types. Zero calls were found that wrap an `error`
variable with `%v` instead of `%w`.

---

## Investigation Steps and Results

### Step 1: Count non-test `fmt.Errorf` calls without `%w`

```
grep -rn 'fmt\.Errorf' pkg/ cmd/ internal/ --include='*.go' | grep -v '%w' | grep -v '_test\.go' | wc -l
```

**Result:** 353

Note: The original estimate of 229 was lower than the actual count. The total
`fmt.Errorf` call count across all files (including tests) is 939.

### Step 2: Find `fmt.Errorf` calls using `%v`

```
grep -rn 'fmt\.Errorf.*%v' pkg/ cmd/ internal/ --include='*.go' | grep -v '_test\.go'
```

**Result:** 19 matching lines across the following files:

```
pkg/tools/constraint_enforcer.go:105
pkg/engine/auto_remediate.go:29
pkg/engine/finalize.go:370
pkg/engine/finalize.go:510
pkg/engine/finalize.go:571
pkg/engine/init_engine.go:68
pkg/engine/daemon.go:79
pkg/engine/prepare.go:362
pkg/engine/prepare.go:377
pkg/engine/prepare.go:507
pkg/engine/finalize_steps.go:36
pkg/engine/finalize_steps.go:140
pkg/engine/finalize_steps.go:234
pkg/engine/finalize_steps.go:309
pkg/engine/finalize_steps.go:344
pkg/engine/finalize_steps.go:404
pkg/engine/run_wave_full.go:48
cmd/sawtools/merge_agents.go:31
internal/git/commands.go:208
```

### Step 3: Type analysis of `%v` arguments

Each occurrence was inspected to determine the type of the variable formatted
with `%v`:

| File | Variable | Type | Eligible for %w? |
|------|----------|------|-----------------|
| `constraint_enforcer.go:105` | `c.AllowedPathPrefixes` | `[]string` | No |
| `auto_remediate.go:29` | `res.Errors` | `[]result.SAWError` | No |
| `finalize.go:370` | `gateRes.Errors` | `[]result.SAWError` | No |
| `finalize.go:510` | `mergeRes.Errors` | `[]result.SAWError` | No |
| `finalize.go:571` | `postGateRes.Errors` | `[]result.SAWError` | No |
| `init_engine.go:68` | `saveResult` | `result.Result[bool]` | No |
| `daemon.go:79` | `res.Errors` | `[]result.SAWError` | No |
| `prepare.go:362` | `baselineRes.Errors` | `[]result.SAWError` | No |
| `prepare.go:377` | `crossRes.Errors` | `[]result.SAWError` | No |
| `prepare.go:507` | `wtRes.Errors` | `[]result.SAWError` | No |
| `finalize_steps.go:36` | `verifyRes.Errors` | `[]result.SAWError` | No |
| `finalize_steps.go:140` | `gateRes.Errors` | `[]result.SAWError` | No |
| `finalize_steps.go:234` | `mergeRes.Errors` | `[]result.SAWError` | No |
| `finalize_steps.go:309` | `verifyBuildRes.Errors` | `[]result.SAWError` | No |
| `finalize_steps.go:344` | `verifyData.TestPassed`, `verifyData.LintPassed` | `bool` | No |
| `finalize_steps.go:404` | `missing` | `[]string` | No |
| `run_wave_full.go:48` | `wtRes.Errors` | `[]result.SAWError` | No |
| `merge_agents.go:31` | `res.Errors` | `[]result.SAWError` | No |
| `commands.go:208` | `unresolvable` | `[]string` | No |

**All 19 usages format non-error types** (`[]result.SAWError`, `result.Result[bool]`,
`[]string`, `bool`). `%w` is only valid when wrapping a value that implements the
`error` interface; none of these qualify.

### Step 4: Search for `fmt.Errorf` wrapping an `err` variable with `%v`

```
grep -rn 'fmt\.Errorf.*%v.*\berr\b' pkg/ cmd/ internal/ --include='*.go' | grep -v '_test\.go'
```

**Result:** Zero matches.

---

## Conclusion

The IMPL task description estimated 229 `fmt.Errorf` calls incorrectly using `%v`
for error wrapping. This estimate is incorrect. The actual finding:

- **353** non-test `fmt.Errorf` calls do not use `%w`
- **19** of those use `%v`, but all format non-error types (`[]SAWError`, `[]string`,
  `bool`, `result.Result[bool]`)
- **0** calls wrap an `error` value with `%v` instead of `%w`

No `%v` → `%w` replacements are needed or appropriate. The codebase is already
correct with respect to error wrapping conventions.
