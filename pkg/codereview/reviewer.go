package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

const (
	defaultModel     = "claude-haiku-4-5"
	defaultThreshold = 70
	maxDiffBytes     = 50000
)

// reviewResponse is the JSON shape we ask the LLM to produce.
type reviewResponse struct {
	Dimensions []DimensionScore `json:"dimensions"`
	Overall    int              `json:"overall"`
	Summary    string           `json:"summary"`
}

// ReviewDiff submits the merged diff to the LLM and returns a ReviewResult.
// Returns a failure result if the API call fails or response cannot be parsed.
// Returns a success result with Skipped=true when diff is empty.
func ReviewDiff(ctx context.Context, diff string, opts ReviewOpts) result.Result[ReviewResult] {
	// Apply defaults.
	if opts.Model == "" {
		opts.Model = defaultModel
	}
	if opts.Threshold == 0 {
		opts.Threshold = defaultThreshold
	}
	dims := opts.Dimensions
	if len(dims) == 0 {
		dims = AllDimensions
	}

	// Empty diff — nothing to review.
	if diff == "" {
		return result.NewSuccess(ReviewResult{
			Dimensions: []DimensionScore{},
			Overall:    100,
			Passed:     true,
			Skipped:    true,
			Summary:    "No changes to review.",
			Model:      opts.Model,
			DiffBytes:  0,
		})
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return result.NewFailure[ReviewResult]([]result.SAWError{
			result.NewFatal("CODEREVIEW_ERROR", "ANTHROPIC_API_KEY not set; code review requires API access"),
		})
	}

	// Truncate large diffs.
	truncated := false
	diffBytes := len(diff)
	if diffBytes > maxDiffBytes {
		diff = diff[:maxDiffBytes]
		truncated = true
	}

	// Build system prompt.
	dimList := strings.Join(dims, ", ")
	systemPrompt := fmt.Sprintf(`You are a senior code reviewer. You will be given a git diff and must score it across the following dimensions: %s.

For each dimension, provide a score from 0 to 100 and a 1-2 sentence rationale.
Also provide an overall score (0-100, weighted average) and a 1-3 sentence summary.

Respond with valid JSON only, in exactly this schema:
{
  "dimensions": [
    {"name": "dimension_name", "score": 85, "rationale": "..."},
    ...
  ],
  "overall": 82,
  "summary": "..."
}

Do not include any text outside the JSON object.`, dimList)

	// Build user message.
	userMsg := fmt.Sprintf("Please review the following git diff:\n\n%s", diff)
	if truncated {
		userMsg += fmt.Sprintf("\n\n[diff truncated to %d bytes]", maxDiffBytes)
	}

	clientOpts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if opts.BaseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(opts.BaseURL))
	}
	client := anthropic.NewClient(clientOpts...)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(opts.Model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
		},
	}

	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		return result.NewFailure[ReviewResult]([]result.SAWError{
			result.NewFatal("CODEREVIEW_API_ERROR",
				fmt.Sprintf("anthropic API error (model=%s, diffBytes=%d): %s", opts.Model, len(diff), err.Error())).WithCause(err),
		})
	}

	// Extract text content.
	var responseText strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			responseText.WriteString(block.AsText().Text)
		}
	}

	// Parse JSON response.
	var parsed reviewResponse
	rawText := strings.TrimSpace(responseText.String())
	if err := json.Unmarshal([]byte(rawText), &parsed); err != nil {
		return result.NewFailure[ReviewResult]([]result.SAWError{
			result.NewFatal("CODEREVIEW_ERROR",
				fmt.Sprintf("failed to parse review response as JSON: %v\nraw response: %s", err, rawText)).WithCause(err),
		})
	}

	if err := validateReviewResponse(parsed); err != nil {
		return result.NewFailure[ReviewResult]([]result.SAWError{
			result.NewFatal("CODEREVIEW_INVALID_RESPONSE", err.Error()),
		})
	}

	res := ReviewResult{
		Dimensions: parsed.Dimensions,
		Overall:    parsed.Overall,
		Passed:     parsed.Overall >= opts.Threshold,
		Summary:    parsed.Summary,
		Model:      opts.Model,
		DiffBytes:  diffBytes,
		Truncated:  truncated,
	}

	return result.NewSuccess(res)
}

// RunCodeReview runs the AI code review gate for the given repo after merge.
// Returns a success result with Skipped=true when code review is disabled (cfg.Enabled == false).
// Gets the diff via git diff HEAD~1..HEAD; falls back to git show HEAD if
// the repo has fewer than two commits.
func RunCodeReview(ctx context.Context, repoPath string, cfg CodeReviewConfig) result.Result[ReviewResult] {
	if !cfg.Enabled {
		return result.NewSuccess(ReviewResult{Skipped: true})
	}

	// Try HEAD~1..HEAD first.
	diffCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "HEAD~1..HEAD")
	diffOut, firstErr := diffCmd.Output()
	if firstErr != nil {
		// Fall back to git show HEAD (works even with a single commit).
		showCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "show", "HEAD")
		var fallbackErr error
		diffOut, fallbackErr = showCmd.Output()
		if fallbackErr != nil {
			return result.NewFailure[ReviewResult]([]result.SAWError{
				result.NewFatal("CODEREVIEW_GIT_FAILED",
					fmt.Sprintf("failed to get diff for code review: %v (fallback error: %v)", firstErr, fallbackErr)),
			})
		}
	}

	reviewOpts := ReviewOpts{
		Model:      cfg.Model,
		Threshold:  cfg.Threshold,
		Dimensions: cfg.Dimensions,
	}

	return ReviewDiff(ctx, string(diffOut), reviewOpts)
}

// validateReviewResponse validates the parsed LLM response structure.
// It checks that Dimensions is non-empty, each dimension score is in [0,100],
// and Summary is non-empty.
func validateReviewResponse(parsed reviewResponse) error {
	if len(parsed.Dimensions) == 0 {
		return fmt.Errorf("review response has no dimensions")
	}
	for _, d := range parsed.Dimensions {
		if d.Score < 0 || d.Score > 100 {
			return fmt.Errorf("dimension %q score %d out of range [0,100]", d.Name, d.Score)
		}
	}
	if parsed.Summary == "" {
		return fmt.Errorf("review response has empty summary")
	}
	return nil
}
