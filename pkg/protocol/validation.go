package protocol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// agentIDRegex validates agent IDs: one uppercase letter, optionally followed by a digit 2-9
var agentIDRegex = regexp.MustCompile(`^[A-Z][2-9]?$`)

// featureSlugRegex validates feature_slug fields: kebab-case with lowercase letters, digits, and hyphens.
// Must not start or end with a hyphen.
var featureSlugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Validate runs all I1-I6 invariant checks and supplementary validations on a
// parsed IMPLManifest. It does not load or save files; pass a pre-loaded manifest.
//
// This is the structural validator. It cannot detect unknown YAML keys because
// those require the raw source bytes. Use ValidateBytes or FullValidate when
// unknown-key detection is also needed.
//
// Called by: FullValidate (full pipeline), ValidateBytes (in-memory shortcut).
// Do not call directly unless you have already loaded the manifest yourself.
func Validate(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	errs = append(errs, validateI1DisjointOwnership(m, m.FeatureSlug)...)
	errs = append(errs, validateI2AgentDependencies(m, m.FeatureSlug)...)
	errs = append(errs, validateI3WaveOrdering(m)...)
	errs = append(errs, validateI4RequiredFields(m)...)
	errs = append(errs, validateI5FileOwnershipComplete(m)...)
	errs = append(errs, validateI6NoCycles(m)...)
	errs = append(errs, validateI5CommitBeforeReport(m)...)
	errs = append(errs, validateE9MergeState(m)...)
	errs = append(errs, validateSM01StateValid(m)...)
	errs = append(errs, validateAgentIDs(m)...)
	errs = append(errs, validateGateTypes(m)...)
	errs = append(errs, ValidateWorktreeNames(m)...)
	errs = append(errs, ValidateVerificationField(m)...)
	errs = append(errs, ValidateCompletionStatuses(m)...)
	errs = append(errs, ValidateFailureTypes(m)...)
	errs = append(errs, ValidatePreMortemRisk(m)...)
	errs = append(errs, validateMultiRepoConsistency(m)...)
	errs = append(errs, ValidateSchema(m)...)
	errs = append(errs, ValidateActionEnums(m)...)
	errs = append(errs, ValidateIntegrationChecklist(m, "")...)
	errs = append(errs, ValidateFileExistence(m, "")...)
	errs = append(errs, validateKnownIssueTitles(m)...)
	errs = append(errs, CheckAgentComplexity(context.TODO(), m)...)

	return errs
}

// validateKnownIssueTitles checks that all KnownIssue entries have a non-empty title.
func validateKnownIssueTitles(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	for i, issue := range m.KnownIssues {
		if issue.Title == "" {
			errs = append(errs, result.SAWError{
				Code:     result.CodeKnownIssueMissingTitle,
				Message:  fmt.Sprintf("known_issues[%d]: title is required", i),
				Severity: "error",
				Field:    fmt.Sprintf("known_issues[%d].title", i),
			})
		}
	}

	return errs
}

// validateI1DisjointOwnership checks that no file is owned by multiple agents within the same wave.
// Files may be owned by different agents across different waves (sequential modification).
func validateI1DisjointOwnership(m *IMPLManifest, slug string) []result.SAWError {
	var errs []result.SAWError

	// Build map of (wave, file) -> agents
	type waveFile struct {
		wave int
		file string
	}
	ownership := make(map[waveFile][]string)

	for _, fo := range m.FileOwnership {
		key := waveFile{wave: fo.Wave, file: fo.File}
		ownership[key] = append(ownership[key], fo.Agent)
	}

	// Check for multiple owners in same wave
	for key, agents := range ownership {
		if len(agents) > 1 {
			e := result.NewError(
				result.CodeDisjointOwnership,
				fmt.Sprintf("file %q owned by multiple agents in wave %d: %v", key.file, key.wave, agents),
			).
				WithContext("slug", slug).
				WithContext("wave", strconv.Itoa(key.wave)).
				WithContext("agent_id", strings.Join(agents, ","))
			e.Field = "file_ownership"
			errs = append(errs, e)
		}
	}

	return errs
}

// ValidateI1DisjointOwnership is a public wrapper around validateI1DisjointOwnership
// for use by prepare-wave runtime enforcement. It validates I1 compliance for a
// specific wave number by filtering the file_ownership table.
func ValidateI1DisjointOwnership(m *IMPLManifest, waveNum int) []result.SAWError {
	// Build a filtered manifest with only this wave's ownership entries
	filtered := &IMPLManifest{
		FeatureSlug:   m.FeatureSlug,
		FileOwnership: []FileOwnership{},
	}
	for _, fo := range m.FileOwnership {
		if fo.Wave == waveNum {
			filtered.FileOwnership = append(filtered.FileOwnership, fo)
		}
	}
	return validateI1DisjointOwnership(filtered, m.FeatureSlug)
}

// validateI2AgentDependencies checks that all agent dependencies reference agents in prior waves only.
// An agent in wave N may only depend on agents in waves 1..(N-1).
func validateI2AgentDependencies(m *IMPLManifest, slug string) []result.SAWError {
	var errs []result.SAWError

	// Build map of agent -> wave
	agentWave := make(map[string]int)
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			agentWave[agent.ID] = wave.Number
		}
	}

	// Check each agent's dependencies
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			for _, dep := range agent.Dependencies {
				depWave, exists := agentWave[dep]
				if !exists {
					ve := result.NewError(
						result.CodeSameWaveDependency,
						fmt.Sprintf("agent %s (wave %d) depends on unknown agent %q", agent.ID, wave.Number, dep),
					).
						WithContext("slug", slug).
						WithContext("wave", strconv.Itoa(wave.Number)).
						WithContext("agent_id", agent.ID)
					ve.Field = fmt.Sprintf("waves[%d].agents[%s].dependencies", wave.Number-1, agent.ID)
					errs = append(errs, ve)
				} else if depWave >= wave.Number {
					ve := result.NewError(
						result.CodeSameWaveDependency,
						fmt.Sprintf("agent %s (wave %d) depends on %s (wave %d) — dependencies must be in prior waves", agent.ID, wave.Number, dep, depWave),
					).
						WithContext("slug", slug).
						WithContext("wave", strconv.Itoa(wave.Number)).
						WithContext("agent_id", agent.ID)
					ve.Field = fmt.Sprintf("waves[%d].agents[%s].dependencies", wave.Number-1, agent.ID)
					errs = append(errs, ve)
				}
			}
		}
	}

	// Also check FileOwnership DependsOn references
	for _, fo := range m.FileOwnership {
		for _, dep := range fo.DependsOn {
			// Extract agent ID from "agent:file" format if present
			// DependsOn can be "AgentB" or "AgentB:path/to/file"
			depAgent := dep
			if idx := strings.Index(dep, ":"); idx != -1 {
				depAgent = dep[:idx]
			}

			depWave, exists := agentWave[depAgent]
			if !exists {
				ve := result.NewError(
					result.CodeSameWaveDependency,
					fmt.Sprintf("file %q (agent %s, wave %d) depends on unknown agent %q", fo.File, fo.Agent, fo.Wave, depAgent),
				).
					WithContext("slug", slug).
					WithContext("wave", strconv.Itoa(fo.Wave)).
					WithContext("agent_id", fo.Agent)
				ve.Field = "file_ownership"
				errs = append(errs, ve)
			} else if depWave >= fo.Wave {
				ve := result.NewError(
					result.CodeSameWaveDependency,
					fmt.Sprintf("file %q (agent %s, wave %d) depends on agent %s (wave %d) — dependencies must be in prior waves", fo.File, fo.Agent, fo.Wave, depAgent, depWave),
				).
					WithContext("slug", slug).
					WithContext("wave", strconv.Itoa(fo.Wave)).
					WithContext("agent_id", fo.Agent)
				ve.Field = "file_ownership"
				errs = append(errs, ve)
			}
		}
	}

	return errs
}

// validateI3WaveOrdering checks that wave numbers are sequential starting from 1.
func validateI3WaveOrdering(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	if len(m.Waves) == 0 {
		return errs
	}

	// Check for sequential numbering: 1, 2, 3, ...
	for i, wave := range m.Waves {
		expected := i + 1
		if wave.Number != expected {
			errs = append(errs, result.SAWError{
				Code:     result.CodeWaveNotOneIndexed,
				Message:  fmt.Sprintf("wave number mismatch: expected wave %d, got wave %d", expected, wave.Number),
				Severity: "error",
				Field:    fmt.Sprintf("waves[%d].number", i),
			})
		}
	}

	return errs
}

// validateI4RequiredFields checks that all required manifest fields are present and non-empty.
func validateI4RequiredFields(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	if strings.TrimSpace(m.Title) == "" {
		errs = append(errs, result.SAWError{
			Code:     result.CodeRequiredFieldsMissing,
			Message:  "title is required",
			Severity: "error",
			Field:    "title",
		})
	}

	if strings.TrimSpace(m.FeatureSlug) == "" {
		errs = append(errs, result.SAWError{
			Code:     result.CodeRequiredFieldsMissing,
			Message:  "feature_slug is required",
			Severity: "error",
			Field:    "feature_slug",
		})
	} else if !featureSlugRegex.MatchString(m.FeatureSlug) {
		errs = append(errs, result.SAWError{
			Code:     result.CodeInvalidFieldValue,
			Message:  fmt.Sprintf("feature_slug %q must be kebab-case (lowercase letters, digits, hyphens; no leading/trailing hyphens)", m.FeatureSlug),
			Severity: "error",
			Field:    "feature_slug",
		})
	}

	if strings.TrimSpace(m.Verdict) == "" {
		errs = append(errs, result.SAWError{
			Code:     result.CodeRequiredFieldsMissing,
			Message:  "verdict is required",
			Severity: "error",
			Field:    "verdict",
		})
	} else {
		// Validate verdict value
		validVerdicts := map[string]bool{
			"SUITABLE":              true,
			"NOT_SUITABLE":          true,
			"SUITABLE_WITH_CAVEATS": true,
		}
		if !validVerdicts[m.Verdict] {
			errs = append(errs, result.SAWError{
				Code:     result.CodeInvalidFieldValue,
				Message:  fmt.Sprintf("verdict must be SUITABLE, NOT_SUITABLE, or SUITABLE_WITH_CAVEATS, got %q", m.Verdict),
				Severity: "error",
				Field:    "verdict",
			})
		}
	}

	return errs
}

// validateI5FileOwnershipComplete checks that all files referenced in agent.Files are present in FileOwnership table.
func validateI5FileOwnershipComplete(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Build set of files in ownership table
	ownedFiles := make(map[string]bool)
	for _, fo := range m.FileOwnership {
		ownedFiles[fo.File] = true
	}

	// Check that all agent files are in ownership table
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			for _, file := range agent.Files {
				if !ownedFiles[file] {
					errs = append(errs, result.SAWError{
						Code:     result.CodeOrphanFile,
						Message:  fmt.Sprintf("agent %s (wave %d) references file %q which is not in file_ownership table", agent.ID, wave.Number, file),
						Severity: "error",
						Field:    fmt.Sprintf("waves[%d].agents[%s].files", wave.Number-1, agent.ID),
					})
				}
			}
		}
	}

	return errs
}

// validateI6NoCycles checks that the dependency graph is acyclic.
// Uses depth-first search with a recursion stack to detect cycles.
func validateI6NoCycles(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Build adjacency list: agent -> dependencies
	deps := make(map[string][]string)
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			deps[agent.ID] = agent.Dependencies
		}
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(agent string, path []string) []string
	dfs = func(agent string, path []string) []string {
		if recStack[agent] {
			// Found cycle: return the cycle path
			cycleStart := 0
			for i, a := range path {
				if a == agent {
					cycleStart = i
					break
				}
			}
			return append(path[cycleStart:], agent)
		}
		if visited[agent] {
			return nil
		}

		visited[agent] = true
		recStack[agent] = true
		path = append(path, agent)

		for _, dep := range deps[agent] {
			if cycle := dfs(dep, path); cycle != nil {
				return cycle
			}
		}

		recStack[agent] = false
		return nil
	}

	// Check all agents for cycles
	for agent := range deps {
		if !visited[agent] {
			if cycle := dfs(agent, nil); cycle != nil {
				errs = append(errs, result.SAWError{
					Code:     result.CodeDependencyCycle,
					Message:  fmt.Sprintf("dependency cycle detected: %s", strings.Join(cycle, " -> ")),
					Severity: "error",
					Field:    "waves",
				})
				// Only report first cycle found
				break
			}
		}
	}

	return errs
}

// validateI5CommitBeforeReport checks that all completion reports have a valid commit hash.
// Enforces I5: agents must commit before reporting (commit field must be non-empty and not "uncommitted").
func validateI5CommitBeforeReport(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	for agentID, report := range m.CompletionReports {
		if strings.TrimSpace(report.Commit) == "" || report.Commit == "uncommitted" {
			errs = append(errs, result.SAWError{
				Code:     result.CodeCommitMissing,
				Message:  fmt.Sprintf("agent %s completion report has no valid commit (commit=%q) — agents must commit before reporting", agentID, report.Commit),
				Severity: "error",
				Field:    fmt.Sprintf("completion_reports[%s].commit", agentID),
			})
		}
	}

	return errs
}

// validateE9MergeState checks that merge_state field contains a valid value.
// Valid values: "idle", "in_progress", "completed", "failed".
// Empty/omitted values are valid (backward compatibility).
func validateE9MergeState(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Empty is valid (backward compat)
	if strings.TrimSpace(string(m.MergeState)) == "" {
		return errs
	}

	validStates := map[MergeState]bool{
		MergeStateIdle:       true,
		MergeStateInProgress: true,
		MergeStateCompleted:  true,
		MergeStateFailed:     true,
	}

	if !validStates[m.MergeState] {
		errs = append(errs, result.SAWError{
			Code:     result.CodeInvalidMergeState,
			Message:  fmt.Sprintf("merge_state has invalid value %q — must be one of: idle, in_progress, completed, failed", m.MergeState),
			Severity: "error",
			Field:    "merge_state",
		})
	}

	return errs
}

// validateSM01StateValid checks that state field contains a valid ProtocolState value.
// Empty/omitted values are valid (backward compatibility).
func validateSM01StateValid(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Empty is valid (backward compat)
	if strings.TrimSpace(string(m.State)) == "" {
		return errs
	}

	validStates := map[ProtocolState]bool{
		StateInterviewing:    true,
		StateScoutPending:    true,
		StateScoutValidating: true,
		StateReviewed:        true,
		StateScaffoldPending: true,
		StateWavePending:     true,
		StateWaveExecuting:   true,
		StateWaveMerging:     true,
		StateWaveVerified:    true,
		StateBlocked:         true,
		StateComplete:        true,
		StateNotSuitable:     true,
	}

	if !validStates[m.State] {
		errs = append(errs, result.SAWError{
			Code:     result.CodeInvalidState,
			Message:  fmt.Sprintf("state has invalid value %q — must be one of: INTERVIEWING, SCOUT_PENDING, SCOUT_VALIDATING, REVIEWED, SCAFFOLD_PENDING, WAVE_PENDING, WAVE_EXECUTING, WAVE_MERGING, WAVE_VERIFIED, BLOCKED, COMPLETE, NOT_SUITABLE", m.State),
			Severity: "error",
			Field:    "state",
		})
	}

	return errs
}

// validateAgentIDs checks that all agent IDs conform to the protocol regex: ^[A-Z][2-9]?$
// Valid examples: "A", "B", "C2", "D9"
// Invalid examples: "a", "AB", "A1", "A10", "1A", ""
func validateAgentIDs(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Check agent IDs in wave definitions
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			if !agentIDRegex.MatchString(agent.ID) {
				errs = append(errs, result.SAWError{
					Code:     result.CodeInvalidAgentID,
					Message:  fmt.Sprintf("agent ID %q in wave %d does not match protocol pattern ^[A-Z][2-9]?$ (one uppercase letter, optionally followed by digit 2-9)", agent.ID, wave.Number),
					Severity: "error",
					Field:    fmt.Sprintf("waves[%d].agents[%s].id", wave.Number-1, agent.ID),
				})
			}
		}
	}

	// Check agent IDs in FileOwnership
	for i, fo := range m.FileOwnership {
		// Allow "Scaffold" for wave 0 entries (scaffold files created before Wave 1)
		if fo.Agent == "Scaffold" && fo.Wave == 0 {
			continue
		}
		if !agentIDRegex.MatchString(fo.Agent) {
			errs = append(errs, result.SAWError{
				Code:     result.CodeInvalidAgentID,
				Message:  fmt.Sprintf("agent ID %q in file_ownership entry %d (file=%q) does not match protocol pattern ^[A-Z][2-9]?$", fo.Agent, i, fo.File),
				Severity: "error",
				Field:    fmt.Sprintf("file_ownership[%d].agent", i),
			})
		}
	}

	// Check agent IDs in CompletionReports map keys
	for agentID := range m.CompletionReports {
		if !agentIDRegex.MatchString(agentID) {
			errs = append(errs, result.SAWError{
				Code:     result.CodeInvalidAgentID,
				Message:  fmt.Sprintf("agent ID %q in completion_reports does not match protocol pattern ^[A-Z][2-9]?$", agentID),
				Severity: "error",
				Field:    fmt.Sprintf("completion_reports[%s]", agentID),
			})
		}
	}

	return errs
}

// ValidGateTypes is the set of allowed quality gate type values.
var ValidGateTypes = map[string]bool{
	"build":     true,
	"lint":      true,
	"test":      true,
	"typecheck": true,
	"format":    true,
	"custom":    true,
}

// FixGateTypes rewrites any unrecognized gate type to "custom".
// Returns the number of gates fixed.
func FixGateTypes(m *IMPLManifest) int {
	if m.QualityGates == nil {
		return 0
	}
	fixed := 0
	for i := range m.QualityGates.Gates {
		if !ValidGateTypes[m.QualityGates.Gates[i].Type] {
			m.QualityGates.Gates[i].Type = "custom"
			fixed++
		}
	}
	return fixed
}

// validateMultiRepoConsistency checks that when any file_ownership entry has a repo: field,
// ALL entries have an explicit repo: field. Mixing explicit and implicit repo tags causes
// the web GUI to misdetect multi-repo IMPLs.
func validateMultiRepoConsistency(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	if len(m.FileOwnership) == 0 {
		return errs
	}

	hasExplicit := false
	hasImplicit := false
	for _, fo := range m.FileOwnership {
		if fo.Repo != "" {
			hasExplicit = true
		} else {
			hasImplicit = true
		}
	}

	if hasExplicit && hasImplicit {
		// Collect the implicit entries for a helpful message
		var missing []string
		for _, fo := range m.FileOwnership {
			if fo.Repo == "" {
				missing = append(missing, fo.File)
				if len(missing) >= 3 {
					break
				}
			}
		}
		suffix := ""
		if len(missing) >= 3 {
			suffix = " ..."
		}
		errs = append(errs, result.SAWError{
			Code:     result.CodeInconsistentRepo,
			Message:  fmt.Sprintf("file_ownership has mixed repo tags: some entries have repo: and some don't — add explicit repo: to all entries (missing on: %s%s)", strings.Join(missing, ", "), suffix),
			Severity: "error",
			Field:    "file_ownership",
		})
	}

	// MR02: If file_ownership spans 2+ repos, quality gates must have repo: scoping.
	// Without repo:, gates run in every repo — a docs-only repo has no Go module
	// and `go build ./...` will fail.
	if hasExplicit {
		repoSet := make(map[string]bool)
		for _, fo := range m.FileOwnership {
			if fo.Repo != "" {
				repoSet[fo.Repo] = true
			}
		}
		if len(repoSet) >= 2 && m.QualityGates != nil {
			for i, gate := range m.QualityGates.Gates {
				if gate.Repo == "" {
					errs = append(errs, result.SAWError{
						Code:     result.CodeUnscopedGate,
						Message:  fmt.Sprintf("quality_gates.gates[%d] (%s): multi-repo IMPL requires repo: on every gate — without it, '%s' runs in all repos including docs-only repos with no build system", i, gate.Type, gate.Command),
						Severity: "error",
						Field:    fmt.Sprintf("quality_gates.gates[%d].repo", i),
					})
				}
			}
		}
	}

	return errs
}

// ValidateBytes unmarshals raw YAML into an IMPLManifest, runs Validate(), and
// also runs DetectUnknownKeys() on the raw bytes to catch keys silently dropped
// by Go's YAML unmarshaler. Returns the combined set of SAWErrors.
//
// Use ValidateBytes when you have the raw YAML source (e.g., reading from disk).
// Use Validate when you already have a parsed *IMPLManifest and only need
// structural/invariant checks (unknown-key detection will not run).
func ValidateBytes(ctx context.Context, yamlData []byte) ([]result.SAWError, error) {
	_ = ctx // reserved for future context-aware validation (e.g., cancellation)
	// Cannot use LoadYAML: yamlData is a []byte parameter, not a file path.
	var m IMPLManifest
	if err := yaml.Unmarshal(yamlData, &m); err != nil {
		return nil, fmt.Errorf("ValidateBytes: unmarshal YAML: %w", err)
	}

	var errs []result.SAWError
	errs = append(errs, Validate(&m)...)
	errs = append(errs, DetectUnknownKeys(yamlData)...)
	return errs, nil
}

// validateGateTypes checks that all quality gate types are valid.
// Valid types: "build", "lint", "test", "typecheck", "custom"
func validateGateTypes(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// If no quality gates defined, return empty
	if m.QualityGates == nil {
		return errs
	}

	for i, gate := range m.QualityGates.Gates {
		if !ValidGateTypes[gate.Type] {
			errs = append(errs, result.SAWError{
				Code:     result.CodeInvalidGateType,
				Message:  fmt.Sprintf("quality gate type %q is invalid — must be one of: build, lint, test, typecheck, format, custom", gate.Type),
				Severity: "error",
				Field:    fmt.Sprintf("quality_gates.gates[%d].type", i),
			})
		}
	}

	return errs
}
