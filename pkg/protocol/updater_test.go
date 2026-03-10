package protocol

import (
	"errors"
	"strings"
	"testing"
)

// TestUpdateAgentPrompt_Success verifies that UpdateAgentPrompt updates the agent task in the correct wave.
func TestUpdateAgentPrompt_Success(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Original task A",
						Files: []string{"file_a.go"},
					},
					{
						ID:    "B",
						Task:  "Original task B",
						Files: []string{"file_b.go"},
					},
				},
			},
		},
	}

	err := UpdateAgentPrompt(m, "A", "Updated task A")
	if err != nil {
		t.Fatalf("UpdateAgentPrompt returned unexpected error: %v", err)
	}

	if m.Waves[0].Agents[0].Task != "Updated task A" {
		t.Errorf("Expected agent A task to be 'Updated task A', got %q", m.Waves[0].Agents[0].Task)
	}

	// Verify agent B task was not changed
	if m.Waves[0].Agents[1].Task != "Original task B" {
		t.Errorf("Expected agent B task to remain unchanged, got %q", m.Waves[0].Agents[1].Task)
	}
}

// TestUpdateAgentPrompt_NotFound verifies that UpdateAgentPrompt returns ErrAgentNotFound for unknown agent.
func TestUpdateAgentPrompt_NotFound(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Original task A",
						Files: []string{"file_a.go"},
					},
				},
			},
		},
	}

	err := UpdateAgentPrompt(m, "Z", "New task")
	if err == nil {
		t.Fatal("Expected error for unknown agent ID, got nil")
	}

	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("Expected ErrAgentNotFound, got %v", err)
	}

	if !strings.Contains(err.Error(), "Z") {
		t.Errorf("Expected error message to contain agent ID 'Z', got %q", err.Error())
	}
}

// TestUpdateAgentPrompt_EmptyID verifies that UpdateAgentPrompt returns error for empty agent ID.
func TestUpdateAgentPrompt_EmptyID(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Original task A",
						Files: []string{"file_a.go"},
					},
				},
			},
		},
	}

	err := UpdateAgentPrompt(m, "", "New task")
	if err == nil {
		t.Fatal("Expected error for empty agent ID, got nil")
	}

	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected error message to mention empty ID, got %q", err.Error())
	}
}

// TestUpdateAgentPrompt_MultiWave verifies that UpdateAgentPrompt finds and updates agent in wave 2.
func TestUpdateAgentPrompt_MultiWave(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Original task A",
						Files: []string{"file_a.go"},
					},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{
						ID:    "B",
						Task:  "Original task B",
						Files: []string{"file_b.go"},
					},
					{
						ID:    "C",
						Task:  "Original task C",
						Files: []string{"file_c.go"},
					},
				},
			},
		},
	}

	err := UpdateAgentPrompt(m, "C", "Updated task C")
	if err != nil {
		t.Fatalf("UpdateAgentPrompt returned unexpected error: %v", err)
	}

	if m.Waves[1].Agents[1].Task != "Updated task C" {
		t.Errorf("Expected agent C task to be 'Updated task C', got %q", m.Waves[1].Agents[1].Task)
	}

	// Verify agents in wave 1 are unchanged
	if m.Waves[0].Agents[0].Task != "Original task A" {
		t.Errorf("Expected agent A task to remain unchanged, got %q", m.Waves[0].Agents[0].Task)
	}

	// Verify agent B in wave 2 is unchanged
	if m.Waves[1].Agents[0].Task != "Original task B" {
		t.Errorf("Expected agent B task to remain unchanged, got %q", m.Waves[1].Agents[0].Task)
	}
}

// TestUpdateAgentPrompt_PreservesOtherFields verifies that UpdateAgentPrompt only changes Task field.
func TestUpdateAgentPrompt_PreservesOtherFields(t *testing.T) {
	m := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:           "A",
						Task:         "Original task A",
						Files:        []string{"file_a.go", "file_a2.go"},
						Dependencies: []string{"B", "C"},
						Model:        "sonnet-4",
					},
				},
			},
		},
	}

	err := UpdateAgentPrompt(m, "A", "Updated task A with new prompt text")
	if err != nil {
		t.Fatalf("UpdateAgentPrompt returned unexpected error: %v", err)
	}

	agent := m.Waves[0].Agents[0]

	if agent.Task != "Updated task A with new prompt text" {
		t.Errorf("Expected task to be updated, got %q", agent.Task)
	}

	if agent.ID != "A" {
		t.Errorf("Expected ID to remain 'A', got %q", agent.ID)
	}

	if len(agent.Files) != 2 || agent.Files[0] != "file_a.go" || agent.Files[1] != "file_a2.go" {
		t.Errorf("Expected Files to remain unchanged, got %v", agent.Files)
	}

	if len(agent.Dependencies) != 2 || agent.Dependencies[0] != "B" || agent.Dependencies[1] != "C" {
		t.Errorf("Expected Dependencies to remain unchanged, got %v", agent.Dependencies)
	}

	if agent.Model != "sonnet-4" {
		t.Errorf("Expected Model to remain 'sonnet-4', got %q", agent.Model)
	}
}

// TestUpdateIMPLStatusBytes_Basic verifies basic status update functionality.
func TestUpdateIMPLStatusBytes_Basic(t *testing.T) {
	input := `# IMPL Doc

### Status

| Wave | Agent | Description | Status |
|------|-------|-------------|--------|
| 1 | A | Task A | TO-DO |
| 1 | B | Task B | TO-DO |
| 2 | C | Task C | TO-DO |
`

	output := UpdateIMPLStatusBytes([]byte(input), []string{"A", "C"})
	result := string(output)

	if !strings.Contains(result, "| 1 | A | Task A | DONE  |") {
		t.Errorf("Expected agent A status to be DONE, got:\n%s", result)
	}

	if !strings.Contains(result, "| 1 | B | Task B | TO-DO |") {
		t.Errorf("Expected agent B status to remain TO-DO, got:\n%s", result)
	}

	if !strings.Contains(result, "| 2 | C | Task C | DONE  |") {
		t.Errorf("Expected agent C status to be DONE, got:\n%s", result)
	}
}

// TestUpdateIMPLStatusBytes_Idempotent verifies that already-done agents are not changed.
func TestUpdateIMPLStatusBytes_Idempotent(t *testing.T) {
	input := `# IMPL Doc

### Status

| Wave | Agent | Description | Status |
|------|-------|-------------|--------|
| 1 | A | Task A | DONE |
| 1 | B | Task B | TO-DO |
`

	output := UpdateIMPLStatusBytes([]byte(input), []string{"A", "B"})
	result := string(output)

	// Agent A should remain DONE (not updated since it doesn't have "| TO-DO |")
	if !strings.Contains(result, "| 1 | A | Task A | DONE |") {
		t.Errorf("Expected agent A status to remain unchanged as DONE, got:\n%s", result)
	}

	if !strings.Contains(result, "| 1 | B | Task B | DONE  |") {
		t.Errorf("Expected agent B status to be updated to DONE, got:\n%s", result)
	}
}

// TestUpdateIMPLStatusBytes_NoStatusSection verifies graceful handling when no status section exists.
func TestUpdateIMPLStatusBytes_NoStatusSection(t *testing.T) {
	input := `# IMPL Doc

### Wave 1

Some content here.
`

	output := UpdateIMPLStatusBytes([]byte(input), []string{"A"})
	result := string(output)

	// Should return unchanged content
	if result != input {
		t.Error("Expected content to remain unchanged when no Status section exists")
	}
}

// TestUpdateIMPLStatusBytes_EmptyAgentList verifies that no updates happen with empty agent list.
func TestUpdateIMPLStatusBytes_EmptyAgentList(t *testing.T) {
	input := `# IMPL Doc

### Status

| Wave | Agent | Description | Status |
|------|-------|-------------|--------|
| 1 | A | Task A | TO-DO |
| 1 | B | Task B | TO-DO |
`

	output := UpdateIMPLStatusBytes([]byte(input), []string{})
	result := string(output)

	// All agents should remain TO-DO
	if !strings.Contains(result, "| 1 | A | Task A | TO-DO |") {
		t.Error("Expected agent A status to remain TO-DO")
	}

	if !strings.Contains(result, "| 1 | B | Task B | TO-DO |") {
		t.Error("Expected agent B status to remain TO-DO")
	}
}
