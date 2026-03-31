package commands

import "context"

// ExtractCommands is a convenience wrapper for web CLI delegation.
// It creates a new Extractor and calls Extract() with default parsers.
func ExtractCommands(ctx context.Context, repoRoot string) (*CommandSet, error) {
	e := New()
	return e.Extract(ctx, repoRoot)
}
