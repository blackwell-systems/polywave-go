package protocol

import (
	"testing"
)

// TestDetectBriefOwnershipDivergence_NoOwnership verifies that a file mentioned
// in a brief with no file_ownership entry generates a WARNING divergence.
func TestDetectBriefOwnershipDivergence_NoOwnership(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:   "A",
						Task: "Implement the handler in pkg/foo.go and call pkg/unowned/bar.go",
					},
				},
			},
		},
	}

	divs := DetectBriefOwnershipDivergence(manifest)

	// pkg/foo.go is owned by A -> no divergence.
	// pkg/unowned/bar.go has no ownership -> divergence.
	foundUnowned := false
	for _, d := range divs {
		if d.FilePath == "pkg/unowned/bar.go" && d.AgentID == "A" && d.OwningAgent == "" {
			foundUnowned = true
		}
		if d.FilePath == "pkg/foo.go" {
			t.Errorf("unexpected divergence for owned file pkg/foo.go: %+v", d)
		}
	}
	if !foundUnowned {
		t.Errorf("expected divergence for pkg/unowned/bar.go, got: %+v", divs)
	}
}

// TestDetectBriefOwnershipDivergence_WrongOwner verifies that a file mentioned
// in a brief but owned by a different agent generates a WARNING divergence.
func TestDetectBriefOwnershipDivergence_WrongOwner(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/a.go", Agent: "A", Wave: 1},
			{File: "pkg/b.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:   "A",
						Task: "Implement pkg/a.go. Do NOT modify pkg/b.go.",
					},
				},
			},
		},
	}

	divs := DetectBriefOwnershipDivergence(manifest)

	// pkg/a.go is owned by A -> no divergence.
	// pkg/b.go is owned by B but mentioned in A's brief -> divergence with OwningAgent=B.
	foundWrongOwner := false
	for _, d := range divs {
		if d.FilePath == "pkg/b.go" && d.AgentID == "A" && d.OwningAgent == "B" {
			foundWrongOwner = true
		}
		if d.FilePath == "pkg/a.go" {
			t.Errorf("unexpected divergence for owned file pkg/a.go: %+v", d)
		}
	}
	if !foundWrongOwner {
		t.Errorf("expected divergence for pkg/b.go (owned by B, mentioned in A), got: %+v", divs)
	}
}

// TestDetectBriefOwnershipDivergence_NilManifest verifies nil safety.
func TestDetectBriefOwnershipDivergence_NilManifest(t *testing.T) {
	divs := DetectBriefOwnershipDivergence(nil)
	if divs != nil {
		t.Errorf("expected nil for nil manifest, got: %+v", divs)
	}
}

// TestDetectBriefOwnershipDivergence_AllOwned verifies no divergences when all
// mentioned files are owned by the mentioning agent.
func TestDetectBriefOwnershipDivergence_AllOwned(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/a.go", Agent: "A", Wave: 1},
			{File: "pkg/b.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:   "A",
						Task: "Implement pkg/a.go and pkg/b.go.",
					},
				},
			},
		},
	}

	divs := DetectBriefOwnershipDivergence(manifest)
	if len(divs) != 0 {
		t.Errorf("expected no divergences when all files are owned by the agent, got: %+v", divs)
	}
}
