# E11 Conflict Prediction

E11 runs automatically inside `sawtools finalize-wave` before any agent branches are merged. It can also be invoked directly via `sawtools predict-conflicts`.

**What E11 detects:** files that appear in two or more agents' completion reports (across `files_changed` and `files_created`) and whose edits are likely to produce a 3-way merge conflict.

**When it runs:** after all agents in a wave report completion, before `merge-agents`. If E11 returns a conflict, `finalize-wave` stops with a failed step result and does not proceed to merge.

**What E11 does not flag:**
- Files under `docs/IMPL/` or `.saw-state/` — these are protocol-managed and are expected to be touched by multiple agents.
- Files reported by only one agent.
- Files where all agents produced identical final content (convergent edits, Pass 1).
- Files where agents edited non-overlapping line ranges (cascade patches, Pass 2).

---

## Two-Pass Algorithm

When a file appears in multiple agents' reports, E11 applies two passes before flagging it.

### Pass 1 — Convergent Edit Detection (SHA Hash)

E11 reads the file content from each agent's branch using `git show branch:file` and computes a SHA-256 hash. If all hashes match, the agents independently produced the same result. Git will auto-resolve this via fast-forward or identical-tree resolution; no conflict is possible. The file is cleared.

This pass requires `manifest.FeatureSlug` to be set (needed to derive branch names). If the slug is absent, this pass is skipped and E11 proceeds conservatively.

### Pass 2 — Hunk-Level Overlap Detection

If the file has not been cleared by Pass 1, E11 performs a hunk-level analysis:

1. Find the common ancestor (`git merge-base`) of the first two agent branches. Because all wave agents branch off the same base commit, this gives the shared base ref for all agents in the wave.
2. For each agent, run `git diff --unified=0 <mergeBase>..<agentBranch> -- <file>` to get the exact line ranges the agent modified.
3. Parse the `@@ -a,b +c,d @@` headers from the unified diff.
4. Check all pairs of agents for overlapping line ranges.

If no pair has overlapping hunks, git 3-way merge will succeed. The file is cleared (no conflict). If any pair overlaps, the file is flagged.

If the git calls fail (unreachable repository, missing branch, merge-base error), E11 defaults to flagging the file — the safe-conservative path.

---

## HunkRange Semantics

`HunkRange` represents a contiguous range of lines in the **base file** (the common ancestor) that an agent's diff modifies.

```
HunkRange{Start: int, End: int}  // both 1-indexed, inclusive
```

`parseDiffHunks` reads `@@ -a,b @@` headers from a `--unified=0` diff:

| Hunk header | Start | End | Notes |
|-------------|-------|-----|-------|
| `@@ -10,5 @@` | 10 | 14 | Modifies lines 10–14 (`start + count - 1`) |
| `@@ -100 @@` | 100 | 100 | Single-line modification (count defaults to 1) |
| `@@ -50,0 @@` | 50 | 50 | **Pure insertion** — no base lines removed |

**Pure-insertion anchor behavior:** A hunk with count 0 (`-a,0`) inserts new lines after line `a` without modifying any existing line. Rather than discarding it, E11 records it as a zero-width anchor `{Start: a, End: a}`. Two agents inserting at the same anchor position will conflict in a 3-way merge (both try to append content after the same line), so the anchor participates in the overlap check the same way a modification hunk does.

**Overlap rule:** Two ranges overlap when `A.Start <= B.End && B.Start <= A.End`. Adjacent ranges (`End == otherStart - 1`) do not overlap.

---

## Cascade Patch Pattern

The cascade patch pattern is the primary reason E11 has Pass 2 at all.

In a typical SAW wave, each agent is assigned ownership of different files. When a wave touches a shared file (a common interface, a constants file, a registry), the Scout assigns each agent a different section of that file — for example, Agent A adds a method to `FuncA` near the top and Agent B adds a method to `FuncB` near the bottom. Both agents report `shared.go` in their `files_changed` list.

Without hunk analysis, this would look like a conflict. With Pass 2, E11 verifies that the two agents' `@@ -a,b @@` ranges do not overlap, confirms git 3-way merge will handle it cleanly, and allows the merge to proceed. No manual intervention required.

This is the expected behavior for well-planned waves. E11 only blocks merges where the line ranges actually intersect — meaning both agents attempted to edit the same lines with different content.

---

## Types

### `ConflictData`

Returned by `PredictConflictsFromReports`.

| Field | Type | Description |
|-------|------|-------------|
| `ConflictsDetected` | `int` | Number of files with predicted conflicts |
| `Conflicts` | `[]ConflictPrediction` | One entry per conflicting file |

### `ConflictPrediction`

| Field | Type | Description |
|-------|------|-------------|
| `File` | `string` | Repo-relative file path |
| `Agents` | `[]string` | Agent IDs that reported touching this file |

### `HunkRange`

| Field | Type | Description |
|-------|------|-------------|
| `Start` | `int` | First modified line (1-indexed, inclusive) |
| `End` | `int` | Last modified line (1-indexed, inclusive) |

For pure insertions, `Start == End == anchor_line`.

---

## `sawtools predict-conflicts`

Runs E11 against an IMPL manifest file. Useful for diagnosing conflict predictions without triggering a full `finalize-wave`.

```
sawtools predict-conflicts <manifest-path> --wave <n>
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--wave <n>` | Yes | Wave number to check |

### Output

JSON to stdout:

```json
{
  "conflicts_detected": 2,
  "conflicts": [
    {"file": "pkg/api/handler.go", "agents": ["A1", "A2"]},
    {"file": "pkg/store/store.go", "agents": ["A2", "A3"]}
  ],
  "warnings": [
    "E11 conflict prediction: 2 file(s) appear in multiple agent reports (merge conflict risk):\n  pkg/api/handler.go has overlapping edits (agents: [A1 A2])\n  pkg/store/store.go has overlapping edits (agents: [A2 A3])",
    "E11 conflict prediction: pkg/api/handler.go has differing edits (agents: [A1 A2])",
    "E11 conflict prediction: pkg/store/store.go has differing edits (agents: [A2 A3])"
  ]
}
```

When no conflicts are detected, `conflicts_detected` is `0`, `conflicts` is an empty array, and `warnings` is omitted.

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No conflicts detected — safe to merge |
| `1` | Overlapping hunks detected — merge conflict likely |

A non-zero exit always accompanies a non-empty `warnings` array in the JSON output. The error message on stderr takes the form:

```
predict-conflicts: <n> file(s) have overlapping edits (merge conflict likely)
```

### Branch Name Convention

E11 derives agent branch names from the manifest's `feature_slug` and the wave number:

```
saw/<feature_slug>/wave<n>-agent-<agentID>
```

For example, with `feature_slug: add-cache`, wave 1, agent A2:

```
saw/add-cache/wave1-agent-A2
```

This convention must match how `prepare-wave` created the worktree branches. If branches do not exist at the expected names, E11's git calls fail and it defaults to flagging the file as a conflict.

---

## Error Code

`CONFLICT_PREDICT_FAILED` — emitted as a warning-severity `SAWError` in the `Partial` result. One warning per conflicting file plus a summary warning. No Fatal result is returned; E11 surfaces conflicts as `Partial` so callers receive both the `ConflictData` and the warnings.

The `finalize-wave` step (`StepPredictConflicts`) treats any non-success result from `PredictConflictsFromReports` as a step failure and stops the pipeline.
