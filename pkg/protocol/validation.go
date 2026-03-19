package protocol

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// agentIDRegex validates agent IDs: one uppercase letter, optionally followed by a digit 2-9
var agentIDRegex = regexp.MustCompile(`^[A-Z][2-9]?$`)

// Validate runs all I1-I6 invariant checks on an IMPLManifest.
// Returns a slice of ValidationErrors (empty if valid).
// Multiple violations may be returned together for comprehensive reporting.
// Note: unknown-key detection requires raw YAML bytes and cannot be performed here.
// Use ValidateBytes to run both structural validation and unknown-key detection together.
func Validate(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateI1DisjointOwnership(m)...)
	errs = append(errs, validateI2AgentDependencies(m)...)
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

	return errs
}

// validateI1DisjointOwnership checks that no file is owned by multiple agents within the same wave.
// Files may be owned by different agents across different waves (sequential modification).
func validateI1DisjointOwnership(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

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
			errs = append(errs, ValidationError{
				Code:    "I1_VIOLATION",
				Message: fmt.Sprintf("file %q owned by multiple agents in wave %d: %v", key.file, key.wave, agents),
				Field:   "file_ownership",
			})
		}
	}

	return errs
}

// validateI2AgentDependencies checks that all agent dependencies reference agents in prior waves only.
// An agent in wave N may only depend on agents in waves 1..(N-1).
func validateI2AgentDependencies(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

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
					errs = append(errs, ValidationError{
						Code:    "I2_MISSING_DEP",
						Message: fmt.Sprintf("agent %s (wave %d) depends on unknown agent %q", agent.ID, wave.Number, dep),
						Field:   fmt.Sprintf("waves[%d].agents[%s].dependencies", wave.Number-1, agent.ID),
					})
				} else if depWave >= wave.Number {
					errs = append(errs, ValidationError{
						Code:    "I2_WAVE_ORDER",
						Message: fmt.Sprintf("agent %s (wave %d) depends on %s (wave %d) — dependencies must be in prior waves", agent.ID, wave.Number, dep, depWave),
						Field:   fmt.Sprintf("waves[%d].agents[%s].dependencies", wave.Number-1, agent.ID),
					})
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
				errs = append(errs, ValidationError{
					Code:    "I2_MISSING_DEP",
					Message: fmt.Sprintf("file %q (agent %s, wave %d) depends on unknown agent %q", fo.File, fo.Agent, fo.Wave, depAgent),
					Field:   "file_ownership",
				})
			} else if depWave >= fo.Wave {
				errs = append(errs, ValidationError{
					Code:    "I2_WAVE_ORDER",
					Message: fmt.Sprintf("file %q (agent %s, wave %d) depends on agent %s (wave %d) — dependencies must be in prior waves", fo.File, fo.Agent, fo.Wave, depAgent, depWave),
					Field:   "file_ownership",
				})
			}
		}
	}

	return errs
}

// validateI3WaveOrdering checks that wave numbers are sequential starting from 1.
func validateI3WaveOrdering(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	if len(m.Waves) == 0 {
		return errs
	}

	// Check for sequential numbering: 1, 2, 3, ...
	for i, wave := range m.Waves {
		expected := i + 1
		if wave.Number != expected {
			errs = append(errs, ValidationError{
				Code:    "I3_WAVE_ORDER",
				Message: fmt.Sprintf("wave number mismatch: expected wave %d, got wave %d", expected, wave.Number),
				Field:   fmt.Sprintf("waves[%d].number", i),
			})
		}
	}

	return errs
}

// validateI4RequiredFields checks that all required manifest fields are present and non-empty.
func validateI4RequiredFields(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(m.Title) == "" {
		errs = append(errs, ValidationError{
			Code:    "I4_MISSING_FIELD",
			Message: "title is required",
			Field:   "title",
		})
	}

	if strings.TrimSpace(m.FeatureSlug) == "" {
		errs = append(errs, ValidationError{
			Code:    "I4_MISSING_FIELD",
			Message: "feature_slug is required",
			Field:   "feature_slug",
		})
	}

	if strings.TrimSpace(m.Verdict) == "" {
		errs = append(errs, ValidationError{
			Code:    "I4_MISSING_FIELD",
			Message: "verdict is required",
			Field:   "verdict",
		})
	} else {
		// Validate verdict value
		validVerdicts := map[string]bool{
			"SUITABLE":               true,
			"NOT_SUITABLE":           true,
			"SUITABLE_WITH_CAVEATS":  true,
		}
		if !validVerdicts[m.Verdict] {
			errs = append(errs, ValidationError{
				Code:    "I4_INVALID_VALUE",
				Message: fmt.Sprintf("verdict must be SUITABLE, NOT_SUITABLE, or SUITABLE_WITH_CAVEATS, got %q", m.Verdict),
				Field:   "verdict",
			})
		}
	}

	return errs
}

// validateI5FileOwnershipComplete checks that all files referenced in agent.Files are present in FileOwnership table.
func validateI5FileOwnershipComplete(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

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
					errs = append(errs, ValidationError{
						Code:    "I5_ORPHAN_FILE",
						Message: fmt.Sprintf("agent %s (wave %d) references file %q which is not in file_ownership table", agent.ID, wave.Number, file),
						Field:   fmt.Sprintf("waves[%d].agents[%s].files", wave.Number-1, agent.ID),
					})
				}
			}
		}
	}

	return errs
}

// validateI6NoCycles checks that the dependency graph is acyclic.
// Uses depth-first search with a recursion stack to detect cycles.
func validateI6NoCycles(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

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
				errs = append(errs, ValidationError{
					Code:    "I6_CYCLE",
					Message: fmt.Sprintf("dependency cycle detected: %s", strings.Join(cycle, " -> ")),
					Field:   "waves",
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
func validateI5CommitBeforeReport(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	for agentID, report := range m.CompletionReports {
		if strings.TrimSpace(report.Commit) == "" || report.Commit == "uncommitted" {
			errs = append(errs, ValidationError{
				Code:    "I5_UNCOMMITTED",
				Message: fmt.Sprintf("agent %s completion report has no valid commit (commit=%q) — agents must commit before reporting", agentID, report.Commit),
				Field:   fmt.Sprintf("completion_reports[%s].commit", agentID),
			})
		}
	}

	return errs
}

// validateE9MergeState checks that merge_state field contains a valid value.
// Valid values: "idle", "in_progress", "completed", "failed".
// Empty/omitted values are valid (backward compatibility).
func validateE9MergeState(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

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
		errs = append(errs, ValidationError{
			Code:    "E9_INVALID_MERGE_STATE",
			Message: fmt.Sprintf("merge_state has invalid value %q — must be one of: idle, in_progress, completed, failed", m.MergeState),
			Field:   "merge_state",
		})
	}

	return errs
}

// validateSM01StateValid checks that state field contains a valid ProtocolState value.
// Empty/omitted values are valid (backward compatibility).
func validateSM01StateValid(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// Empty is valid (backward compat)
	if strings.TrimSpace(string(m.State)) == "" {
		return errs
	}

	validStates := map[ProtocolState]bool{
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
		errs = append(errs, ValidationError{
			Code:    "SM01_INVALID_STATE",
			Message: fmt.Sprintf("state has invalid value %q — must be one of: SCOUT_PENDING, SCOUT_VALIDATING, REVIEWED, SCAFFOLD_PENDING, WAVE_PENDING, WAVE_EXECUTING, WAVE_MERGING, WAVE_VERIFIED, BLOCKED, COMPLETE, NOT_SUITABLE", m.State),
			Field:   "state",
		})
	}

	return errs
}

// validateAgentIDs checks that all agent IDs conform to the protocol regex: ^[A-Z][2-9]?$
// Valid examples: "A", "B", "C2", "D9"
// Invalid examples: "a", "AB", "A1", "A10", "1A", ""
func validateAgentIDs(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// Check agent IDs in wave definitions
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			if !agentIDRegex.MatchString(agent.ID) {
				errs = append(errs, ValidationError{
					Code:    "DC04_INVALID_AGENT_ID",
					Message: fmt.Sprintf("agent ID %q in wave %d does not match protocol pattern ^[A-Z][2-9]?$ (one uppercase letter, optionally followed by digit 2-9)", agent.ID, wave.Number),
					Field:   fmt.Sprintf("waves[%d].agents[%s].id", wave.Number-1, agent.ID),
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
			errs = append(errs, ValidationError{
				Code:    "DC04_INVALID_AGENT_ID",
				Message: fmt.Sprintf("agent ID %q in file_ownership entry %d (file=%q) does not match protocol pattern ^[A-Z][2-9]?$", fo.Agent, i, fo.File),
				Field:   fmt.Sprintf("file_ownership[%d].agent", i),
			})
		}
	}

	// Check agent IDs in CompletionReports map keys
	for agentID := range m.CompletionReports {
		if !agentIDRegex.MatchString(agentID) {
			errs = append(errs, ValidationError{
				Code:    "DC04_INVALID_AGENT_ID",
				Message: fmt.Sprintf("agent ID %q in completion_reports does not match protocol pattern ^[A-Z][2-9]?$", agentID),
				Field:   fmt.Sprintf("completion_reports[%s]", agentID),
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
func validateMultiRepoConsistency(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

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
		errs = append(errs, ValidationError{
			Code:    "MR01_INCONSISTENT_REPO",
			Message: fmt.Sprintf("file_ownership has mixed repo tags: some entries have repo: and some don't — add explicit repo: to all entries (missing on: %s%s)", strings.Join(missing, ", "), suffix),
			Field:   "file_ownership",
		})
	}

	return errs
}

// ValidateBytes unmarshals raw YAML into an IMPLManifest, runs Validate(), and
// also runs DetectUnknownKeys() on the raw bytes to catch keys silently dropped
// by Go's YAML unmarshaler. Returns the combined set of ValidationErrors.
//
// Use ValidateBytes when you have the raw YAML source (e.g., reading from disk).
// Use Validate when you already have a parsed *IMPLManifest and only need
// structural/invariant checks (unknown-key detection will not run).
func ValidateBytes(yamlData []byte) ([]ValidationError, error) {
	var m IMPLManifest
	if err := yaml.Unmarshal(yamlData, &m); err != nil {
		return nil, fmt.Errorf("ValidateBytes: unmarshal YAML: %w", err)
	}

	var errs []ValidationError
	errs = append(errs, Validate(&m)...)
	errs = append(errs, DetectUnknownKeys(yamlData)...)
	return errs, nil
}

// validateGateTypes checks that all quality gate types are valid.
// Valid types: "build", "lint", "test", "typecheck", "custom"
func validateGateTypes(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// If no quality gates defined, return empty
	if m.QualityGates == nil {
		return errs
	}

	for i, gate := range m.QualityGates.Gates {
		if !ValidGateTypes[gate.Type] {
			errs = append(errs, ValidationError{
				Code:    "DC07_INVALID_GATE_TYPE",
				Message: fmt.Sprintf("quality gate type %q is invalid — must be one of: build, lint, test, typecheck, format, custom", gate.Type),
				Field:   fmt.Sprintf("quality_gates.gates[%d].type", i),
			})
		}
	}

	return errs
}
