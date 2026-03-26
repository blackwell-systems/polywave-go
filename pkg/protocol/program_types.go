package protocol

import "github.com/blackwell-systems/scout-and-wave-go/pkg/result"

// PROGRAMManifest is the structured representation of a SAW PROGRAM document.
// It coordinates multiple IMPL docs into tiered execution.
type PROGRAMManifest struct {
	Title            string             `yaml:"title" json:"title"`
	ProgramSlug      string             `yaml:"program_slug" json:"program_slug"`
	State            ProgramState       `yaml:"state" json:"state"`
	Created          string             `yaml:"created,omitempty" json:"created,omitempty"`
	Updated          string             `yaml:"updated,omitempty" json:"updated,omitempty"`
	Requirements     string             `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	ProgramContracts []ProgramContract  `yaml:"program_contracts,omitempty" json:"program_contracts,omitempty"`
	Impls            []ProgramIMPL      `yaml:"impls" json:"impls"`
	Tiers            []ProgramTier      `yaml:"tiers" json:"tiers"`
	TierGates        []QualityGate      `yaml:"tier_gates,omitempty" json:"tier_gates,omitempty"`
	Completion       ProgramCompletion  `yaml:"completion" json:"completion"`
	PreMortem        []PreMortemRow     `yaml:"pre_mortem,omitempty" json:"pre_mortem,omitempty"`
}

// ProgramState represents the current state of the PROGRAM manifest.
type ProgramState string

const (
	ProgramStatePlanning      ProgramState = "PLANNING"
	ProgramStateValidating    ProgramState = "VALIDATING"
	ProgramStateReviewed      ProgramState = "REVIEWED"
	ProgramStateScaffold      ProgramState = "SCAFFOLD"
	ProgramStateTierExecuting ProgramState = "TIER_EXECUTING"
	ProgramStateTierVerified  ProgramState = "TIER_VERIFIED"
	ProgramStateComplete      ProgramState = "COMPLETE"
	ProgramStateBlocked       ProgramState = "BLOCKED"
	ProgramStateNotSuitable   ProgramState = "NOT_SUITABLE"
)

// ProgramContract defines a cross-IMPL interface contract.
type ProgramContract struct {
	Name        string                    `yaml:"name" json:"name"`
	Description string                    `yaml:"description,omitempty" json:"description,omitempty"`
	Definition  string                    `yaml:"definition" json:"definition"`
	Consumers   []ProgramContractConsumer `yaml:"consumers,omitempty" json:"consumers,omitempty"`
	Location    string                    `yaml:"location" json:"location"`
	FreezeAt    string                    `yaml:"freeze_at,omitempty" json:"freeze_at,omitempty"`
}

// ProgramContractConsumer identifies an IMPL that consumes a program contract.
type ProgramContractConsumer struct {
	Impl  string `yaml:"impl" json:"impl"`
	Usage string `yaml:"usage" json:"usage"`
}

// ProgramIMPL represents an IMPL within the PROGRAM manifest.
type ProgramIMPL struct {
	Slug            string   `yaml:"slug" json:"slug"`
	AbsPath         string   `yaml:"abs_path,omitempty" json:"abs_path,omitempty"`
	Title           string   `yaml:"title" json:"title"`
	Tier            int      `yaml:"tier" json:"tier"`
	DependsOn       []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	EstimatedAgents int      `yaml:"estimated_agents,omitempty" json:"estimated_agents,omitempty"`
	EstimatedWaves  int      `yaml:"estimated_waves,omitempty" json:"estimated_waves,omitempty"`
	KeyOutputs      []string `yaml:"key_outputs,omitempty" json:"key_outputs,omitempty"`
	Status            string `yaml:"status" json:"status"`
	PriorityScore     int    `yaml:"priority_score,omitempty" json:"priority_score,omitempty"`
	PriorityReasoning string `yaml:"priority_reasoning,omitempty" json:"priority_reasoning,omitempty"`
	// SerialWaves lists wave numbers that must not execute concurrently with the
	// same-numbered wave of any other IMPL in the same program tier.
	// Empty (default) means all waves can run in parallel with tier peers.
	// Populated automatically by create-program based on wave-level conflict detection.
	SerialWaves []int `yaml:"serial_waves,omitempty" json:"serial_waves,omitempty"`
}

// ProgramTier groups IMPLs that can execute in parallel.
type ProgramTier struct {
	Number      int      `yaml:"number" json:"number"`
	Impls       []string `yaml:"impls" json:"impls"`
	Description    string `yaml:"description,omitempty" json:"description,omitempty"`
	ConcurrencyCap int    `yaml:"concurrency_cap,omitempty" json:"concurrency_cap,omitempty"`
}

// ProgramCompletion tracks overall program progress.
type ProgramCompletion struct {
	TiersComplete int `yaml:"tiers_complete" json:"tiers_complete"`
	TiersTotal    int `yaml:"tiers_total" json:"tiers_total"`
	ImplsComplete int `yaml:"impls_complete" json:"impls_complete"`
	ImplsTotal    int `yaml:"impls_total" json:"impls_total"`
	TotalAgents   int `yaml:"total_agents" json:"total_agents"`
	TotalWaves    int `yaml:"total_waves" json:"total_waves"`
}

// ImportedIMPL describes a single IMPL that was imported into a PROGRAM manifest.
type ImportedIMPL struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	AssignedTier int    `json:"assigned_tier"`
	AgentCount   int    `json:"agent_count"`
	WaveCount    int    `json:"wave_count"`
}

// ImportIMPLsData is the data payload for the import-impls operation.
// Use result.Result[ImportIMPLsData] as the return type for functions
// that perform IMPL imports.
type ImportIMPLsData struct {
	ManifestPath    string         `json:"manifest_path"`
	ImplsImported   []ImportedIMPL `json:"impls_imported"`
	ImplsDiscovered []string       `json:"impls_discovered,omitempty"`
	TierAssignments map[string]int `json:"tier_assignments"`
	P1Conflicts     []string       `json:"p1_conflicts,omitempty"`
	P2Conflicts     []string       `json:"p2_conflicts,omitempty"`
	Created         bool           `json:"created"`
	Updated         bool           `json:"updated"`
}

// NewImportIMPLsResult creates a successful result.Result wrapping ImportIMPLsData.
func NewImportIMPLsResult(data ImportIMPLsData) result.Result[ImportIMPLsData] {
	return result.NewSuccess(data)
}
