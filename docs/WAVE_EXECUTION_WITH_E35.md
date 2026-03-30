# Wave Execution with E35 Pre-Flight Check

## Updated Wave Execution Pipeline

### Before (Without E35 Detection)

```bash
# Scout phase
sawtools run-scout "feature description" > IMPL-feature.yaml

# Wave 1 execution
sawtools create-worktrees IMPL-feature.yaml --wave 1
sawtools run-wave IMPL-feature.yaml --wave 1

# Post-wave
sawtools verify-commits IMPL-feature.yaml --wave 1
sawtools merge-agents IMPL-feature.yaml --wave 1
```

**Problem**: Build failures after merge when agents modify function signatures without owning all call sites.

### After (With E35 Detection)

```bash
# Scout phase
sawtools run-scout "feature description" > IMPL-feature.yaml

# ✨ NEW: Pre-wave validation
sawtools pre-wave-validate IMPL-feature.yaml --wave 1
if [ $? -ne 0 ]; then
    echo "E35 gaps detected. Fix ownership before executing wave."
    exit 1
fi

# Wave 1 execution (only if validation passes)
sawtools create-worktrees IMPL-feature.yaml --wave 1
sawtools run-wave IMPL-feature.yaml --wave 1

# Post-wave
sawtools verify-commits IMPL-feature.yaml --wave 1
sawtools merge-agents IMPL-feature.yaml --wave 1
```

**Benefit**: Build failures caught before worktree creation. No wasted agent turns, no manual merge recovery.

## Integration Points

### Option 1: Batch Command (Recommended)

Combine pre-wave validation with prepare-wave:

```bash
sawtools prepare-wave IMPL-feature.yaml --wave 1
  # Internally runs:
  # 1. pre-wave-validate (E16 + E35)
  # 2. create-worktrees (if validation passes)
  # 3. extract-context
```

### Option 2: Explicit Pre-Flight

Run validation as separate step:

```bash
# Pre-flight checks
sawtools pre-wave-validate IMPL-feature.yaml --wave 1 --fix || exit 1

# Wave execution
sawtools prepare-wave IMPL-feature.yaml --wave 1
sawtools run-wave IMPL-feature.yaml --wave 1
```

### Option 3: CI/CD Pipeline

Add to GitHub Actions / CI:

```yaml
name: SAW Wave Execution

on:
  workflow_dispatch:
    inputs:
      impl_path:
        description: 'Path to IMPL manifest'
        required: true
      wave:
        description: 'Wave number'
        required: true

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Pre-wave validation
        run: |
          sawtools pre-wave-validate ${{ inputs.impl_path }} \
            --wave ${{ inputs.wave }} || exit 1

  execute:
    needs: validate
    runs-on: ubuntu-latest
    steps:
      - name: Run wave
        run: |
          sawtools create-worktrees ${{ inputs.impl_path }} --wave ${{ inputs.wave }}
          sawtools run-wave ${{ inputs.impl_path }} --wave ${{ inputs.wave }}
```

## Error Handling

### E35 Gaps Detected

```bash
$ sawtools pre-wave-validate IMPL-feature.yaml --wave 1
{
  "validation": {"valid": true, ...},
  "e35_gaps": {
    "passed": false,
    "gaps": [
      {
        "agent": "C",
        "function_name": "CreateProgramWorktrees",
        "defined_in": "pkg/protocol/worktree.go",
        "called_from": [
          "pkg/protocol/program_tier_prepare.go:45"
        ]
      }
    ]
  }
}
Error: pre-wave validation failed
```

**Actions**:
1. Review gap details
2. Choose remediation strategy (reassign files, add wiring, defer to later wave)
3. Update IMPL manifest
4. Re-run validation
5. Proceed with wave execution

### E16 Validation Failed

```bash
$ sawtools pre-wave-validate IMPL-feature.yaml --wave 1
{
  "validation": {
    "valid": false,
    "errors": [
      {"code": "DISJOINT_OWNERSHIP", "message": "file owned by 2 agents"}
    ]
  },
  "e35_gaps": {"passed": true, ...}
}
Error: pre-wave validation failed
```

**Actions**:
1. Fix E16 violations (use `--fix` flag for auto-fixable issues)
2. Re-run validation
3. Proceed with wave execution

## Performance Considerations

### Detection Cost

- **Fast**: ~100-500ms for typical wave (3-5 agents, 10-20 files)
- **Moderate**: ~1-3s for large wave (8-10 agents, 50+ files)
- **Scalable**: O(n*m) where n=functions, m=same-package files

### When to Skip

E35 detection can be skipped if:
- Wave has only 1 agent (no ownership conflicts possible)
- Wave modifies only new files (no existing call sites)
- Wave is integration-only (type: "integration", no function changes)

```bash
# Skip E35 for integration waves
if [ "$(yq '.waves[0].type' IMPL.yaml)" = "integration" ]; then
    sawtools validate IMPL.yaml  # E16 only
else
    sawtools pre-wave-validate IMPL.yaml --wave 1  # E16 + E35
fi
```

## Metrics and Observability

### Success Metrics

Track over time:
- E35 gap detection rate (% of waves with gaps)
- Pre-wave validation time
- Build failure rate post-merge (should decrease)

### Failure Analysis

When gaps detected:
1. Log gap details to metrics system
2. Track which agents/packages have most violations
3. Adjust Scout decomposition strategy

### Example Metrics

```json
{
  "event": "pre_wave_validation",
  "timestamp": "2024-03-30T10:30:00Z",
  "wave": 1,
  "e16_passed": true,
  "e35_passed": false,
  "e35_gaps_count": 2,
  "validation_time_ms": 450,
  "remediation_action": "reassign_files"
}
```

## Rollout Strategy

### Phase 1: Advisory (Weeks 1-2)

Run E35 detection but don't block wave execution:

```bash
sawtools pre-wave-validate IMPL.yaml --wave 1 || echo "⚠️  E35 gaps found (advisory)"
sawtools create-worktrees IMPL.yaml --wave 1  # Continue anyway
```

**Goal**: Collect baseline gap rate, identify common patterns

### Phase 2: Blocking (Weeks 3-4)

Block wave execution when gaps detected:

```bash
sawtools pre-wave-validate IMPL.yaml --wave 1 || exit 1
sawtools create-worktrees IMPL.yaml --wave 1
```

**Goal**: Enforce E35 checks, measure impact on execution time

### Phase 3: Automated Remediation (Month 2+)

Auto-suggest fixes:

```bash
sawtools pre-wave-validate IMPL.yaml --wave 1 --suggest-fixes
  # Output:
  # E35 gap detected. Suggested fix:
  #   Move pkg/protocol/program_tier_prepare.go from Agent B to Agent C
```

## See Also

- [E35 Detection Documentation](./E35_DETECTION.md)
- [Wave Execution Pipeline](./WAVE_EXECUTION.md)
- [Validation Rules](./VALIDATION.md)
