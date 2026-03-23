package dedup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
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
