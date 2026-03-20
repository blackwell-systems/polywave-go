package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/interview"
	"github.com/spf13/cobra"
)

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
				err      error
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
				doc, question, err = mgr.Resume(absResume)
				if err != nil {
					return fmt.Errorf("failed to resume interview: %w", err)
				}
			} else {
				// Start new interview
				cfg := interview.InterviewConfig{
					Description: description,
					Mode:        interview.InterviewMode(mode),
					MaxQuestions: maxQuestions,
					ProjectPath: projectPath,
				}
				doc, question, err = mgr.Start(cfg)
				if err != nil {
					return fmt.Errorf("failed to start interview: %w", err)
				}
			}

			// Resolve output path
			if outputPath == "" {
				outputPath = filepath.Join(docsDir, "REQUIREMENTS.md")
			}

			// Run the question-answer loop
			reader := bufio.NewScanner(cmd.InOrStdin())
			writer := cmd.OutOrStdout()

			for question != nil {
				// Print the question prompt
				pct := 0
				if doc.MaxQuestions > 0 {
					pct = int(float64(doc.QuestionCursor) / float64(doc.MaxQuestions) * 100)
				}
				fmt.Fprintf(writer, "[Phase: %s | Q %d/%d | %d%%]\n",
					capitalize(string(question.Phase)),
					doc.QuestionCursor+1,
					doc.MaxQuestions,
					pct,
				)
				fmt.Fprintf(writer, "%s\n", question.Text)
				if question.Hint != "" {
					fmt.Fprintf(writer, "(%s)\n", question.Hint)
				}
				fmt.Fprint(writer, "> ")

				// Read the answer from stdin
				if !reader.Scan() {
					// stdin closed before interview complete — save state and exit 2
					docPath := interviewDocPath(docsDir, doc.Slug)
					if saveErr := mgr.Save(doc, docPath); saveErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save interview state: %v\n", saveErr)
					}
					fmt.Fprintf(writer, "\nInterview paused. Resume with:\n  sawtools interview --resume %s\n", docPath)
					os.Exit(2)
				}
				answer := reader.Text()

				// Record the answer and get next question
				doc, question, err = mgr.Answer(doc, answer)
				if err != nil {
					return fmt.Errorf("failed to record answer: %w", err)
				}

				// Save state after each turn
				docPath := interviewDocPath(docsDir, doc.Slug)
				if saveErr := mgr.Save(doc, docPath); saveErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save interview state: %v\n", saveErr)
				}
			}

			// Interview complete — compile to REQUIREMENTS.md
			outPath, compileErr := mgr.Compile(doc, outputPath)
			if compileErr != nil {
				return fmt.Errorf("failed to compile requirements: %w", compileErr)
			}

			// Save final state
			docPath := interviewDocPath(docsDir, doc.Slug)
			if saveErr := mgr.Save(doc, docPath); saveErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to save final interview state: %v\n", saveErr)
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

// capitalize returns the string with its first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}
