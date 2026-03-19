package protocol

import (
	"testing"
)

// makeManifest is a helper that builds a PROGRAMManifest from a slice of
// (slug, status, dependsOn...) tuples.
func makeManifest(impls []ProgramIMPL) *PROGRAMManifest {
	return &PROGRAMManifest{
		Impls: impls,
	}
}

func TestUnblockingScore_SingleNoDeps(t *testing.T) {
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "alpha", Status: "pending"},
	})
	score := UnblockingScore(manifest, "alpha")
	if score != 0 {
		t.Errorf("expected score 0 for single IMPL with no deps, got %d", score)
	}
}

func TestUnblockingScore_LinearChain(t *testing.T) {
	// A -> B -> C means B depends_on A, C depends_on B.
	// UnblockingScore(A): completing A unblocks B and C => 2 * 100 = 200
	// UnblockingScore(B): completing B unblocks C => 1 * 100 = 100
	// UnblockingScore(C): completing C unblocks nothing => 0
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending"},
		{Slug: "B", Status: "pending", DependsOn: []string{"A"}},
		{Slug: "C", Status: "pending", DependsOn: []string{"B"}},
	})

	tests := []struct {
		slug  string
		want  int
	}{
		{"A", 200},
		{"B", 100},
		{"C", 0},
	}
	for _, tt := range tests {
		got := UnblockingScore(manifest, tt.slug)
		if got != tt.want {
			t.Errorf("UnblockingScore(linear, %q) = %d, want %d", tt.slug, got, tt.want)
		}
	}
}

func TestUnblockingScore_Diamond(t *testing.T) {
	// Diamond: B and C both depend on A; D depends on B and C.
	// Completing A unblocks B, C, and D => 3 * 100 = 300
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending"},
		{Slug: "B", Status: "pending", DependsOn: []string{"A"}},
		{Slug: "C", Status: "pending", DependsOn: []string{"A"}},
		{Slug: "D", Status: "pending", DependsOn: []string{"B", "C"}},
	})

	score := UnblockingScore(manifest, "A")
	if score != 300 {
		t.Errorf("UnblockingScore(diamond, A) = %d, want 300", score)
	}
}

func TestUnblockingScore_CycleNoInfiniteLoop(t *testing.T) {
	// A depends on B, B depends on A — mutual cycle.
	// Neither A nor B should count itself. Neither is downstream of the other
	// in a way that terminates, but the visited set prevents infinite loops.
	// Both should return 0 because the only reachable pending IMPL through the
	// reverse graph of A is B, and the only reachable one from B is A, but
	// the BFS starts by marking the start node visited.
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending", DependsOn: []string{"B"}},
		{Slug: "B", Status: "pending", DependsOn: []string{"A"}},
	})

	// This must not panic or hang.
	scoreA := UnblockingScore(manifest, "A")
	scoreB := UnblockingScore(manifest, "B")

	// In a pure 2-cycle the reverse graph has A->B and B->A.
	// BFS from A: visit A, enqueue B (status pending, count=1), then from B
	// enqueue A (already visited, skip). So score = 1*100 = 100.
	// Same logic applies for B. Both are 100.
	if scoreA != 100 {
		t.Errorf("UnblockingScore(cycle, A) = %d, want 100", scoreA)
	}
	if scoreB != 100 {
		t.Errorf("UnblockingScore(cycle, B) = %d, want 100", scoreB)
	}
}

func TestUnblockingScore_CompletedNotCounted(t *testing.T) {
	// B and C depend on A, but B is completed — only C should be counted.
	// UnblockingScore(A) = 1 * 100 = 100 (only C is pending/blocked)
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending"},
		{Slug: "B", Status: "complete", DependsOn: []string{"A"}},
		{Slug: "C", Status: "pending", DependsOn: []string{"A"}},
	})

	score := UnblockingScore(manifest, "A")
	if score != 100 {
		t.Errorf("UnblockingScore(completed not counted) = %d, want 100", score)
	}
}

func TestUnblockingScore_InProgressNotCounted(t *testing.T) {
	// B is in_progress, C is blocked — only C counts.
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending"},
		{Slug: "B", Status: "in_progress", DependsOn: []string{"A"}},
		{Slug: "C", Status: "blocked", DependsOn: []string{"A"}},
	})

	score := UnblockingScore(manifest, "A")
	if score != 100 {
		t.Errorf("UnblockingScore(in_progress not counted) = %d, want 100", score)
	}
}

func TestPrioritizeIMPLs_SortsDescending(t *testing.T) {
	// Linear chain A -> B -> C.
	// Scores: A=200, B=100, C=0.
	// Input in reversed order: [C, B, A] — should come out [A, B, C].
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending"},
		{Slug: "B", Status: "pending", DependsOn: []string{"A"}},
		{Slug: "C", Status: "pending", DependsOn: []string{"B"}},
	})

	got := PrioritizeIMPLs(manifest, []string{"C", "B", "A"})
	want := []string{"A", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("PrioritizeIMPLs: len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("PrioritizeIMPLs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrioritizeIMPLs_StableOnTies(t *testing.T) {
	// Two IMPLs with equal score (both leaf nodes, score=0).
	// Input order [X, Y] must be preserved.
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "X", Status: "pending"},
		{Slug: "Y", Status: "pending"},
	})

	got := PrioritizeIMPLs(manifest, []string{"X", "Y"})
	if got[0] != "X" || got[1] != "Y" {
		t.Errorf("PrioritizeIMPLs stable tie: got %v, want [X Y]", got)
	}
}

func TestPrioritizeIMPLs_DoesNotMutateInput(t *testing.T) {
	manifest := makeManifest([]ProgramIMPL{
		{Slug: "A", Status: "pending"},
		{Slug: "B", Status: "pending", DependsOn: []string{"A"}},
	})

	input := []string{"B", "A"}
	inputCopy := []string{"B", "A"}
	PrioritizeIMPLs(manifest, input)

	for i := range inputCopy {
		if input[i] != inputCopy[i] {
			t.Errorf("PrioritizeIMPLs mutated input at index %d: got %q, want %q", i, input[i], inputCopy[i])
		}
	}
}
