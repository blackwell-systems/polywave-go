package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// findGroup finds a ChecklistGroup by title within a slice. Returns nil if not found.
func findGroup(groups []ChecklistGroup, title string) *ChecklistGroup {
	for i := range groups {
		if groups[i].Title == title {
			return &groups[i]
		}
	}
	return nil
}

// newChecklistTestManifest returns a minimal IMPLManifest with the given file_ownership.
func newChecklistTestManifest(ownership []FileOwnership) *IMPLManifest {
	return &IMPLManifest{
		Title:         "Test Feature",
		FeatureSlug:   "test-feature",
		Verdict:       "SUITABLE",
		TestCommand:   "go test ./...",
		LintCommand:   "go vet ./...",
		FileOwnership: ownership,
	}
}

// createHandlerFile creates a Go handler file with handler functions in the given repo.
func createHandlerFile(t *testing.T, repoRoot, relPath string) {
	t.Helper()
	fullPath := filepath.Join(repoRoot, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	content := `package api

import "net/http"

type Server struct{}

func (s *Server) handleGet` + "Request" + `(w http.ResponseWriter, r *http.Request) {}
`
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
}

// createReactComponentFile creates a TSX component file with a default export.
func createReactComponentFile(t *testing.T, repoRoot, relPath, componentName string) {
	t.Helper()
	fullPath := filepath.Join(repoRoot, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	content := `import React from 'react';

export default function ` + componentName + `() {
  return <div>` + componentName + `</div>;
}
`
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
}

// createCLICmdFile creates a Go CLI command file with a cobra.Command.
func createCLICmdFile(t *testing.T, repoRoot, relPath, cmdName, funcName string) {
	t.Helper()
	fullPath := filepath.Join(repoRoot, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	content := `package main

import "github.com/spf13/cobra"

func ` + funcName + `() *cobra.Command {
	return &cobra.Command{
		Use:   "` + cmdName + `",
		Short: "Description of ` + cmdName + `",
	}
}
`
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
}

// ---------------------------------------------------------------------------
// TestDetectAPIHandlers
// ---------------------------------------------------------------------------

func TestDetectAPIHandlers(t *testing.T) {
	t.Run("new api handler generates route registration item", func(t *testing.T) {
		// Given: file_ownership with pkg/api/pipeline_handler.go (action:new)
		repoRoot := t.TempDir()
		createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")

		ownership := []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new"},
		}
		// When: detectAPIHandlers runs
		items := detectAPIHandlers(ownership, repoRoot)
		// Then: returns checklist item for route registration
		require.Len(t, items, 1)
		assert.Contains(t, items[0].Description, "Register routes")
		assert.Contains(t, items[0].Command, "grep")
	})

	t.Run("multiple new api handlers generate multiple items", func(t *testing.T) {
		repoRoot := t.TempDir()
		createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")
		createHandlerFile(t, repoRoot, "pkg/api/queue_handler.go")
		createHandlerFile(t, repoRoot, "pkg/api/resume_handler.go")

		ownership := []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/api/queue_handler.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/api/resume_handler.go", Agent: "B", Wave: 1, Action: "new"},
		}
		items := detectAPIHandlers(ownership, repoRoot)
		require.Len(t, items, 3)
	})

	t.Run("modified api handler does not generate item", func(t *testing.T) {
		ownership := []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "modify"},
		}
		items := detectAPIHandlers(ownership, "/fake/repo")
		assert.Len(t, items, 0)
	})

	t.Run("non-handler go file does not generate item", func(t *testing.T) {
		ownership := []FileOwnership{
			{File: "pkg/api/pipeline_service.go", Agent: "A", Wave: 1, Action: "new"},
		}
		items := detectAPIHandlers(ownership, "/fake/repo")
		assert.Len(t, items, 0)
	})

	t.Run("file outside pkg/api does not generate item", func(t *testing.T) {
		ownership := []FileOwnership{
			{File: "internal/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new"},
		}
		items := detectAPIHandlers(ownership, "/fake/repo")
		assert.Len(t, items, 0)
	})

	t.Run("checklist item description mentions server.go", func(t *testing.T) {
		repoRoot := t.TempDir()
		createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")

		ownership := []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new"},
		}
		items := detectAPIHandlers(ownership, repoRoot)
		require.Len(t, items, 1)
		assert.Contains(t, items[0].Description, "server.go")
	})
}

// TestDetectAPIHandlers_WithRealFile tests parsing of an actual handler file.
func TestDetectAPIHandlers_WithRealFile(t *testing.T) {
	// Create a temp directory acting as a fake repo
	repoRoot := t.TempDir()
	apiDir := filepath.Join(repoRoot, "pkg", "api")
	require.NoError(t, os.MkdirAll(apiDir, 0755))

	// Write a minimal handler file with handler functions
	handlerContent := `package api

import "net/http"

type Server struct{}

func (s *Server) handleGetPipeline(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {}
`
	handlerPath := filepath.Join(apiDir, "pipeline_handler.go")
	require.NoError(t, os.WriteFile(handlerPath, []byte(handlerContent), 0644))

	ownership := []FileOwnership{
		{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new"},
	}
	items := detectAPIHandlers(ownership, repoRoot)

	// Should produce at least one item per handler function
	require.NotEmpty(t, items)
	for _, item := range items {
		assert.NotEmpty(t, item.Description)
	}
}

// ---------------------------------------------------------------------------
// TestDetectReactComponents
// ---------------------------------------------------------------------------

func TestDetectReactComponents(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		action      string
		wantItems   int
		setupFile   bool // whether to create a real file with default export
		description string
	}{
		{
			name:        "top-level component generates item",
			file:        "web/src/components/PipelineView.tsx",
			action:      "new",
			wantItems:   1,
			setupFile:   true,
			description: "generates checklist item for navigation wiring",
		},
		{
			name:        "UI primitive is filtered out",
			file:        "web/src/components/ui/button.tsx",
			action:      "new",
			wantItems:   0,
			setupFile:   false,
			description: "components/ui/ directory is filtered",
		},
		{
			name:        "test file is filtered out",
			file:        "web/src/components/PipelineView.test.tsx",
			action:      "new",
			wantItems:   0,
			setupFile:   false,
			description: "*.test.tsx files are filtered",
		},
		{
			name:        "spec file is filtered out",
			file:        "web/src/components/PipelineView.spec.tsx",
			action:      "new",
			wantItems:   0,
			setupFile:   false,
			description: "*.spec.tsx files are filtered",
		},
		{
			name:        "modify action does not generate item",
			file:        "web/src/components/PipelineView.tsx",
			action:      "modify",
			wantItems:   0,
			setupFile:   false,
			description: "only new files trigger detection",
		},
		{
			name:        "non-tsx file does not generate item",
			file:        "web/src/components/PipelineView.jsx",
			action:      "new",
			wantItems:   0,
			setupFile:   false,
			description: "only .tsx files are detected",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			if tc.setupFile {
				createReactComponentFile(t, repoRoot, tc.file, "PipelineView")
			}

			ownership := []FileOwnership{
				{File: tc.file, Agent: "C", Wave: 1, Action: tc.action},
			}
			items := detectReactComponents(ownership, repoRoot)
			assert.Len(t, items, tc.wantItems, tc.description)
		})
	}
}

// TestDetectReactComponents_WithRealFile tests parsing of an actual TSX file.
func TestDetectReactComponents_WithRealFile(t *testing.T) {
	repoRoot := t.TempDir()
	compDir := filepath.Join(repoRoot, "web", "src", "components")
	require.NoError(t, os.MkdirAll(compDir, 0755))

	t.Run("file with default export generates item", func(t *testing.T) {
		content := `import React from 'react';

export default function PipelineView() {
  return <div>Pipeline</div>;
}
`
		require.NoError(t, os.WriteFile(
			filepath.Join(compDir, "PipelineView.tsx"),
			[]byte(content), 0644,
		))

		ownership := []FileOwnership{
			{File: "web/src/components/PipelineView.tsx", Agent: "C", Wave: 1, Action: "new"},
		}
		items := detectReactComponents(ownership, repoRoot)
		require.Len(t, items, 1)
		assert.Contains(t, items[0].Description, "PipelineView")
	})

	t.Run("file without default export does not generate item", func(t *testing.T) {
		content := `import React from 'react';

export function useHelper() {
  return null;
}
`
		require.NoError(t, os.WriteFile(
			filepath.Join(compDir, "useHelper.tsx"),
			[]byte(content), 0644,
		))

		ownership := []FileOwnership{
			{File: "web/src/components/useHelper.tsx", Agent: "C", Wave: 1, Action: "new"},
		}
		items := detectReactComponents(ownership, repoRoot)
		assert.Len(t, items, 0)
	})
}

// ---------------------------------------------------------------------------
// TestDetectCLICommands
// ---------------------------------------------------------------------------

func TestDetectCLICommands(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		action    string
		wantItems int
		setupFile bool // whether to create real cmd file
	}{
		{
			name:      "new _cmd.go file generates item",
			file:      "cmd/polywave-tools/populate_integration_checklist_cmd.go",
			action:    "new",
			wantItems: 1,
			setupFile: true,
		},
		{
			name:      "cmd main.go with modify does not generate item",
			file:      "cmd/polywave-tools/main.go",
			action:    "modify",
			wantItems: 0,
			setupFile: false,
		},
		{
			name:      "new main.go does not generate item (not a _cmd.go file)",
			file:      "cmd/polywave-tools/main.go",
			action:    "new",
			wantItems: 0,
			setupFile: false,
		},
		{
			name:      "non-cmd directory does not generate item",
			file:      "pkg/protocol/some_cmd.go",
			action:    "new",
			wantItems: 0,
			setupFile: false,
		},
		{
			name:      "cmd file in subdirectory generates item",
			file:      "cmd/finalize_impl_cmd.go",
			action:    "new",
			wantItems: 1,
			setupFile: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			if tc.setupFile {
				createCLICmdFile(t, repoRoot, tc.file, "test-command", "newTestCommandCmd")
			}

			ownership := []FileOwnership{
				{File: tc.file, Agent: "D", Wave: 1, Action: tc.action},
			}
			items := detectCLICommands(ownership, repoRoot)
			assert.Len(t, items, tc.wantItems)
		})
	}
}

// TestDetectCLICommands_WithRealFile tests parsing an actual CLI command file.
func TestDetectCLICommands_WithRealFile(t *testing.T) {
	repoRoot := t.TempDir()
	cmdDir := filepath.Join(repoRoot, "cmd", "polywave-tools")
	require.NoError(t, os.MkdirAll(cmdDir, 0755))

	cmdContent := `package main

import "github.com/spf13/cobra"

func newPopulateIntegrationChecklistCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "populate-integration-checklist",
		Short: "Populates post-merge integration checklist",
		RunE:  runPopulateIntegrationChecklist,
	}
}

func runPopulateIntegrationChecklist(cmd *cobra.Command, args []string) error {
	return nil
}
`
	filePath := filepath.Join(cmdDir, "populate_integration_checklist_cmd.go")
	require.NoError(t, os.WriteFile(filePath, []byte(cmdContent), 0644))

	ownership := []FileOwnership{
		{File: "cmd/polywave-tools/populate_integration_checklist_cmd.go", Agent: "D", Wave: 1, Action: "new"},
	}
	items := detectCLICommands(ownership, repoRoot)
	require.Len(t, items, 1)
	assert.NotEmpty(t, items[0].Description)
	// Should mention registration in main.go
	assert.Contains(t, items[0].Description, "main.go")
}

// ---------------------------------------------------------------------------
// TestDetectBackgroundServices
// ---------------------------------------------------------------------------

func TestDetectBackgroundServices(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		wantItems   int
		description string
	}{
		{
			name: "file with go s.notificationBus.Start generates item",
			fileContent: `package notification

func (s *Service) Run(ctx context.Context) {
	go s.notificationBus.Start(ctx)
}
`,
			wantItems:   1,
			description: "goroutine starting a named service should be detected",
		},
		{
			name: "file with time.NewTicker generates item",
			fileContent: `package processor

import "time"

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			s.process()
		}
	}()
}
`,
			wantItems:   1,
			description: "time.NewTicker pattern should be detected",
		},
		{
			name: "file with go func generates item (acceptable false positive)",
			fileContent: `package handler

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	go func() {
		s.processAsync(r)
	}()
}
`,
			wantItems:   1,
			description: "one-off go func is an acceptable false positive",
		},
		{
			name: "file with no goroutines does not generate item",
			fileContent: `package util

func Add(a, b int) int {
	return a + b
}
`,
			wantItems:   0,
			description: "no goroutine patterns means no item",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			pkgDir := filepath.Join(repoRoot, "pkg", "service")
			require.NoError(t, os.MkdirAll(pkgDir, 0755))

			filePath := filepath.Join(pkgDir, "service.go")
			require.NoError(t, os.WriteFile(filePath, []byte(tc.fileContent), 0644))

			ownership := []FileOwnership{
				{File: "pkg/service/service.go", Agent: "A", Wave: 1, Action: "new"},
			}
			items := detectBackgroundServices(ownership, repoRoot)
			assert.Len(t, items, tc.wantItems, tc.description)
		})
	}
}

func TestDetectBackgroundServices_ModifyFileSkipped(t *testing.T) {
	repoRoot := t.TempDir()
	pkgDir := filepath.Join(repoRoot, "pkg", "service")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))

	content := `package service
func (s *Server) Run() { go s.worker.Start() }
`
	require.NoError(t, os.WriteFile(
		filepath.Join(pkgDir, "service.go"),
		[]byte(content), 0644,
	))

	// action: modify should not trigger detection
	ownership := []FileOwnership{
		{File: "pkg/service/service.go", Agent: "A", Wave: 1, Action: "modify"},
	}
	items := detectBackgroundServices(ownership, repoRoot)
	assert.Len(t, items, 0, "modify files should not trigger background service detection")
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_MultiPattern
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_MultiPattern(t *testing.T) {
	// Set up temp repo with real files so the parsers can actually detect items
	repoRoot := t.TempDir()

	// Create API handler files
	createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")
	createHandlerFile(t, repoRoot, "pkg/api/queue_handler.go")
	createHandlerFile(t, repoRoot, "pkg/api/resume_handler.go")

	// Create React component files
	createReactComponentFile(t, repoRoot, "web/src/components/PipelineView.tsx", "PipelineView")
	createReactComponentFile(t, repoRoot, "web/src/components/QueuePanel.tsx", "QueuePanel")

	// Create CLI command file
	createCLICmdFile(t, repoRoot, "cmd/polywave-tools/populate_integration_checklist_cmd.go",
		"populate-integration-checklist", "newPopulateIntegrationChecklistCmd")

	// Given: manifest with 3 new API handlers, 2 React components, 1 CLI command
	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/api/queue_handler.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/api/resume_handler.go", Agent: "B", Wave: 1, Action: "new"},
			{File: "web/src/components/PipelineView.tsx", Agent: "C", Wave: 1, Action: "new"},
			{File: "web/src/components/QueuePanel.tsx", Agent: "C", Wave: 1, Action: "new"},
			{File: "cmd/polywave-tools/populate_integration_checklist_cmd.go", Agent: "D", Wave: 1, Action: "new"},
		},
	}

	// Temporarily set manifest to use repoRoot for resolving files.
	// Since PopulateIntegrationChecklist uses repoRoot="" for single-repo IMPLs,
	// we need to either set file.Repo or use the repoRoot parameter.
	// We'll set Repo on each file to use the repoRoot path.
	for i := range manifest.FileOwnership {
		manifest.FileOwnership[i].Repo = repoRoot
	}

	// When: PopulateIntegrationChecklist runs
	result, err := PopulateIntegrationChecklist(manifest)

	// Then: returns 3 groups (API, React, CLI) with correct item counts
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.PostMergeChecklist)
	require.Len(t, result.PostMergeChecklist.Groups, 3)

	// Verify API group
	apiGroup := findGroup(result.PostMergeChecklist.Groups, "API Route Registration")
	require.NotNil(t, apiGroup, "expected 'API Route Registration' group")
	assert.Len(t, apiGroup.Items, 3)

	// Verify React group
	reactGroup := findGroup(result.PostMergeChecklist.Groups, "React Navigation Wiring")
	require.NotNil(t, reactGroup, "expected 'React Navigation Wiring' group")
	assert.Len(t, reactGroup.Items, 2)

	// Verify CLI group
	cliGroup := findGroup(result.PostMergeChecklist.Groups, "CLI Command Registration")
	require.NotNil(t, cliGroup, "expected 'CLI Command Registration' group")
	assert.Len(t, cliGroup.Items, 1)
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_Idempotent
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_Idempotent(t *testing.T) {
	repoRoot := t.TempDir()

	// Create real files
	createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")
	createReactComponentFile(t, repoRoot, "web/src/components/PipelineView.tsx", "PipelineView")

	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new", Repo: repoRoot},
			{File: "web/src/components/PipelineView.tsx", Agent: "C", Wave: 1, Action: "new", Repo: repoRoot},
		},
	}

	// First run
	result1, err := PopulateIntegrationChecklist(manifest)
	require.NoError(t, err)
	require.NotNil(t, result1.PostMergeChecklist)
	groupCount1 := len(result1.PostMergeChecklist.Groups)

	// Second run on same manifest
	result2, err := PopulateIntegrationChecklist(manifest)
	require.NoError(t, err)
	require.NotNil(t, result2.PostMergeChecklist)
	groupCount2 := len(result2.PostMergeChecklist.Groups)

	// Group counts should be the same (no duplicates)
	assert.Equal(t, groupCount1, groupCount2, "second run should not add duplicate groups")

	// Run on already-populated result should not add duplicates
	result3, err := PopulateIntegrationChecklist(result1)
	require.NoError(t, err)
	require.NotNil(t, result3.PostMergeChecklist)
	groupCount3 := len(result3.PostMergeChecklist.Groups)
	assert.Equal(t, groupCount1, groupCount3, "running on already-populated manifest should not duplicate groups")

	for _, g := range result3.PostMergeChecklist.Groups {
		// Count items in result1 for this group
		g1 := findGroup(result1.PostMergeChecklist.Groups, g.Title)
		if g1 != nil {
			assert.Equal(t, len(g1.Items), len(g.Items),
				"group %q should have same number of items, not duplicated", g.Title)
		}
	}
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_EmptyOwnership
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_EmptyOwnership(t *testing.T) {
	// Given: manifest with empty file_ownership
	manifest := &IMPLManifest{
		Title:         "Test Feature",
		FeatureSlug:   "test-feature",
		Verdict:       "SUITABLE",
		FileOwnership: []FileOwnership{},
	}

	// When: PopulateIntegrationChecklist runs
	result, err := PopulateIntegrationChecklist(manifest)

	// Then: returns manifest unchanged (no groups added)
	require.NoError(t, err)
	require.NotNil(t, result)

	if result.PostMergeChecklist != nil {
		assert.Len(t, result.PostMergeChecklist.Groups, 0,
			"empty file_ownership should produce no groups")
	}
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_NoNewFiles
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_NoNewFiles(t *testing.T) {
	// Given: manifest with only action:modify files
	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "modify"},
			{File: "web/src/components/PipelineView.tsx", Agent: "C", Wave: 1, Action: "modify"},
			{File: "cmd/polywave-tools/main.go", Agent: "D", Wave: 1, Action: "modify"},
		},
	}

	// When: PopulateIntegrationChecklist runs
	result, err := PopulateIntegrationChecklist(manifest)

	// Then: returns manifest unchanged (only action:new files trigger detection)
	require.NoError(t, err)
	require.NotNil(t, result)

	if result.PostMergeChecklist != nil {
		assert.Len(t, result.PostMergeChecklist.Groups, 0,
			"only action:modify files should produce no groups")
	}
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_DoesNotMutateInput
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_DoesNotMutateInput(t *testing.T) {
	repoRoot := t.TempDir()
	createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")

	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new", Repo: repoRoot},
		},
	}

	// Capture original state
	originalChecklist := manifest.PostMergeChecklist

	_, err := PopulateIntegrationChecklist(manifest)
	require.NoError(t, err)

	// Original manifest should be unchanged
	assert.Equal(t, originalChecklist, manifest.PostMergeChecklist,
		"PopulateIntegrationChecklist should not mutate the input manifest")
}

// ---------------------------------------------------------------------------
// TestParseHandlerFunctions
// ---------------------------------------------------------------------------

func TestParseHandlerFunctions(t *testing.T) {
	t.Run("parses handler functions from valid Go file", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "handler.go")
		content := `package api

import "net/http"

type Server struct{}

func (s *Server) handleGetPipeline(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {}
func (s *Server) helperMethod() {}
`
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		fns, err := parseHandlerFunctions(tmpFile)
		require.NoError(t, err)
		assert.Contains(t, fns, "handleGetPipeline")
		assert.Contains(t, fns, "handleCreatePipeline")
		assert.NotContains(t, fns, "helperMethod")
	})

	t.Run("returns empty slice for file with no handler functions", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "service.go")
		content := `package api

type Service struct{}

func (s *Service) process() {}
`
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		fns, err := parseHandlerFunctions(tmpFile)
		require.NoError(t, err)
		assert.Empty(t, fns)
	})

	t.Run("returns empty slice for non-existent file", func(t *testing.T) {
		fns, err := parseHandlerFunctions("/nonexistent/path/handler.go")
		// Should not panic; either err or empty slice - both acceptable
		if err != nil {
			assert.Empty(t, fns)
		}
	})
}

// ---------------------------------------------------------------------------
// TestParseReactComponentName
// ---------------------------------------------------------------------------

func TestParseReactComponentName(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "export default function",
			content: `import React from 'react';
export default function PipelineView() { return null; }`,
			want: "PipelineView",
		},
		{
			name: "export default variable",
			content: `const QueuePanel = () => null;
export default QueuePanel;`,
			want: "QueuePanel",
		},
		{
			name:    "no default export",
			content: `export function helper() { return null; }`,
			want:    "",
		},
		{
			name:    "empty file",
			content: ``,
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "component.tsx")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tc.content), 0644))

			got, err := parseReactComponentName(tmpFile)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("non-existent file returns empty string", func(t *testing.T) {
		got, err := parseReactComponentName("/nonexistent/Component.tsx")
		// Should not panic; either err or empty string - both acceptable
		if err != nil {
			assert.Empty(t, got)
		}
	})
}

// ---------------------------------------------------------------------------
// TestParseCLICommandName
// ---------------------------------------------------------------------------

func TestParseCLICommandName(t *testing.T) {
	t.Run("parses command name from cobra Use field", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "populate_cmd.go")
		content := `package main

import "github.com/spf13/cobra"

func newPopulateIntegrationChecklistCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "populate-integration-checklist",
		Short: "Populates the post-merge checklist",
	}
}
`
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		name, err := parseCLICommandName(tmpFile)
		require.NoError(t, err)
		assert.Equal(t, "populate-integration-checklist", name)
	})

	t.Run("returns empty string for file with no cobra command", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "helper.go")
		content := `package main

func helper() string { return "help" }
`
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		name, err := parseCLICommandName(tmpFile)
		require.NoError(t, err)
		assert.Empty(t, name)
	})

	t.Run("non-existent file returns empty string", func(t *testing.T) {
		name, err := parseCLICommandName("/nonexistent/cmd.go")
		// Should not panic; either err or empty string - both acceptable
		if err != nil {
			assert.Empty(t, name)
		}
	})
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_GroupTitles
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_GroupTitles(t *testing.T) {
	repoRoot := t.TempDir()
	createHandlerFile(t, repoRoot, "pkg/api/auth_handler.go")

	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/auth_handler.go", Agent: "A", Wave: 1, Action: "new", Repo: repoRoot},
		},
	}

	result, err := PopulateIntegrationChecklist(manifest)
	require.NoError(t, err)
	require.NotNil(t, result.PostMergeChecklist)

	// Verify that the API group has the expected title
	apiGroup := findGroup(result.PostMergeChecklist.Groups, "API Route Registration")
	assert.NotNil(t, apiGroup, "expected group titled 'API Route Registration'")
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_ChecklistItemFields
// ---------------------------------------------------------------------------

func TestPopulateIntegrationChecklist_ChecklistItemFields(t *testing.T) {
	repoRoot := t.TempDir()
	createHandlerFile(t, repoRoot, "pkg/api/pipeline_handler.go")

	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/api/pipeline_handler.go", Agent: "A", Wave: 1, Action: "new", Repo: repoRoot},
		},
	}

	result, err := PopulateIntegrationChecklist(manifest)
	require.NoError(t, err)
	require.NotNil(t, result.PostMergeChecklist)

	apiGroup := findGroup(result.PostMergeChecklist.Groups, "API Route Registration")
	require.NotNil(t, apiGroup)
	require.NotEmpty(t, apiGroup.Items)

	for _, item := range apiGroup.Items {
		assert.NotEmpty(t, item.Description, "checklist item should have a non-empty Description")
		// Command field is optional but if set should not be empty
		if item.Command != "" {
			assert.NotEmpty(t, item.Command)
		}
	}
}

// ---------------------------------------------------------------------------
// TestPopulateIntegrationChecklist_RepoResolution
// ---------------------------------------------------------------------------

// TestPopulateIntegrationChecklist_RepoResolution tests that files with
// explicit repo field are resolved correctly.
func TestPopulateIntegrationChecklist_RepoResolution(t *testing.T) {
	repoRoot := t.TempDir()
	createHandlerFile(t, repoRoot, "pkg/api/auth_handler.go")

	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			// With explicit Repo field pointing to our temp dir
			{File: "pkg/api/auth_handler.go", Agent: "A", Wave: 1, Action: "new", Repo: repoRoot},
		},
	}

	result, err := PopulateIntegrationChecklist(manifest)
	require.NoError(t, err)
	require.NotNil(t, result.PostMergeChecklist)
	assert.NotEmpty(t, result.PostMergeChecklist.Groups)
}

// ---------------------------------------------------------------------------
// TestDetectBackgroundServices_MultiplePatterns
// ---------------------------------------------------------------------------

func TestDetectBackgroundServices_MultiplePatterns(t *testing.T) {
	repoRoot := t.TempDir()
	pkgDir := filepath.Join(repoRoot, "pkg", "worker")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))

	// File with multiple goroutine patterns
	content := `package worker

import (
	"context"
	"time"
)

type Worker struct{}

func (w *Worker) Start(ctx context.Context) {
	// Long-running ticker
	ticker := time.NewTicker(10 * time.Second)
	
	// Background goroutine  
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.process()
			}
		}
	}()
}

func (w *Worker) process() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "worker.go"), []byte(content), 0644))

	ownership := []FileOwnership{
		{File: "pkg/worker/worker.go", Agent: "A", Wave: 1, Action: "new"},
	}
	items := detectBackgroundServices(ownership, repoRoot)

	// One item for the file (not one per pattern)
	assert.Len(t, items, 1, "should generate exactly one item per file with goroutine patterns")
	assert.NotEmpty(t, items[0].Description)
	assert.Contains(t, items[0].Description, "worker.go")
}
