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
	// Handle "back" command first.
	if HandleBackCommand(doc, answer) {
		// Back was processed — regenerate current question and return.
		currentQ := generateQuestion(doc)
		doc.UpdatedAt = time.Now()

		// Save state after going back.
		docPath := filepath.Join(m.docsDir, fmt.Sprintf("INTERVIEW-%s.yaml", doc.Slug))
		if err := m.Save(doc, docPath); err != nil {
			return doc, currentQ, fmt.Errorf("saving interview state: %w", err)
		}

		return doc, currentQ, nil
	}

	// Get current question to record it.
	currentQ := generateQuestion(doc)
	if currentQ == nil {
		return doc, nil, nil
	}

	// Validate required fields.
	if validationErr := ValidateRequiredField(currentQ, answer); validationErr != "" {
		// Validation failed — return same question with error in Hint field.
		currentQ.Hint = validationErr
		return doc, currentQ, nil
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

// ValidateRequiredField checks if a required field answer is valid.
// Returns an error message string if invalid, empty string if valid.
func ValidateRequiredField(q *InterviewQuestion, answer string) string {
	if !q.Required {
		return ""
	}

	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return "This field is required. Please provide an answer."
	}

	// "skip" is not allowed for required fields.
	if strings.EqualFold(trimmed, "skip") {
		return "This field is required and cannot be skipped. Please provide an answer."
	}

	return ""
}

// HandleBackCommand detects "back" answer and reverts to previous question.
// Returns true if back was handled (caller should re-generate question).
func HandleBackCommand(doc *InterviewDoc, answer string) bool {
	trimmed := strings.TrimSpace(answer)
	if !strings.EqualFold(trimmed, "back") {
		return false
	}

	// Can't go back from first question.
	if doc.QuestionCursor == 0 {
		return false
	}

	// Remove last history entry.
	if len(doc.History) > 0 {
		lastTurn := doc.History[len(doc.History)-1]
		doc.History = doc.History[:len(doc.History)-1]

		// Clear the spec data field that was populated by this turn.
		clearSpecField(doc, lastTurn.Phase, getFieldNameFromHistory(doc, lastTurn))
	}

	// Decrement cursor.
	doc.QuestionCursor--

	// Recalculate phase by replaying phase transitions from the beginning.
	recalculatePhase(doc)

	// Update progress.
	doc.Progress = float64(doc.QuestionCursor) / float64(doc.MaxQuestions)
	if doc.Progress < 0 {
		doc.Progress = 0
	}

	return true
}

// getFieldNameFromHistory extracts the field name from a history turn.
// We need to match the question text to the phaseQuestions list.
func getFieldNameFromHistory(doc *InterviewDoc, turn InterviewTurn) string {
	// Match question text to find field name.
	for _, q := range phaseQuestions {
		if q.Phase == turn.Phase && strings.Contains(turn.Question, q.Text[:min(len(q.Text), 20)]) {
			return q.Field
		}
	}
	return ""
}

// clearSpecField clears a specific field in the spec data.
func clearSpecField(doc *InterviewDoc, phase InterviewPhase, fieldName string) {
	switch phase {
	case PhaseOverview:
		switch fieldName {
		case "title":
			doc.SpecData.Overview.Title = ""
		case "goal":
			doc.SpecData.Overview.Goal = ""
		case "success_metrics":
			doc.SpecData.Overview.SuccessMetrics = nil
		case "non_goals":
			doc.SpecData.Overview.NonGoals = nil
		}
	case PhaseScope:
		switch fieldName {
		case "in_scope":
			doc.SpecData.Scope.InScope = nil
		case "out_of_scope":
			doc.SpecData.Scope.OutOfScope = nil
		case "assumptions":
			doc.SpecData.Scope.Assumptions = nil
		}
	case PhaseRequirements:
		switch fieldName {
		case "functional":
			doc.SpecData.Requirements.Functional = nil
		case "non_functional":
			doc.SpecData.Requirements.NonFunctional = nil
		case "constraints":
			doc.SpecData.Requirements.Constraints = nil
		}
	case PhaseInterfaces:
		switch fieldName {
		case "data_models":
			doc.SpecData.Interfaces.DataModels = nil
		case "apis":
			doc.SpecData.Interfaces.APIs = nil
		case "external":
			doc.SpecData.Interfaces.External = nil
		}
	case PhaseStories:
		if fieldName == "stories" {
			doc.SpecData.Stories = nil
		}
	case PhaseReview:
		if fieldName == "open_questions" {
			doc.SpecData.OpenQuestions = nil
		}
	}
}

// recalculatePhase recalculates the current phase from scratch by checking transitions.
func recalculatePhase(doc *InterviewDoc) {
	// Start from Overview and apply transitions based on current spec data.
	doc.Phase = PhaseOverview

	// Keep checking transitions until we can't advance anymore.
	for {
		oldPhase := doc.Phase
		checkPhaseTransition(doc)
		if doc.Phase == oldPhase {
			// No transition happened, we're at the correct phase.
			break
		}
		// If we've reached the phase where there are unanswered questions, stop.
		if doc.Phase == PhaseComplete {
			break
		}
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// newID generates a random UUID-formatted ID (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
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
