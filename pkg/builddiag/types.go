package builddiag

// ErrorPattern represents a recognized build error with fix recommendation
type ErrorPattern struct {
	Name        string  // e.g., "missing_import", "type_mismatch"
	Regex       string  // Regex pattern to match error message
	Fix         string  // Command or action to fix
	Rationale   string  // Why this fix works
	AutoFixable bool    // Whether fix can be automated
	Confidence  float64 // Match confidence (0.0-1.0)
}

// Diagnosis is the result of pattern matching
type Diagnosis struct {
	Pattern     string  `yaml:"diagnosis"`
	Confidence  float64 `yaml:"confidence"`
	Fix         string  `yaml:"fix"`
	Rationale   string  `yaml:"rationale"`
	AutoFixable bool    `yaml:"auto_fixable"`
}
