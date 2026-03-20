package interview

import (
	"os"
	"path/filepath"
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
	err = mgr.Save(doc, docPath)
	if err != nil {
		t.Fatalf("Save error: %v", err)
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
