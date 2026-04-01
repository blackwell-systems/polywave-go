package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

// mockChatResponse builds a non-streaming chat completion JSON response.
func mockChatResponse(finishReason, content string, toolCalls []toolCall) []byte {
	msg := map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	resp := map[string]interface{}{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"choices": []map[string]interface{}{{"finish_reason": finishReason, "message": msg}},
	}
	data, _ := json.Marshal(resp)
	return data
}

// TestRun_SingleTurn verifies that Run returns the assistant text on a single stop turn.
func TestRun_SingleTurn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockChatResponse("stop", "hello from gpt", nil))
	}))
	defer srv.Close()

	client := New(backend.Config{}).WithAPIKey("test-key").WithBaseURL(srv.URL)
	result, err := client.Run(context.Background(), "sys", "user msg", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello from gpt" {
		t.Errorf("expected %q, got %q", "hello from gpt", result)
	}
}

// TestRun_ToolCallLoop verifies that Run executes a tool call then returns the final text.
func TestRun_ToolCallLoop(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// First turn: return a tool_calls response.
			tcs := []toolCall{{
				ID:   "call_1",
				Type: "function",
				Function: toolFunction{
					Name:      "Bash",
					Arguments: `{"command":"echo hello"}`,
				},
			}}
			w.Write(mockChatResponse("tool_calls", "", tcs))
		} else {
			// Second turn: return stop.
			w.Write(mockChatResponse("stop", "tool done", nil))
		}
	}))
	defer srv.Close()

	client := New(backend.Config{}).WithAPIKey("test-key").WithBaseURL(srv.URL)
	result, err := client.Run(context.Background(), "", "run bash", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "tool done" {
		t.Errorf("expected %q, got %q", "tool done", result)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (tool_calls + stop), got %d", callCount)
	}
}

// TestRunStreaming_CallsOnChunk verifies that RunStreaming calls onChunk with SSE fragments.
func TestRunStreaming_CallsOnChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if streaming was requested.
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		streaming, _ := body["stream"].(bool)

		if streaming {
			// Serve SSE stream.
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}
			chunks := []string{"hello", " ", "world"}
			for _, chunk := range chunks {
				event := map[string]interface{}{
					"choices": []map[string]interface{}{
						{"delta": map[string]interface{}{"content": chunk}, "finish_reason": nil},
					},
				}
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
			}
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		} else {
			// Non-streaming probe: return stop so we re-issue as streaming.
			w.Header().Set("Content-Type", "application/json")
			w.Write(mockChatResponse("stop", "", nil))
		}
	}))
	defer srv.Close()

	client := New(backend.Config{}).WithAPIKey("test-key").WithBaseURL(srv.URL)

	var chunks []string
	result, err := client.RunStreaming(context.Background(), "", "stream test", t.TempDir(), func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	full := strings.Join(chunks, "")
	if full != "hello world" {
		t.Errorf("expected chunks to join as %q, got %q", "hello world", full)
	}
	if result != "hello world" {
		t.Errorf("expected result %q, got %q", "hello world", result)
	}
}

// TestNew_CfgAPIKey verifies that New() uses cfg.APIKey when provided.
func TestNew_CfgAPIKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockChatResponse("stop", "ok", nil))
	}))
	defer srv.Close()

	client := New(backend.Config{APIKey: "cfg-key"}).WithBaseURL(srv.URL)
	_, err := client.Run(context.Background(), "", "ping", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer cfg-key" {
		t.Errorf("expected Authorization %q, got %q", "Bearer cfg-key", gotAuth)
	}
}

// TestNew_APIKeyFromEnv verifies that OPENAI_API_KEY is used when cfg.APIKey is not set.
func TestNew_APIKeyFromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-test-key")

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockChatResponse("stop", "ok", nil))
	}))
	defer srv.Close()

	client := New(backend.Config{}).WithBaseURL(srv.URL)
	_, err := client.Run(context.Background(), "", "ping", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer env-test-key" {
		t.Errorf("expected Authorization %q, got %q", "Bearer env-test-key", gotAuth)
	}
}

// TestNew_BaseURLOverride verifies that requests reach the mock server when BaseURL is overridden.
func TestNew_BaseURLOverride(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockChatResponse("stop", "override ok", nil))
	}))
	defer srv.Close()

	client := New(backend.Config{}).WithAPIKey("key").WithBaseURL(srv.URL)
	result, err := client.Run(context.Background(), "", "test", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reached {
		t.Error("mock server was not reached")
	}
	if result != "override ok" {
		t.Errorf("expected %q, got %q", "override ok", result)
	}
}

// TestParseContentToolCall_EmptyContent verifies that empty content returns nil.
func TestParseContentToolCall_EmptyContent(t *testing.T) {
	nameSet := map[string]bool{"Bash": true}
	if got := parseContentToolCall("", nameSet); got != nil {
		t.Errorf("expected nil for empty content, got %+v", got)
	}
	if got := parseContentToolCall("   ", nameSet); got != nil {
		t.Errorf("expected nil for whitespace content, got %+v", got)
	}
}

// TestParseContentToolCall_ValidJSONNoName verifies that JSON without a "name" field returns nil.
func TestParseContentToolCall_ValidJSONNoName(t *testing.T) {
	nameSet := map[string]bool{"Bash": true}
	content := `{"arguments": {"command": "echo hello"}}`
	if got := parseContentToolCall(content, nameSet); got != nil {
		t.Errorf("expected nil for JSON without name, got %+v", got)
	}
}

// TestParseContentToolCall_UnknownToolName verifies that JSON with an unregistered tool returns nil.
func TestParseContentToolCall_UnknownToolName(t *testing.T) {
	nameSet := map[string]bool{"Bash": true}
	content := `{"name": "UnknownTool", "arguments": {"key": "value"}}`
	if got := parseContentToolCall(content, nameSet); got != nil {
		t.Errorf("expected nil for unknown tool name, got %+v", got)
	}
}

// TestParseContentToolCall_KnownToolName verifies that valid JSON with a known tool name succeeds.
func TestParseContentToolCall_KnownToolName(t *testing.T) {
	nameSet := map[string]bool{"Bash": true, "Read": true}
	content := `{"name": "Bash", "arguments": {"command": "echo hello"}}`
	got := parseContentToolCall(content, nameSet)
	if got == nil {
		t.Fatal("expected non-nil result for known tool name")
	}
	if got.Name != "Bash" {
		t.Errorf("expected Name %q, got %q", "Bash", got.Name)
	}
	cmd, ok := got.Arguments["command"].(string)
	if !ok || cmd != "echo hello" {
		t.Errorf("expected arguments[command] = %q, got %v", "echo hello", got.Arguments["command"])
	}
}

// TestParseContentToolCall_NotJSON verifies that non-JSON content returns nil.
func TestParseContentToolCall_NotJSON(t *testing.T) {
	nameSet := map[string]bool{"Bash": true}
	if got := parseContentToolCall("Hello, I'm a chat message", nameSet); got != nil {
		t.Errorf("expected nil for non-JSON content, got %+v", got)
	}
}

// TestRunStreamingWithTools_EmitsToolCallEvents verifies that onToolCall is called during tool use.
func TestRunStreamingWithTools_EmitsToolCallEvents(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			tcs := []toolCall{{
				ID:   "call_1",
				Type: "function",
				Function: toolFunction{
					Name:      "Bash",
					Arguments: `{"command":"echo hello"}`,
				},
			}}
			w.Write(mockChatResponse("tool_calls", "", tcs))
		} else {
			w.Write(mockChatResponse("stop", "done", nil))
		}
	}))
	defer srv.Close()

	var toolEvents []backend.ToolCallEvent
	client := New(backend.Config{}).WithAPIKey("test-key").WithBaseURL(srv.URL)
	result, err := client.RunStreamingWithTools(
		context.Background(), "", "run bash", t.TempDir(),
		nil, // no chunk callback
		func(ev backend.ToolCallEvent) {
			toolEvents = append(toolEvents, ev)
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Errorf("expected %q, got %q", "done", result)
	}
	if len(toolEvents) == 0 {
		t.Error("expected at least one tool call event, got none")
	}
	if len(toolEvents) > 0 && toolEvents[0].Name != "Bash" {
		t.Errorf("expected tool event Name %q, got %q", "Bash", toolEvents[0].Name)
	}
}

// TestCommitCount_NilConstraints verifies CommitCount returns 0 without constraints.
func TestCommitCount_NilConstraints(t *testing.T) {
	client := New(backend.Config{})
	if got := client.CommitCount(); got != 0 {
		t.Errorf("CommitCount() = %d, want 0", got)
	}
}
