package interview

import (
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// InterviewPhase is the current section of the requirements being explored.
type InterviewPhase string

const (
	PhaseOverview     InterviewPhase = "overview"
	PhaseScope        InterviewPhase = "scope"
	PhaseRequirements InterviewPhase = "requirements"
	PhaseInterfaces   InterviewPhase = "interfaces"
	PhaseStories      InterviewPhase = "stories"
	PhaseReview       InterviewPhase = "review"
	PhaseComplete     InterviewPhase = "complete"
)

// InterviewMode controls question generation strategy.
type InterviewMode string

const (
	ModeDeterministic InterviewMode = "deterministic"
	ModeLLM           InterviewMode = "llm"
)

// InterviewQuestion is a single question in the interview flow.
type InterviewQuestion struct {
	ID        string         `yaml:"id"`
	Phase     InterviewPhase `yaml:"phase"`
	FieldName string         `yaml:"field_name"`
	Text      string         `yaml:"text"`
	Hint      string         `yaml:"hint,omitempty"`
	Required  bool           `yaml:"required"`
}

// InterviewTurn is a recorded question-answer pair.
type InterviewTurn struct {
	TurnNumber int            `yaml:"turn_number"`
	Phase      InterviewPhase `yaml:"phase"`
	Question   string         `yaml:"question"`
	Answer     string         `yaml:"answer"`
	Timestamp  time.Time      `yaml:"timestamp"`
	FieldName  string         `yaml:"field_name,omitempty"`
}

// SpecData holds the structured output being built across all phases.
type SpecData struct {
	Overview      OverviewSpec     `yaml:"overview,omitempty"`
	Scope         ScopeSpec        `yaml:"scope,omitempty"`
	Requirements  RequirementsSpec `yaml:"requirements,omitempty"`
	Interfaces    InterfacesSpec   `yaml:"interfaces,omitempty"`
	Stories       []string         `yaml:"stories,omitempty"`
	OpenQuestions []string         `yaml:"open_questions,omitempty"`
}

// OverviewSpec holds phase-1 answers.
type OverviewSpec struct {
	Title          string   `yaml:"title,omitempty"`
	Goal           string   `yaml:"goal,omitempty"`
	SuccessMetrics []string `yaml:"success_metrics,omitempty"`
	NonGoals       []string `yaml:"non_goals,omitempty"`
}

// ScopeSpec holds phase-2 answers.
type ScopeSpec struct {
	InScope    []string `yaml:"in_scope,omitempty"`
	OutOfScope []string `yaml:"out_of_scope,omitempty"`
	Assumptions []string `yaml:"assumptions,omitempty"`
}

// RequirementsSpec holds phase-3 answers.
type RequirementsSpec struct {
	Functional    []string `yaml:"functional,omitempty"`
	NonFunctional []string `yaml:"non_functional,omitempty"`
	Constraints   []string `yaml:"constraints,omitempty"`
}

// InterfacesSpec holds phase-4 answers.
type InterfacesSpec struct {
	DataModels []string `yaml:"data_models,omitempty"`
	APIs       []string `yaml:"apis,omitempty"`
	External   []string `yaml:"external,omitempty"`
}

// InterviewDoc is the persisted state written to docs/INTERVIEW-<slug>.yaml.
// It is the single source of truth for a running or completed interview.
type InterviewDoc struct {
	// Metadata
	ID          string        `yaml:"id"`
	Slug        string        `yaml:"slug"`
	Status      string        `yaml:"status"` // "in_progress" | "complete"
	Mode        InterviewMode `yaml:"mode"`
	Description string        `yaml:"description"`
	CreatedAt   time.Time     `yaml:"created_at"`
	UpdatedAt   time.Time     `yaml:"updated_at"`

	// Progress
	Phase          InterviewPhase `yaml:"phase"`
	QuestionCursor int            `yaml:"question_cursor"`
	MaxQuestions   int            `yaml:"max_questions"`
	Progress       float64        `yaml:"progress"`

	// Accumulated data
	SpecData SpecData       `yaml:"spec_data"`
	History  []InterviewTurn `yaml:"history"`

	// Output: path to the generated REQUIREMENTS.md (set when status=complete)
	RequirementsPath string `yaml:"requirements_path,omitempty"`
}

// InterviewConfig holds parameters for starting or resuming an interview.
type InterviewConfig struct {
	Description  string        `yaml:"description"`
	Slug         string        `yaml:"slug"`
	Mode         InterviewMode `yaml:"mode"`
	MaxQuestions int           `yaml:"max_questions"`
	ProjectPath  string        `yaml:"project_path,omitempty"`
}

// StartData holds the result payload returned by Manager.Start.
type StartData struct {
	Doc      *InterviewDoc
	Question *InterviewQuestion
}

// ResumeData holds the result payload returned by Manager.Resume.
type ResumeData struct {
	Doc      *InterviewDoc
	Question *InterviewQuestion // nil when Status == "complete"
}

// AnswerData holds the result payload returned by Manager.Answer.
type AnswerData struct {
	Doc      *InterviewDoc
	Question *InterviewQuestion // nil when PhaseComplete reached
}

// CompileData holds the result payload returned by Manager.Compile.
type CompileData struct {
	OutputPath string
}

// SaveDocData holds metadata returned from a successful Save call.
type SaveDocData struct {
	DocPath   string
	Timestamp time.Time
}

// WriteReqData holds metadata returned from a successful WriteRequirementsFile call.
type WriteReqData struct {
	OutputPath string
	LineCount  int
}

// Manager is the interface for the interview state machine.
// Deterministic and LLM-backed implementations satisfy this interface.
type Manager interface {
	// Start initializes a new interview and returns the first question.
	Start(cfg InterviewConfig) result.Result[StartData]

	// Resume loads an existing interview from its YAML file and returns the current question.
	Resume(docPath string) result.Result[ResumeData]

	// Answer records a user response, advances the state machine, and returns the next question.
	// Returns a Result containing (doc, nil question) when the interview reaches PhaseComplete.
	Answer(doc *InterviewDoc, answer string) result.Result[AnswerData]

	// Compile generates docs/REQUIREMENTS.md from a complete InterviewDoc.
	// Returns the path to the generated file.
	Compile(doc *InterviewDoc, outputPath string) result.Result[CompileData]

	// Save persists the InterviewDoc to its YAML file.
	Save(doc *InterviewDoc, docPath string) result.Result[SaveDocData]
}
