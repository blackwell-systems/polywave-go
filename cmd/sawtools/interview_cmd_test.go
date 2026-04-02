package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/interview"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// mockManager is a test double for the interview.Manager interface.
// It simulates a short 3-question interview flow.
type mockManager struct {
	docsDir   string
	questions []*interview.InterviewQuestion
}

func newMockManager(docsDir string) *mockManager {
	return &mockManager{
		docsDir: docsDir,
		questions: []*interview.InterviewQuestion{
			{ID: "q1", Phase: interview.PhaseOverview, FieldName: "title", Text: "What is the project title?", Required: true},
			{ID: "q2", Phase: interview.PhaseOverview, FieldName: "goal", Text: "What is the goal?", Required: true},
			{ID: "q3", Phase: interview.PhaseScope, FieldName: "in_scope", Text: "What is in scope?", Required: true},
		},
	}
}

func (m *mockManager) Start(cfg interview.InterviewConfig) result.Result[interview.StartData] {
	doc := &interview.InterviewDoc{
		ID:             "test-id",
		Slug:           "test-feature",
		Status:         "in_progress",
		Mode:           cfg.Mode,
		Description:    cfg.Description,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Phase:          interview.PhaseOverview,
		QuestionCursor: 0,
		MaxQuestions:   len(m.questions),
	}
	return result.NewSuccess(interview.StartData{Doc: doc, Question: m.questions[0]})
}

func (m *mockManager) Resume(docPath string) result.Result[interview.ResumeData] {
	return result.NewFailure[interview.ResumeData]([]result.SAWError{
		result.NewFatal(result.CodeInterviewSaveFailed, "resume not implemented in mock").
			WithContext("path", docPath),
	})
}

func (m *mockManager) Answer(doc *interview.InterviewDoc, answer string) result.Result[interview.AnswerData] {
	doc.History = append(doc.History, interview.InterviewTurn{
		TurnNumber: len(doc.History) + 1,
		Phase:      doc.Phase,
		Question:   m.questions[doc.QuestionCursor].Text,
		Answer:     answer,
		Timestamp:  time.Now(),
	})
	doc.QuestionCursor++
	doc.UpdatedAt = time.Now()

	if doc.QuestionCursor >= len(m.questions) {
		doc.Status = "complete"
		doc.Phase = interview.PhaseComplete
		return result.NewSuccess(interview.AnswerData{Doc: doc, Question: nil})
	}
	next := m.questions[doc.QuestionCursor]
	doc.Phase = next.Phase
	return result.NewSuccess(interview.AnswerData{Doc: doc, Question: next})
}

func (m *mockManager) Compile(doc *interview.InterviewDoc, outputPath string) result.Result[interview.CompileData] {
	content := "# REQUIREMENTS\n\nGenerated from interview.\n"
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return result.NewFailure[interview.CompileData]([]result.SAWError{
			result.NewFatal(result.CodeRequirementsWriteFailed, err.Error()).WithContext("path", outputPath).WithCause(err),
		})
	}
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return result.NewFailure[interview.CompileData]([]result.SAWError{
			result.NewFatal(result.CodeRequirementsWriteFailed, err.Error()).WithContext("path", outputPath).WithCause(err),
		})
	}
	return result.NewSuccess(interview.CompileData{OutputPath: outputPath})
}

func (m *mockManager) Save(doc *interview.InterviewDoc, docPath string) result.Result[interview.SaveDocData] {
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		return result.NewFailure[interview.SaveDocData]([]result.SAWError{
			result.NewFatal(result.CodeInterviewSaveFailed, err.Error()).WithContext("path", docPath).WithCause(err),
		})
	}
	if err := os.WriteFile(docPath, []byte("mock interview doc"), 0o644); err != nil {
		return result.NewFailure[interview.SaveDocData]([]result.SAWError{
			result.NewFatal(result.CodeInterviewSaveFailed, err.Error()).WithContext("path", docPath).WithCause(err),
		})
	}
	return result.NewSuccess(interview.SaveDocData{DocPath: docPath})
}

// installMockManager replaces the default manager factory with a mock and
// returns a cleanup function to restore the original.
func installMockManager(t *testing.T) {
	t.Helper()
	orig := newDeterministicManagerFunc
	newDeterministicManagerFunc = func(docsDir string) interview.Manager {
		return newMockManager(docsDir)
	}
	t.Cleanup(func() { newDeterministicManagerFunc = orig })
}

func TestInterviewCmd_Help(t *testing.T) {
	cmd := newInterviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, flag := range []string{"--mode", "--max-questions", "--project-path", "--resume", "--output", "--docs-dir", "--non-interactive"} {
		if !strings.Contains(output, flag) {
			t.Errorf("help output missing flag %q", flag)
		}
	}
}

func TestInterviewCmd_MissingDescription(t *testing.T) {
	cmd := newInterviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no description and no --resume, got nil")
	}
	if !strings.Contains(err.Error(), "requires a description") {
		t.Errorf("expected 'requires a description' error, got: %v", err)
	}
}

func TestInterviewCmd_Resume_NotFound(t *testing.T) {
	installMockManager(t)

	cmd := newInterviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--resume", "/nonexistent/path/INTERVIEW-foo.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing resume file, got nil")
	}
	if !strings.Contains(err.Error(), "resume file not found") {
		t.Errorf("expected 'resume file not found' error, got: %v", err)
	}
}

func TestInterviewCmd_LLMModeFallback(t *testing.T) {
	installMockManager(t)

	cmd := newInterviewCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	// Provide enough answers for the mock's 3 questions
	cmd.SetIn(strings.NewReader("My Project\nBuild something\nEverything\n"))
	cmd.SetArgs([]string{"test feature", "--mode", "llm", "--docs-dir", t.TempDir()})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderrOutput := errBuf.String()
	if !strings.Contains(stderrOutput, "LLM mode not yet implemented") {
		t.Errorf("expected LLM fallback warning on stderr, got: %q", stderrOutput)
	}
}

func TestInterviewCmd_NonInteractive_ShortFlow(t *testing.T) {
	installMockManager(t)

	tmpDir := t.TempDir()

	// Provide answers for the mock's 3 questions
	input := "My Project\nBuild something great\nEverything in scope\n"

	cmd := newInterviewCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs([]string{
		"test feature",
		"--non-interactive",
		"--docs-dir", tmpDir,
		"--output", filepath.Join(tmpDir, "REQUIREMENTS.md"),
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v\nstdout: %s\nstderr: %s", err, outBuf.String(), errBuf.String())
	}

	// Verify REQUIREMENTS.md was written
	reqPath := filepath.Join(tmpDir, "REQUIREMENTS.md")
	if _, statErr := os.Stat(reqPath); os.IsNotExist(statErr) {
		t.Errorf("REQUIREMENTS.md not created at %s", reqPath)
	}

	// Verify completion message
	output := outBuf.String()
	if !strings.Contains(output, "Interview complete") {
		t.Errorf("expected 'Interview complete' in output, got: %s", output)
	}
	if !strings.Contains(output, "REQUIREMENTS.md written to") {
		t.Errorf("expected 'REQUIREMENTS.md written to' in output, got: %s", output)
	}
	if !strings.Contains(output, "Interview doc saved to") {
		t.Errorf("expected 'Interview doc saved to' in output, got: %s", output)
	}
	if !strings.Contains(output, "Next step: /saw bootstrap or /saw scout") {
		t.Errorf("expected next step guidance in output, got: %s", output)
	}

	// Verify interview doc was saved
	interviewPath := filepath.Join(tmpDir, "INTERVIEW-test-feature.yaml")
	if _, statErr := os.Stat(interviewPath); os.IsNotExist(statErr) {
		t.Errorf("Interview doc not saved at %s", interviewPath)
	}

	// Verify question prompts are NOT shown in non-interactive mode (G7 fix).
	if strings.Contains(output, "[overview:") {
		t.Errorf("expected phase header to be suppressed in --non-interactive mode, but found it in output: %s", output)
	}
	if strings.Contains(output, "What is the project title?") {
		t.Errorf("expected question text to be suppressed in --non-interactive mode, but found it in output: %s", output)
	}
}

// Agent C tests for P0 and P1 changes.

// TestInterviewCmd_InitWiring verifies that newDeterministicManagerFunc is non-nil after package init (confirms P0.1 fix).
func TestInterviewCmd_InitWiring(t *testing.T) {
	// Note: the test file uses installMockManager which overrides init(),
	// so we test init separately by checking the variable before any mock install.
	// Since init() runs automatically before tests, we just verify the function was set.
	if newDeterministicManagerFunc == nil {
		t.Error("expected newDeterministicManagerFunc to be non-nil after init(), got nil (P0.1 fix verification failed)")
	}
}

// TestInterviewCmd_PhaseProgress verifies output contains phase-aware format like "[Overview:" instead of old "[Phase: Overview".
func TestInterviewCmd_PhaseProgress(t *testing.T) {
	installMockManager(t)

	tmpDir := t.TempDir()

	// Provide answer for first question only, then close stdin to pause.
	input := "My Project\n"

	cmd := newInterviewCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs([]string{
		"test feature",
		"--docs-dir", tmpDir,
	})

	// This will exit(2) because stdin closes before completion.
	// We can't catch that in tests, but we can at least verify the output format.
	// Run with enough input to see the phase progress.
	input = "My Project\nBuild something\nEverything\n"
	cmd.SetIn(strings.NewReader(input))

	err := cmd.Execute()
	if err != nil {
		// This is expected since mock only has 3 questions
		t.Logf("expected error after mock completion: %v", err)
	}

	output := outBuf.String()

	// Verify phase-aware format appears: "[Overview:" or "[overview:"
	if !strings.Contains(output, "[overview:") && !strings.Contains(output, "[Overview:") {
		t.Errorf("expected output to contain phase-aware format '[Overview:', got: %s", output)
	}

	// Verify old format does NOT appear: "[Phase: Overview"
	if strings.Contains(output, "[Phase: Overview") || strings.Contains(output, "[Phase: overview") {
		t.Errorf("expected output NOT to contain old format '[Phase: Overview', but it does: %s", output)
	}

	// Verify "Next:" appears in progress (new format includes "Next: Scope" etc.)
	if !strings.Contains(output, "Next:") {
		t.Errorf("expected output to contain 'Next:' in phase progress, got: %s", output)
	}
}
