package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/interview"
	"github.com/spf13/cobra"
)

func init() {
	newDeterministicManagerFunc = func(docsDir string) interview.Manager {
		return interview.NewDeterministicManager(docsDir)
	}
}

// newInterviewCmd creates the interview subcommand.
// Usage: sawtools interview "<description>" [flags]
//
// Drives the AskUserQuestion interaction loop: prints each question to stdout,
// reads stdin for each answer, writes state to INTERVIEW-<slug>.yaml after each
// turn. On completion writes REQUIREMENTS.md.
func newInterviewCmd() *cobra.Command {
	var (
		mode           string
		maxQuestions   int
		projectPath    string
		resumePath     string
		outputPath     string
		docsDir        string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   `interview "<description>"`,
		Short: "Conduct a structured requirements interview",
		Long: `Conduct a structured requirements interview that produces a REQUIREMENTS.md
file suitable for /saw bootstrap or /saw scout.

The interview walks through 6 phases: overview, scope, requirements, interfaces,
stories, and review. Each phase collects structured data that is compiled into
a complete requirements document.

Examples:
  sawtools interview "Build a REST API for user management"
  sawtools interview "Add OAuth2 support" --max-questions 12
  sawtools interview --resume docs/INTERVIEW-my-feature.yaml
  echo "My App\nA CLI tool\n..." | sawtools interview "test" --non-interactive`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate: need either a description or --resume
			if len(args) == 0 && resumePath == "" {
				return fmt.Errorf("requires a description argument or --resume flag")
			}

			description := ""
			if len(args) > 0 {
				description = args[0]
			}

			// Build the manager
			mgr := buildManager(mode, docsDir, cmd.ErrOrStderr())

			var (
				doc      *interview.InterviewDoc
				question *interview.InterviewQuestion
			)

			if resumePath != "" {
				// Resume existing interview
				absResume, absErr := filepath.Abs(resumePath)
				if absErr != nil {
					return fmt.Errorf("invalid resume path: %w", absErr)
				}
				if _, statErr := os.Stat(absResume); os.IsNotExist(statErr) {
					return fmt.Errorf("resume file not found: %s", absResume)
				}
				resumeResult := mgr.Resume(absResume)
				if resumeResult.IsFatal() {
					return fmt.Errorf("failed to resume interview: %s", resumeResult.Errors[0].Message)
				}
				resumeData := resumeResult.GetData()
				doc = resumeData.Doc
				question = resumeData.Question
			} else {
				// Start new interview
				cfg := interview.InterviewConfig{
					Description:  description,
					Mode:         interview.InterviewMode(mode),
					MaxQuestions: maxQuestions,
					ProjectPath:  projectPath,
				}
				startResult := mgr.Start(cfg)
				if startResult.IsFatal() {
					return fmt.Errorf("failed to start interview: %s", startResult.Errors[0].Message)
				}
				startData := startResult.GetData()
				doc = startData.Doc
				question = startData.Question
			}

			// Resolve output path
			if outputPath == "" {
				outputPath = filepath.Join(docsDir, "REQUIREMENTS.md")
			}

			// Run the question-answer loop
			reader := bufio.NewScanner(cmd.InOrStdin())
			writer := cmd.OutOrStdout()

			for question != nil {
				if !nonInteractive {
					// Show preview before final confirmation
					if question.FieldName == "_confirm" {
						previewResult := interview.CompileToRequirements(doc)
						if !previewResult.IsFatal() && previewResult.Data != nil {
							fmt.Fprintf(writer, "\n--- Preview of REQUIREMENTS.md ---\n%s\n", *previewResult.Data)
						}
					}

					// Print the question prompt with phase-aware progress
					fmt.Fprintf(writer, "%s\n", interview.FormatPhaseProgress(doc))
					fmt.Fprintf(writer, "%s\n", question.Text)
					if question.Hint != "" {
						fmt.Fprintf(writer, "(%s)\n", question.Hint)
					}
					fmt.Fprint(writer, "> ")
				}

				// Read the answer from stdin
				if !reader.Scan() {
					// stdin closed before interview complete — save state and return error
					docPath := interviewDocPath(docsDir, doc.Slug)
					if saveResult := mgr.Save(doc, docPath); saveResult.IsFatal() {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save interview state: %v\n", saveResult.Errors)
					}
					fmt.Fprintf(writer, "\nInterview paused. Resume with:\n  sawtools interview --resume %s\n", docPath)
					return fmt.Errorf("interview paused: resume with --resume %s", docPath)
				}
				answer := reader.Text()

				// Record the answer and get next question
				ansResult := mgr.Answer(doc, answer)
				if ansResult.IsFatal() {
					return fmt.Errorf("failed to record answer: %s", ansResult.Errors[0].Message)
				}
				ansData := ansResult.GetData()
				doc = ansData.Doc
				question = ansData.Question

				// Save state after each turn
				docPath := interviewDocPath(docsDir, doc.Slug)
				if saveResult := mgr.Save(doc, docPath); saveResult.IsFatal() {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save interview state: %v\n", saveResult.Errors)
				}
			}

			// Interview complete — compile to REQUIREMENTS.md
			compileResult := mgr.Compile(doc, outputPath)
			if compileResult.IsFatal() {
				return fmt.Errorf("failed to compile requirements: %s", compileResult.Errors[0].Message)
			}
			outPath := compileResult.GetData().OutputPath

			// Save final state
			docPath := interviewDocPath(docsDir, doc.Slug)
			if saveResult := mgr.Save(doc, docPath); saveResult.IsFatal() {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save final interview state: %v\n", saveResult.Errors)
			}

			// Print completion message
			fmt.Fprintf(writer, "\nInterview complete. (%d/%d questions)\n", len(doc.History), doc.MaxQuestions)
			fmt.Fprintf(writer, "REQUIREMENTS.md written to: %s\n", outPath)
			fmt.Fprintf(writer, "Interview doc saved to: %s\n", docPath)
			fmt.Fprintf(writer, "Next step: /saw bootstrap or /saw scout\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "deterministic", "Question mode: deterministic or llm")
	cmd.Flags().IntVar(&maxQuestions, "max-questions", 18, "Soft cap on total questions")
	cmd.Flags().StringVar(&projectPath, "project-path", "", "Optional path to existing project for context")
	cmd.Flags().StringVar(&resumePath, "resume", "", "Path to an existing INTERVIEW-<slug>.yaml to resume")
	cmd.Flags().StringVar(&outputPath, "output", "", "Path for output REQUIREMENTS.md (default docs/REQUIREMENTS.md)")
	cmd.Flags().StringVar(&docsDir, "docs-dir", "docs", "Directory to write INTERVIEW doc")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Read answers from stdin without prompts (for testing/piping)")

	return cmd
}

// newDeterministicManagerFunc is the factory for creating DeterministicManager instances.
// It is a package-level variable to allow test injection. After Agent A's merge,
// the Integration Agent wires this to interview.NewDeterministicManager.
// Tests override this via installMockManager in interview_cmd_test.go.
var newDeterministicManagerFunc func(docsDir string) interview.Manager

// buildManager constructs the appropriate Manager implementation.
func buildManager(mode string, docsDir string, stderr io.Writer) interview.Manager {
	if mode == "llm" {
		fmt.Fprintln(stderr, "LLM mode not yet implemented, falling back to deterministic")
	}
	if newDeterministicManagerFunc == nil {
		panic("interview: newDeterministicManagerFunc not initialized — wire interview.NewDeterministicManager in init()")
	}
	return newDeterministicManagerFunc(docsDir)
}

// interviewDocPath returns the file path for an interview YAML doc.
func interviewDocPath(docsDir, slug string) string {
	if slug == "" {
		slug = "untitled"
	}
	return filepath.Join(docsDir, fmt.Sprintf("INTERVIEW-%s.yaml", slug))
}
