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
)

const (
	defaultModel     = "claude-haiku-4-5"
	defaultThreshold = 70
	maxDiffBytes     = 50000
)

// testBaseURL overrides the Anthropic API base URL when non-empty.
// Set this in tests to point at a mock httptest.Server.
var testBaseURL string

// reviewResponse is the JSON shape we ask the LLM to produce.
type reviewResponse struct {
	Dimensions []DimensionScore `json:"dimensions"`
	Overall    int              `json:"overall"`
	Summary    string           `json:"summary"`
}

// ReviewDiff submits the merged diff to the LLM and returns a ReviewResult.
// Returns error if the API call fails or response cannot be parsed.
// Returns a zero ReviewResult (Passed=true) when diff is empty.
func ReviewDiff(ctx context.Context, diff string, opts ReviewOpts) (*ReviewResult, error) {
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
		return &ReviewResult{
			Dimensions: []DimensionScore{},
			Overall:    0,
			Passed:     true,
			Summary:    "No changes to review.",
			Model:      opts.Model,
			DiffBytes:  0,
		}, nil
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set; code review requires API access")
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
		userMsg += "\n\n[diff truncated to 50000 bytes]"
	}

	clientOpts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if testBaseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(testBaseURL))
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
		return nil, fmt.Errorf("anthropic API error: %w", err)
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
		return nil, fmt.Errorf("failed to parse review response as JSON: %w\nraw response: %s", err, rawText)
	}

	result := &ReviewResult{
		Dimensions: parsed.Dimensions,
		Overall:    parsed.Overall,
		Passed:     parsed.Overall >= opts.Threshold,
		Summary:    parsed.Summary,
		Model:      opts.Model,
		DiffBytes:  diffBytes,
		Truncated:  truncated,
	}

	return result, nil
}

// RunCodeReview runs the AI code review gate for the given repo after merge.
// Returns (nil, nil) when code review is disabled (cfg.Enabled == false).
// Gets the diff via git diff HEAD~1..HEAD; falls back to git show HEAD if
// the repo has fewer than two commits.
func RunCodeReview(ctx context.Context, repoPath string, cfg CodeReviewConfig) (*ReviewResult, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Try HEAD~1..HEAD first.
	diffCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "HEAD~1..HEAD")
	diffOut, err := diffCmd.Output()
	if err != nil {
		// Fall back to git show HEAD (works even with a single commit).
		showCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "show", "HEAD")
		diffOut, err = showCmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get diff for code review: %w", err)
		}
	}

	reviewOpts := ReviewOpts{
		Model:      cfg.Model,
		Threshold:  cfg.Threshold,
		Dimensions: cfg.Dimensions,
	}

	return ReviewDiff(ctx, string(diffOut), reviewOpts)
}
