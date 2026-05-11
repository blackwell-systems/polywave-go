package scaffold

import (
	"sort"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
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

	if len(scaffold.Locations) != 2 {
		t.Errorf("expected 2 references, got %d", len(scaffold.Locations))
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

			gotTypes := make([]string, len(result.ScaffoldsNeeded))
			for i, s := range result.ScaffoldsNeeded {
				gotTypes[i] = s.TypeName
			}
			sort.Strings(gotTypes)
			wantSorted := make([]string, len(tc.wantTypes))
			copy(wantSorted, tc.wantTypes)
			sort.Strings(wantSorted)
			for i := range wantSorted {
				if gotTypes[i] != wantSorted[i] {
					t.Errorf("expected type name '%s', got '%s'", wantSorted[i], gotTypes[i])
				}
			}
		})
	}
}

func TestDetectScaffoldsPreAgent_DeterministicOrder(t *testing.T) {
	contracts := []protocol.InterfaceContract{
		{Name: "C1", Definition: `type Zebra struct { X int }`, Location: "pkg/z/z.go"},
		{Name: "C2", Definition: `type Zebra struct { X int }`, Location: "pkg/a/a.go"},
		{Name: "C3", Definition: `type Apple struct { Y string }`, Location: "pkg/z/z.go"},
		{Name: "C4", Definition: `type Apple struct { Y string }`, Location: "pkg/b/b.go"},
	}

	result1, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result2, err := DetectScaffoldsPreAgent(contracts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result1.ScaffoldsNeeded) != len(result2.ScaffoldsNeeded) {
		t.Fatalf("non-deterministic length: %d vs %d", len(result1.ScaffoldsNeeded), len(result2.ScaffoldsNeeded))
	}

	for i := range result1.ScaffoldsNeeded {
		s1, s2 := result1.ScaffoldsNeeded[i], result2.ScaffoldsNeeded[i]
		if s1.TypeName != s2.TypeName {
			t.Errorf("non-deterministic TypeName at index %d: %q vs %q", i, s1.TypeName, s2.TypeName)
		}
		for j := range s1.Locations {
			if s1.Locations[j] != s2.Locations[j] {
				t.Errorf("non-deterministic Locations[%d] for type %q: %q vs %q",
					j, s1.TypeName, s1.Locations[j], s2.Locations[j])
			}
		}
	}

	// Verify alphabetical order: Apple before Zebra
	if result1.ScaffoldsNeeded[0].TypeName != "Apple" {
		t.Errorf("expected Apple first (alphabetical), got %q", result1.ScaffoldsNeeded[0].TypeName)
	}
	if result1.ScaffoldsNeeded[1].TypeName != "Zebra" {
		t.Errorf("expected Zebra second (alphabetical), got %q", result1.ScaffoldsNeeded[1].TypeName)
	}
}

func TestExtractTypeNames_AllPatterns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "Go struct keyword",
			input: "type Config struct {",
			want:  []string{"Config"},
		},
		{
			name:  "Go interface keyword",
			input: "type Logger interface {",
			want:  []string{"Logger"},
		},
		{
			name:  "Bare struct (Rust)",
			input: "struct Metrics {",
			want:  []string{"Metrics"},
		},
		{
			name:  "Bare interface (TypeScript)",
			input: "interface ApiResponse {",
			want:  []string{"ApiResponse"},
		},
		{
			name:  "Python class",
			input: "class DataProcessor {",
			want:  []string{"DataProcessor"},
		},
		{
			name:  "Enum",
			input: "enum Status {",
			want:  []string{"Status"},
		},
		{
			name:  "Multiple types",
			input: "type A struct {\ntype B interface {",
			want:  []string{"A", "B"},
		},
		{
			name:  "Deduplication",
			input: "type A struct {\ntype A struct {",
			want:  []string{"A"},
		},
		{
			name:  "Empty string",
			input: "",
			want:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTypeNames(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("extractTypeNames(%q) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("extractTypeNames[%d] = %q, want %q", i, got[i], tc.want[i])
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

func TestExtractTypeDefinition_NestedBraceTruncation(t *testing.T) {
	// Documents known limitation: [^}]* regex truncates on nested braces.
	// See pre_agent.go:87-90 comment.
	definition := `type Outer struct {
	Field1 string
	Inner  struct {
		Nested string
	}
	Field2 int
}`
	got := extractTypeDefinition(definition, "Outer")
	// The regex matches up to the first closing brace, truncating nested content.
	// This is the documented limitation — the test ensures we don't silently change behavior.
	if got == definition {
		t.Error("expected truncated extraction due to nested brace limitation, but got full definition")
	}
	// Should at least contain the type name and struct keyword
	if !strings.Contains(got, "Outer") || !strings.Contains(got, "struct") {
		t.Errorf("expected partial extraction containing type name, got: %s", got)
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
	if len(result.ScaffoldsNeeded[0].Locations) != 2 {
		t.Errorf("expected 2 unique references (deduplicated by location), got %d", len(result.ScaffoldsNeeded[0].Locations))
	}
}
