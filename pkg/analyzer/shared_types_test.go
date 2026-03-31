package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectSharedTypes_BasicCase(t *testing.T) {
	// Agent A owns models.rs, Agents B and C both import PreviewData from models
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "src/models.rs", Agent: "A", Wave: 1},
			{File: "src/upgrade/splitter.rs", Agent: "B", Wave: 1},
			{File: "src/upgrade/mod.rs", Agent: "C", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Define PreviewData struct in src/models.rs",
						Files: []string{"src/models.rs"},
					},
					{
						ID:    "B",
						Task:  "import PreviewData from crate::models",
						Files: []string{"src/upgrade/splitter.rs"},
					},
					{
						ID:    "C",
						Task:  "import PreviewData from crate::models",
						Files: []string{"src/upgrade/mod.rs"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)

	assert.Equal(t, "PreviewData", candidates[0].TypeName)
	assert.Equal(t, "A", candidates[0].DefiningAgent)
	assert.Equal(t, "src/models.rs", candidates[0].DefiningFile)
	assert.ElementsMatch(t, []string{"B", "C"}, candidates[0].ReferencingAgents)
	assert.Contains(t, candidates[0].Reason, "Agents")
}

func TestDetectSharedTypes_NoSharing(t *testing.T) {
	// Each agent imports only from external packages
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "src/main.rs", Agent: "A", Wave: 1},
			{File: "src/lib.rs", Agent: "B", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "import std::collections::HashMap",
						Files: []string{"src/main.rs"},
					},
					{
						ID:    "B",
						Task:  "import tokio::runtime::Runtime",
						Files: []string{"src/lib.rs"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	assert.Empty(t, candidates, "No shared types should be detected when agents only import from external packages")
}

func TestDetectSharedTypes_MultipleRefs(t *testing.T) {
	// Agent A owns models.rs, Agents B and C both import from it
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/models/types.go", Agent: "A", Wave: 1},
			{File: "pkg/handler/http.go", Agent: "B", Wave: 1},
			{File: "pkg/service/processor.go", Agent: "C", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Define Config struct in pkg/models/types.go",
						Files: []string{"pkg/models/types.go"},
					},
					{
						ID:    "B",
						Task:  "reference Config from pkg/models/types.go",
						Files: []string{"pkg/handler/http.go"},
					},
					{
						ID:    "C",
						Task:  "import Config from pkg/models/types.go",
						Files: []string{"pkg/service/processor.go"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)

	assert.Equal(t, "Config", candidates[0].TypeName)
	assert.Equal(t, "A", candidates[0].DefiningAgent)
	assert.Equal(t, "pkg/models/types.go", candidates[0].DefiningFile)
	assert.ElementsMatch(t, []string{"B", "C"}, candidates[0].ReferencingAgents)
	assert.Contains(t, candidates[0].Reason, "Agents")
}

func TestDetectSharedTypes_TypeExistsInCodebase(t *testing.T) {
	// Create a temporary directory and file to simulate existing type
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "pkg", "models")
	require.NoError(t, os.MkdirAll(modelsDir, 0755))
	typesFile := filepath.Join(modelsDir, "types.go")
	require.NoError(t, os.WriteFile(typesFile, []byte("package models\ntype Config struct {}"), 0644))

	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/models/types.go", Agent: "A", Wave: 1},
			{File: "pkg/handler/http.go", Agent: "B", Wave: 1},
			{File: "pkg/service/processor.go", Agent: "C", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Update Config struct in pkg/models/types.go",
						Files: []string{"pkg/models/types.go"},
					},
					{
						ID:    "B",
						Task:  "reference Config from pkg/models/types.go",
						Files: []string{"pkg/handler/http.go"},
					},
					{
						ID:    "C",
						Task:  "import Config from pkg/models/types.go",
						Files: []string{"pkg/service/processor.go"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, tmpDir)
	require.NoError(t, err)
	require.Len(t, candidates, 1)

	assert.Equal(t, "Config", candidates[0].TypeName)
	assert.Contains(t, candidates[0].Reason, "Type exists")
	assert.Contains(t, candidates[0].Reason, "verify imports are correct")
}

func TestDetectSharedTypes_CircularDep(t *testing.T) {
	// Agent A depends on B, B depends on A
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/a.go", Agent: "A", Wave: 1},
			{File: "pkg/b.go", Agent: "B", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:           "A",
						Task:         "Implement A",
						Files:        []string{"pkg/a.go"},
						Dependencies: []string{"B"},
					},
					{
						ID:           "B",
						Task:         "Implement B",
						Files:        []string{"pkg/b.go"},
						Dependencies: []string{"A"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
	assert.Nil(t, candidates)
}

func TestDetectSharedTypes_NilManifest(t *testing.T) {
	candidates, err := DetectSharedTypes(context.Background(), nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest cannot be nil")
	assert.Empty(t, candidates)
}

func TestDetectSharedTypes_EmptyManifest(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{},
		Waves:         []protocol.Wave{},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestDetectSharedTypes_SingleReference(t *testing.T) {
	// Only one agent references the type, should not be a candidate
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/types.go", Agent: "A", Wave: 1},
			{File: "pkg/handler.go", Agent: "B", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Define MyType struct",
						Files: []string{"pkg/types.go"},
					},
					{
						ID:    "B",
						Task:  "reference MyType from pkg/types.go",
						Files: []string{"pkg/handler.go"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	assert.Empty(t, candidates, "Should not detect types referenced by only one agent")
}

func TestDetectSharedTypes_TypeScriptImports(t *testing.T) {
	// Test TypeScript import pattern
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "src/types.ts", Agent: "A", Wave: 1},
			{File: "src/handler.ts", Agent: "B", Wave: 1},
			{File: "src/service.ts", Agent: "C", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Define User interface",
						Files: []string{"src/types.ts"},
					},
					{
						ID:    "B",
						Task:  "import { User } from \"./types\"",
						Files: []string{"src/handler.ts"},
					},
					{
						ID:    "C",
						Task:  "import { User } from './types'",
						Files: []string{"src/service.ts"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "User", candidates[0].TypeName)
	assert.ElementsMatch(t, []string{"B", "C"}, candidates[0].ReferencingAgents)
}

func TestDetectSharedTypes_PythonImports(t *testing.T) {
	// Test Python import pattern
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "models/user.py", Agent: "A", Wave: 1},
			{File: "handlers/auth.py", Agent: "B", Wave: 1},
			{File: "services/user.py", Agent: "C", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Define User class",
						Files: []string{"models/user.py"},
					},
					{
						ID:    "B",
						Task:  "from models.user import User",
						Files: []string{"handlers/auth.py"},
					},
					{
						ID:    "C",
						Task:  "from models.user import User",
						Files: []string{"services/user.py"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "User", candidates[0].TypeName)
	assert.Equal(t, "models/user.py", candidates[0].DefiningFile)
	assert.ElementsMatch(t, []string{"B", "C"}, candidates[0].ReferencingAgents)
}

func TestDetectSharedTypes_MultipleTypesFromSameFile(t *testing.T) {
	// Multiple types referenced from the same file
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/models/types.go", Agent: "A", Wave: 1},
			{File: "pkg/handler.go", Agent: "B", Wave: 1},
			{File: "pkg/service.go", Agent: "C", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "Define Config and State structs",
						Files: []string{"pkg/models/types.go"},
					},
					{
						ID:    "B",
						Task:  "reference Config from pkg/models/types.go and reference State from pkg/models/types.go",
						Files: []string{"pkg/handler.go"},
					},
					{
						ID:    "C",
						Task:  "import Config from pkg/models/types.go",
						Files: []string{"pkg/service.go"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1, "Only Config should be detected as shared (State only referenced by B)")
	assert.Equal(t, "Config", candidates[0].TypeName)
	assert.ElementsMatch(t, []string{"B", "C"}, candidates[0].ReferencingAgents)
}

func TestDetectSharedTypes_NotOwnedByAnyAgent(t *testing.T) {
	// Type is imported from a file not owned by any agent (existing infrastructure)
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/handler.go", Agent: "A", Wave: 1},
			{File: "pkg/service.go", Agent: "B", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  "reference Logger from pkg/common/logger.go",
						Files: []string{"pkg/handler.go"},
					},
					{
						ID:    "B",
						Task:  "import Logger from pkg/common/logger.go",
						Files: []string{"pkg/service.go"},
					},
				},
			},
		},
	}

	candidates, err := DetectSharedTypes(context.Background(), manifest, "")
	require.NoError(t, err)
	assert.Empty(t, candidates, "Should not detect types from files not owned by any agent")
}
