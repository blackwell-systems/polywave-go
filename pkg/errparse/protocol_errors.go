package errparse

import "strings"

// ProtocolErrorInfo holds the structured explanation for a protocol error.
type ProtocolErrorInfo struct {
	Code        string   `json:"code"`
	Title       string   `json:"title"`
	PlainText   string   `json:"plain_text"`
	Remediation []string `json:"remediation"`
	SeeAlso     string   `json:"see_also"`
}

// protocolErrors is the package-level map from error code to explanation.
var protocolErrors = map[string]ProtocolErrorInfo{}

func init() {
	entries := []ProtocolErrorInfo{
		{
			Code:      "I1",
			Title:     "Disjoint File Ownership",
			PlainText: "Two agents tried to modify the same file.",
			Remediation: []string{
				"Check file_ownership in the IMPL doc. Each file must be assigned to exactly one agent per wave.",
			},
			SeeAlso: "protocol/invariants.md#I1",
		},
		{
			Code:      "I2",
			Title:     "Cross-Wave Dependencies Only",
			PlainText: "An agent depends on another agent in the same wave.",
			Remediation: []string{
				"Move the dependent agent to a later wave, or remove the dependency.",
			},
			SeeAlso: "protocol/invariants.md#I2",
		},
		{
			Code:      "I3",
			Title:     "1-Indexed Waves",
			PlainText: "Wave numbering starts at 0 instead of 1.",
			Remediation: []string{
				"Change wave numbers to start at 1.",
			},
			SeeAlso: "protocol/invariants.md#I3",
		},
		{
			Code:      "I4",
			Title:     "Scout Write Boundaries",
			PlainText: "Scout agent tried to write source code.",
			Remediation: []string{
				"Scout agents only write IMPL docs (docs/IMPL/IMPL-*.yaml).",
			},
			SeeAlso: "protocol/invariants.md#I4",
		},
		{
			Code:      "I5",
			Title:     "Agent Commit Verification",
			PlainText: "Agent did not commit its changes.",
			Remediation: []string{
				"Ensure each agent commits all changes before completion.",
			},
			SeeAlso: "protocol/invariants.md#I5",
		},
		{
			Code:      "I6",
			Title:     "Role Separation",
			PlainText: "Orchestrator is performing agent work directly.",
			Remediation: []string{
				"Launch the appropriate agent type instead of implementing directly.",
			},
			SeeAlso: "protocol/invariants.md#I6",
		},
		{
			Code:      "E2",
			Title:     "Interface Freeze",
			PlainText: "Interface changed after worktrees were created.",
			Remediation: []string{
				"Delete worktrees, update interfaces, recreate worktrees.",
			},
			SeeAlso: "protocol/execution-rules.md#E2",
		},
		{
			Code:      "E16",
			Title:     "IMPL Validation",
			PlainText: "IMPL doc has structural errors.",
			Remediation: []string{
				"Run polywave-tools validate --fix to auto-correct common issues.",
			},
			SeeAlso: "protocol/execution-rules.md#E16",
		},
		{
			Code:      "E20",
			Title:     "Stub Scan",
			PlainText: "Implementation contains stub/placeholder code.",
			Remediation: []string{
				"Replace TODO/FIXME/stub markers with real implementation.",
			},
			SeeAlso: "protocol/execution-rules.md#E20",
		},
		{
			Code:      "E21",
			Title:     "Quality Gate Failure",
			PlainText: "Build, test, or lint check failed.",
			Remediation: []string{
				"Fix the failing check. Run the gate command manually to see details.",
			},
			SeeAlso: "protocol/execution-rules.md#E21",
		},
		{
			Code:      "E21A",
			Title:     "Baseline Gate",
			PlainText: "Quality gate was already failing before agent work.",
			Remediation: []string{
				"Fix pre-existing failures first, or mark as known issue.",
			},
			SeeAlso: "protocol/execution-rules.md#E21A",
		},
		{
			Code:      "E25",
			Title:     "Integration Gap",
			PlainText: "Exported function not wired into caller code.",
			Remediation: []string{
				"Add integration_connectors or create an integration wave.",
			},
			SeeAlso: "protocol/execution-rules.md#E25",
		},
		{
			Code:      "E37",
			Title:     "Critic Review",
			PlainText: "Pre-wave brief review found issues.",
			Remediation: []string{
				"Address critic findings before launching wave agents.",
			},
			SeeAlso: "protocol/execution-rules.md#E37",
		},
	}

	for _, e := range entries {
		protocolErrors[e.Code] = e
	}
}

// ExplainProtocolError returns a human-readable explanation for a
// SAW protocol error code. Returns nil if code is unknown.
// Lookup is case-insensitive (e.g. "e16" matches "E16").
func ExplainProtocolError(code string) *ProtocolErrorInfo {
	upper := strings.ToUpper(code)
	if info, ok := protocolErrors[upper]; ok {
		return &info
	}
	return nil
}

// AllProtocolErrors returns all known error explanations.
func AllProtocolErrors() []ProtocolErrorInfo {
	result := make([]ProtocolErrorInfo, 0, len(protocolErrors))
	for _, info := range protocolErrors {
		result = append(result, info)
	}
	return result
}
