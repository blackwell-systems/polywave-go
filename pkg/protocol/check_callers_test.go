package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCallers_FindsCallSites(t *testing.T) {
	// Create a temp directory with a Go file that calls a known function
	dir := t.TempDir()

	mainFile := filepath.Join(dir, "main.go")
	err := os.WriteFile(mainFile, []byte(`package main

import "fmt"

func main() {
	result := GetData()
	fmt.Println(result)
}

func GetData() string {
	return "hello"
}
`), 0644)
	if err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	ctx := context.Background()
	res := CheckCallers(ctx, dir, "GetData")

	if !res.IsSuccess() {
		t.Fatalf("expected success, got FATAL: %v", res.Errors)
	}

	sites := res.GetData()
	if len(sites) == 0 {
		t.Fatal("expected at least one caller site, got none")
	}

	// Verify at least one site has GetData in context
	found := false
	for _, s := range sites {
		if s.File != "" && s.Line > 0 && s.Context != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("caller sites missing required fields")
	}
}

func TestCheckCallers_IncludesTestFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a regular Go file
	err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(`package foo

func MyFunc() {}
`), 0644)
	if err != nil {
		t.Fatalf("failed to write foo.go: %v", err)
	}

	// Create a test file that calls MyFunc
	err = os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(`package foo

import "testing"

func TestMyFunc(t *testing.T) {
	MyFunc()
}
`), 0644)
	if err != nil {
		t.Fatalf("failed to write foo_test.go: %v", err)
	}

	ctx := context.Background()
	res := CheckCallers(ctx, dir, "MyFunc")

	if !res.IsSuccess() {
		t.Fatalf("expected success, got FATAL: %v", res.Errors)
	}

	sites := res.GetData()

	// Verify we found caller sites in test files
	foundTestFile := false
	for _, s := range sites {
		if filepath.Ext(s.File) == ".go" {
			// Check if any site is in a test file
			base := filepath.Base(s.File)
			if len(base) > 8 && base[len(base)-8:] == "_test.go" {
				foundTestFile = true
				break
			}
		}
	}

	if !foundTestFile {
		t.Error("expected caller sites in _test.go files, found none")
		for _, s := range sites {
			t.Logf("  site: file=%s line=%d context=%s", s.File, s.Line, s.Context)
		}
	}
}

func TestCheckCallers_EmptySymbolNameReturnsFatal(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	res := CheckCallers(ctx, dir, "")

	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for empty symbolName, got code=%s", res.Code)
	}

	if len(res.Errors) == 0 {
		t.Fatal("expected errors in FATAL result")
	}

	if res.Errors[0].Code != "X001_INVALID_INPUT" {
		t.Errorf("expected error code X001_INVALID_INPUT, got %s", res.Errors[0].Code)
	}
}

func TestCheckCallers_NoCallersReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(`package foo

func Bar() {}
`), 0644)
	if err != nil {
		t.Fatalf("failed to write foo.go: %v", err)
	}

	ctx := context.Background()
	res := CheckCallers(ctx, dir, "NonExistentFunctionXYZ")

	if !res.IsSuccess() {
		t.Fatalf("expected success, got FATAL: %v", res.Errors)
	}

	sites := res.GetData()
	if sites == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(sites) != 0 {
		t.Errorf("expected 0 callers, got %d", len(sites))
	}
}

func TestCheckCallers_UnreadableRepoDirReturnsFatal(t *testing.T) {
	ctx := context.Background()
	res := CheckCallers(ctx, "/nonexistent/path/that/does/not/exist", "SomeFunc")

	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for unreadable repoDir, got code=%s", res.Code)
	}

	if len(res.Errors) == 0 {
		t.Fatal("expected errors in FATAL result")
	}
}

func TestListErrorRanges_ReturnsRanges(t *testing.T) {
	// Use the actual repo root — navigate from test package location
	// The test file is at pkg/protocol/ so repo root is ../..
	repoDir := filepath.Join("..", "..")

	ctx := context.Background()
	res := ListErrorRanges(ctx, repoDir)

	if !res.IsSuccess() {
		t.Fatalf("expected success, got FATAL: %v", res.Errors)
	}

	ranges := res.GetData()
	if len(ranges) == 0 {
		t.Fatal("expected at least one error code range, got none")
	}

	// Verify the expected prefixes are present
	expectedPrefixes := []string{"V", "W", "B", "G", "A", "N", "K", "C"}
	found := make(map[string]bool)
	for _, r := range ranges {
		found[r.Prefix] = true
		// Sanity check fields
		if r.Start <= 0 {
			t.Errorf("range %s has invalid Start=%d", r.Prefix, r.Start)
		}
		if r.End < r.Start {
			t.Errorf("range %s has End=%d < Start=%d", r.Prefix, r.End, r.Start)
		}
		if r.Description == "" {
			t.Errorf("range %s has empty Description", r.Prefix)
		}
	}

	for _, prefix := range expectedPrefixes {
		if !found[prefix] {
			t.Errorf("expected prefix %q in ListErrorRanges output, not found", prefix)
		}
	}
}

func TestListErrorRanges_MissingFileReturnsFatal(t *testing.T) {
	ctx := context.Background()
	res := ListErrorRanges(ctx, "/nonexistent/repo/path")

	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for missing file, got code=%s", res.Code)
	}

	if len(res.Errors) == 0 {
		t.Fatal("expected errors in FATAL result")
	}

	if res.Errors[0].Code != "X002_FILE_READ" {
		t.Errorf("expected error code X002_FILE_READ, got %s", res.Errors[0].Code)
	}
}
