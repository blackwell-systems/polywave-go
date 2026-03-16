package scaffold

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestDetectScaffoldsPreAgent_SingleType(t *testing.T) {
	contracts := []protocol.InterfaceContract{
		{
			Name:        "MetricsAPI",
			Description: "Metrics collection interface",
			Definition: `type MetricSnapshot struct {
	Timestamp int64
	Value     float64
}`,
			Location: "pkg/metrics/collector.go",
		},
		{
			Name:        "MetricsStorage",
			Description: "Metrics persistence interface",
			Definition: `type MetricSnapshot struct {
	Timestamp int64
	Value     float64
}

func StoreMetric(m MetricSnapshot) error`,
			Location: "pkg/storage/metrics.go",
		},
	}

	result, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ScaffoldsNeeded) != 1 {
		t.Fatalf("expected 1 scaffold, got %d", len(result.ScaffoldsNeeded))
	}

	scaffold := result.ScaffoldsNeeded[0]
	if scaffold.TypeName != "MetricSnapshot" {
		t.Errorf("expected TypeName 'MetricSnapshot', got '%s'", scaffold.TypeName)
	}

	if len(scaffold.ReferencedBy) != 2 {
		t.Errorf("expected 2 references, got %d", len(scaffold.ReferencedBy))
	}

	if scaffold.SuggestedFile != "internal/types/metricsnapshot.go" {
		t.Errorf("expected suggested file 'internal/types/metricsnapshot.go', got '%s'", scaffold.SuggestedFile)
	}

	if scaffold.Definition == "" {
		t.Error("expected non-empty definition")
	}
}

func TestDetectScaffoldsPreAgent_NoSharedTypes(t *testing.T) {
	contracts := []protocol.InterfaceContract{
		{
			Name:        "UserAPI",
			Description: "User management",
			Definition: `type User struct {
	ID   string
	Name string
}`,
			Location: "pkg/user/api.go",
		},
		{
			Name:        "ProductAPI",
			Description: "Product catalog",
			Definition: `type Product struct {
	SKU   string
	Price float64
}`,
			Location: "pkg/product/api.go",
		},
	}

	result, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ScaffoldsNeeded) != 0 {
		t.Errorf("expected 0 scaffolds for non-shared types, got %d", len(result.ScaffoldsNeeded))
	}
}

func TestDetectScaffoldsPreAgent_MultipleTypes(t *testing.T) {
	contracts := []protocol.InterfaceContract{
		{
			Name: "AgentA",
			Definition: `type Config struct {
	Port int
}

type Logger interface {
	Log(msg string)
}

type DataPoint struct {
	Value float64
}`,
			Location: "pkg/agent/a.go",
		},
		{
			Name: "AgentB",
			Definition: `type Config struct {
	Port int
}

type Logger interface {
	Log(msg string)
}

type Result struct {
	Status string
}`,
			Location: "pkg/agent/b.go",
		},
		{
			Name: "AgentC",
			Definition: `type Logger interface {
	Log(msg string)
}

type Parser interface {
	Parse(data []byte)
}`,
			Location: "pkg/agent/c.go",
		},
	}

	result, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect Config (2 refs) and Logger (3 refs), but not DataPoint, Result, or Parser (1 ref each)
	if len(result.ScaffoldsNeeded) != 2 {
		t.Fatalf("expected 2 scaffolds, got %d", len(result.ScaffoldsNeeded))
	}

	// Check that we have Config and Logger
	typeNames := make(map[string]bool)
	for _, scaffold := range result.ScaffoldsNeeded {
		typeNames[scaffold.TypeName] = true
	}

	if !typeNames["Config"] {
		t.Error("expected Config to be detected as shared type")
	}

	if !typeNames["Logger"] {
		t.Error("expected Logger to be detected as shared type")
	}
}

func TestDetectScaffoldsPreAgent_TypeInDefinitionOnly(t *testing.T) {
	// Test case: a type is mentioned in documentation or as a parameter,
	// but not defined multiple times
	contracts := []protocol.InterfaceContract{
		{
			Name: "UserService",
			Definition: `type User struct {
	ID string
}

// CreateUser creates a new user
func CreateUser(name string) User`,
			Location: "pkg/user/service.go",
		},
		{
			Name: "UserRepo",
			Definition: `// StoreUser persists a User to the database
// It accepts a User struct from the UserService
func StoreUser(u interface{}) error`,
			Location: "pkg/user/repo.go",
		},
	}

	result, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User is only defined once (in UserService), just mentioned in UserRepo comment
	// Should not be detected as scaffold
	if len(result.ScaffoldsNeeded) != 0 {
		t.Errorf("expected 0 scaffolds, got %d (types mentioned in comments should not be detected)", len(result.ScaffoldsNeeded))
	}
}

func TestExtractTypeNames(t *testing.T) {
	testCases := []struct {
		name       string
		definition string
		wantTypes  []string
	}{
		{
			name: "Go struct",
			definition: `type User struct {
	ID string
}`,
			wantTypes: []string{"User"},
		},
		{
			name: "Go interface",
			definition: `type Logger interface {
	Log(msg string)
}`,
			wantTypes: []string{"Logger"},
		},
		{
			name: "Multiple Go types",
			definition: `type Config struct {
	Port int
}

type Logger interface {
	Log(msg string)
}`,
			wantTypes: []string{"Config", "Logger"},
		},
		{
			name:       "Rust struct",
			definition: `struct Metrics {`,
			wantTypes:  []string{"Metrics"},
		},
		{
			name:       "TypeScript interface",
			definition: `interface ApiResponse {`,
			wantTypes:  []string{"ApiResponse"},
		},
		{
			name:       "Python class",
			definition: `class DataProcessor {`,
			wantTypes:  []string{"DataProcessor"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contracts := []protocol.InterfaceContract{
				{
					Name:       "Test",
					Definition: tc.definition,
					Location:   "test.go",
				},
				{
					Name:       "Test2",
					Definition: tc.definition,
					Location:   "test2.go",
				},
			}

			result, err := DetectScaffoldsPreAgent(contracts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.ScaffoldsNeeded) != len(tc.wantTypes) {
				t.Fatalf("expected %d types, got %d", len(tc.wantTypes), len(result.ScaffoldsNeeded))
			}

			for i, typeName := range tc.wantTypes {
				if result.ScaffoldsNeeded[i].TypeName != typeName {
					t.Errorf("expected type name '%s', got '%s'", typeName, result.ScaffoldsNeeded[i].TypeName)
				}
			}
		})
	}
}

func TestDetectScaffoldsPreAgent_EmptyContracts(t *testing.T) {
	result, err := DetectScaffoldsPreAgent([]protocol.InterfaceContract{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ScaffoldsNeeded) != 0 {
		t.Errorf("expected empty result for empty contracts, got %d scaffolds", len(result.ScaffoldsNeeded))
	}
}

func TestDetectScaffoldsPreAgent_DuplicateReferences(t *testing.T) {
	// Test that if the same agent references a type in multiple contracts,
	// it's counted as one reference (based on location)
	contracts := []protocol.InterfaceContract{
		{
			Name: "ContractA",
			Definition: `type SharedType struct {
	Field string
}`,
			Location: "pkg/module/file.go",
		},
		{
			Name: "ContractB",
			Definition: `type SharedType struct {
	Field string
}`,
			Location: "pkg/module/file.go", // Same location = same agent
		},
		{
			Name: "ContractC",
			Definition: `type SharedType struct {
	Field string
}`,
			Location: "pkg/other/file.go", // Different location = different agent
		},
	}

	result, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ScaffoldsNeeded) != 1 {
		t.Fatalf("expected 1 scaffold, got %d", len(result.ScaffoldsNeeded))
	}

	// Should have exactly 2 unique locations
	if len(result.ScaffoldsNeeded[0].ReferencedBy) != 2 {
		t.Errorf("expected 2 unique references (deduplicated by location), got %d", len(result.ScaffoldsNeeded[0].ReferencedBy))
	}
}
