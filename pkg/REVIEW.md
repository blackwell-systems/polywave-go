# pkg/ Review Tracker

Deep-review each package for bugs, non-conforming return types, non-conforming error handling,
duplications, consistency issues, dead code, test gaps, and integration mismatches.
Each reviewed package gets scouted → IMPL'd → closed before being marked done.

## Status

| Package | Reviewed | IMPL Scouted | IMPL Closed |
|---------|----------|--------------|-------------|
| agent | ✅ | ✅ | ✅ |
| agent/backend | ✅ (with agent) | ✅ | ✅ |
| agent/backend/api | ✅ (with agent) | ✅ | ✅ |
| agent/backend/bedrock | ✅ (with agent) | ✅ | ✅ |
| agent/backend/cli | ✅ (with agent) | ✅ | ✅ |
| agent/backend/openai | ✅ (with agent) | ✅ | ✅ |
| agent/dedup | ✅ (with agent) | ✅ | ✅ |
| analyzer | ✅ | ✅ | ✅ |
| autonomy | ✅ | ✅ | ✅ |
| builddiag | ✅ | ❌ (NOT_SUITABLE) | N/A |
| codereview | ✅ | ❌ (NOT_SUITABLE) | N/A |
| collision | ✅ | ✅ | ✅ |
| commands | ✅ | ✅ | ✅ |
| config | ✅ | ✅ | ✅ |
| deps | ✅ | ✅ | ✅ |
| engine | ✅ | ✅ | ✅ |
| errparse | ✅ | ✅ | ✅ |
| format | ✅ | ❌ (NOT_SUITABLE) | N/A |
| gatecache | ✅ | ✅ | ✅ |
| hooks | ✅ | ✅ | ✅ |
| idgen | ✅ | ✅ | ✅ |
| interview | ✅ | ✅ | ✅ |
| journal | ✅ | ✅ | ✅ |
| notify | ✅ | ✅ | ✅ |
| observability | | | |
| orchestrator | ✅ | ✅ | ✅ |
| pipeline | ✅ | ✅ | ❌ (pending execution) |
| protocol | ✅ | ✅ | ✅ |
| queue | ✅ | ✅ | ✅ |
| result | ✅ | ✅ | ✅ |
| resume | ✅ | ✅ | ✅ |
| retry | ✅ | ✅ | ✅ |
| scaffold | ✅ | ✅ | ✅ |
| scaffoldval | ✅ | ✅ | ✅ |
| solver | ✅ | ✅ | ✅ |
| suitability | ✅ | ✅ | ✅ |
| tools | ✅ | ✅ | ✅ |
| worktree | ✅ | ✅ | ✅ |
