package protocol

// WiringDeclaration declares that symbol DefinedIn.Symbol must be called
// from CallerFile. Written by Scout in the IMPL doc wiring: block.
// Enforced by prepare-wave pre-flight (Layer 3A) and validate-integration
// post-merge check (Layer 3B). Injected into agent briefs by prepare-wave
// (Layer 3C).
type WiringDeclaration struct {
	Symbol             string `yaml:"symbol" json:"symbol"`
	DefinedIn          string `yaml:"defined_in" json:"defined_in"`
	MustBeCalledFrom   string `yaml:"must_be_called_from" json:"must_be_called_from"`
	Agent              string `yaml:"agent" json:"agent"`
	Wave               int    `yaml:"wave" json:"wave"`
	IntegrationPattern string `yaml:"integration_pattern,omitempty" json:"integration_pattern,omitempty"`
}

// WiringValidationData is returned by ValidateWiringDeclarations.
type WiringValidationData struct {
	Gaps    []WiringGap `json:"gaps" yaml:"gaps"`
	Valid   bool        `json:"valid" yaml:"valid"`
	Summary string      `json:"summary" yaml:"summary"`
}

// WiringGap is a single missing wiring: the symbol is declared but not
// found in MustBeCalledFrom.
type WiringGap struct {
	Declaration WiringDeclaration `json:"declaration" yaml:"declaration"`
	Reason      string            `json:"reason" yaml:"reason"`
	Severity    string            `json:"severity" yaml:"severity"` // always "error"
}
