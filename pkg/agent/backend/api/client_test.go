package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

// TestNew_ExplicitAPIKey verifies that New() with an explicit apiKey stores it
// and does not fall back to the environment variable.
func TestNew_ExplicitAPIKey(t *testing.T) {
	key := "sk-test-explicit-key"
	client := New(key, backend.Config{})
	if client.apiKey != key {
		t.Errorf("expected apiKey %q, got %q", key, client.apiKey)
	}
}

// TestNew_DefaultModel verifies that New() defaults to claude-sonnet-4-5 when
// cfg.Model is empty.
func TestNew_DefaultModel(t *testing.T) {
	client := New("test-key", backend.Config{})
	if client.model != defaultModel {
		t.Errorf("expected model %q, got %q", defaultModel, client.model)
	}
}

// TestNew_CustomModel verifies that New() uses cfg.Model when provided.
func TestNew_CustomModel(t *testing.T) {
	client := New("test-key", backend.Config{Model: "claude-opus-4-6"})
	if client.model != "claude-opus-4-6" {
		t.Errorf("expected model %q, got %q", "claude-opus-4-6", client.model)
	}
}

// TestNew_DefaultMaxTokens verifies that New() defaults MaxTokens to 8096.
func TestNew_DefaultMaxTokens(t *testing.T) {
	client := New("test-key", backend.Config{})
	if client.maxTokens != defaultMaxTokens {
		t.Errorf("expected maxTokens %d, got %d", defaultMaxTokens, client.maxTokens)
	}
}

// TestNew_DefaultMaxTurns verifies that New() defaults MaxTurns to 50.
func TestNew_DefaultMaxTurns(t *testing.T) {
	client := New("test-key", backend.Config{})
	if client.maxTurns != defaultMaxTurns {
		t.Errorf("expected maxTurns %d, got %d", defaultMaxTurns, client.maxTurns)
	}
}

// mockAnthropicResponse builds a non-streaming Anthropic Messages API JSON response.
func mockAnthropicResponse(stopReason string, content []map[string]interface{}) []byte {
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-5",
		"stop_reason": stopReason,
		"content":     content,
		"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
	}
	data, _ := json.Marshal(resp)
	return data
}

// TestRun_MultiTurnToolExecution verifies that Run executes tool calls over
// multiple turns: the first response requests a tool_use, the second returns
// end_turn with the final assistant text.
func TestRun_MultiTurnToolExecution(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// First turn: tool_use stop reason with a tool_use content block.
			toolInput, _ := json.Marshal(map[string]string{"command": "echo hello"})
			content := []map[string]interface{}{
				{
					"type": "tool_use",
					"id":   "toolu_01",
					"name": "Bash",
					"input": json.RawMessage(toolInput),
				},
			}
			w.Write(mockAnthropicResponse("tool_use", content))
		} else {
			// Second turn: end_turn with text.
			content := []map[string]interface{}{
				{
					"type": "text",
					"text": "Tool executed successfully.",
				},
			}
			w.Write(mockAnthropicResponse("end_turn", content))
		}
	}))
	defer srv.Close()

	client := New("test-key", backend.Config{}).WithBaseURL(srv.URL)
	result, err := client.Run(context.Background(), "system prompt", "run a command", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Tool executed successfully." {
		t.Errorf("expected %q, got %q", "Tool executed successfully.", result)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (tool_use + end_turn), got %d", callCount)
	}
}

// TestRun_SingleTurnEndTurn verifies that Run returns the text immediately
// when the first response has stop_reason=end_turn.
func TestRun_SingleTurnEndTurn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		content := []map[string]interface{}{
			{"type": "text", "text": "Hello!"},
		}
		w.Write(mockAnthropicResponse("end_turn", content))
	}))
	defer srv.Close()

	client := New("test-key", backend.Config{}).WithBaseURL(srv.URL)
	result, err := client.Run(context.Background(), "", "hi", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello!" {
		t.Errorf("expected %q, got %q", "Hello!", result)
	}
}

// TestDedupStats_NilBeforeRun verifies that DedupStats returns nil before any run.
func TestDedupStats_NilBeforeRun(t *testing.T) {
	client := New("test-key", backend.Config{})
	if stats := client.DedupStats(); stats != nil {
		t.Errorf("expected nil DedupStats before run, got %+v", stats)
	}
}

// TestCommitCount_ZeroBeforeRun verifies that CommitCount returns 0 before any run.
func TestCommitCount_ZeroBeforeRun(t *testing.T) {
	client := New("test-key", backend.Config{})
	if count := client.CommitCount(); count != 0 {
		t.Errorf("expected CommitCount 0 before run, got %d", count)
	}
}
