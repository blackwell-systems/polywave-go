package protocol

// TierGateResult is the output of running tier-level quality gates.
// It reuses the existing GateResult type from gates.go.
type TierGateResult struct {
	TierNumber   int              `json:"tier_number"`
	Passed       bool             `json:"passed"`
	GateResults  []GateResult     `json:"gate_results"`
	ImplStatuses []ImplTierStatus `json:"impl_statuses"`
	AllImplsDone bool             `json:"all_impls_done"`
}

// ImplTierStatus captures the status of a single IMPL within a tier.
type ImplTierStatus struct {
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

// FreezeContractsResult is the output of freezing program contracts at a tier boundary.
type FreezeContractsResult struct {
	TierNumber       int              `json:"tier_number"`
	ContractsFrozen  []FrozenContract `json:"contracts_frozen"`
	ContractsSkipped []string         `json:"contracts_skipped"`
	Success          bool             `json:"success"`
	Errors           []string         `json:"errors,omitempty"`
}

// FrozenContract captures details about a single frozen contract.
type FrozenContract struct {
	Name       string `json:"name"`
	Location   string `json:"location"`
	FreezeAt   string `json:"freeze_at"`
	FileExists bool   `json:"file_exists"`
	Committed  bool   `json:"committed"`
}

// ProgramStatusResult is the full status report for a PROGRAM manifest.
type ProgramStatusResult struct {
	ProgramSlug      string               `json:"program_slug"`
	Title            string               `json:"title"`
	State            ProgramState         `json:"state"`
	CurrentTier      int                  `json:"current_tier"`
	TierStatuses     []TierStatusDetail   `json:"tier_statuses"`
	ContractStatuses []ContractStatus     `json:"contract_statuses"`
	Completion       ProgramCompletion    `json:"completion"`
}

// TierStatusDetail provides detailed status for a single tier.
type TierStatusDetail struct {
	Number       int              `json:"number"`
	Description  string           `json:"description,omitempty"`
	ImplStatuses []ImplTierStatus `json:"impl_statuses"`
	Complete     bool             `json:"complete"`
}

// ContractStatus tracks freeze state of a program contract.
type ContractStatus struct {
	Name         string `json:"name"`
	Location     string `json:"location"`
	FreezeAt     string `json:"freeze_at"`
	Frozen       bool   `json:"frozen"`
	FrozenAtTier int    `json:"frozen_at_tier,omitempty"`
}
