// Package protocol parses and updates IMPL documents — the single source of
// truth for SAW protocol execution. It extracts wave/agent structure,
// reads completion reports written by agents, and ticks status checkboxes
// once agents report completion. All IMPL doc I/O is concentrated here.
package protocol

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)


// ValidateInvariants checks Invariant I1: no file appears in two different
// agents' ownership lists within the same wave. For cross-repo waves, the key
// is "repo:file" so files with the same name in different repos are not flagged.
// Returns a descriptive error for the first violation found, or nil if the
// document is clean.
func ValidateInvariants(doc *types.IMPLDoc) error {
	if doc == nil {
		return nil
	}
	for _, wave := range doc.Waves {
		seen := make(map[string]string) // "repo:file" -> agent letter
		for _, agent := range wave.Agents {
			for _, file := range agent.FilesOwned {
				// Build composite key: use repo from ownership table if available.
				repo := ""
				if info, ok := doc.FileOwnership[file]; ok {
					repo = info.Repo
				}
				key := repo + ":" + file
				if prev, ok := seen[key]; ok {
					return fmt.Errorf(
						"I1 violation in Wave %d: file %q claimed by both Agent %s and Agent %s",
						wave.Number, file, prev, agent.Letter,
					)
				}
				seen[key] = agent.Letter
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
