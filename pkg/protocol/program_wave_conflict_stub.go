package protocol

// TEMPORARY STUB — remove after Agent A (saw/program-wave-conflict/wave1-agent-A) merges.
// Provides minimal type and function definitions so Agent B's changes compile
// in isolation. The real implementations live in program_conflict.go (Agent A).

// WaveConflict describes a file owned by multiple IMPLs in the same wave number.
type WaveConflict struct {
	File    string   `json:"file"`
	WaveNum int      `json:"wave_num"`
	Impls   []string `json:"impls"`
	Repos   []string `json:"repos,omitempty"`
}

// WaveConflictReport extends ConflictReport with per-wave analysis.
// ConflictReport.Conflicts (IMPL-level) is still populated for backwards compat.
// WaveConflicts contains only the concurrent-wave-level conflicts.
// SerialWaves maps each IMPL slug to the set of wave numbers that must serialize.
type WaveConflictReport struct {
	ConflictReport                 // embedded: Conflicts, DisjointSets, TierSuggestion
	WaveConflicts []WaveConflict   `json:"wave_conflicts"`
	SerialWaves   map[string][]int `json:"serial_waves"`
}

// CheckIMPLConflictsWaveLevel performs wave-aware cross-IMPL conflict detection.
// STUB: returns a WaveConflictReport wrapping a zero-value ConflictReport.
// Real implementation provided by Agent A in program_conflict.go.
func CheckIMPLConflictsWaveLevel(implRefs []string, repoPath string) (*WaveConflictReport, error) {
	report, err := CheckIMPLConflicts(implRefs, repoPath)
	if err != nil {
		return nil, err
	}
	return &WaveConflictReport{ConflictReport: *report}, nil
}
