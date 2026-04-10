package protocol

// StaleConstraintWarning is emitted when an agent's task text references a
// file path that is owned by a different agent in the manifest.
type StaleConstraintWarning struct {
	AgentID       string `json:"agent_id"`
	MentionedFile string `json:"mentioned_file"`
	OwnedByAgent  string `json:"owned_by_agent"`
	Message       string `json:"message"`
}

// DetectStaleConstraints scans each agent's task text for file path references
// that appear in another agent's file_ownership, emitting a warning for each match.
// Returns nil if no stale references are found.
func DetectStaleConstraints(m *IMPLManifest) []StaleConstraintWarning {
	// stub — Agent A implements
	return nil
}
