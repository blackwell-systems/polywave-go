package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunTierGate_AllComplete_GatesPass(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
			{Slug: "impl-b", Status: "complete"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a", "impl-b"},
			},
		},
		TierGates: []QualityGate{
			{
				Type:     "test",
				Command:  "echo 'ok'",
				Required: true,
			},
			{
				Type:     "lint",
				Command:  "echo 'lint ok'",
				Required: false,
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// All IMPLs should be done
	assert.True(t, result.AllImplsDone)
	assert.Equal(t, 2, len(result.ImplStatuses))
	assert.Equal(t, "impl-a", result.ImplStatuses[0].Slug)
	assert.Equal(t, "complete", result.ImplStatuses[0].Status)
	assert.Equal(t, "impl-b", result.ImplStatuses[1].Slug)
	assert.Equal(t, "complete", result.ImplStatuses[1].Status)

	// All gates should pass
	assert.Equal(t, 2, len(result.GateResults))
	assert.True(t, result.GateResults[0].Passed)
	assert.True(t, result.GateResults[1].Passed)

	// Overall should pass
	assert.True(t, result.Passed)
}

func TestRunTierGate_NotAllComplete(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
			{Slug: "impl-b", Status: "executing"},
			{Slug: "impl-c", Status: "pending"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a", "impl-b", "impl-c"},
			},
		},
		TierGates: []QualityGate{
			{
				Type:     "test",
				Command:  "echo 'ok'",
				Required: true,
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// Not all IMPLs should be done
	assert.False(t, result.AllImplsDone)
	assert.Equal(t, 3, len(result.ImplStatuses))

	// Verify individual statuses
	assert.Equal(t, "complete", result.ImplStatuses[0].Status)
	assert.Equal(t, "executing", result.ImplStatuses[1].Status)
	assert.Equal(t, "pending", result.ImplStatuses[2].Status)

	// Should not pass because not all IMPLs are done
	assert.False(t, result.Passed)

	// Gates should not be run
	assert.Equal(t, 0, len(result.GateResults))
}

func TestRunTierGate_GateFails_Required(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a"},
			},
		},
		TierGates: []QualityGate{
			{
				Type:     "build",
				Command:  "echo 'build ok'",
				Required: true,
			},
			{
				Type:     "test",
				Command:  "false", // This will fail
				Required: true,
			},
			{
				Type:     "lint",
				Command:  "echo 'lint ok'",
				Required: false,
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// All IMPLs should be done
	assert.True(t, result.AllImplsDone)

	// All gates should be run
	assert.Equal(t, 3, len(result.GateResults))

	// First gate should pass
	assert.True(t, result.GateResults[0].Passed)
	assert.Equal(t, "build", result.GateResults[0].Type)

	// Second gate should fail
	assert.False(t, result.GateResults[1].Passed)
	assert.Equal(t, "test", result.GateResults[1].Type)
	assert.True(t, result.GateResults[1].Required)

	// Third gate should pass
	assert.True(t, result.GateResults[2].Passed)
	assert.Equal(t, "lint", result.GateResults[2].Type)

	// Overall should fail because a required gate failed
	assert.False(t, result.Passed)
}

func TestRunTierGate_GateFails_NotRequired(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a"},
			},
		},
		TierGates: []QualityGate{
			{
				Type:     "build",
				Command:  "echo 'build ok'",
				Required: true,
			},
			{
				Type:     "lint",
				Command:  "false", // This will fail
				Required: false,   // But it's optional
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// All IMPLs should be done
	assert.True(t, result.AllImplsDone)

	// Both gates should be run
	assert.Equal(t, 2, len(result.GateResults))

	// First gate should pass
	assert.True(t, result.GateResults[0].Passed)

	// Second gate should fail but it's not required
	assert.False(t, result.GateResults[1].Passed)
	assert.False(t, result.GateResults[1].Required)

	// Overall should pass because only optional gate failed
	assert.True(t, result.Passed)
}

func TestRunTierGate_InvalidTier(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a"},
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Request a tier that doesn't exist
	res := RunTierGate(context.Background(), manifest, 99, tmpDir)
	assert.True(t, res.IsFatal(), "expected fatal result for invalid tier")
}

func TestRunTierGate_NoGates(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
			{Slug: "impl-b", Status: "complete"},
		},
		Tiers: []ProgramTier{
			{
				Number: 2,
				Impls:  []string{"impl-a", "impl-b"},
			},
		},
		TierGates: []QualityGate{}, // No gates defined
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 2, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// All IMPLs should be done
	assert.True(t, result.AllImplsDone)

	// No gates to run
	assert.Equal(t, 0, len(result.GateResults))

	// Should pass with no gates
	assert.True(t, result.Passed)
}

func TestRunTierGate_ImplNotFoundInManifest(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
			// impl-b is missing from the impls list
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a", "impl-b"},
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// Not all IMPLs should be done (impl-b is not found)
	assert.False(t, result.AllImplsDone)

	// Check statuses
	assert.Equal(t, 2, len(result.ImplStatuses))
	assert.Equal(t, "impl-a", result.ImplStatuses[0].Slug)
	assert.Equal(t, "complete", result.ImplStatuses[0].Status)
	assert.Equal(t, "impl-b", result.ImplStatuses[1].Slug)
	assert.Equal(t, "not_found", result.ImplStatuses[1].Status)

	// Should not pass
	assert.False(t, result.Passed)
}

func TestRunTierGate_GateCommandWithWorkingDirectory(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a"},
			},
		},
		TierGates: []QualityGate{
			{
				Type:     "test",
				Command:  "pwd",
				Required: true,
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// Gate should pass
	assert.True(t, result.Passed)
	assert.True(t, result.GateResults[0].Passed)

	// Output should contain the temp directory path (check Stdout)
	assert.Contains(t, result.GateResults[0].Stdout, filepath.Base(tmpDir))
}

func TestRunTierGate_ReadsFromDisk_Complete(t *testing.T) {
	// Scenario: PROGRAM manifest has status="pending" but IMPL doc on disk has state: COMPLETE.
	// The tier gate should pass (enriched from disk).
	tmpDir, err := os.MkdirTemp("", "tier-gate-disk-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	require.NoError(t, os.MkdirAll(implDir, 0755))

	implContent := "state: COMPLETE\nfeature_slug: my-impl\ntitle: My Impl\n"
	require.NoError(t, os.WriteFile(filepath.Join(implDir, "IMPL-my-impl.yaml"), []byte(implContent), 0644))

	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "my-impl", Status: "pending"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"my-impl"},
			},
		},
	}

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	require.NotNil(t, result)

	assert.True(t, result.AllImplsDone, "expected AllImplsDone=true when disk state is COMPLETE")
	assert.True(t, result.Passed, "expected Passed=true when disk state is COMPLETE")
}

func TestRunTierGate_FallsBackToManifest_WhenDocMissing(t *testing.T) {
	// Scenario: PROGRAM manifest has status="pending" and no IMPL doc on disk.
	// The tier gate should fail (falls back to manifest status).
	tmpDir, err := os.MkdirTemp("", "tier-gate-fallback-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "missing-impl", Status: "pending"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"missing-impl"},
			},
		},
	}

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	require.NotNil(t, result)

	assert.False(t, result.AllImplsDone, "expected AllImplsDone=false when doc is missing")
	assert.False(t, result.Passed, "expected Passed=false when doc is missing")
}

func TestRunTierGate_MixedStatuses(t *testing.T) {
	manifest := &PROGRAMManifest{
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
			{Slug: "impl-b", Status: "complete"},
			{Slug: "impl-c", Status: "blocked"},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"impl-a", "impl-b", "impl-c"},
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "tier-gate-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	res := RunTierGate(context.Background(), manifest, 1, tmpDir)
	require.False(t, res.IsFatal(), "unexpected fatal: %+v", res.Errors)
	result := res.GetData()
	assert.NotNil(t, result)

	// Not all IMPLs are done
	assert.False(t, result.AllImplsDone)

	// Check individual statuses
	assert.Equal(t, "complete", result.ImplStatuses[0].Status)
	assert.Equal(t, "complete", result.ImplStatuses[1].Status)
	assert.Equal(t, "blocked", result.ImplStatuses[2].Status)

	// Should not pass
	assert.False(t, result.Passed)
}
