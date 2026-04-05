package commands

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ExtractCommands is a convenience wrapper for web CLI delegation.
// It creates a new Extractor pre-loaded with all standard parsers
// (GithubActionsParser, MakefileParser, PackageJSONParser) and calls Extract().
func ExtractCommands(ctx context.Context, repoRoot string) result.Result[ExtractData] {
	e := New()
	e.RegisterCIParser(&GithubActionsParser{})
	e.RegisterBuildSystemParser(&MakefileParser{})
	e.RegisterBuildSystemParser(&PackageJSONParser{})
	return e.Extract(ctx, repoRoot)
}
