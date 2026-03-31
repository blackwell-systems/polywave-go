# Autonomy Levels

The autonomy system controls how much human approval the daemon requires before
advancing through the Scout-and-Wave execution cycle. Every decision point in
the cycle is a **stage**; each stage is either auto-approved or gated depending
on the configured **level**.

---

## Levels

### `gated` (default)

Every stage requires explicit human action. The daemon pauses and emits a
`daemon_awaiting_review` event; execution resumes only when the daemon is
restarted or the approval is delivered externally. This is the safest mode and
the default when no config is present.

### `supervised`

IMPL review requires human approval; everything else is automatic. Use this
when you want to vet the Scout's plan before agents start writing code, but do
not want to hand-hold wave transitions, gate failures, or queue advancement.

### `autonomous`

All stages are auto-approved. The daemon runs end-to-end without any human
intervention: Scout, wave execution, gate-failure remediation, and queue
advancement all proceed without pausing.

---

## Stages

| Stage constant | JSON string | When it fires |
|---|---|---|
| `StageIMPLReview` | `impl_review` | After Scout writes the IMPL doc, before the first wave starts. The pause point for human review of the plan. |
| `StageWaveAdvance` | `wave_advance` | Reserved: will fire between waves when per-wave gating is added. Currently the daemon advances waves unconditionally inside the active IMPL loop. |
| `StageGateFailure` | `gate_failure` | When `FinalizeWave` fails (build, test, or lint error). Controls whether `AutoRemediate` is invoked automatically. |
| `StageQueueAdvance` | `queue_advance` | When `CheckQueue` finds the next eligible IMPL item. Controls whether the item is marked `in_progress` and handed to Scout without human approval. |

---

## Approval matrix

`auto` = the daemon proceeds without pausing.
`gate` = the daemon pauses and emits a `daemon_awaiting_review` event.

| Stage | `gated` | `supervised` | `autonomous` |
|---|---|---|---|
| `impl_review` | gate | **gate** | auto |
| `wave_advance` | gate | auto | auto |
| `gate_failure` | gate | auto | auto |
| `queue_advance` | gate | auto | auto |

The matrix is implemented in `pkg/autonomy/autonomy.go` (`ShouldAutoApprove`).
The authoritative logic is:

- `gated` — all stages return `false` (gate everything).
- `autonomous` — all stages return `true` (approve everything).
- `supervised` — only `impl_review` returns `false`; all other stages return `true`.

---

## Config fields

Autonomy configuration lives under the `"autonomy"` key in `saw.config.json`.

| Field | Type | Default | Purpose |
|---|---|---|---|
| `level` | string | `"gated"` | Autonomy level: `gated`, `supervised`, or `autonomous`. |
| `max_auto_retries` | int | `2` | Maximum remediation retries when `gate_failure` is auto-approved. Passed to `AutoRemediate` as `MaxRetries`. If `0` or omitted the engine falls back to `2`. |
| `max_queue_depth` | int | `10` | Maximum number of items that may be queued simultaneously. Enforced by queue management; does not affect level gating directly. |

### Defaults

If `saw.config.json` does not exist, or the `"autonomy"` key is absent, the
engine uses:

```json
{
  "level": "gated",
  "max_auto_retries": 2,
  "max_queue_depth": 10
}
```

Invalid JSON in the file causes a fatal `CONFIG_LOAD_FAILED` error; other
top-level keys are preserved when the autonomy section is written back.

---

## Setting autonomy level in saw.config.json

Add or update the `"autonomy"` key. Other top-level sections are preserved.

**Minimal — level only:**

```json
{
  "autonomy": {
    "level": "supervised"
  }
}
```

**Full:**

```json
{
  "autonomy": {
    "level": "autonomous",
    "max_auto_retries": 3,
    "max_queue_depth": 20
  }
}
```

Valid level strings are `gated`, `supervised`, and `autonomous` (lowercase,
exact). Any other value causes `ParseLevel` to return an error.

### Per-item override

Individual queue items can override the project-level autonomy via the
`autonomy_override` field in their YAML file. The override is resolved by
`EffectiveLevel(cfg, item.AutonomyOverride)` — a non-empty, valid level string
takes precedence over `cfg.Level`; an empty or invalid string falls back to the
config level.

```yaml
title: "Migrate auth to JWT"
autonomy_override: "gated"
feature_description: |
  ...
```

### CLI override

The `sawtools daemon` command accepts an `--autonomy` flag that replaces
`cfg.Level` before the daemon starts:

```
sawtools daemon --autonomy supervised
```

This takes precedence over `saw.config.json` for the lifetime of that daemon
process.

---

## How autonomy affects IMPL execution

The daemon (`pkg/engine/daemon.go`) calls `ShouldAutoApprove` at three points
during each IMPL cycle:

1. **After Scout completes** (`StageIMPLReview`) — if gated, the daemon emits
   `daemon_awaiting_review` and blocks on context cancellation. The item
   remains `in_progress`; a human must restart the daemon or provide approval
   out-of-band.

2. **When FinalizeWave fails** (`StageGateFailure`) — if auto-approved, the
   daemon calls `AutoRemediate` up to `MaxAutoRetries` times. If remediation
   cannot fix the failure the item is marked `blocked`. If gated, the item is
   marked `blocked` immediately without attempting remediation.

3. **When CheckQueue finds the next item** (`StageQueueAdvance`) — if auto-
   approved, the item transitions from `queued` to `in_progress` and Scout is
   invoked. If gated, `CheckQueue` returns `reason: "autonomy_blocked"` and the
   daemon sleeps until the next poll interval.

`StageWaveAdvance` is reserved for a future per-wave gating feature; the
current daemon iterates through all waves in a single IMPL unconditionally.
