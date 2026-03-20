package interview

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DeterministicManager implements the Manager interface using a fixed question set.
type DeterministicManager struct {
	docsDir string
}

// NewDeterministicManager creates a new deterministic interview manager.
// docsDir is the base directory for writing INTERVIEW-<slug>.yaml files.
func NewDeterministicManager(docsDir string) *DeterministicManager {
	return &DeterministicManager{docsDir: docsDir}
}

// Start initializes a new interview and returns the first question.
func (m *DeterministicManager) Start(cfg InterviewConfig) (*InterviewDoc, *InterviewQuestion, error) {
	slug := cfg.Slug
	if slug == "" {
		slug = generateSlug(cfg.Description)
	}

	maxQ := cfg.MaxQuestions
	if maxQ == 0 {
		maxQ = 18
	}

	now := time.Now()
	doc := &InterviewDoc{
		ID:             newID(),
		Slug:           slug,
		Status:         "in_progress",
		Mode:           ModeDeterministic,
		Description:    cfg.Description,
		CreatedAt:      now,
		UpdatedAt:      now,
		Phase:          PhaseOverview,
		QuestionCursor: 0,
		MaxQuestions:    maxQ,
		Progress:       0.0,
		SpecData:       SpecData{},
		History:        []InterviewTurn{},
	}

	q := generateQuestion(doc)
	return doc, q, nil
}

// Resume loads an existing interview from its YAML file and returns the current question.
func (m *DeterministicManager) Resume(docPath string) (*InterviewDoc, *InterviewQuestion, error) {
	data, err := os.ReadFile(docPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading interview doc: %w", err)
	}

	var doc InterviewDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parsing interview doc: %w", err)
	}

	if doc.Status == "complete" {
		return &doc, nil, nil
	}

	q := generateQuestion(&doc)
	return &doc, q, nil
}

// Answer records a user response, advances the state machine, and returns the next question.
func (m *DeterministicManager) Answer(doc *InterviewDoc, answer string) (*InterviewDoc, *InterviewQuestion, error) {
	// Get current question to record it.
	currentQ := generateQuestion(doc)
	if currentQ == nil {
		return doc, nil, nil
	}

	// Record the turn.
	doc.History = append(doc.History, InterviewTurn{
		TurnNumber: len(doc.History) + 1,
		Phase:      doc.Phase,
		Question:   currentQ.Text,
		Answer:     answer,
		Timestamp:  time.Now(),
	})

	// Apply answer to spec data.
	applyAnswerToSpec(doc, currentQ, answer)

	// Increment cursor.
	doc.QuestionCursor++

	// Update progress.
	doc.Progress = float64(doc.QuestionCursor) / float64(doc.MaxQuestions)
	if doc.Progress > 1.0 {
		doc.Progress = 1.0
	}

	// Check phase transition.
	checkPhaseTransition(doc)

	// If phase is complete, mark status.
	if doc.Phase == PhaseComplete {
		doc.Status = "complete"
		doc.Progress = 1.0
	}

	doc.UpdatedAt = time.Now()

	// Generate next question.
	nextQ := generateQuestion(doc)

	// Save state.
	docPath := filepath.Join(m.docsDir, fmt.Sprintf("INTERVIEW-%s.yaml", doc.Slug))
	if err := m.Save(doc, docPath); err != nil {
		return doc, nextQ, fmt.Errorf("saving interview state: %w", err)
	}

	return doc, nextQ, nil
}

// compileFunc is the function used to generate requirements markdown.
// Defaults to CompileToRequirements from compiler.go (Agent B).
// This indirection allows the build to succeed before compiler.go is merged.
var compileFunc func(doc *InterviewDoc) (string, error)

// Compile generates REQUIREMENTS.md from a complete InterviewDoc.
// Delegates content generation to CompileToRequirements (compiler.go),
// then writes the result to outputPath.
func (m *DeterministicManager) Compile(doc *InterviewDoc, outputPath string) (string, error) {
	fn := compileFunc
	if fn == nil {
		return "", fmt.Errorf("CompileToRequirements not available (compiler.go not yet merged)")
	}
	content, err := fn(doc)
	if err != nil {
		return "", fmt.Errorf("compiling requirements: %w", err)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing requirements: %w", err)
	}

	doc.RequirementsPath = outputPath
	return outputPath, nil
}

// RegisterCompiler sets the compile function. Called by compiler.go's init().
func RegisterCompiler(fn func(doc *InterviewDoc) (string, error)) {
	compileFunc = fn
}

// Save persists the InterviewDoc to a YAML file.
func (m *DeterministicManager) Save(doc *InterviewDoc, docPath string) error {
	dir := filepath.Dir(docPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshaling interview doc: %w", err)
	}

	if err := os.WriteFile(docPath, data, 0o644); err != nil {
		return fmt.Errorf("writing interview doc: %w", err)
	}

	return nil
}

// newID generates a random hex ID (not a full UUID, but sufficient for interview docs).
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// generateSlug creates a URL-friendly slug from a description.
// Lowercase, spaces to hyphens, strip non-alphanumeric except hyphens, max 40 chars.
func generateSlug(desc string) string {
	s := strings.ToLower(strings.TrimSpace(desc))
	s = strings.ReplaceAll(s, " ", "-")

	re := regexp.MustCompile(`[^a-z0-9-]`)
	s = re.ReplaceAllString(s, "")

	// Collapse multiple hyphens.
	re2 := regexp.MustCompile(`-+`)
	s = re2.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")

	if len(s) > 40 {
		s = s[:40]
		s = strings.TrimRight(s, "-")
	}

	if s == "" {
		s = "interview"
	}
	return s
}

// applyAnswerToSpec updates the SpecData based on the current question and answer.
func applyAnswerToSpec(doc *InterviewDoc, q *InterviewQuestion, answer string) {
	isSkip := strings.EqualFold(strings.TrimSpace(answer), "skip")

	switch q.Phase {
	case PhaseOverview:
		applyOverviewAnswer(doc, q.FieldName, answer, isSkip)
	case PhaseScope:
		applyScopeAnswer(doc, q.FieldName, answer, isSkip)
	case PhaseRequirements:
		applyRequirementsAnswer(doc, q.FieldName, answer, isSkip)
	case PhaseInterfaces:
		applyInterfacesAnswer(doc, q.FieldName, answer, isSkip)
	case PhaseStories:
		applyStoriesAnswer(doc, q.FieldName, answer, isSkip)
	case PhaseReview:
		applyReviewAnswer(doc, q.FieldName, answer, isSkip)
	}
}

func applyOverviewAnswer(doc *InterviewDoc, field, answer string, isSkip bool) {
	switch field {
	case "title":
		doc.SpecData.Overview.Title = strings.TrimSpace(answer)
	case "goal":
		doc.SpecData.Overview.Goal = strings.TrimSpace(answer)
	case "success_metrics":
		if isSkip {
			doc.SpecData.Overview.SuccessMetrics = []string{}
		} else {
			doc.SpecData.Overview.SuccessMetrics = splitCSV(answer)
		}
	case "non_goals":
		if isSkip {
			doc.SpecData.Overview.NonGoals = []string{}
		} else {
			doc.SpecData.Overview.NonGoals = splitCSV(answer)
		}
	}
}

func applyScopeAnswer(doc *InterviewDoc, field, answer string, isSkip bool) {
	switch field {
	case "in_scope":
		if isSkip {
			doc.SpecData.Scope.InScope = []string{}
		} else {
			doc.SpecData.Scope.InScope = splitCSV(answer)
		}
	case "out_of_scope":
		if isSkip {
			doc.SpecData.Scope.OutOfScope = []string{}
		} else {
			doc.SpecData.Scope.OutOfScope = splitCSV(answer)
		}
	case "assumptions":
		if isSkip {
			doc.SpecData.Scope.Assumptions = []string{}
		} else {
			doc.SpecData.Scope.Assumptions = splitCSV(answer)
		}
	}
}

func applyRequirementsAnswer(doc *InterviewDoc, field, answer string, isSkip bool) {
	switch field {
	case "functional":
		if isSkip {
			doc.SpecData.Requirements.Functional = []string{}
		} else {
			doc.SpecData.Requirements.Functional = splitCSV(answer)
		}
	case "non_functional":
		if isSkip {
			doc.SpecData.Requirements.NonFunctional = []string{}
		} else {
			doc.SpecData.Requirements.NonFunctional = splitCSV(answer)
		}
	case "constraints":
		if isSkip {
			doc.SpecData.Requirements.Constraints = []string{}
		} else {
			doc.SpecData.Requirements.Constraints = splitCSV(answer)
		}
	}
}

func applyInterfacesAnswer(doc *InterviewDoc, field, answer string, isSkip bool) {
	switch field {
	case "data_models":
		if isSkip {
			doc.SpecData.Interfaces.DataModels = []string{}
		} else {
			doc.SpecData.Interfaces.DataModels = splitCSV(answer)
		}
	case "apis":
		if isSkip {
			doc.SpecData.Interfaces.APIs = []string{}
		} else {
			doc.SpecData.Interfaces.APIs = splitCSV(answer)
		}
	case "external":
		if isSkip {
			doc.SpecData.Interfaces.External = []string{}
		} else {
			doc.SpecData.Interfaces.External = splitCSV(answer)
		}
	}
}

func applyStoriesAnswer(doc *InterviewDoc, field, answer string, isSkip bool) {
	if field == "stories" {
		if isSkip {
			doc.SpecData.Stories = []string{}
		} else {
			doc.SpecData.Stories = splitCSV(answer)
		}
	}
}

func applyReviewAnswer(doc *InterviewDoc, field, answer string, isSkip bool) {
	switch field {
	case "open_questions":
		if isSkip {
			doc.SpecData.OpenQuestions = []string{}
		} else {
			doc.SpecData.OpenQuestions = splitCSV(answer)
		}
	case "_confirm":
		a := strings.ToLower(strings.TrimSpace(answer))
		if a == "yes" || a == "y" {
			doc.Phase = PhaseComplete
		}
	}
}

// splitCSV splits a comma-separated string into trimmed, non-empty strings.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if result == nil {
		result = []string{}
	}
	return result
}
