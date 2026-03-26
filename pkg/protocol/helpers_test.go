package protocol

import "testing"

func TestIsSoloWave_True(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A", Files: []string{"file1.go"}},
		},
	}

	if !IsSoloWave(wave) {
		t.Errorf("IsSoloWave(%+v) = false, want true", wave)
	}
}

func TestIsSoloWave_False(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A", Files: []string{"file1.go"}},
			{ID: "B", Task: "Task B", Files: []string{"file2.go"}},
		},
	}

	if IsSoloWave(wave) {
		t.Errorf("IsSoloWave(%+v) = true, want false", wave)
	}
}

func TestIsSoloWave_Nil(t *testing.T) {
	if IsSoloWave(nil) {
		t.Errorf("IsSoloWave(nil) = true, want false")
	}
}

func TestIsSoloWave_Empty(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{},
	}

	if IsSoloWave(wave) {
		t.Errorf("IsSoloWave(empty wave) = true, want false")
	}
}

func TestIsWaveComplete_AllComplete(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A"},
			{ID: "B", Task: "Task B"},
		},
	}

	reports := map[string]CompletionReport{
		"A": {Status: "complete"},
		"B": {Status: "complete"},
	}

	if !IsWaveComplete(wave, reports) {
		t.Errorf("IsWaveComplete(%+v, %+v) = false, want true", wave, reports)
	}
}

func TestIsWaveComplete_OneMissing(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A"},
			{ID: "B", Task: "Task B"},
		},
	}

	reports := map[string]CompletionReport{
		"A": {Status: "complete"},
		// B is missing
	}

	if IsWaveComplete(wave, reports) {
		t.Errorf("IsWaveComplete(wave with missing agent) = true, want false")
	}
}

func TestIsWaveComplete_OnePartial(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A"},
			{ID: "B", Task: "Task B"},
		},
	}

	reports := map[string]CompletionReport{
		"A": {Status: "complete"},
		"B": {Status: "partial"},
	}

	if IsWaveComplete(wave, reports) {
		t.Errorf("IsWaveComplete(wave with partial agent) = true, want false")
	}
}

func TestIsWaveComplete_Nil(t *testing.T) {
	reports := map[string]CompletionReport{
		"A": {Status: "complete"},
	}

	if IsWaveComplete(nil, reports) {
		t.Errorf("IsWaveComplete(nil wave) = true, want false")
	}
}

func TestIsWaveComplete_EmptyWave(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{},
	}

	reports := map[string]CompletionReport{}

	// An empty wave is technically complete (no agents to check)
	if !IsWaveComplete(wave, reports) {
		t.Errorf("IsWaveComplete(empty wave) = false, want true")
	}
}

func TestIsFinalWave_True(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
			{Number: 2, Agents: []Agent{{ID: "B"}}},
			{Number: 3, Agents: []Agent{{ID: "C"}}},
		},
	}

	if !IsFinalWave(manifest, 3) {
		t.Errorf("IsFinalWave(manifest with 3 waves, waveNumber=3) = false, want true")
	}
}

func TestIsFinalWave_False(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
			{Number: 2, Agents: []Agent{{ID: "B"}}},
			{Number: 3, Agents: []Agent{{ID: "C"}}},
		},
	}

	if IsFinalWave(manifest, 1) {
		t.Errorf("IsFinalWave(manifest with 3 waves, waveNumber=1) = true, want false")
	}
}

func TestIsFinalWave_SingleWave(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
		},
	}

	if !IsFinalWave(manifest, 1) {
		t.Errorf("IsFinalWave(manifest with 1 wave, waveNumber=1) = false, want true")
	}
}

func TestIsFinalWave_NilManifest(t *testing.T) {
	if IsFinalWave(nil, 1) {
		t.Errorf("IsFinalWave(nil manifest) = true, want false")
	}
}

func TestIsFinalWave_EmptyManifest(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{},
	}

	if IsFinalWave(manifest, 1) {
		t.Errorf("IsFinalWave(empty manifest) = true, want false")
	}
}

func TestIsFinalWave_OutOfBounds(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
			{Number: 2, Agents: []Agent{{ID: "B"}}},
		},
	}

	// Wave number too high
	if IsFinalWave(manifest, 5) {
		t.Errorf("IsFinalWave(manifest with 2 waves, waveNumber=5) = true, want false")
	}

	// Wave number zero (invalid)
	if IsFinalWave(manifest, 0) {
		t.Errorf("IsFinalWave(manifest with 2 waves, waveNumber=0) = true, want false")
	}
}

func TestFindWave_Found(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
			{Number: 2, Agents: []Agent{{ID: "B"}}},
		},
	}

	wave := manifest.FindWave(2)
	if wave == nil {
		t.Fatal("FindWave(2) = nil, want non-nil")
	}
	if wave.Number != 2 {
		t.Errorf("FindWave(2).Number = %d, want 2", wave.Number)
	}
}

func TestFindWave_NotFound(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
		},
	}

	wave := manifest.FindWave(99)
	if wave != nil {
		t.Errorf("FindWave(99) = %+v, want nil", wave)
	}
}

func TestFindAgent_Found(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A"},
			{ID: "B", Task: "Task B"},
		},
	}

	agent := wave.FindAgent("B")
	if agent == nil {
		t.Fatal("FindAgent(\"B\") = nil, want non-nil")
	}
	if agent.ID != "B" {
		t.Errorf("FindAgent(\"B\").ID = %q, want \"B\"", agent.ID)
	}
}

func TestFindAgent_NotFound(t *testing.T) {
	wave := &Wave{
		Number: 1,
		Agents: []Agent{
			{ID: "A", Task: "Task A"},
		},
	}

	agent := wave.FindAgent("Z")
	if agent != nil {
		t.Errorf("FindAgent(\"Z\") = %+v, want nil", agent)
	}
}

func TestFindWaveWithAgent_Found(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
			{Number: 2, Agents: []Agent{{ID: "B"}, {ID: "C"}}},
		},
	}

	wave, agent := manifest.FindWaveWithAgent("C")
	if wave == nil {
		t.Fatal("FindWaveWithAgent(\"C\").wave = nil, want non-nil")
	}
	if agent == nil {
		t.Fatal("FindWaveWithAgent(\"C\").agent = nil, want non-nil")
	}
	if wave.Number != 2 {
		t.Errorf("FindWaveWithAgent(\"C\").wave.Number = %d, want 2", wave.Number)
	}
	if agent.ID != "C" {
		t.Errorf("FindWaveWithAgent(\"C\").agent.ID = %q, want \"C\"", agent.ID)
	}
}

func TestFindWaveWithAgent_NotFound(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
		},
	}

	wave, agent := manifest.FindWaveWithAgent("Z")
	if wave != nil {
		t.Errorf("FindWaveWithAgent(\"Z\").wave = %+v, want nil", wave)
	}
	if agent != nil {
		t.Errorf("FindWaveWithAgent(\"Z\").agent = %+v, want nil", agent)
	}
}
