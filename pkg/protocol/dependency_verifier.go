package protocol

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// DependencyVerificationData holds the outcome of checking that all dependency
// agents for a given wave have completed successfully.
type DependencyVerificationData struct {
	Wave   int                    `json:"wave"`
	Valid  bool                   `json:"all_available"`
	Agents []AgentDependencyCheck `json:"agents"`
}

// AgentDependencyCheck records the dependency check for a single agent.
type AgentDependencyCheck struct {
	Agent        string   `json:"agent"`
	Dependencies []string `json:"dependencies"`
	Missing      []string `json:"missing,omitempty"`
	Available    bool     `json:"available"`
}

// VerifyDependenciesAvailable verifies that all dependency outputs exist before
// launching the specified wave. For each agent in waveNum, every agent listed in
// its Dependencies must have a completion report with Status == "complete".
//
// Wave 1 agents (which have no dependencies) will always return Valid=true.
//
// Returns a FATAL result if waveNum is not found in the manifest.
func VerifyDependenciesAvailable(manifest *IMPLManifest, waveNum int) result.Result[DependencyVerificationData] {
	// Find the target wave
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}
	if targetWave == nil {
		return result.NewFailure[DependencyVerificationData]([]result.SAWError{
			{
				Code:     "E_WAVE_NOT_FOUND",
				Message:  formatWaveNotFound(waveNum),
				Severity: "fatal",
			},
		})
	}

	data := DependencyVerificationData{
		Wave:   waveNum,
		Valid:  true,
		Agents: make([]AgentDependencyCheck, 0, len(targetWave.Agents)),
	}

	for _, agent := range targetWave.Agents {
		check := AgentDependencyCheck{
			Agent:        agent.ID,
			Dependencies: agent.Dependencies,
			Available:    true,
		}

		for _, depID := range agent.Dependencies {
			report, exists := manifest.CompletionReports[depID]
			if !exists || report.Status != "complete" {
				check.Missing = append(check.Missing, depID)
				check.Available = false
			}
		}

		if !check.Available {
			data.Valid = false
		}

		data.Agents = append(data.Agents, check)
	}

	return result.NewSuccess(data)
}

// waveNotFoundError is returned when a wave number is not found in the manifest.
type waveNotFoundError struct {
	waveNum int
}

func (e *waveNotFoundError) Error() string {
	return formatWaveNotFound(e.waveNum)
}

func formatWaveNotFound(waveNum int) string {
	return "wave " + itoa(waveNum) + " not found in manifest"
}

// itoa converts an int to a decimal string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
