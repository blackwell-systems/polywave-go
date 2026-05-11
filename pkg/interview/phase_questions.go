package interview

import (
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// phaseOrder maps each InterviewPhase to its sequential position (0-indexed).
// Used to detect invalid skip-phase transitions in checkPhaseTransition.
var phaseOrder = map[InterviewPhase]int{
	PhaseOverview:     0,
	PhaseScope:        1,
	PhaseRequirements: 2,
	PhaseInterfaces:   3,
	PhaseStories:      4,
	PhaseReview:       5,
	PhaseComplete:     6,
}

// phaseQuestionDef defines a fixed question for a given phase/field.
type phaseQuestionDef struct {
	Phase    InterviewPhase
	Field    string
	Text     string
	Required bool
}

// phaseQuestions is the ordered list of all deterministic questions.
var phaseQuestions = []phaseQuestionDef{
	// Overview
	{PhaseOverview, "title", "What is the title of this project or feature?", true},
	{PhaseOverview, "goal", "What is the primary goal? (one sentence)", true},
	{PhaseOverview, "success_metrics", "What are the success metrics? (comma-separated) (or type 'skip' to skip)", false},
	{PhaseOverview, "non_goals", "What is explicitly out of scope? (comma-separated) (or type 'skip' to skip)", false},

	// Scope
	{PhaseScope, "in_scope", "What is in scope? List the key deliverables (comma-separated)", true},
	{PhaseScope, "out_of_scope", "What is out of scope? (comma-separated) (or type 'skip' to skip)", false},
	{PhaseScope, "assumptions", "What assumptions are you making? (comma-separated) (or type 'skip' to skip)", false},

	// Requirements
	{PhaseRequirements, "functional", "List the functional requirements (one per line or comma-separated)", true},
	{PhaseRequirements, "non_functional", "Any non-functional requirements? (e.g., performance, security) (or type 'skip' to skip)", false},
	{PhaseRequirements, "constraints", "Any technical constraints? (e.g., Go 1.21+, no CGO) (or type 'skip' to skip)", false},

	// Interfaces
	{PhaseInterfaces, "data_models", "What are the key data models or types? (or type 'skip' to skip)", false},
	{PhaseInterfaces, "apis", "What are the key APIs or command interfaces? (or type 'skip' to skip)", false},
	{PhaseInterfaces, "external", "Any external integrations? (or type 'skip' to skip)", false},

	// Stories
	{PhaseStories, "stories", "List the key user stories or tasks (one per line) (or type 'skip' to skip)", false},

	// Review
	{PhaseReview, "open_questions", "Any open questions or unresolved decisions? (or type 'skip' to skip)", false},
	{PhaseReview, "_confirm", "Review complete. Ready to generate REQUIREMENTS.md? (yes/no)", true},
}

// generateQuestion returns the next question for the current phase,
// or nil if the interview is complete.
func generateQuestion(doc *InterviewDoc) *InterviewQuestion {
	if doc.Phase == PhaseComplete {
		return nil
	}

	// Find the first unanswered question for the current phase.
	for i, q := range phaseQuestions {
		if q.Phase != doc.Phase {
			continue
		}
		if fieldIsPopulated(doc, q.Phase, q.Field) {
			continue
		}
		return &InterviewQuestion{
			ID:        fmt.Sprintf("%s_%s_%d", q.Phase, q.Field, i),
			Phase:     q.Phase,
			FieldName: q.Field,
			Text:      q.Text,
			Required:  q.Required,
		}
	}

	// All questions for this phase answered — shouldn't normally reach here
	// because checkPhaseTransition would have advanced the phase.
	// Return nil to signal no more questions in current phase.
	return nil
}

// fieldIsPopulated checks whether a given spec_data field already has a value.
func fieldIsPopulated(doc *InterviewDoc, phase InterviewPhase, field string) bool {
	switch phase {
	case PhaseOverview:
		switch field {
		case "title":
			return doc.SpecData.Overview.Title != ""
		case "goal":
			return doc.SpecData.Overview.Goal != ""
		case "success_metrics":
			return doc.SpecData.Overview.SuccessMetrics != nil
		case "non_goals":
			return doc.SpecData.Overview.NonGoals != nil
		}
	case PhaseScope:
		switch field {
		case "in_scope":
			return doc.SpecData.Scope.InScope != nil
		case "out_of_scope":
			return doc.SpecData.Scope.OutOfScope != nil
		case "assumptions":
			return doc.SpecData.Scope.Assumptions != nil
		}
	case PhaseRequirements:
		switch field {
		case "functional":
			return doc.SpecData.Requirements.Functional != nil
		case "non_functional":
			return doc.SpecData.Requirements.NonFunctional != nil
		case "constraints":
			return doc.SpecData.Requirements.Constraints != nil
		}
	case PhaseInterfaces:
		switch field {
		case "data_models":
			return doc.SpecData.Interfaces.DataModels != nil
		case "apis":
			return doc.SpecData.Interfaces.APIs != nil
		case "external":
			return doc.SpecData.Interfaces.External != nil
		}
	case PhaseStories:
		switch field {
		case "stories":
			return doc.SpecData.Stories != nil
		}
	case PhaseReview:
		switch field {
		case "open_questions":
			return doc.SpecData.OpenQuestions != nil
		case "_confirm":
			// _confirm is never "populated" — it's always asked once per review phase
			return false
		}
	}
	return false
}

// checkPhaseTransition checks if all required fields for the current phase
// are filled and advances to the next phase if so.
// Guard: if the transition would skip more than one phase forward, returns a PolywaveError.
// Backward transitions are NOT guarded here — they are handled by
// recalculatePhase (which resets to PhaseOverview and replays forward-only).
func checkPhaseTransition(doc *InterviewDoc) *result.PolywaveError {
	prevPhase := doc.Phase

	switch doc.Phase {
	case PhaseOverview:
		if doc.SpecData.Overview.Title != "" && doc.SpecData.Overview.Goal != "" {
			// Optional fields: success_metrics, non_goals — check if asked (nil means not yet asked)
			if allOverviewQuestionsAsked(doc) {
				doc.Phase = PhaseScope
			}
		}
	case PhaseScope:
		if doc.SpecData.Scope.InScope != nil && allScopeQuestionsAsked(doc) {
			doc.Phase = PhaseRequirements
		}
	case PhaseRequirements:
		if len(doc.SpecData.Requirements.Functional) > 0 && allRequirementsQuestionsAsked(doc) {
			doc.Phase = PhaseInterfaces
		}
	case PhaseInterfaces:
		if allInterfacesQuestionsAsked(doc) {
			doc.Phase = PhaseStories
		}
	case PhaseStories:
		if doc.SpecData.Stories != nil {
			doc.Phase = PhaseReview
		}
	case PhaseReview:
		// Review advances to complete only via _confirm answer
		// handled directly in applyAnswerToSpec
	}

	// Guard: detect skip-phase transitions (jumping 2+ phases forward).
	// Single-step forward transitions are normal and allowed.
	// Backward transitions are handled by recalculatePhase — not guarded here.
	if doc.Phase != prevPhase {
		newOrder, newOk := phaseOrder[doc.Phase]
		prevOrder, prevOk := phaseOrder[prevPhase]
		if newOk && prevOk && newOrder > prevOrder+1 {
			sawErr := result.NewError(result.CodeInvalidState,
				fmt.Sprintf("interview: invalid phase skip: %s → %s", prevPhase, doc.Phase))
			return &sawErr
		}
	}
	return nil
}

// Helper functions to check if all questions in a phase have been asked.
// A nil slice means "not yet asked"; an empty slice means "asked, skipped".

func allOverviewQuestionsAsked(doc *InterviewDoc) bool {
	return doc.SpecData.Overview.SuccessMetrics != nil &&
		doc.SpecData.Overview.NonGoals != nil
}

func allScopeQuestionsAsked(doc *InterviewDoc) bool {
	return doc.SpecData.Scope.OutOfScope != nil &&
		doc.SpecData.Scope.Assumptions != nil
}

func allRequirementsQuestionsAsked(doc *InterviewDoc) bool {
	return doc.SpecData.Requirements.NonFunctional != nil &&
		doc.SpecData.Requirements.Constraints != nil
}

func allInterfacesQuestionsAsked(doc *InterviewDoc) bool {
	return doc.SpecData.Interfaces.DataModels != nil &&
		doc.SpecData.Interfaces.APIs != nil &&
		doc.SpecData.Interfaces.External != nil
}

// FormatPhaseProgress generates phase-aware progress string like "[Overview: 2/4 | Next: Scope]".
func FormatPhaseProgress(doc *InterviewDoc) string {
	if doc.Phase == PhaseComplete {
		return "[Complete]"
	}

	// Count questions in current phase.
	totalInPhase := questionsInPhase(doc.Phase)

	// Count answered questions in current phase.
	answeredInPhase := 0
	for _, q := range phaseQuestions {
		if q.Phase != doc.Phase {
			continue
		}
		if fieldIsPopulated(doc, q.Phase, q.Field) {
			answeredInPhase++
		}
	}

	// Get next phase name.
	next := nextPhaseName(doc.Phase)

	// Format: "[Overview: 2/4 | Next: Scope]"
	return fmt.Sprintf("[%s: %d/%d | Next: %s]", doc.Phase, answeredInPhase, totalInPhase, next)
}

// questionsInPhase counts the number of questions for a given phase.
func questionsInPhase(phase InterviewPhase) int {
	count := 0
	for _, q := range phaseQuestions {
		if q.Phase == phase {
			count++
		}
	}
	return count
}

// nextPhaseName returns the display name of the next phase.
func nextPhaseName(phase InterviewPhase) string {
	switch phase {
	case PhaseOverview:
		return "Scope"
	case PhaseScope:
		return "Requirements"
	case PhaseRequirements:
		return "Interfaces"
	case PhaseInterfaces:
		return "Stories"
	case PhaseStories:
		return "Review"
	case PhaseReview:
		return "Done"
	case PhaseComplete:
		return "Done"
	default:
		return "Unknown"
	}
}
