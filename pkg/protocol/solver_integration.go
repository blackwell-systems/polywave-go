package protocol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/solver"
	"gopkg.in/yaml.v3"
)

// waveMismatch records a discrepancy between the manifest's wave assignment
// and the solver's computed wave assignment for a single agent.
type waveMismatch struct {
	agentID      string
	manifestWave int
	solverWave   int
}

// ValidateWithSolver runs solver-based validation on the manifest, then
// appends the results of the standard Validate() checks. Solver errors
// use SOLVER_* code prefixes to distinguish them from invariant errors.
func ValidateWithSolver(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Convert manifest to solver nodes and run the solver.
	nodes := manifestToNodes(m)
	solveResult := solver.Solve(nodes)

	if !solveResult.Valid {
		// Map solver errors to SAWErrors by inspecting the message.
		for _, errStr := range solveResult.Errors {
			code := "SOLVER_ERROR"
			lower := strings.ToLower(errStr)
			if strings.Contains(lower, "cycle") {
				code = "SOLVER_CYCLE"
			} else if strings.Contains(lower, "missing") || strings.Contains(lower, "unknown") {
				code = "SOLVER_MISSING_DEP"
			}
			errs = append(errs, result.SAWError{
				Code:     code,
				Message:  errStr,
				Severity: "error",
				Field:    "waves",
			})
		}
		// Do not run further solver checks on an invalid graph,
		// but still run base validation.
		errs = append(errs, Validate(m)...)
		return errs
	}

	// Solver succeeded — compare assignments.
	mismatches := compareAssignments(m, solveResult)
	for _, mm := range mismatches {
		errs = append(errs, result.SAWError{
			Code:     "SOLVER_WAVE_MISMATCH",
			Message:  fmt.Sprintf("agent %s is in wave %d but solver computed wave %d", mm.agentID, mm.manifestWave, mm.solverWave),
			Severity: "error",
			Field:    fmt.Sprintf("waves[%d]", mm.manifestWave-1),
		})
	}

	// Append standard validation errors.
	errs = append(errs, Validate(m)...)
	return errs
}

// SolveManifest is the "computed, not guessed" entry point. It runs the solver,
// compares assignments, and if mismatches exist, returns a corrected manifest
// with optimal wave assignments. The changes slice contains human-readable
// descriptions of each reassignment.
func SolveManifest(m *IMPLManifest) (*IMPLManifest, []string, error) {
	nodes := manifestToNodes(m)
	solveResult := solver.Solve(nodes)

	if !solveResult.Valid {
		return nil, nil, fmt.Errorf("solver failed: %s", strings.Join(solveResult.Errors, "; "))
	}

	mismatches := compareAssignments(m, solveResult)
	if len(mismatches) == 0 {
		return m, nil, nil
	}

	fixed, err := applyResult(solveResult, m)
	if err != nil {
		return nil, nil, fmt.Errorf("applying solver result: %w", err)
	}

	changes := make([]string, len(mismatches))
	for i, mm := range mismatches {
		changes[i] = fmt.Sprintf("agent %s: wave %d → wave %d", mm.agentID, mm.manifestWave, mm.solverWave)
	}

	return fixed, changes, nil
}

// manifestToNodes converts an IMPLManifest into a flat slice of solver.DepNode,
// deliberately ignoring the manifest's wave assignments so the solver can
// compute optimal ones. The result is deduplicated and sorted by AgentID.
func manifestToNodes(m *IMPLManifest) []solver.DepNode {
	seen := make(map[string]bool)
	var nodes []solver.DepNode

	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			if seen[agent.ID] {
				continue
			}
			seen[agent.ID] = true

			// Copy slices so the caller can't mutate them.
			var deps []string
			if len(agent.Dependencies) > 0 {
				deps = make([]string, len(agent.Dependencies))
				copy(deps, agent.Dependencies)
			}
			var files []string
			if len(agent.Files) > 0 {
				files = make([]string, len(agent.Files))
				copy(files, agent.Files)
			}

			nodes = append(nodes, solver.DepNode{
				AgentID:   agent.ID,
				DependsOn: deps,
				Files:     files,
			})
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].AgentID < nodes[j].AgentID
	})
	return nodes
}

// applyResult creates a deep copy of the manifest and rebuilds its Waves
// and FileOwnership based on the solver's computed assignments. The input
// manifest is never mutated.
func applyResult(solveResult solver.SolveResult, m *IMPLManifest) (*IMPLManifest, error) {
	if !solveResult.Valid {
		return nil, fmt.Errorf("cannot apply invalid solver result")
	}

	// Build agentID -> computed wave map.
	agentWave := make(map[string]int, len(solveResult.Assignments))
	for _, a := range solveResult.Assignments {
		agentWave[a.AgentID] = a.Wave
	}

	// Deep copy via YAML round-trip.
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest for deep copy: %w", err)
	}
	var copy IMPLManifest
	if err := yaml.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("unmarshaling manifest for deep copy: %w", err)
	}

	// Collect all agents from the copy, keyed by ID for lookup.
	agentMap := make(map[string]Agent)
	for _, wave := range copy.Waves {
		for _, agent := range wave.Agents {
			agentMap[agent.ID] = agent
		}
	}

	// Group agents by their solver-computed wave.
	waveGroups := make(map[int][]Agent)
	for id, wave := range agentWave {
		if agent, ok := agentMap[id]; ok {
			waveGroups[wave] = append(waveGroups[wave], agent)
		}
	}

	// Build sorted wave numbers.
	waveNums := make([]int, 0, len(waveGroups))
	for n := range waveGroups {
		waveNums = append(waveNums, n)
	}
	sort.Ints(waveNums)

	// Rebuild Waves slice.
	newWaves := make([]Wave, 0, len(waveNums))
	for _, n := range waveNums {
		agents := waveGroups[n]
		sort.Slice(agents, func(i, j int) bool {
			return agents[i].ID < agents[j].ID
		})
		newWaves = append(newWaves, Wave{
			Number: n,
			Agents: agents,
		})
	}
	copy.Waves = newWaves

	// Update FileOwnership wave numbers.
	for i := range copy.FileOwnership {
		if w, ok := agentWave[copy.FileOwnership[i].Agent]; ok {
			copy.FileOwnership[i].Wave = w
		}
	}

	return &copy, nil
}

// compareAssignments checks the manifest's current wave assignments against the
// solver's computed assignments and returns any mismatches. Returns nil if all
// assignments match.
func compareAssignments(m *IMPLManifest, solveResult solver.SolveResult) []waveMismatch {
	// Build agent -> manifest wave map.
	manifestWave := make(map[string]int)
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			manifestWave[agent.ID] = wave.Number
		}
	}

	var mismatches []waveMismatch
	for _, a := range solveResult.Assignments {
		mw, ok := manifestWave[a.AgentID]
		if !ok {
			continue
		}
		if mw != a.Wave {
			mismatches = append(mismatches, waveMismatch{
				agentID:      a.AgentID,
				manifestWave: mw,
				solverWave:   a.Wave,
			})
		}
	}

	if len(mismatches) == 0 {
		return nil
	}

	// Sort for determinism.
	sort.Slice(mismatches, func(i, j int) bool {
		return mismatches[i].agentID < mismatches[j].agentID
	})
	return mismatches
}
