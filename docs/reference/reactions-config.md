# reactions — IMPL Manifest Field Reference

`reactions` is an optional top-level field in the IMPL manifest YAML. When present,
it overrides the E19 hardcoded failure-routing defaults for any failure type that has
an entry. Absent entries fall back to E19 behavior unchanged.

## Schema

```yaml
reactions:
  transient:
    action: <string>        # required
    max_attempts: <int>     # optional; 0 = use E19 default
  timeout:
    action: <string>
    max_attempts: <int>
  fixable:
    action: <string>
    max_attempts: <int>
  needs_replan:
    action: <string>
    max_attempts: <int>
  escalate:
    action: <string>
    max_attempts: <int>
```

All five keys (`transient`, `timeout`, `fixable`, `needs_replan`, `escalate`) are
optional. A reactions block that sets none of them is valid but a no-op.

## ReactionsConfig type

Defined in `pkg/protocol/types.go`.

| Field | Type | YAML key | Required |
|---|---|---|---|
| Transient | `*ReactionEntry` | `transient` | no |
| Timeout | `*ReactionEntry` | `timeout` | no |
| Fixable | `*ReactionEntry` | `fixable` | no |
| NeedsReplan | `*ReactionEntry` | `needs_replan` | no |
| Escalate | `*ReactionEntry` | `escalate` | no |

## ReactionEntry type

| Field | Type | YAML key | Required |
|---|---|---|---|
| Action | `string` | `action` | yes (when entry is present) |
| MaxAttempts | `int` | `max_attempts` | no |

### action values

| Value | Orchestrator action | Notes |
|---|---|---|
| `retry` | `ActionRetry` — plain retry | For `timeout` failures, this is a plain retry **without** the E19 scope-reduction note. Omit the timeout entry and let E19 defaults apply if scope-reduced retry is desired. |
| `send-fix-prompt` | `ActionApplyAndRelaunch` — apply fix from completion report, then relaunch | Equivalent to E19 behavior for `fixable`. |
| `pause` | `ActionEscalate` — surfaces to human | Current model has no separate pause state; treated as escalate. |
| `auto-scout` | `ActionReplan` — re-engages Scout | Equivalent to E19 behavior for `needs_replan`. |

Unknown or empty `action` values are rejected at schema validation time and produce
a `V`-series error (`CodeInvalidEnum`).

### max_attempts semantics

`max_attempts` is the **total number of launch attempts**, including the first. It is
only meaningful when `action` is `retry` or `send-fix-prompt`.

- `0` or absent: use the E19 default for the failure type (see table below).
- Any positive integer: override the cap. Setting `max_attempts: 1` effectively
  disables retries (one attempt, no retry).
- Negative values are rejected at validation.

## E19 defaults per failure type

These are the values used when no reactions entry is present for a given type, or
when `max_attempts` is `0`.

| Failure type | E19 action | Default max\_attempts (total launches) | Auto-retryable |
|---|---|---|---|
| `transient` | Retry automatically | 2 | yes |
| `fixable` | Apply agent fix, then retry | 2 | yes |
| `timeout` | Retry with scope-reduction note | 1 | yes |
| `needs_replan` | Re-engage Scout | 0 | no |
| `escalate` | Surface to human | 0 | no |

For `needs_replan` and `escalate`, `max_attempts` in a reaction entry has no
meaningful effect on auto-retry behavior because those types are not retried by
default.

Absent `failure_type` in a completion report (empty string) is routed as `escalate`
(conservative fallback).

## Validation

`ValidateReactions` in `pkg/protocol/reactions_validation.go` is called by
`ValidateSchema`. It produces errors for:

- A present entry with an empty `action`.
- An `action` value not in the allowed set.
- A `max_attempts` value less than zero.

## Routing resolution

The orchestrator calls `RouteFailureWithReactions` and `MaxAttemptsFor`
(both in `pkg/orchestrator/failure.go`). The resolution order is:

1. If `reactions` is nil, use `RouteFailure` (pure E19).
2. If a reactions entry exists for the failure type, use its `action` and `max_attempts`.
3. If `max_attempts` is `0` in the entry, fall back to `defaultMaxAttempts` (E19 cap).

## Examples

### Increase transient retry cap

```yaml
reactions:
  transient:
    action: retry
    max_attempts: 4
```

### Suppress auto-replan — escalate instead

```yaml
reactions:
  needs_replan:
    action: pause
```

### Send a fix prompt for timeout failures (instead of scope-reduced retry)

```yaml
reactions:
  timeout:
    action: send-fix-prompt
    max_attempts: 2
```

### Convert fixable to auto-scout

```yaml
reactions:
  fixable:
    action: auto-scout
```

### Full block with all types

```yaml
reactions:
  transient:
    action: retry
    max_attempts: 3
  timeout:
    action: retry
    max_attempts: 2
  fixable:
    action: send-fix-prompt
    max_attempts: 2
  needs_replan:
    action: auto-scout
  escalate:
    action: pause
```
