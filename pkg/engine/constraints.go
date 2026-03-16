package engine

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// buildConstraints constructs a tools.Constraints from the IMPL manifest
// for a specific agent in a specific role.
//
// Behavior by role:
//   - "scout":    AllowedPathPrefixes=["docs/IMPL/IMPL-"], no OwnedFiles, no freeze
//   - "scaffold": AllowedPathPrefixes from manifest.Scaffolds file paths, no OwnedFiles
//   - "wave":     OwnedFiles from manifest.FileOwnership where Agent==agentID,
//     FrozenPaths from manifest.Scaffolds file paths + interface contract locations,
//     FreezeTime from manifest.WorktreesCreatedAt, TrackCommits=true
//
// Returns nil if manifest is nil (backward compatible, no enforcement).
func buildConstraints(manifest *protocol.IMPLManifest, agentID string, role string) *tools.Constraints {
	if manifest == nil {
		return nil
	}

	c := &tools.Constraints{
		AgentRole: role,
		AgentID:   agentID,
	}

	switch role {
	case "scout":
		c.AllowedPathPrefixes = []string{"docs/IMPL/IMPL-"}

	case "scaffold":
		prefixes := make([]string, 0, len(manifest.Scaffolds))
		for _, sf := range manifest.Scaffolds {
			if sf.FilePath != "" {
				prefixes = append(prefixes, sf.FilePath)
			}
		}
		c.AllowedPathPrefixes = prefixes

	case "wave":
		// I1: Owned files for this agent
		owned := make(map[string]bool)
		for _, fo := range manifest.FileOwnership {
			if fo.Agent == agentID {
				owned[fo.File] = true
			}
		}
		c.OwnedFiles = owned

		// I2: Frozen paths = scaffold file paths + interface contract locations
		frozen := make(map[string]bool)
		for _, sf := range manifest.Scaffolds {
			if sf.FilePath != "" {
				frozen[sf.FilePath] = true
			}
		}
		for _, ic := range manifest.InterfaceContracts {
			if ic.Location != "" {
				frozen[ic.Location] = true
			}
		}
		c.FrozenPaths = frozen

		// FreezeTime from manifest.WorktreesCreatedAt
		c.FreezeTime = manifest.WorktreesCreatedAt

		// I5: Track commits for wave agents
		c.TrackCommits = true
	}

	return c
}

// BuildWaveConstraints builds constraints for a wave agent from the IMPL manifest.
// This is the exported entry point for the orchestrator to call when launching
// wave agents. It loads the manifest from implPath and builds wave-role constraints.
//
// Returns nil constraints (not an error) if the manifest cannot be loaded,
// preserving backward compatibility for IMPL docs without constraint support.
func BuildWaveConstraints(implPath string, agentID string) (*tools.Constraints, error) {
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return nil, fmt.Errorf("BuildWaveConstraints: load manifest: %w", err)
	}
	return buildConstraints(manifest, agentID, "wave"), nil
}
