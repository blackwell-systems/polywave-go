package protocol

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// UpdateContextData contains the data of updating the project context file.
type UpdateContextData struct {
	ContextPath string   `json:"context_path"`
	Updated     bool     `json:"updated"`
	NewEntries  []string `json:"new_entries"`
}

// UpdateContext appends a completion entry to the project CONTEXT.md file.
// It loads the manifest to extract feature metadata and appends a completion record per E18 schema.
// If CONTEXT.md doesn't exist, it creates it with the standard header.
func UpdateContext(ctx context.Context, manifestPath string, projectRoot string) result.Result[*UpdateContextData] {
	// Load manifest to get feature metadata
	manifest, err := Load(ctx, manifestPath)
	if err != nil {
		return result.NewFailure[*UpdateContextData]([]result.SAWError{{
			Code:     result.CodeContextError,
			Message:  fmt.Sprintf("failed to load manifest: %v", err),
			Severity: "fatal",
		}})
	}

	// Calculate wave count and agent count
	waveCount := len(manifest.Waves)
	agentCount := 0
	for _, wave := range manifest.Waves {
		agentCount += len(wave.Agents)
	}

	// Determine context file path
	contextPath := ContextMDPath(projectRoot)

	// Ensure docs directory exists
	docsDir := filepath.Dir(contextPath)
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return result.NewFailure[*UpdateContextData]([]result.SAWError{{
			Code:     result.CodeContextError,
			Message:  fmt.Sprintf("failed to create docs directory: %v", err),
			Severity: "fatal",
		}})
	}

	// Check if CONTEXT.md exists
	var content string
	data, err := os.ReadFile(contextPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return result.NewFailure[*UpdateContextData]([]result.SAWError{{
				Code:     result.CodeContextError,
				Message:  fmt.Sprintf("failed to read context file: %v", err),
				Severity: "fatal",
			}})
		}
		// Create new file with header
		content = "# Project Context\n\n## Features Completed\n"
	} else {
		content = string(data)
	}

	// Get relative path to manifest from project root
	relManifestPath, err := filepath.Rel(projectRoot, manifestPath)
	if err != nil {
		// If we can't get relative path, use absolute path
		relManifestPath = manifestPath
	}

	// Format date as YYYY-MM-DD
	date := time.Now().Format("2006-01-02")

	// Build entry per E18 schema format
	entry := fmt.Sprintf("- **%s**: completed %s, %d waves, %d agents\n  - IMPL doc: %s\n",
		manifest.FeatureSlug,
		date,
		waveCount,
		agentCount,
		relManifestPath)

	// Append entry
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry

	// Write updated content
	if err := os.WriteFile(contextPath, []byte(content), 0644); err != nil {
		return result.NewFailure[*UpdateContextData]([]result.SAWError{{
			Code:     result.CodeContextError,
			Message:  fmt.Sprintf("failed to write context file: %v", err),
			Severity: "fatal",
		}})
	}

	return result.NewSuccess(&UpdateContextData{
		ContextPath: contextPath,
		Updated:     true,
		NewEntries:  []string{manifest.FeatureSlug},
	})
}
