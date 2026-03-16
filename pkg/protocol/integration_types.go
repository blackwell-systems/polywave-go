package protocol

// IntegrationGap represents a detected unconnected export.
// Used by ValidateIntegration (E25) to identify exports that were created
// by wave agents but have no call-sites in the codebase.
type IntegrationGap struct {
	ExportName    string   `json:"export_name" yaml:"export_name"`
	FilePath      string   `json:"file_path" yaml:"file_path"`
	AgentID       string   `json:"agent_id" yaml:"agent_id"`
	Category      string   `json:"category" yaml:"category"`                 // "function_call", "type_usage", "field_init"
	Severity      string   `json:"severity" yaml:"severity"`                 // "error", "warning", "info"
	Reason        string   `json:"reason" yaml:"reason"`
	SuggestedFix  string   `json:"suggested_fix" yaml:"suggested_fix"`
	SearchResults []string `json:"search_results" yaml:"search_results"` // files that might need to call it
}

// IntegrationReport summarizes all integration gaps for a wave.
// Produced by ValidateIntegration (E25) after wave agents complete.
// Consumed by the Integration Agent (E26) to wire gaps into caller files.
type IntegrationReport struct {
	Wave    int              `json:"wave" yaml:"wave"`
	Gaps    []IntegrationGap `json:"gaps" yaml:"gaps"`
	Valid   bool             `json:"valid" yaml:"valid"`       // true if no gaps detected
	Summary string           `json:"summary" yaml:"summary"`
}

// IntegrationConnector defines a file that the integration agent is allowed to modify.
// These are caller files (orchestrator.go, main.go, server.go) that need wiring.
// Listed in the IMPL manifest's integration_connectors field.
type IntegrationConnector struct {
	File   string `json:"file" yaml:"file"`
	Reason string `json:"reason" yaml:"reason"`
}
