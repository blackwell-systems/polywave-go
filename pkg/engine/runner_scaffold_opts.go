package engine

import "context"

// RunScaffoldOpts holds parameters for RunScaffold.
type RunScaffoldOpts struct {
	Ctx         context.Context
	ImplPath    string
	RepoPath    string
	SAWRepoPath string
	Model       string
	OnEvent     func(Event)
}
