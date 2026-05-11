// Package protocol parses and updates IMPL documents — the single source of
// truth for SAW protocol execution. It extracts wave/agent structure,
// reads completion reports written by agents, and ticks status checkboxes
// once agents report completion. All IMPL doc I/O is concentrated here.
package protocol

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)


// ValidateInvariants checks protocol Invariant I1 (disjoint file ownership).
// For full I1–I6 validation use Validate.
func ValidateInvariants(manifest *IMPLManifest) []result.PolywaveError {
	if manifest == nil {
		return nil
	}

	// Build a lookup map from the FileOwnership slice.
	ownershipByFile := make(map[string]FileOwnership, len(manifest.FileOwnership))
	for _, fo := range manifest.FileOwnership {
		ownershipByFile[fo.File] = fo
	}

	for _, wave := range manifest.Waves {
		seen := make(map[string]string) // "repo:file" -> agent ID
		for _, agent := range wave.Agents {
			for _, file := range agent.Files {
				// Build composite key: use repo from ownership table if available.
				repo := ""
				if fo, ok := ownershipByFile[file]; ok {
					repo = fo.Repo
				}
				key := repo + ":" + file
				if prev, ok := seen[key]; ok {
					msg := fmt.Sprintf(
						"I1 violation in Wave %d: file %q claimed by both Agent %s and Agent %s",
						wave.Number, file, prev, agent.ID,
					)
					return []result.PolywaveError{result.NewFatal(result.CodeDisjointOwnership, msg)}
				}
				seen[key] = agent.ID
			}
		}
	}
	return nil
}



// ── helpers ──────────────────────────────────────────────────────────────────

// extractTypedBlock reads lines from scanner until closing fence.
// Returns (blockLines, found). blockLines excludes fence markers.
// Scanner is positioned after closing fence on return.
func extractTypedBlock(scanner *bufio.Scanner) ([]string, bool) {
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			return lines, true
		}
		lines = append(lines, line)
	}
	return lines, false // EOF without closing fence
}
