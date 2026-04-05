package collision

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	igit "github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a minimal git repository with a main branch and an initial commit.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	_, err := igit.Run(dir, "init")
	require.NoError(t, err, "git init")
	_, err = igit.Run(dir, "config", "user.email", "test@example.com")
	require.NoError(t, err, "git config user.email")
	_, err = igit.Run(dir, "config", "user.name", "Test User")
	require.NoError(t, err, "git config user.name")

	// Create initial commit on main
	initialFile := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(initialFile, []byte("# Test Repo\n"), 0644))
	_, err = igit.Run(dir, "add", ".")
	require.NoError(t, err, "git add")
	_, err = igit.Run(dir, "commit", "-m", "Initial commit")
	require.NoError(t, err, "git commit")
	_, err = igit.Run(dir, "branch", "-M", "main")
	require.NoError(t, err, "rename to main")
}

func TestExtractTypesFromFiles(t *testing.T) {
	t.Run("multiple valid files extracts types", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		branchName := "test-branch"
		_, err := igit.Run(repoDir, "checkout", "-b", branchName)
		require.NoError(t, err)

		// Create two .go files with type declarations
		file1 := filepath.Join(repoDir, "pkg", "models", "user.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(file1), 0755))
		require.NoError(t, os.WriteFile(file1, []byte(`package models

type User struct {
	ID string
}
`), 0644))

		file2 := filepath.Join(repoDir, "pkg", "service", "logger.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(file2), 0755))
		require.NoError(t, os.WriteFile(file2, []byte(`package service

type Logger interface {
	Log(msg string)
}
`), 0644))

		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add types")
		require.NoError(t, err)

		res := extractTypesFromFiles(context.Background(), repoDir, branchName, []string{
			"pkg/models/user.go",
			"pkg/service/logger.go",
		})
		require.True(t, res.IsSuccess(), "extractTypesFromFiles should succeed: %v", res.Errors)

		types := res.GetData()
		assert.Len(t, types, 2)
		assert.Contains(t, types, TypeDeclaration{Name: "User", Package: "pkg/models", Kind: "struct"})
		assert.Contains(t, types, TypeDeclaration{Name: "Logger", Package: "pkg/service", Kind: "interface"})
	})

	t.Run("git show failure on one file returns error", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		branchName := "test-branch"
		_, err := igit.Run(repoDir, "checkout", "-b", branchName)
		require.NoError(t, err)

		// Commit a file
		file1 := filepath.Join(repoDir, "test.go")
		require.NoError(t, os.WriteFile(file1, []byte("package main\n\ntype Test struct{}\n"), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add test")
		require.NoError(t, err)

		// Try to extract from a file that doesn't exist in the branch
		res := extractTypesFromFiles(context.Background(), repoDir, branchName, []string{
			"nonexistent.go",
		})
		assert.True(t, res.IsFatal(), "should fail when git show fails")
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionGitShowFailed, res.Errors[0].Code)
	})

	t.Run("parse failure on one file returns error", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		branchName := "test-branch"
		_, err := igit.Run(repoDir, "checkout", "-b", branchName)
		require.NoError(t, err)

		// Create a .go file with syntax error
		badFile := filepath.Join(repoDir, "bad.go")
		require.NoError(t, os.WriteFile(badFile, []byte("package bad\n\ntype Foo struct {\n"), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add bad file")
		require.NoError(t, err)

		res := extractTypesFromFiles(context.Background(), repoDir, branchName, []string{"bad.go"})
		assert.True(t, res.IsFatal())
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionParseFailed, res.Errors[0].Code)
	})

	t.Run("context cancellation", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		branchName := "test-branch"
		_, err := igit.Run(repoDir, "checkout", "-b", branchName)
		require.NoError(t, err)

		file1 := filepath.Join(repoDir, "test.go")
		require.NoError(t, os.WriteFile(file1, []byte("package main\n"), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "test")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		res := extractTypesFromFiles(ctx, repoDir, branchName, []string{"test.go"})
		assert.True(t, res.IsFatal())
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionContextCancelled, res.Errors[0].Code)
	})
}

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
			name:     "root package file - filepath.Dir returns dot",
			filePath: "main.go",
			content: `package main

type AppConfig struct {
    Port int
}
`,
			want: []TypeDeclaration{
				{Name: "AppConfig", Package: "", Kind: "struct"},
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
			res := extractTypeDecls(tt.filePath, tt.content)
			if tt.wantErr {
				if !res.IsFatal() {
					t.Errorf("extractTypeDecls() expected failure, got success")
				}
				return
			}
			if !res.IsSuccess() {
				t.Errorf("extractTypeDecls() failed: %v", res.Errors)
				return
			}
			got := res.GetData()
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
	t.Run("valid branch with changed go files", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		// Create a branch with a modified .go file
		branchName := "saw/test-slug/wave1-agent-A"
		_, err := igit.Run(repoDir, "checkout", "-b", branchName)
		require.NoError(t, err, "create branch")

		// Add a new .go file
		newFile := filepath.Join(repoDir, "pkg", "service", "handler.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(newFile), 0755))
		require.NoError(t, os.WriteFile(newFile, []byte("package service\n\ntype Handler struct{}\n"), 0644))

		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err, "git add")
		_, err = igit.Run(repoDir, "commit", "-m", "Add handler")
		require.NoError(t, err, "git commit")

		// Switch back to main so we can diff
		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err, "checkout main")

		// Test getChangedGoFiles
		res := getChangedGoFiles(context.Background(), repoDir, branchName)
		require.True(t, res.IsSuccess(), "getChangedGoFiles should succeed: %v", res.Errors)

		files := res.GetData()
		assert.Contains(t, files, "pkg/service/handler.go")
	})

	t.Run("branch with no go files", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		// Create a branch with non-.go changes
		branchName := "test-branch-no-go"
		_, err := igit.Run(repoDir, "checkout", "-b", branchName)
		require.NoError(t, err)

		readmePath := filepath.Join(repoDir, "README.txt")
		require.NoError(t, os.WriteFile(readmePath, []byte("test"), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add README")
		require.NoError(t, err)

		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)

		res := getChangedGoFiles(context.Background(), repoDir, branchName)
		require.True(t, res.IsSuccess())
		files := res.GetData()
		assert.Empty(t, files, "should return empty slice for no .go files")
	})

	t.Run("invalid branch returns error", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		res := getChangedGoFiles(context.Background(), repoDir, "nonexistent-branch")
		assert.True(t, res.IsFatal(), "should fail for nonexistent branch")
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionGitDiffFailed, res.Errors[0].Code)
	})

	t.Run("context cancellation", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		res := getChangedGoFiles(ctx, repoDir, "main")
		assert.True(t, res.IsFatal())
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionContextCancelled, res.Errors[0].Code)
	})
}

func TestDetectCollisions(t *testing.T) {
	t.Run("manifest load failure", func(t *testing.T) {
		res := DetectCollisions(context.Background(), "/nonexistent/manifest.yaml", 1, "/tmp")
		assert.True(t, res.IsFatal())
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionLoadManifestFailed, res.Errors[0].Code)
	})

	t.Run("invalid wave number", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		// Create a minimal IMPL manifest
		manifestPath := filepath.Join(repoDir, "IMPL.yaml")
		manifestContent := `title: Test
feature_slug: test-slug
verdict: SUITABLE
waves:
  - number: 1
    agents:
      - id: A
        task: test
        files: []
`
		require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

		// Request wave 5 when only wave 1 exists
		res := DetectCollisions(context.Background(), manifestPath, 5, repoDir)
		assert.True(t, res.IsFatal())
		require.NotEmpty(t, res.Errors)
		assert.Equal(t, result.CodeCollisionInvalidWave, res.Errors[0].Code)
	})

	t.Run("no collisions integration", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		const slug = "test-feature"
		const waveNum = 1

		// Create agent A branch with a unique type.
		branchA := "saw/" + slug + "/wave1-agent-A"
		_, err := igit.Run(repoDir, "checkout", "-b", branchA)
		require.NoError(t, err)
		fileA := filepath.Join(repoDir, "pkg", "svc", "handler.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(fileA), 0755))
		require.NoError(t, os.WriteFile(fileA, []byte("package svc\n\ntype Handler struct{}\n"), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add Handler")
		require.NoError(t, err)

		// Create agent B branch with a distinct type.
		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)
		branchB := "saw/" + slug + "/wave1-agent-B"
		_, err = igit.Run(repoDir, "checkout", "-b", branchB)
		require.NoError(t, err)
		fileB := filepath.Join(repoDir, "pkg", "svc", "logger.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(fileB), 0755))
		require.NoError(t, os.WriteFile(fileB, []byte("package svc\n\ntype Logger interface{ Log(string) }\n"), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add Logger")
		require.NoError(t, err)

		// Return to main before running DetectCollisions.
		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)

		// Write a minimal IMPL manifest referencing agents A and B.
		manifestPath := filepath.Join(repoDir, "IMPL.yaml")
		manifestContent := "title: Test\nfeature_slug: " + slug + "\nverdict: SUITABLE\nwaves:\n  - number: 1\n    agents:\n      - id: A\n        task: test\n        files: []\n      - id: B\n        task: test\n        files: []\n"
		require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

		res := DetectCollisions(context.Background(), manifestPath, waveNum, repoDir)
		require.True(t, res.IsSuccess(), "DetectCollisions should succeed: %v", res.Errors)
		report := res.GetData()
		assert.True(t, report.Valid, "no type collisions expected")
		assert.Empty(t, report.Collisions)
	})

	t.Run("successful collision detection integration", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		const slug = "collision-feature"
		const waveNum = 1

		// Both agents define the same type in the same package — a collision.
		const sharedType = "package svc\n\ntype SharedEntry struct{ ID string }\n"

		branchA := "saw/" + slug + "/wave1-agent-A"
		_, err := igit.Run(repoDir, "checkout", "-b", branchA)
		require.NoError(t, err)
		fileA := filepath.Join(repoDir, "pkg", "svc", "entry.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(fileA), 0755))
		require.NoError(t, os.WriteFile(fileA, []byte(sharedType), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add SharedEntry (agent A)")
		require.NoError(t, err)

		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)
		branchB := "saw/" + slug + "/wave1-agent-B"
		_, err = igit.Run(repoDir, "checkout", "-b", branchB)
		require.NoError(t, err)
		fileB := filepath.Join(repoDir, "pkg", "svc", "entry.go")
		require.NoError(t, os.MkdirAll(filepath.Dir(fileB), 0755))
		require.NoError(t, os.WriteFile(fileB, []byte(sharedType), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add SharedEntry (agent B)")
		require.NoError(t, err)

		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)

		manifestPath := filepath.Join(repoDir, "IMPL.yaml")
		manifestContent := "title: Test\nfeature_slug: " + slug + "\nverdict: SUITABLE\nwaves:\n  - number: 1\n    agents:\n      - id: A\n        task: test\n        files: []\n      - id: B\n        task: test\n        files: []\n"
		require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

		res := DetectCollisions(context.Background(), manifestPath, waveNum, repoDir)
		require.True(t, res.IsSuccess(), "DetectCollisions should succeed: %v", res.Errors)
		report := res.GetData()
		assert.False(t, report.Valid, "collision expected between agents A and B")
		require.Len(t, report.Collisions, 1)
		assert.Equal(t, "SharedEntry", report.Collisions[0].TypeName)
		assert.Equal(t, "pkg/svc", report.Collisions[0].Package)
		assert.Equal(t, []string{"A", "B"}, report.Collisions[0].Agents)
	})
}

func TestDetectCollisionsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	res := DetectCollisions(ctx, "nonexistent.yaml", 1, "/tmp")
	if !res.IsFatal() {
		t.Error("DetectCollisions() expected fatal error on cancelled context")
	}
	// Verify it's specifically a context cancellation error
	require.NotEmpty(t, res.Errors)
	assert.Equal(t, result.CodeCollisionContextCancelled, res.Errors[0].Code)
}

// TestDetectCollisionsKeyParsing tests the fix for finding #20: fragile key parsing.
// The bug was in detectCollisionsInTypes at line 243-245, where lastSlash could be -1
// for types with no package (root package types like main.go).
func TestDetectCollisionsKeyParsing(t *testing.T) {
	t.Run("root package collision handling", func(t *testing.T) {
		// Simulate the internal state that would trigger the bug:
		// Two agents both define a type in the root package (Package = "")
		agentTypes := map[string][]TypeDeclaration{
			"A": {
				{Name: "AppConfig", Package: "", Kind: "struct"},
			},
			"B": {
				{Name: "AppConfig", Package: "", Kind: "struct"},
			},
		}

		collisions := detectCollisionsInTypes(agentTypes)

		// Before the fix: would panic or produce malformed collision with Package=""
		// After the fix: should detect collision correctly
		require.Len(t, collisions, 1, "should detect collision for root package types")
		assert.Equal(t, "AppConfig", collisions[0].TypeName)
		assert.Equal(t, "", collisions[0].Package, "package should be empty string for root package")
		assert.Equal(t, []string{"A", "B"}, collisions[0].Agents)
	})

	t.Run("mixed root and pkg collisions", func(t *testing.T) {
		agentTypes := map[string][]TypeDeclaration{
			"A": {
				{Name: "Config", Package: "", Kind: "struct"},
				{Name: "Handler", Package: "pkg/service", Kind: "struct"},
			},
			"B": {
				{Name: "Config", Package: "", Kind: "struct"},
				{Name: "Logger", Package: "pkg/service", Kind: "interface"},
			},
		}

		collisions := detectCollisionsInTypes(agentTypes)

		// Should detect the root package collision only
		require.Len(t, collisions, 1)
		assert.Equal(t, "Config", collisions[0].TypeName)
		assert.Equal(t, "", collisions[0].Package)
	})
}

// TestRootPackageCollision tests finding #19: root package types (Package="")
// should be treated as a distinct namespace, not ignored or conflated with other packages.
func TestRootPackageCollision(t *testing.T) {
	t.Run("root package types across multiple agents", func(t *testing.T) {
		// Three agents all define types in root package
		agentTypes := map[string][]TypeDeclaration{
			"A": {
				{Name: "Main", Package: "", Kind: "struct"},
			},
			"B": {
				{Name: "Main", Package: "", Kind: "struct"},
			},
			"C": {
				{Name: "Main", Package: "", Kind: "struct"},
			},
		}

		collisions := detectCollisionsInTypes(agentTypes)

		require.Len(t, collisions, 1)
		assert.Equal(t, "Main", collisions[0].TypeName)
		assert.Equal(t, "", collisions[0].Package)
		assert.Equal(t, []string{"A", "B", "C"}, collisions[0].Agents)
		assert.Equal(t, "Keep A, remove from B and C", collisions[0].Resolution)
	})

	t.Run("same type name in root vs pkg is not a collision", func(t *testing.T) {
		agentTypes := map[string][]TypeDeclaration{
			"A": {
				{Name: "Config", Package: "", Kind: "struct"}, // root package
			},
			"B": {
				{Name: "Config", Package: "pkg/config", Kind: "struct"}, // pkg/config
			},
		}

		collisions := detectCollisionsInTypes(agentTypes)

		// These are in different namespaces, so no collision
		assert.Empty(t, collisions, "same type name in different packages should not collide")
	})

	t.Run("integration test with real repo and root package types", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestRepo(t, repoDir)

		const slug = "rootpkg-feature"
		const waveNum = 1

		// Both agents define AppConfig in the root package (no subdirectory).
		const rootType = "package main\n\ntype AppConfig struct{ Port int }\n"

		branchA := "saw/" + slug + "/wave1-agent-A"
		_, err := igit.Run(repoDir, "checkout", "-b", branchA)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "config.go"), []byte(rootType), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add AppConfig (agent A)")
		require.NoError(t, err)

		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)
		branchB := "saw/" + slug + "/wave1-agent-B"
		_, err = igit.Run(repoDir, "checkout", "-b", branchB)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "config.go"), []byte(rootType), 0644))
		_, err = igit.Run(repoDir, "add", ".")
		require.NoError(t, err)
		_, err = igit.Run(repoDir, "commit", "-m", "Add AppConfig (agent B)")
		require.NoError(t, err)

		_, err = igit.Run(repoDir, "checkout", "main")
		require.NoError(t, err)

		manifestPath := filepath.Join(repoDir, "IMPL.yaml")
		manifestContent := "title: Test\nfeature_slug: " + slug + "\nverdict: SUITABLE\nwaves:\n  - number: 1\n    agents:\n      - id: A\n        task: test\n        files: []\n      - id: B\n        task: test\n        files: []\n"
		require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

		res := DetectCollisions(context.Background(), manifestPath, waveNum, repoDir)
		require.True(t, res.IsSuccess(), "DetectCollisions should succeed: %v", res.Errors)
		report := res.GetData()
		assert.False(t, report.Valid, "collision expected for root package types")
		require.Len(t, report.Collisions, 1)
		assert.Equal(t, "AppConfig", report.Collisions[0].TypeName)
		assert.Equal(t, "", report.Collisions[0].Package, "root package should have empty package path")
	})
}
