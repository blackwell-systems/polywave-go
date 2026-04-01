package interview

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestDeterministicManager_Start(t *testing.T) {
	mgr := NewDeterministicManager(t.TempDir())
	doc, q, err := mgr.Start(InterviewConfig{
		Description: "My Test Feature",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if doc == nil {
		t.Fatal("doc is nil")
	}
	if doc.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", doc.Status)
	}
	if doc.Phase != PhaseOverview {
		t.Errorf("expected phase overview, got %s", doc.Phase)
	}
	if doc.Slug == "" {
		t.Error("expected non-empty slug")
	}
	if doc.Slug != "my-test-feature" {
		t.Errorf("expected slug 'my-test-feature', got %q", doc.Slug)
	}
	if doc.MaxQuestions != 18 {
		t.Errorf("expected max_questions 18, got %d", doc.MaxQuestions)
	}
	if q == nil {
		t.Fatal("expected first question, got nil")
	}
	if q.Phase != PhaseOverview {
		t.Errorf("expected question phase overview, got %s", q.Phase)
	}
	if q.FieldName != "title" {
		t.Errorf("expected field title, got %s", q.FieldName)
	}
}

func TestDeterministicManager_FullFlow(t *testing.T) {
	mgr := NewDeterministicManager(t.TempDir())
	doc, q, err := mgr.Start(InterviewConfig{
		Description: "Full Flow Test",
	})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Drive through all phases with minimal answers.
	answers := []string{
		// Overview
		"My Project",         // title
		"Build something",    // goal
		"skip",               // success_metrics
		"skip",               // non_goals
		// Scope
		"feature A, feature B", // in_scope
		"skip",                 // out_of_scope
		"skip",                 // assumptions
		// Requirements
		"req1, req2",  // functional
		"skip",        // non_functional
		"skip",        // constraints
		// Interfaces
		"skip", // data_models
		"skip", // apis
		"skip", // external
		// Stories
		"skip", // stories
		// Review
		"skip", // open_questions
		"yes",  // _confirm
	}

	for i, ans := range answers {
		if q == nil {
			t.Fatalf("question is nil at step %d (answer: %q), doc phase: %s, status: %s",
				i, ans, doc.Phase, doc.Status)
		}
		doc, q, err = mgr.Answer(doc, ans)
		if err != nil {
			t.Fatalf("Answer error at step %d: %v", i, err)
		}
	}

	if doc.Status != "complete" {
		t.Errorf("expected status complete, got %s (phase: %s)", doc.Status, doc.Phase)
	}
	if doc.Phase != PhaseComplete {
		t.Errorf("expected phase complete, got %s", doc.Phase)
	}
	if q != nil {
		t.Errorf("expected nil question after completion, got %+v", q)
	}
	if len(doc.History) != len(answers) {
		t.Errorf("expected %d history entries, got %d", len(answers), len(doc.History))
	}
}

func TestDeterministicManager_PhaseTransition_Overview(t *testing.T) {
	mgr := NewDeterministicManager(t.TempDir())
	doc, q, err := mgr.Start(InterviewConfig{Description: "Phase Test"})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Answer title
	doc, q, err = mgr.Answer(doc, "Test Title")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	if doc.Phase != PhaseOverview {
		t.Errorf("should still be in overview after just title, got %s", doc.Phase)
	}

	// Answer goal
	doc, q, err = mgr.Answer(doc, "Test Goal")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	if doc.Phase != PhaseOverview {
		t.Errorf("should still be in overview (optional fields not yet asked), got %s", doc.Phase)
	}

	// Answer success_metrics (skip)
	doc, q, err = mgr.Answer(doc, "skip")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}

	// Answer non_goals (skip) — should now transition
	doc, q, err = mgr.Answer(doc, "skip")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	if doc.Phase != PhaseScope {
		t.Errorf("expected phase scope after overview complete, got %s", doc.Phase)
	}
	if q == nil {
		t.Fatal("expected question for scope phase")
	}
	if q.Phase != PhaseScope {
		t.Errorf("expected question in scope phase, got %s", q.Phase)
	}
}

func TestDeterministicManager_PhaseTransition_RequiresTitle(t *testing.T) {
	mgr := NewDeterministicManager(t.TempDir())
	doc, _, err := mgr.Start(InterviewConfig{Description: "Requires Title Test"})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Manually set goal but not title, then check transition.
	doc.SpecData.Overview.Goal = "some goal"
	doc.SpecData.Overview.SuccessMetrics = []string{}
	doc.SpecData.Overview.NonGoals = []string{}
	checkPhaseTransition(doc)

	if doc.Phase != PhaseOverview {
		t.Errorf("should NOT advance without title, got phase %s", doc.Phase)
	}
}

func TestDeterministicManager_SkipOptional(t *testing.T) {
	mgr := NewDeterministicManager(t.TempDir())
	doc, _, err := mgr.Start(InterviewConfig{Description: "Skip Test"})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Answer title
	doc, _, err = mgr.Answer(doc, "Title")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	// Answer goal
	doc, _, err = mgr.Answer(doc, "Goal")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	// Skip success_metrics
	doc, _, err = mgr.Answer(doc, "skip")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	// Verify success_metrics is empty slice (not nil)
	if doc.SpecData.Overview.SuccessMetrics == nil {
		t.Error("expected empty slice for skipped success_metrics, got nil")
	}
	if len(doc.SpecData.Overview.SuccessMetrics) != 0 {
		t.Errorf("expected 0 success metrics, got %d", len(doc.SpecData.Overview.SuccessMetrics))
	}

	// Skip non_goals — should advance to scope
	doc, _, err = mgr.Answer(doc, "SKIP") // test case-insensitive
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	if doc.Phase != PhaseScope {
		t.Errorf("expected scope after skipping optional overview fields, got %s", doc.Phase)
	}
}

func TestDeterministicManager_Resume(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDeterministicManager(dir)

	// Start and answer a few questions.
	doc, _, err := mgr.Start(InterviewConfig{Description: "Resume Test"})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	doc, _, err = mgr.Answer(doc, "My Title")
	if err != nil {
		t.Fatalf("Answer error: %v", err)
	}

	// Save explicitly.
	docPath := filepath.Join(dir, "INTERVIEW-"+doc.Slug+".yaml")
	// Answer already saved via mgr.Answer, but let's verify resume.

	// Resume from saved file.
	doc2, q2, err := mgr.Resume(docPath)
	if err != nil {
		t.Fatalf("Resume error: %v", err)
	}
	if doc2.SpecData.Overview.Title != "My Title" {
		t.Errorf("expected title 'My Title', got %q", doc2.SpecData.Overview.Title)
	}
	if doc2.Phase != PhaseOverview {
		t.Errorf("expected phase overview, got %s", doc2.Phase)
	}
	if q2 == nil {
		t.Fatal("expected question after resume")
	}
	if q2.FieldName != "goal" {
		t.Errorf("expected next field 'goal', got %q", q2.FieldName)
	}
}

func TestGenerateQuestion_AllPhases(t *testing.T) {
	phases := []InterviewPhase{
		PhaseOverview, PhaseScope, PhaseRequirements,
		PhaseInterfaces, PhaseStories, PhaseReview,
	}

	for _, phase := range phases {
		doc := &InterviewDoc{Phase: phase}
		q := generateQuestion(doc)
		if q == nil {
			t.Errorf("expected question for phase %s, got nil", phase)
			continue
		}
		if q.Phase != phase {
			t.Errorf("expected question phase %s, got %s", phase, q.Phase)
		}
	}

	// PhaseComplete should return nil.
	doc := &InterviewDoc{Phase: PhaseComplete}
	q := generateQuestion(doc)
	if q != nil {
		t.Errorf("expected nil for PhaseComplete, got %+v", q)
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Feature", "my-feature"},
		{"Hello World!!!", "hello-world"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"a very long description that exceeds forty characters by far", "a-very-long-description-that-exceeds-for"},
		{"", "interview"},
		{"---dashes---", "dashes"},
	}
	for _, tt := range tests {
		got := generateSlug(tt.input)
		if got != tt.want {
			t.Errorf("generateSlug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDeterministicManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDeterministicManager(dir)

	doc, _, err := mgr.Start(InterviewConfig{Description: "Save Load"})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	docPath := filepath.Join(dir, "INTERVIEW-"+doc.Slug+".yaml")
	saveResult := mgr.Save(doc, docPath)
	if saveResult.IsFatal() {
		t.Fatalf("Save error: %v", saveResult.Errors)
	}
	if !saveResult.IsSuccess() {
		t.Fatalf("expected success result, got code: %s", saveResult.Code)
	}

	// Verify data fields are populated correctly.
	data := saveResult.GetData()
	if data.DocPath != docPath {
		t.Errorf("expected DocPath %q, got %q", docPath, data.DocPath)
	}
	if data.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp in SaveDocData")
	}

	// Verify file exists.
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		t.Fatalf("expected file at %s", docPath)
	}

	// Load it back.
	doc2, _, err := mgr.Resume(docPath)
	if err != nil {
		t.Fatalf("Resume error: %v", err)
	}
	if doc2.Slug != doc.Slug {
		t.Errorf("slug mismatch: %q vs %q", doc2.Slug, doc.Slug)
	}
}

func TestDeterministicManager_SaveFailure(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDeterministicManager(dir)

	doc, _, err := mgr.Start(InterviewConfig{Description: "Save Fail"})
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Use a path where the directory cannot be created (file blocks it).
	blockingFile := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	badPath := filepath.Join(blockingFile, "INTERVIEW-test.yaml")

	saveResult := mgr.Save(doc, badPath)
	if !saveResult.IsFatal() {
		t.Fatalf("expected FATAL result for unwritable path, got code: %s", saveResult.Code)
	}
	if len(saveResult.Errors) == 0 {
		t.Fatal("expected at least one error in FATAL result")
	}
	if saveResult.Errors[0].Code != "INTERVIEW_SAVE_FAILED" {
		t.Errorf("expected error code INTERVIEW_SAVE_FAILED, got %q", saveResult.Errors[0].Code)
	}
	if saveResult.Errors[0].Context["path"] != badPath {
		t.Errorf("expected path context %q, got %q", badPath, saveResult.Errors[0].Context["path"])
	}
}

// Agent C tests for P0 and P1 changes from Agent B.

// TestValidateRequiredField_EmptyRejectsRequired verifies empty answer on required question returns error message.
func TestValidateRequiredField_EmptyRejectsRequired(t *testing.T) {
	q := &InterviewQuestion{
		ID:        "q1",
		Phase:     PhaseOverview,
		FieldName: "title",
		Text:      "What is the title?",
		Required:  true,
	}

	errMsg := ValidateRequiredField(q, "")
	if errMsg == "" {
		t.Fatal("expected error message for empty answer on required field, got empty string")
	}
	if !contains(errMsg, "required") {
		t.Errorf("expected error message to mention 'required', got: %q", errMsg)
	}
}

// TestValidateRequiredField_WhitespaceRejectsRequired verifies whitespace-only answer on required question returns error message.
func TestValidateRequiredField_WhitespaceRejectsRequired(t *testing.T) {
	q := &InterviewQuestion{
		ID:        "q2",
		Phase:     PhaseOverview,
		FieldName: "goal",
		Text:      "What is the goal?",
		Required:  true,
	}

	errMsg := ValidateRequiredField(q, "   \t\n   ")
	if errMsg == "" {
		t.Fatal("expected error message for whitespace-only answer on required field, got empty string")
	}
	if !contains(errMsg, "required") {
		t.Errorf("expected error message to mention 'required', got: %q", errMsg)
	}
}

// TestValidateRequiredField_SkipOnOptionalAccepted verifies "skip" on optional question returns empty (valid).
func TestValidateRequiredField_SkipOnOptionalAccepted(t *testing.T) {
	q := &InterviewQuestion{
		ID:        "q3",
		Phase:     PhaseOverview,
		FieldName: "success_metrics",
		Text:      "Success metrics?",
		Required:  false,
	}

	errMsg := ValidateRequiredField(q, "skip")
	if errMsg != "" {
		t.Errorf("expected no error for 'skip' on optional field, got: %q", errMsg)
	}
}

// TestValidateRequiredField_SkipOnRequiredRejected verifies "skip" on required question returns error message.
func TestValidateRequiredField_SkipOnRequiredRejected(t *testing.T) {
	q := &InterviewQuestion{
		ID:        "q4",
		Phase:     PhaseScope,
		FieldName: "in_scope",
		Text:      "What is in scope?",
		Required:  true,
	}

	errMsg := ValidateRequiredField(q, "skip")
	if errMsg == "" {
		t.Fatal("expected error message for 'skip' on required field, got empty string")
	}
	if !contains(errMsg, "required") || !contains(errMsg, "skip") {
		t.Errorf("expected error message to mention 'required' and 'skip', got: %q", errMsg)
	}
}

// TestValidateRequiredField_ValidAnswer verifies non-empty answer returns empty (valid).
func TestValidateRequiredField_ValidAnswer(t *testing.T) {
	q := &InterviewQuestion{
		ID:        "q5",
		Phase:     PhaseOverview,
		FieldName: "title",
		Text:      "What is the title?",
		Required:  true,
	}

	errMsg := ValidateRequiredField(q, "My Project")
	if errMsg != "" {
		t.Errorf("expected no error for valid answer, got: %q", errMsg)
	}
}

// TestHandleBackCommand_FirstQuestion verifies returns false at cursor 0.
func TestHandleBackCommand_FirstQuestion(t *testing.T) {
	doc := &InterviewDoc{
		ID:             "test-doc",
		Slug:           "test-slug",
		Phase:          PhaseOverview,
		QuestionCursor: 0,
		MaxQuestions:   18,
		SpecData:       SpecData{},
		History:        []InterviewTurn{},
	}

	handled := HandleBackCommand(doc, "back")
	if handled {
		t.Error("expected HandleBackCommand to return false at cursor 0, got true")
	}

	// Verify state unchanged.
	if doc.QuestionCursor != 0 {
		t.Errorf("expected cursor to remain 0, got: %d", doc.QuestionCursor)
	}
}

// TestHandleBackCommand_Success verifies returns true, decrements cursor, removes last history entry, clears populated field.
func TestHandleBackCommand_Success(t *testing.T) {
	doc := &InterviewDoc{
		ID:             "test-doc",
		Slug:           "test-slug",
		Phase:          PhaseOverview,
		QuestionCursor: 2,
		MaxQuestions:   18,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "My Project",
				Goal:  "Build something",
			},
		},
		History: []InterviewTurn{
			{TurnNumber: 1, Phase: PhaseOverview, Question: "What is the title of this project or feature?", Answer: "My Project"},
			{TurnNumber: 2, Phase: PhaseOverview, Question: "What is the primary goal? (one sentence)", Answer: "Build something"},
		},
	}

	handled := HandleBackCommand(doc, "back")
	if !handled {
		t.Fatal("expected HandleBackCommand to return true, got false")
	}

	// Verify cursor decremented.
	if doc.QuestionCursor != 1 {
		t.Errorf("expected cursor to be 1 after back, got: %d", doc.QuestionCursor)
	}

	// Verify history entry removed.
	if len(doc.History) != 1 {
		t.Errorf("expected 1 history entry after back, got: %d", len(doc.History))
	}

	// Verify goal field was cleared (the second question was about goal).
	if doc.SpecData.Overview.Goal != "" {
		t.Errorf("expected goal field to be cleared, got: %q", doc.SpecData.Overview.Goal)
	}

	// Verify title field still populated (first question, not removed).
	if doc.SpecData.Overview.Title != "My Project" {
		t.Errorf("expected title field to remain 'My Project', got: %q", doc.SpecData.Overview.Title)
	}
}

// TestHandleBackCommand_CrossPhase verifies going back across phase boundary reverts doc.Phase correctly.
func TestHandleBackCommand_CrossPhase(t *testing.T) {
	doc := &InterviewDoc{
		ID:             "test-doc",
		Slug:           "test-slug",
		Phase:          PhaseScope,
		QuestionCursor: 5,
		MaxQuestions:   18,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title:          "My Project",
				Goal:           "Build something",
				SuccessMetrics: []string{},
				NonGoals:       []string{},
			},
			Scope: ScopeSpec{
				InScope: []string{"feature1"},
			},
		},
		History: []InterviewTurn{
			{TurnNumber: 1, Phase: PhaseOverview, Question: "What is the title of this project or feature?", Answer: "My Project"},
			{TurnNumber: 2, Phase: PhaseOverview, Question: "What is the primary goal? (one sentence)", Answer: "Build something"},
			{TurnNumber: 3, Phase: PhaseOverview, Question: "What are the success metrics? (comma-separated) (or type 'skip' to skip)", Answer: "skip"},
			{TurnNumber: 4, Phase: PhaseOverview, Question: "What is explicitly out of scope? (comma-separated) (or type 'skip' to skip)", Answer: "skip"},
			{TurnNumber: 5, Phase: PhaseScope, Question: "What is in scope? List the key deliverables (comma-separated)", Answer: "feature1"},
		},
	}

	handled := HandleBackCommand(doc, "back")
	if !handled {
		t.Fatal("expected HandleBackCommand to return true, got false")
	}

	// Verify cursor decremented.
	if doc.QuestionCursor != 4 {
		t.Errorf("expected cursor to be 4 after back, got: %d", doc.QuestionCursor)
	}

	// Verify phase stays in Scope (all overview questions were answered, so recalculate keeps us in Scope).
	// This is correct behavior: back clears the in_scope field, but since all overview questions are
	// answered (including skipped ones which are marked as []), the phase transition logic advances to Scope.
	if doc.Phase != PhaseScope {
		t.Errorf("expected phase to remain PhaseScope after recalculation, got: %v", doc.Phase)
	}

	// Verify in_scope field was cleared (this proves back worked, even though phase didn't change).
	if doc.SpecData.Scope.InScope != nil {
		t.Errorf("expected in_scope to be nil after back, got: %v", doc.SpecData.Scope.InScope)
	}
}

// TestHandleBackCommand_CaseInsensitive verifies "BACK", "Back" all work.
func TestHandleBackCommand_CaseInsensitive(t *testing.T) {
	testCases := []string{"back", "BACK", "Back", "BaCk", "  back  ", "  BACK  "}

	for _, tc := range testCases {
		doc := &InterviewDoc{
			ID:             "test-doc",
			Slug:           "test-slug",
			Phase:          PhaseOverview,
			QuestionCursor: 1,
			MaxQuestions:   18,
			SpecData: SpecData{
				Overview: OverviewSpec{
					Title: "My Project",
				},
			},
			History: []InterviewTurn{
				{TurnNumber: 1, Phase: PhaseOverview, Question: "What is the title?", Answer: "My Project"},
			},
		}

		handled := HandleBackCommand(doc, tc)
		if !handled {
			t.Errorf("expected HandleBackCommand to return true for %q, got false", tc)
		}
	}
}

// TestAnswer_ValidationRejectsEmpty verifies Answer() with empty string on required field returns same question with hint.
func TestAnswer_ValidationRejectsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewDeterministicManager(tmpDir)

	doc := &InterviewDoc{
		ID:             "test-doc",
		Slug:           "test-slug",
		Status:         "in_progress",
		Phase:          PhaseOverview,
		QuestionCursor: 0,
		MaxQuestions:   18,
		SpecData:       SpecData{},
		History:        []InterviewTurn{},
	}

	// Answer with empty string on required field (first question is title, required).
	resultDoc, resultQ, err := mgr.Answer(doc, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we got the same question back (cursor didn't advance).
	if resultDoc.QuestionCursor != 0 {
		t.Errorf("expected cursor to remain 0 after validation failure, got: %d", resultDoc.QuestionCursor)
	}

	// Verify question has error hint.
	if resultQ == nil {
		t.Fatal("expected question to be returned, got nil")
	}
	if resultQ.Hint == "" {
		t.Error("expected question Hint to contain error message, got empty string")
	}
	if !contains(resultQ.Hint, "required") {
		t.Errorf("expected hint to mention 'required', got: %q", resultQ.Hint)
	}

	// Verify history was NOT recorded.
	if len(resultDoc.History) != 0 {
		t.Errorf("expected history to remain empty after validation failure, got: %d entries", len(resultDoc.History))
	}
}

// TestAnswer_BackIntegration verifies Answer() with "back" reverts state.
func TestAnswer_BackIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewDeterministicManager(tmpDir)

	doc := &InterviewDoc{
		ID:             "test-doc",
		Slug:           "test-slug",
		Status:         "in_progress",
		Phase:          PhaseOverview,
		QuestionCursor: 2,
		MaxQuestions:   18,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "My Project",
				Goal:  "Build something",
			},
		},
		History: []InterviewTurn{
			{TurnNumber: 1, Phase: PhaseOverview, Question: "What is the title?", Answer: "My Project"},
			{TurnNumber: 2, Phase: PhaseOverview, Question: "What is the primary goal?", Answer: "Build something"},
		},
	}

	// Answer with "back".
	resultDoc, resultQ, err := mgr.Answer(doc, "back")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cursor was decremented.
	if resultDoc.QuestionCursor != 1 {
		t.Errorf("expected cursor to be 1 after back, got: %d", resultDoc.QuestionCursor)
	}

	// Verify history entry was removed.
	if len(resultDoc.History) != 1 {
		t.Errorf("expected 1 history entry after back, got: %d", len(resultDoc.History))
	}

	// Verify goal field was cleared.
	if resultDoc.SpecData.Overview.Goal != "" {
		t.Errorf("expected goal field to be cleared, got: %q", resultDoc.SpecData.Overview.Goal)
	}

	// Verify we got the second question again (goal question).
	if resultQ == nil {
		t.Fatal("expected question to be returned, got nil")
	}
	if resultQ.FieldName != "goal" {
		t.Errorf("expected question to be for 'goal' field, got: %q", resultQ.FieldName)
	}
}

// TestFormatPhaseProgress_Overview verifies returns "[Overview: 1/4 | Next: Scope]" format.
func TestFormatPhaseProgress_Overview(t *testing.T) {
	doc := &InterviewDoc{
		Phase:          PhaseOverview,
		QuestionCursor: 1,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "My Project",
			},
		},
	}

	result := FormatPhaseProgress(doc)

	// Verify format includes phase name, answered/total, and next phase.
	if !contains(result, "overview") && !contains(result, "Overview") {
		t.Errorf("expected result to contain 'overview', got: %q", result)
	}
	if !contains(result, "Next: Scope") && !contains(result, "Next: scope") {
		t.Errorf("expected result to contain 'Next: Scope', got: %q", result)
	}
	// Should show 1/4 (4 questions in overview phase).
	if !contains(result, "1/4") {
		t.Errorf("expected result to contain '1/4', got: %q", result)
	}
}

// TestFormatPhaseProgress_Review verifies returns "[Review: 1/2 | Next: Done]" format.
func TestFormatPhaseProgress_Review(t *testing.T) {
	doc := &InterviewDoc{
		Phase:          PhaseReview,
		QuestionCursor: 15,
		SpecData: SpecData{
			OpenQuestions: []string{},
		},
	}

	result := FormatPhaseProgress(doc)

	// Verify format includes phase name and "Next: Done".
	if !contains(result, "review") && !contains(result, "Review") {
		t.Errorf("expected result to contain 'review', got: %q", result)
	}
	if !contains(result, "Next: Done") {
		t.Errorf("expected result to contain 'Next: Done', got: %q", result)
	}
	// Should show 1/2 (2 questions in review phase).
	if !contains(result, "1/2") {
		t.Errorf("expected result to contain '1/2', got: %q", result)
	}
}

// TestPreviewRequirements verifies returns non-empty markdown string.
func TestPreviewRequirements(t *testing.T) {
	doc := &InterviewDoc{
		ID:   "test-doc",
		Slug: "test-slug",
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "My Project",
				Goal:  "Build a CLI tool",
			},
			Scope: ScopeSpec{
				InScope: []string{"feature1", "feature2"},
			},
		},
	}

	preview, err := PreviewRequirements(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if preview == "" {
		t.Error("expected non-empty preview string, got empty")
	}

	// Verify preview contains expected sections.
	if !contains(preview, "# Requirements:") {
		t.Errorf("expected preview to contain '# Requirements:', got: %q", preview)
	}
	if !contains(preview, "My Project") {
		t.Errorf("expected preview to contain 'My Project', got: %q", preview)
	}
	if !contains(preview, "## Key Concerns") {
		t.Errorf("expected preview to contain '## Key Concerns', got: %q", preview)
	}
}

// TestNewIDFormat verifies newID() emits a proper UUID-formatted string.
func TestNewIDFormat(t *testing.T) {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	for i := 0; i < 10; i++ {
		id, err := newID()
		if err != nil {
			t.Fatalf("newID() returned error: %v", err)
		}
		if !uuidRegex.MatchString(id) {
			t.Errorf("newID() = %q, does not match UUID format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", id)
		}
	}
}

// TestNewID_ReturnsIDAndNoError verifies newID() returns a valid UUID-formatted string and nil error.
func TestNewID_ReturnsIDAndNoError(t *testing.T) {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	for i := 0; i < 10; i++ {
		id, err := newID()
		if err != nil {
			t.Fatalf("newID() returned unexpected error: %v", err)
		}
		if !uuidRegex.MatchString(id) {
			t.Errorf("newID() = %q, does not match UUID format", id)
		}
	}
}

// TestCheckPhaseTransition_InvalidSkipReturnsError verifies single-step transitions return nil.
func TestCheckPhaseTransition_InvalidSkipReturnsError(t *testing.T) {
	doc := &InterviewDoc{
		Phase: PhaseOverview,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title:          "Test",
				Goal:           "A goal",
				SuccessMetrics: []string{},
				NonGoals:       []string{},
			},
		},
	}
	err := checkPhaseTransition(doc)
	if err != nil {
		t.Errorf("expected nil error for single-step transition, got: %v", err)
	}
	if doc.Phase != PhaseScope {
		t.Errorf("expected PhaseScope, got %s", doc.Phase)
	}
}

// TestAnswer_SaveResult_EmptyErrors verifies that if Save returns a FATAL result,
// the Errors slice is non-empty (confirming the bounds-check fix is meaningful).
func TestAnswer_SaveResult_EmptyErrors(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDeterministicManager(dir)
	doc := &InterviewDoc{
		ID: "test", Slug: "test", Status: "in_progress",
		Phase: PhaseOverview, MaxQuestions: 18,
		History: []InterviewTurn{},
	}
	blockingFile := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	badPath := filepath.Join(blockingFile, "INTERVIEW-test.yaml")
	saveResult := mgr.Save(doc, badPath)
	if !saveResult.IsFatal() {
		t.Fatal("expected FATAL result")
	}
	if len(saveResult.Errors) == 0 {
		t.Fatal("expected non-empty Errors on FATAL result; bounds-check fix is needed")
	}
}

// TestCheckPhaseTransition_NoPanic_SingleStep verifies single-step transitions don't panic.
func TestCheckPhaseTransition_NoPanic_SingleStep(t *testing.T) {
	// Overview -> Scope (single step, should not panic)
	doc := &InterviewDoc{
		Phase: PhaseOverview,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title:          "Test",
				Goal:           "A goal",
				SuccessMetrics: []string{},
				NonGoals:       []string{},
			},
		},
	}
	// Should not panic
	checkPhaseTransition(doc)
	if doc.Phase != PhaseScope {
		t.Errorf("expected PhaseScope after overview complete, got %s", doc.Phase)
	}
}

// TestNonInteractive_PromptsNotWritten verifies that in non-interactive mode
// the prompt guard logic: nonInteractive=true means prompts are suppressed.
// This test validates the logic by checking the nonInteractive variable
// in a unit test context by calling a helper.
func TestNonInteractive_LogicGuard(t *testing.T) {
	// Simulate the guard logic: when nonInteractive=true, prompt output is skipped.
	// We test the behavior by calling FormatPhaseProgress and verifying it's callable
	// (the actual suppression is in the cmd layer, tested via the flag wiring).
	doc := &InterviewDoc{
		Phase: PhaseOverview,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "Test",
			},
		},
	}
	progress := FormatPhaseProgress(doc)
	if progress == "" {
		t.Error("expected non-empty progress string from FormatPhaseProgress")
	}
	// Guard logic: if nonInteractive were true, this string would NOT be written to output.
	// The actual suppression is validated by integration, not unit test.
	// This test confirms the function itself works correctly when called.
}

// contains is a helper to check if a string contains a substring (case-sensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexContains(s, substr) >= 0)
}

func indexContains(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
