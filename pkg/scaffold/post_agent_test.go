package scaffold

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestDetectScaffoldsPostAgent_DuplicateType(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID: "A",
						Task: `## Interfaces to Implement

type AuthToken struct {
    Token string
    UserID string
}`,
						Files: []string{"pkg/auth/token.go"},
					},
					{
						ID: "B",
						Task: `## What to Implement

We need an AuthToken type to store session data.

type AuthToken struct {
    Value string
    Expires time.Time
}`,
						Files: []string{"pkg/session/token.go"},
					},
				},
			},
		},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}

	conflict := result.Conflicts[0]
	if conflict.TypeName != "AuthToken" {
		t.Errorf("expected type name 'AuthToken', got '%s'", conflict.TypeName)
	}

	if len(conflict.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(conflict.Agents))
	}

	// Check agents contains both A and B
	agentMap := make(map[string]bool)
	for _, agent := range conflict.Agents {
		agentMap[agent] = true
	}
	if !agentMap["A"] || !agentMap["B"] {
		t.Errorf("expected agents A and B, got %v", conflict.Agents)
	}

	if len(conflict.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(conflict.Files))
	}

	expectedResolution := "Extract to internal/types/authtoken.go"
	if conflict.Resolution != expectedResolution {
		t.Errorf("expected resolution '%s', got '%s'", expectedResolution, conflict.Resolution)
	}
}

func TestDetectScaffoldsPostAgent_NoDuplicates(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID: "A",
						Task: `type UserProfile struct {
    Name string
}`,
						Files: []string{"pkg/user/profile.go"},
					},
					{
						ID: "B",
						Task: `type SessionData struct {
    Token string
}`,
						Files: []string{"pkg/session/data.go"},
					},
				},
			},
		},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(result.Conflicts))
	}
}

func TestDetectScaffoldsPostAgent_ThreeAgentConflict(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  `type Config struct {}`,
						Files: []string{"pkg/app/config.go"},
					},
					{
						ID:    "B",
						Task:  `type Config struct {}`,
						Files: []string{"pkg/server/config.go"},
					},
					{
						ID:    "C",
						Task:  `type Config struct {}`,
						Files: []string{"pkg/db/config.go"},
					},
				},
			},
		},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}

	conflict := result.Conflicts[0]
	if conflict.TypeName != "Config" {
		t.Errorf("expected type name 'Config', got '%s'", conflict.TypeName)
	}

	if len(conflict.Agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(conflict.Agents))
	}

	// Check all three agents are present
	agentMap := make(map[string]bool)
	for _, agent := range conflict.Agents {
		agentMap[agent] = true
	}
	if !agentMap["A"] || !agentMap["B"] || !agentMap["C"] {
		t.Errorf("expected agents A, B, and C, got %v", conflict.Agents)
	}

	if len(conflict.Files) != 3 {
		t.Errorf("expected 3 files, got %d", len(conflict.Files))
	}
}

func TestDetectScaffoldsPostAgent_SameNameDifferentKind(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID: "A",
						Task: `type AuthToken struct {
    Value string
}`,
						Files: []string{"pkg/auth/token.go"},
					},
					{
						ID: "B",
						Task: `interface AuthToken {
    GetValue() string
}`,
						Files: []string{"pkg/session/token.go"},
					},
				},
			},
		},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still detect as conflict - naming collision regardless of kind
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict (naming collision), got %d", len(result.Conflicts))
	}

	conflict := result.Conflicts[0]
	if conflict.TypeName != "AuthToken" {
		t.Errorf("expected type name 'AuthToken', got '%s'", conflict.TypeName)
	}

	if len(conflict.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(conflict.Agents))
	}
}

func TestExtractTypeNames_PostAgent(t *testing.T) {
	tests := []struct {
		name     string
		taskText string
		expected []string
	}{
		{
			name:     "Go struct",
			taskText: `type UserProfile struct { Name string }`,
			expected: []string{"UserProfile"},
		},
		{
			name:     "Go interface",
			taskText: `type Repository interface { Save() error }`,
			expected: []string{"Repository"},
		},
		{
			name:     "Interface without type keyword",
			taskText: `interface Validator { Validate() bool }`,
			expected: []string{"Validator"},
		},
		{
			name:     "Enum",
			taskText: `enum Status { Active, Inactive }`,
			expected: []string{"Status"},
		},
		{
			name:     "Class",
			taskText: `class AuthManager { constructor() {} }`,
			expected: []string{"AuthManager"},
		},
		{
			name: "Multiple types",
			taskText: `
type Config struct {}
type Handler interface {}
enum Mode { Dev, Prod }
`,
			expected: []string{"Config", "Handler", "Mode"},
		},
		{
			name: "Duplicate type definitions (deduplication)",
			taskText: `
type User struct {}
type User struct {}
`,
			expected: []string{"User"},
		},
		{
			name:     "No types",
			taskText: `This is just text with no type definitions.`,
			expected: []string{},
		},
		{
			name: "Mixed with prose",
			taskText: `
We need to implement an authentication system.
type AuthToken struct {
    Token string
}
interface SessionManager {
    Create() AuthToken
}
`,
			// Structured definitions at start of line are matched.
			expected: []string{"AuthToken", "SessionManager"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTypeNames(tt.taskText)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d types, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Convert to map for order-independent comparison
			resultMap := make(map[string]bool)
			for _, r := range result {
				resultMap[r] = true
			}

			for _, exp := range tt.expected {
				if !resultMap[exp] {
					t.Errorf("expected type '%s' not found in result: %v", exp, result)
				}
			}
		})
	}
}

func TestDetectScaffoldsPostAgent_EmptyManifest(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts for empty manifest, got %d", len(result.Conflicts))
	}
}

func TestDetectScaffoldsPostAgent_MultipleWaves(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  `type Logger interface {}`,
						Files: []string{"pkg/log/logger.go"},
					},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{
						ID:    "B",
						Task:  `type Logger struct {}`,
						Files: []string{"pkg/app/logger.go"},
					},
				},
			},
		},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect conflict across waves
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict across waves, got %d", len(result.Conflicts))
	}

	conflict := result.Conflicts[0]
	if conflict.TypeName != "Logger" {
		t.Errorf("expected type name 'Logger', got '%s'", conflict.TypeName)
	}
}

func TestDetectScaffoldsPostAgent_DeterministicOrder(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: `type Zebra struct {}`, Files: []string{"pkg/z/z.go"}},
					{ID: "B", Task: `type Zebra struct {}`, Files: []string{"pkg/a/a.go"}},
					{ID: "C", Task: `type Apple struct {}`, Files: []string{"pkg/c/c.go"}},
					{ID: "D", Task: `type Apple struct {}`, Files: []string{"pkg/d/d.go"}},
				},
			},
		},
	}

	result1, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result2, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result1.Conflicts) != len(result2.Conflicts) {
		t.Fatalf("non-deterministic conflict count: %d vs %d",
			len(result1.Conflicts), len(result2.Conflicts))
	}

	for i := range result1.Conflicts {
		c1, c2 := result1.Conflicts[i], result2.Conflicts[i]
		if c1.TypeName != c2.TypeName {
			t.Errorf("non-deterministic TypeName at index %d: %q vs %q",
				i, c1.TypeName, c2.TypeName)
		}
	}

	// Verify alphabetical order: Apple before Zebra
	if len(result1.Conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(result1.Conflicts))
	}
	if result1.Conflicts[0].TypeName != "Apple" {
		t.Errorf("expected Apple first, got %q", result1.Conflicts[0].TypeName)
	}
	if result1.Conflicts[1].TypeName != "Zebra" {
		t.Errorf("expected Zebra second, got %q", result1.Conflicts[1].TypeName)
	}
}

func TestDetectScaffoldsPostAgent_AgentWithMultipleFiles(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "A",
						Task:  `type Request struct {}`,
						Files: []string{"pkg/api/handler.go", "pkg/api/middleware.go", "pkg/api/request.go"},
					},
					{
						ID:    "B",
						Task:  `type Request struct {}`,
						Files: []string{"pkg/client/request.go"},
					},
				},
			},
		},
	}

	result, err := DetectScaffoldsPostAgent(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}

	conflict := result.Conflicts[0]

	// Should include all files from both agents
	if len(conflict.Files) != 4 {
		t.Errorf("expected 4 files total, got %d: %v", len(conflict.Files), conflict.Files)
	}
}
