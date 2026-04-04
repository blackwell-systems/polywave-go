package analyzer

import (
	"context"
	"sort"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectWiring_DirectCall(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/engine/scout.go", Agent: "A", Wave: 1},
			{File: "pkg/cli/main.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "RunScout",
				Definition: "func RunScout() error",
				Location:   "pkg/engine/scout.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement RunScout function"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "CLI that calls `RunScout()` to start scout"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	require.Len(t, declarations, 1)

	assert.Equal(t, "RunScout", declarations[0].Symbol)
	assert.Equal(t, "pkg/engine/scout.go", declarations[0].DefinedIn)
	assert.Equal(t, "pkg/cli/main.go", declarations[0].MustBeCalledFrom)
	assert.Equal(t, "B", declarations[0].Agent)
	assert.Equal(t, 2, declarations[0].Wave)
	assert.Equal(t, "call", declarations[0].IntegrationPattern)
}

func TestDetectWiring_PackageQualified(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/engine/scout.go", Agent: "A", Wave: 1},
			{File: "pkg/cli/commands.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "engine.RunScout",
				Definition: "func RunScout() error",
				Location:   "pkg/engine/scout.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement scout engine"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "Command uses `engine.RunScout` to execute"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	require.Len(t, declarations, 1)

	assert.Equal(t, "RunScout", declarations[0].Symbol)
	assert.Equal(t, "pkg/engine/scout.go", declarations[0].DefinedIn)
	assert.Equal(t, "pkg/cli/commands.go", declarations[0].MustBeCalledFrom)
	assert.Equal(t, "B", declarations[0].Agent)
	assert.Equal(t, 2, declarations[0].Wave)
}

func TestDetectWiring_Delegation(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/critic/engine.go", Agent: "A", Wave: 1},
			{File: "pkg/orchestrator/wave.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "RunCritic",
				Definition: "func RunCritic() error",
				Location:   "pkg/critic/engine.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement critic"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "Orchestrator delegates to `RunCritic` for quality checks"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	require.Len(t, declarations, 1)

	assert.Equal(t, "RunCritic", declarations[0].Symbol)
	assert.Equal(t, "pkg/critic/engine.go", declarations[0].DefinedIn)
	assert.Equal(t, "pkg/orchestrator/wave.go", declarations[0].MustBeCalledFrom)
	assert.Equal(t, "B", declarations[0].Agent)
	assert.Equal(t, 2, declarations[0].Wave)
}

func TestDetectWiring_Invokes(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/validator/check.go", Agent: "A", Wave: 1},
			{File: "pkg/cli/verify.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "ValidateManifest",
				Definition: "func ValidateManifest() error",
				Location:   "pkg/validator/check.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement validator"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "CLI invokes `ValidateManifest` before wave execution"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	require.Len(t, declarations, 1)

	assert.Equal(t, "ValidateManifest", declarations[0].Symbol)
	assert.Equal(t, "pkg/validator/check.go", declarations[0].DefinedIn)
	assert.Equal(t, "pkg/cli/verify.go", declarations[0].MustBeCalledFrom)
	assert.Equal(t, "B", declarations[0].Agent)
}

func TestDetectWiring_SameAgentNoWiring(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/engine/scout.go", Agent: "A", Wave: 1},
			{File: "pkg/engine/helpers.go", Agent: "A", Wave: 1},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "helperFunc",
				Definition: "func helperFunc() string",
				Location:   "pkg/engine/helpers.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement scout that calls `helperFunc()` internally"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	assert.Len(t, declarations, 0, "Same agent calling its own function should not emit wiring")
}

func TestDetectWiring_NoInterfaceContract(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/cli/main.go", Agent: "A", Wave: 1},
		},
		InterfaceContracts: []protocol.InterfaceContract{},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "CLI calls `fmt.Println()` to print output"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	assert.Len(t, declarations, 0, "External stdlib function should not emit wiring")
}

func TestDetectWiring_MultipleCallers(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/core/validate.go", Agent: "A", Wave: 1},
			{File: "pkg/cli/scout.go", Agent: "B", Wave: 2},
			{File: "pkg/cli/wave.go", Agent: "C", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "ValidateIMPL",
				Definition: "func ValidateIMPL() error",
				Location:   "pkg/core/validate.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement validation"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "Scout command calls `ValidateIMPL()` before generating"},
					{ID: "C", Task: "Wave command calls `ValidateIMPL()` before execution"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	require.Len(t, declarations, 2, "Two agents calling same function should emit 2 wiring entries")

	// Sort by Agent field to eliminate map-iteration ordering flakes (P2-5 fix)
	sort.Slice(declarations, func(i, j int) bool {
		return declarations[i].Agent < declarations[j].Agent
	})

	// Check first wiring entry (Agent B)
	assert.Equal(t, "ValidateIMPL", declarations[0].Symbol)
	assert.Equal(t, "pkg/core/validate.go", declarations[0].DefinedIn)
	assert.Equal(t, "pkg/cli/scout.go", declarations[0].MustBeCalledFrom)
	assert.Equal(t, "B", declarations[0].Agent)
	assert.Equal(t, 2, declarations[0].Wave)

	// Check second wiring entry (Agent C)
	assert.Equal(t, "ValidateIMPL", declarations[1].Symbol)
	assert.Equal(t, "pkg/core/validate.go", declarations[1].DefinedIn)
	assert.Equal(t, "pkg/cli/wave.go", declarations[1].MustBeCalledFrom)
	assert.Equal(t, "C", declarations[1].Agent)
	assert.Equal(t, 2, declarations[1].Wave)
}

func TestDetectWiring_EmptyManifest(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership:      []protocol.FileOwnership{{File: "dummy.go", Agent: "X", Wave: 1}},
		InterfaceContracts: []protocol.InterfaceContract{},
		Waves:              []protocol.Wave{},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	assert.Len(t, declarations, 0, "Empty waves should return empty result")
}

func TestDetectWiring_NilManifest(t *testing.T) {
	res := DetectWiring(context.Background(), nil, "/repo")
	assert.True(t, res.IsFatal())
	assert.True(t, res.HasErrors())
	assert.Contains(t, res.Errors[0].Message, "manifest is nil")
}

func TestDetectWiring_EmptyFileOwnership(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership:      []protocol.FileOwnership{},
		InterfaceContracts: []protocol.InterfaceContract{},
		Waves:              []protocol.Wave{},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	assert.True(t, res.IsFatal())
	assert.True(t, res.HasErrors())
	assert.Contains(t, res.Errors[0].Message, "file_ownership is empty")
}

func TestDetectWiring_MultiplePatternsInOneTask(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/engine/scout.go", Agent: "A", Wave: 1},
			{File: "pkg/engine/wave.go", Agent: "A", Wave: 1},
			{File: "pkg/cli/main.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "RunScout",
				Definition: "func RunScout() error",
				Location:   "pkg/engine/scout.go",
			},
			{
				Name:       "RunWave",
				Definition: "func RunWave() error",
				Location:   "pkg/engine/wave.go",
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement engine"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "CLI calls `RunScout()` and delegates to `RunWave` for execution"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	require.Len(t, declarations, 2, "Should detect multiple function calls in one task")

	symbols := make(map[string]bool)
	for _, decl := range declarations {
		symbols[decl.Symbol] = true
	}
	assert.True(t, symbols["RunScout"])
	assert.True(t, symbols["RunWave"])
}

func TestDetectWiring_ContractWithoutLocation(t *testing.T) {
	// Contract exists but has no location - should use heuristic fallback
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/util/helpers.go", Agent: "A", Wave: 1},
			{File: "pkg/cli/main.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "FormatOutput",
				Definition: "func FormatOutput(s string) string",
				Location:   "", // No location specified
			},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement utilities"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "CLI uses `FormatOutput()` helper"},
				},
			},
		},
	}

	res := DetectWiring(context.Background(), manifest, "/repo")
	require.True(t, res.IsSuccess())
	declarations := res.GetData()
	// Contract exists but location empty - should use heuristic
	require.Len(t, declarations, 1)

	assert.Equal(t, "FormatOutput", declarations[0].Symbol)
	assert.Equal(t, "pkg/util/helpers.go", declarations[0].DefinedIn) // A's first file (heuristic)
	assert.Equal(t, "pkg/cli/main.go", declarations[0].MustBeCalledFrom)
	assert.Equal(t, "B", declarations[0].Agent)
	assert.Equal(t, 2, declarations[0].Wave)
}

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"FunctionName", "FunctionName"},
		{"FunctionName()", "FunctionName"},
		{"pkg.FunctionName", "FunctionName"},
		{"pkg.sub.FunctionName", "FunctionName"},
		{"`FunctionName`", "FunctionName"},
		{"`pkg.FunctionName()`", "FunctionName"},
		{"engine.RunScout", "RunScout"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractFunctionName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
