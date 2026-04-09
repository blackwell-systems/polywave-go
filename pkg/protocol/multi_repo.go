package protocol

import "strings"

// ResolveAgentRepo returns the absolute repo root path for an agent.
// It looks up the agent's repo name from fileOwnership, then resolves
// the path from the repos registry. Falls back to fallbackRoot if the
// agent has no repo field or the repo is not found in the registry.
func ResolveAgentRepo(
	fileOwnership []FileOwnership,
	agentID string,
	fallbackRoot string,
	repos []RepoEntry,
) string {
	repoName := ""
	for _, fo := range fileOwnership {
		if fo.Agent == agentID && fo.Repo != "" {
			repoName = fo.Repo
			break
		}
	}
	if repoName == "" {
		return fallbackRoot
	}
	for _, r := range repos {
		if strings.EqualFold(r.Name, repoName) {
			return r.Path
		}
	}
	return fallbackRoot
}

// AgentRepoName returns the repo name for an agent from file ownership.
// Returns empty string if the agent has no repo field (single-repo case).
// This is the name-only counterpart to ResolveAgentRepo.
func AgentRepoName(fileOwnership []FileOwnership, agentID string) string {
	for _, fo := range fileOwnership {
		if fo.Agent == agentID && fo.Repo != "" {
			return fo.Repo
		}
	}
	return ""
}
