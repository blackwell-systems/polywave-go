package dedup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/tools"
)

// TestCheckMiss verifies that the first read of a file returns (false, "").
func TestCheckMiss(t *testing.T) {
	c := New()
	deduped, summary := c.Check("/some/file.go", []byte("hello world"))
	if deduped {
		t.Errorf("expected miss, got hit")
	}
	if summary != "" {
		t.Errorf("expected empty summary on miss, got %q", summary)
	}
}

// TestCheckHit verifies that reading the same content twice returns (true, summary).
func TestCheckHit(t *testing.T) {
	c := New()
	content := []byte("line one\nline two\nline three\n")

	// First read: miss
	deduped1, _ := c.Check("/file.txt", content)
	if deduped1 {
		t.Fatalf("first read should be a miss")
	}

	// Second read: hit
	deduped2, summary := c.Check("/file.txt", content)
	if !deduped2 {
		t.Fatalf("second read of same content should be a hit")
	}
	if !strings.Contains(summary, "file unchanged since last read") {
		t.Errorf("summary should mention unchanged file, got: %q", summary)
	}
	if !strings.Contains(summary, "3 lines") {
		t.Errorf("summary should mention line count, got: %q", summary)
	}
}

// TestCheckChanged verifies that changed content returns (false, "").
func TestCheckChanged(t *testing.T) {
	c := New()
	c.Check("/file.txt", []byte("original content\n"))

	deduped, summary := c.Check("/file.txt", []byte("modified content\n"))
	if deduped {
		t.Errorf("changed content should not be deduped")
	}
	if summary != "" {
		t.Errorf("expected empty summary for changed content, got %q", summary)
	}
}

// TestInvalidate verifies that an invalidated path returns miss on next read.
func TestInvalidate(t *testing.T) {
	c := New()
	content := []byte("some content\n")

	// Populate the cache
	c.Check("/file.txt", content)
	// Verify it would be a hit
	deduped, _ := c.Check("/file.txt", content)
	if !deduped {
		t.Fatalf("expected hit before invalidation")
	}

	// Invalidate
	c.Invalidate("/file.txt")

	// Should be a miss now
	deduped2, _ := c.Check("/file.txt", content)
	if deduped2 {
		t.Errorf("expected miss after invalidation, got hit")
	}
}

// TestStats verifies Hits/Misses/TokensSavedEstimate counts.
func TestStats(t *testing.T) {
	c := New()

	// 3 lines of content
	content := []byte("line1\nline2\nline3\n")

	// Miss
	c.Check("/a.txt", content)
	// Hit (3 lines * 4 tokens = 12)
	c.Check("/a.txt", content)
	// Miss (new path)
	c.Check("/b.txt", content)
	// Hit
	c.Check("/b.txt", content)
	// Another hit on /a.txt
	c.Check("/a.txt", content)

	stats := c.Stats()
	if stats.Misses != 2 {
		t.Errorf("expected 2 misses, got %d", stats.Misses)
	}
	if stats.Hits != 3 {
		t.Errorf("expected 3 hits, got %d", stats.Hits)
	}
	// 3 hits * 3 lines * 4 tokens = 36
	expectedTokens := 3 * 3 * 4
	if stats.TokensSavedEstimate != expectedTokens {
		t.Errorf("expected %d tokens saved, got %d", expectedTokens, stats.TokensSavedEstimate)
	}
}

// TestConcurrentAccess launches 10 goroutines doing Check/Invalidate in parallel.
// Run with -race to detect data races.
func TestConcurrentAccess(t *testing.T) {
	c := New()
	content := []byte("concurrent content\n")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := fmt.Sprintf("/file%d.txt", i%3) // intentional overlap
			for j := 0; j < 20; j++ {
				c.Check(path, content)
				if j%5 == 0 {
					c.Invalidate(path)
				}
			}
		}(i)
	}
	wg.Wait()

	stats := c.Stats()
	if stats.Hits+stats.Misses == 0 {
		t.Error("expected some operations to have occurred")
	}
}

// TestWithDedup verifies the middleware wrapping behavior:
// - Second read_file call returns dedup summary
// - After write_file, next read_file returns full content
func TestWithDedup(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	content := "hello\nworld\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Create a workshop with dedup
	base := tools.StandardTools(dir)
	w, cache := WithDedup(base)

	ctx := context.Background()
	execCtx := tools.ExecutionContext{WorkDir: dir}

	readTool, ok := w.Get("read_file")
	if !ok {
		t.Fatalf("read_file tool not found")
	}

	// First read: should return full content (miss)
	result1, err := readTool.Executor.Execute(ctx, execCtx, map[string]interface{}{
		"file_path": fpath,
	})
	if err != nil {
		t.Fatalf("first read error: %v", err)
	}
	if !strings.Contains(result1, "hello") {
		t.Errorf("first read should return full content, got: %q", result1)
	}

	// Second read: should return dedup summary (hit)
	result2, err := readTool.Executor.Execute(ctx, execCtx, map[string]interface{}{
		"file_path": fpath,
	})
	if err != nil {
		t.Fatalf("second read error: %v", err)
	}
	if !strings.Contains(result2, "file unchanged since last read") {
		t.Errorf("second read should return dedup summary, got: %q", result2)
	}

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit after second read, got %d", stats.Hits)
	}

	// Write to the file (invalidates cache)
	writeTool, ok := w.Get("write_file")
	if !ok {
		t.Fatalf("write_file tool not found")
	}
	newContent := "updated\ncontent\nhere\n"
	_, err = writeTool.Executor.Execute(ctx, execCtx, map[string]interface{}{
		"file_path": fpath,
		"content":   newContent,
	})
	if err != nil {
		t.Fatalf("write_file error: %v", err)
	}

	// Read after write: should return full content again (cache invalidated)
	result3, err := readTool.Executor.Execute(ctx, execCtx, map[string]interface{}{
		"file_path": fpath,
	})
	if err != nil {
		t.Fatalf("third read error: %v", err)
	}
	if strings.Contains(result3, "file unchanged since last read") {
		t.Errorf("read after write should return full content, got dedup summary: %q", result3)
	}
	if !strings.Contains(result3, "updated") {
		t.Errorf("read after write should contain new content, got: %q", result3)
	}
}

// TestMiddleware_ReadError verifies that reading a non-existent file propagates
// the error and does NOT populate the cache.
func TestMiddleware_ReadError(t *testing.T) {
	dir := t.TempDir()
	base := tools.StandardTools(dir)
	w, cache := WithDedup(base)

	ctx := context.Background()
	execCtx := tools.ExecutionContext{WorkDir: dir}

	readTool, ok := w.Get("read_file")
	if !ok {
		t.Fatalf("read_file tool not found")
	}

	// Read a non-existent file — the tool returns an "error:..." string, not an err.
	result, err := readTool.Executor.Execute(ctx, execCtx, map[string]interface{}{
		"file_path": filepath.Join(dir, "nonexistent.txt"),
	})
	if err != nil {
		// Some implementations may return an error directly.
		// Either way, cache should not be populated.
		_ = result
	} else {
		// The read_file tool typically returns "error: ..." as a string result.
		if !strings.HasPrefix(result, "error:") && !strings.Contains(result, "no such file") {
			t.Fatalf("expected error result for missing file, got: %q", result)
		}
	}

	// Cache should have zero hits and zero misses for error reads.
	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("error reads should not produce cache hits, got %d", stats.Hits)
	}
	// An error read should not count as a miss (no cache entry created).
	if stats.Misses != 0 {
		t.Errorf("error reads should not produce cache misses, got %d", stats.Misses)
	}
}

// TestInvalidate_NonExistent verifies that invalidating a path that was never
// cached does not panic or error.
func TestInvalidate_NonExistent(t *testing.T) {
	c := New()
	// Should not panic on empty cache.
	c.Invalidate("/does/not/exist")

	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("expected zero stats after invalidating non-existent path, got hits=%d misses=%d",
			stats.Hits, stats.Misses)
	}
}

// TestMiddleware_ErrorPropagation verifies that errors from the underlying
// executor propagate through the dedup middleware without being swallowed.
func TestMiddleware_ErrorPropagation(t *testing.T) {
	// Create a workshop with a mock read_file that returns an error.
	mockErr := errors.New("disk I/O failure")
	ws := tools.NewWorkshop()
	ws.Register(tools.Tool{ //nolint:errcheck
		Name: "read_file",
		Executor: executorFunc(func(_ context.Context, _ tools.ExecutionContext, _ map[string]interface{}) (string, error) {
			return "", mockErr
		}),
	})

	w, cache := WithDedup(ws)
	ctx := context.Background()
	execCtx := tools.ExecutionContext{WorkDir: "/tmp"}

	readTool, ok := w.Get("read_file")
	if !ok {
		t.Fatalf("read_file tool not found in wrapped workshop")
	}

	result, err := readTool.Executor.Execute(ctx, execCtx, map[string]interface{}{
		"file_path": "/some/file.txt",
	})
	if err == nil {
		t.Fatalf("expected error to propagate, got result: %q", result)
	}
	if !errors.Is(err, mockErr) {
		t.Errorf("expected original error %v, got %v", mockErr, err)
	}

	// Cache should not be populated on error.
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("error should not affect cache stats, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}
}

// TestCache_EmptyContent verifies that Check handles empty content (0 lines)
// correctly on both miss and hit.
func TestCache_EmptyContent(t *testing.T) {
	c := New()

	// First check: miss
	deduped, summary := c.Check("/file.txt", []byte{})
	if deduped {
		t.Errorf("first check of empty content should be a miss")
	}
	if summary != "" {
		t.Errorf("expected empty summary on miss, got %q", summary)
	}

	// Second check: hit
	deduped2, summary2 := c.Check("/file.txt", []byte{})
	if !deduped2 {
		t.Fatalf("second check of same empty content should be a hit")
	}
	if !strings.Contains(summary2, "0 lines") {
		t.Errorf("summary should mention 0 lines for empty content, got: %q", summary2)
	}

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	// 0 lines * 4 tokens = 0
	if stats.TokensSavedEstimate != 0 {
		t.Errorf("expected 0 tokens saved for empty content, got %d", stats.TokensSavedEstimate)
	}
}

// TestCache_LargeContent verifies that Check handles large content (10000 lines)
// and that stats reflect the correct line count on hit.
func TestCache_LargeContent(t *testing.T) {
	c := New()

	// Build 10000-line content
	var b strings.Builder
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	content := []byte(b.String())

	// Miss
	deduped, _ := c.Check("/large.txt", content)
	if deduped {
		t.Fatalf("first read should be a miss")
	}

	// Hit
	deduped2, summary := c.Check("/large.txt", content)
	if !deduped2 {
		t.Fatalf("second read of same content should be a hit")
	}
	if !strings.Contains(summary, "10000 lines") {
		t.Errorf("summary should mention 10000 lines, got: %q", summary)
	}

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	// 1 hit * 10000 lines * 4 tokens = 40000
	expectedTokens := 10000 * 4
	if stats.TokensSavedEstimate != expectedTokens {
		t.Errorf("expected %d tokens saved, got %d", expectedTokens, stats.TokensSavedEstimate)
	}
}
