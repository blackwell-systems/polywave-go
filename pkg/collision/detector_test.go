package collision

import (
	"sort"
	"testing"
)

func TestExtractTypeDecls(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		content  string
		want     []TypeDeclaration
		wantErr  bool
	}{
		{
			name:     "struct type",
			filePath: "pkg/service/handler.go",
			content: `package service

type Handler struct {
	Name string
}
`,
			want: []TypeDeclaration{
				{Name: "Handler", Package: "pkg/service", Kind: "struct"},
			},
			wantErr: false,
		},
		{
			name:     "interface type",
			filePath: "pkg/service/logger.go",
			content: `package service

type Logger interface {
	Log(msg string)
}
`,
			want: []TypeDeclaration{
				{Name: "Logger", Package: "pkg/service", Kind: "interface"},
			},
			wantErr: false,
		},
		{
			name:     "type alias",
			filePath: "pkg/types/alias.go",
			content: `package types

type UserID = string
`,
			want: []TypeDeclaration{
				{Name: "UserID", Package: "pkg/types", Kind: "alias"},
			},
			wantErr: false,
		},
		{
			name:     "multiple types",
			filePath: "pkg/models/types.go",
			content: `package models

type User struct {
	ID string
}

type UserRepo interface {
	Get(id string) User
}

type Status = int
`,
			want: []TypeDeclaration{
				{Name: "User", Package: "pkg/models", Kind: "struct"},
				{Name: "UserRepo", Package: "pkg/models", Kind: "interface"},
				{Name: "Status", Package: "pkg/models", Kind: "alias"},
			},
			wantErr: false,
		},
		{
			name:     "no types",
			filePath: "pkg/util/helper.go",
			content: `package util

func Helper() string {
	return "help"
}
`,
			want:    []TypeDeclaration{},
			wantErr: false,
		},
		{
			name:     "syntax error",
			filePath: "pkg/bad/bad.go",
			content: `package bad

type Foo struct {
	Name string
`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractTypeDecls(tt.filePath, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractTypeDecls() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("extractTypeDecls() got %d types, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Name != tt.want[i].Name || got[i].Package != tt.want[i].Package || got[i].Kind != tt.want[i].Kind {
					t.Errorf("extractTypeDecls()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectCollisionsInTypes(t *testing.T) {
	tests := []struct {
		name       string
		agentTypes map[string][]TypeDeclaration
		want       []TypeCollision
	}{
		{
			name: "no collisions - different types",
			agentTypes: map[string][]TypeDeclaration{
				"A": {
					{Name: "Handler", Package: "pkg/service", Kind: "struct"},
				},
				"B": {
					{Name: "Logger", Package: "pkg/service", Kind: "interface"},
				},
				"C": {
					{Name: "Config", Package: "pkg/config", Kind: "struct"},
				},
			},
			want: []TypeCollision{},
		},
		{
			name: "no collisions - same type different packages",
			agentTypes: map[string][]TypeDeclaration{
				"A": {
					{Name: "Foo", Package: "pkg/a", Kind: "struct"},
				},
				"B": {
					{Name: "Foo", Package: "pkg/b", Kind: "struct"},
				},
			},
			want: []TypeCollision{},
		},
		{
			name: "single collision - 2 agents",
			agentTypes: map[string][]TypeDeclaration{
				"A": {
					{Name: "RepoEntry", Package: "pkg/service", Kind: "struct"},
				},
				"B": {
					{Name: "RepoEntry", Package: "pkg/service", Kind: "struct"},
				},
			},
			want: []TypeCollision{
				{
					TypeName:   "RepoEntry",
					Package:    "pkg/service",
					Agents:     []string{"A", "B"},
					Resolution: "Keep A, remove from B",
				},
			},
		},
		{
			name: "multi-collision - 3 agents",
			agentTypes: map[string][]TypeDeclaration{
				"A": {
					{Name: "SAWConfig", Package: "pkg/config", Kind: "struct"},
				},
				"B": {
					{Name: "SAWConfig", Package: "pkg/config", Kind: "struct"},
				},
				"C": {
					{Name: "SAWConfig", Package: "pkg/config", Kind: "struct"},
				},
			},
			want: []TypeCollision{
				{
					TypeName:   "SAWConfig",
					Package:    "pkg/config",
					Agents:     []string{"A", "B", "C"},
					Resolution: "Keep A, remove from B and C",
				},
			},
		},
		{
			name: "mixed kinds collision",
			agentTypes: map[string][]TypeDeclaration{
				"A": {
					{Name: "Logger", Package: "pkg/log", Kind: "interface"},
				},
				"B": {
					{Name: "Logger", Package: "pkg/log", Kind: "struct"},
				},
			},
			want: []TypeCollision{
				{
					TypeName:   "Logger",
					Package:    "pkg/log",
					Agents:     []string{"A", "B"},
					Resolution: "Keep A, remove from B",
				},
			},
		},
		{
			name: "multiple collisions in different packages",
			agentTypes: map[string][]TypeDeclaration{
				"A": {
					{Name: "Handler", Package: "pkg/api", Kind: "struct"},
					{Name: "Config", Package: "pkg/config", Kind: "struct"},
				},
				"B": {
					{Name: "Handler", Package: "pkg/api", Kind: "struct"},
					{Name: "Config", Package: "pkg/config", Kind: "struct"},
				},
			},
			want: []TypeCollision{
				{
					TypeName:   "Handler",
					Package:    "pkg/api",
					Agents:     []string{"A", "B"},
					Resolution: "Keep A, remove from B",
				},
				{
					TypeName:   "Config",
					Package:    "pkg/config",
					Agents:     []string{"A", "B"},
					Resolution: "Keep A, remove from B",
				},
			},
		},
		{
			name: "agent order matters - B before A alphabetically",
			agentTypes: map[string][]TypeDeclaration{
				"B": {
					{Name: "Entry", Package: "pkg/db", Kind: "struct"},
				},
				"A": {
					{Name: "Entry", Package: "pkg/db", Kind: "struct"},
				},
			},
			want: []TypeCollision{
				{
					TypeName:   "Entry",
					Package:    "pkg/db",
					Agents:     []string{"A", "B"}, // Alphabetically sorted
					Resolution: "Keep A, remove from B",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectCollisionsInTypes(tt.agentTypes)
			if len(got) != len(tt.want) {
				t.Errorf("detectCollisionsInTypes() got %d collisions, want %d", len(got), len(tt.want))
				return
			}
			// Sort both slices by TypeName+Package for deterministic comparison
			sort.Slice(got, func(i, j int) bool {
				if got[i].Package != got[j].Package {
					return got[i].Package < got[j].Package
				}
				return got[i].TypeName < got[j].TypeName
			})
			sort.Slice(tt.want, func(i, j int) bool {
				if tt.want[i].Package != tt.want[j].Package {
					return tt.want[i].Package < tt.want[j].Package
				}
				return tt.want[i].TypeName < tt.want[j].TypeName
			})
			for i := range got {
				if got[i].TypeName != tt.want[i].TypeName {
					t.Errorf("collision[%d].TypeName = %v, want %v", i, got[i].TypeName, tt.want[i].TypeName)
				}
				if got[i].Package != tt.want[i].Package {
					t.Errorf("collision[%d].Package = %v, want %v", i, got[i].Package, tt.want[i].Package)
				}
				if len(got[i].Agents) != len(tt.want[i].Agents) {
					t.Errorf("collision[%d].Agents length = %v, want %v", i, len(got[i].Agents), len(tt.want[i].Agents))
				}
				for j := range got[i].Agents {
					if got[i].Agents[j] != tt.want[i].Agents[j] {
						t.Errorf("collision[%d].Agents[%d] = %v, want %v", i, j, got[i].Agents[j], tt.want[i].Agents[j])
					}
				}
				if got[i].Resolution != tt.want[i].Resolution {
					t.Errorf("collision[%d].Resolution = %v, want %v", i, got[i].Resolution, tt.want[i].Resolution)
				}
			}
		})
	}
}

func TestBuildBranchName(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		waveNum int
		agentID string
		want    string
	}{
		{
			name:    "slug-scoped format",
			slug:    "my-feature",
			waveNum: 1,
			agentID: "A",
			want:    "saw/my-feature/wave1-agent-A",
		},
		{
			name:    "legacy format",
			slug:    "",
			waveNum: 2,
			agentID: "B",
			want:    "wave2-agent-B",
		},
		{
			name:    "slug with hyphens",
			slug:    "type-collision-detection",
			waveNum: 1,
			agentID: "A",
			want:    "saw/type-collision-detection/wave1-agent-A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBranchName(tt.slug, tt.waveNum, tt.agentID)
			if got != tt.want {
				t.Errorf("buildBranchName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetChangedGoFiles(t *testing.T) {
	// This test requires a real git repo with branches, so we'll skip it
	// in unit tests. Integration tests would cover this.
	t.Skip("Requires real git repository")
}

func TestDetectCollisions(t *testing.T) {
	// This test requires a real IMPL manifest and git repo with branches,
	// so we'll skip it in unit tests. Integration tests would cover this.
	t.Skip("Requires real IMPL manifest and git repository")
}
