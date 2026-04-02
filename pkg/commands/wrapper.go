package commands

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ExtractCommands is a convenience wrapper for web CLI delegation.
// It creates a new Extractor and calls Extract() with default parsers.
func ExtractCommands(ctx context.Context, repoRoot string) result.Result[ExtractData] {
	e := New()
	return e.Extract(ctx, repoRoot)
}
