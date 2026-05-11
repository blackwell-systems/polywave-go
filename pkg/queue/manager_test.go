package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

func TestAdd_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:              "Dark Mode",
		Priority:           1,
		Slug:               "dark-mode",
		FeatureDescription: "Add dark mode support",
	}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	path := filepath.Join(dir, "docs", "IMPL", "queue", "001-dark-mode.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var loaded Item
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Title != "Dark Mode" {
		t.Errorf("title = %q, want %q", loaded.Title, "Dark Mode")
	}
	if loaded.Status != "queued" {
		t.Errorf("status = %q, want %q", loaded.Status, "queued")
	}
	if loaded.Slug != "dark-mode" {
		t.Errorf("slug = %q, want %q", loaded.Slug, "dark-mode")
	}
}

func TestAdd_SlugInResult(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:    "My Feature",
		Priority: 3,
	}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	d := res.GetData()
	if d.Slug != "my-feature" {
		t.Errorf("AddData.Slug = %q, want %q", d.Slug, "my-feature")
	}
	if d.FilePath == "" {
		t.Error("AddData.FilePath should not be empty")
	}
}

func TestAdd_AutoSlug(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:    "My Cool Feature!",
		Priority: 5,
	}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	path := filepath.Join(dir, "docs", "IMPL", "queue", "005-my-cool-feature.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded Item
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Slug != "my-cool-feature" {
		t.Errorf("slug = %q, want %q", loaded.Slug, "my-cool-feature")
	}
}

func TestList_SortsByPriority(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Add items out of order
	items := []Item{
		{Title: "Third", Priority: 3, Slug: "third"},
		{Title: "First", Priority: 1, Slug: "first"},
		{Title: "Second", Priority: 2, Slug: "second"},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add %q: %v", item.Title, res.Errors[0].Message)
		}
	}

	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %v", listRes.Errors[0].Message)
	}
	listed := listRes.GetData().Items
	if len(listed) != 3 {
		t.Fatalf("len = %d, want 3", len(listed))
	}
	if listed[0].Title != "First" {
		t.Errorf("listed[0].Title = %q, want %q", listed[0].Title, "First")
	}
	if listed[1].Title != "Second" {
		t.Errorf("listed[1].Title = %q, want %q", listed[1].Title, "Second")
	}
	if listed[2].Title != "Third" {
		t.Errorf("listed[2].Title = %q, want %q", listed[2].Title, "Third")
	}

	// Verify FilePath is populated
	for _, item := range listed {
		if item.FilePath == "" {
			t.Errorf("FilePath is empty for %q", item.Title)
		}
	}
}

func TestList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %v", listRes.Errors[0].Message)
	}
	listed := listRes.GetData().Items
	if len(listed) != 0 {
		t.Errorf("len = %d, want 0", len(listed))
	}
}

func TestNext_ReturnsHighestPriority(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	items := []Item{
		{Title: "Low", Priority: 10, Slug: "low"},
		{Title: "High", Priority: 1, Slug: "high"},
		{Title: "Mid", Priority: 5, Slug: "mid"},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	next := nextRes.GetData()
	if next.Slug != "high" {
		t.Errorf("slug = %q, want %q", next.Slug, "high")
	}
}

func TestNext_QueueEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	nextRes := mgr.Next()
	if !nextRes.IsFatal() {
		t.Fatal("expected Fatal result when queue is empty")
	}
	if len(nextRes.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if nextRes.Errors[0].Code != result.CodeQueueEmpty {
		t.Errorf("error code = %q, want %q", nextRes.Errors[0].Code, result.CodeQueueEmpty)
	}
}

func TestNext_SkipsDependencies(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Item A has no deps (priority 2)
	// Item B depends on "prereq" which is not complete (priority 1)
	items := []Item{
		{Title: "B", Priority: 1, Slug: "b", DependsOn: []string{"prereq"}},
		{Title: "A", Priority: 2, Slug: "a"},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	// B has unmet dep, so A should be returned even though B has higher priority
	next := nextRes.GetData()
	if next.Slug != "a" {
		t.Errorf("slug = %q, want %q", next.Slug, "a")
	}
}

func TestNext_NoneEligible(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// All items have unmet dependencies
	items := []Item{
		{Title: "A", Priority: 1, Slug: "a", DependsOn: []string{"missing"}},
		{Title: "B", Priority: 2, Slug: "b", DependsOn: []string{"also-missing"}},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if !nextRes.IsFatal() {
		t.Errorf("expected Fatal when no items eligible, got Code=%q", nextRes.Code)
	}
	if nextRes.Errors[0].Code != result.CodeQueueEmpty {
		t.Errorf("error code = %q, want %q", nextRes.Errors[0].Code, result.CodeQueueEmpty)
	}
}

func TestNext_DependencyMetByCompleteQueueItem(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// prereq is complete in the queue
	items := []Item{
		{Title: "Prereq", Priority: 1, Slug: "prereq", Status: "complete"},
		{Title: "Dependent", Priority: 2, Slug: "dependent", DependsOn: []string{"prereq"}},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	next := nextRes.GetData()
	if next.Slug != "dependent" {
		t.Errorf("slug = %q, want %q", next.Slug, "dependent")
	}
}

func TestNext_DependencyMetByCompleteDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create an IMPL file in docs/IMPL/complete/ with feature_slug matching the dep
	completeDir := filepath.Join(dir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	implContent := "feature_slug: prereq\ntitle: Prereq Feature\nstate: COMPLETE\n"
	if err := os.WriteFile(filepath.Join(completeDir, "IMPL-prereq.yaml"), []byte(implContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add item that depends on "prereq"
	item := Item{Title: "Dependent", Priority: 1, Slug: "dependent", DependsOn: []string{"prereq"}}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	next := nextRes.GetData()
	if next.Slug != "dependent" {
		t.Errorf("slug = %q, want %q", next.Slug, "dependent")
	}
}

func TestUpdateStatus(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "Test", Priority: 1, Slug: "test"}
	if addRes := mgr.Add(item); addRes.IsFatal() {
		t.Fatalf("Add: %v", addRes.Errors[0].Message)
	}

	updateRes := mgr.UpdateStatus("test", "in_progress")
	if updateRes.IsFatal() {
		t.Fatalf("UpdateStatus: %v", updateRes.Errors[0].Message)
	}

	// Verify returned data
	d := updateRes.GetData()
	if d.Slug != "test" {
		t.Errorf("UpdateStatusData.Slug = %q, want %q", d.Slug, "test")
	}
	if d.NewStatus != "in_progress" {
		t.Errorf("UpdateStatusData.NewStatus = %q, want %q", d.NewStatus, "in_progress")
	}

	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %v", listRes.Errors[0].Message)
	}
	listed := listRes.GetData().Items
	if len(listed) != 1 {
		t.Fatalf("len = %d, want 1", len(listed))
	}
	if listed[0].Status != "in_progress" {
		t.Errorf("status = %q, want %q", listed[0].Status, "in_progress")
	}
}

func TestUpdateStatus_ReturnsCorrectNewStatus(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "Status Test", Priority: 1, Slug: "status-test"}
	if addRes := mgr.Add(item); addRes.IsFatal() {
		t.Fatalf("Add: %v", addRes.Errors[0].Message)
	}

	updateRes := mgr.UpdateStatus("status-test", "complete")
	if updateRes.IsFatal() {
		t.Fatalf("UpdateStatus: %v", updateRes.Errors[0].Message)
	}

	d := updateRes.GetData()
	if d.NewStatus != "complete" {
		t.Errorf("UpdateStatusData.NewStatus = %q, want %q", d.NewStatus, "complete")
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create queue dir but no items
	queueDir := filepath.Join(dir, "docs", "IMPL", "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}

	updateRes := mgr.UpdateStatus("nonexistent", "complete")
	if !updateRes.IsFatal() {
		t.Fatal("expected Fatal result for nonexistent slug")
	}
	if updateRes.Errors[0].Code != result.CodeQueueStatusUpdateFailed {
		t.Errorf("error code = %q, want %q", updateRes.Errors[0].Code, result.CodeQueueStatusUpdateFailed)
	}
}

func TestSlugFromTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Dark Mode", "dark-mode"},
		{"My Cool Feature!", "my-cool-feature"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPERCASE", "uppercase"},
		{"a---b", "a-b"},
		{"", "item"},
		{"!!!@@@", "item"},
	}
	for _, tt := range tests {
		got := slugFromTitle(tt.title)
		if got != tt.want {
			t.Errorf("slugFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestAdd_MkdirFails(t *testing.T) {
	// Create a temp dir and make it read-only to simulate MkdirAll failure
	baseDir := t.TempDir()
	readOnlyDir := filepath.Join(baseDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o755); err != nil {
		t.Fatalf("setup: create dir: %v", err)
	}

	// Make it read-only to prevent subdirectory creation
	if err := os.Chmod(readOnlyDir, 0o444); err != nil {
		t.Fatalf("setup: chmod: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0o755) // Restore for cleanup

	mgr := NewManager(readOnlyDir)
	item := Item{
		Title:    "Test Feature",
		Priority: 1,
		Slug:     "test-feature",
	}

	res := mgr.Add(item)
	if !res.IsFatal() {
		t.Fatal("expected Fatal result when MkdirAll fails")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if res.Errors[0].Code != result.CodeQueueAddFailed {
		t.Errorf("error code = %q, want %q", res.Errors[0].Code, result.CodeQueueAddFailed)
	}
}

func TestList_CorruptedYAML(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create queue directory
	queueDir := filepath.Join(dir, "docs", "IMPL", "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatalf("setup: create dir: %v", err)
	}

	// Write corrupted YAML
	corruptedPath := filepath.Join(queueDir, "001-test.yaml")
	corruptedContent := "[]]]corrupted"
	if err := os.WriteFile(corruptedPath, []byte(corruptedContent), 0o644); err != nil {
		t.Fatalf("setup: write corrupted file: %v", err)
	}

	listRes := mgr.List()
	if !listRes.IsFatal() {
		t.Fatal("expected Fatal result when YAML is corrupted")
	}
	if len(listRes.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if listRes.Errors[0].Code != result.CodeQueueListFailed {
		t.Errorf("error code = %q, want %q", listRes.Errors[0].Code, result.CodeQueueListFailed)
	}
}

func TestCompletedSlugs_CompleteDir_ReadFailure(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create complete directory
	completeDir := filepath.Join(dir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0o755); err != nil {
		t.Fatalf("setup: create complete dir: %v", err)
	}

	// Add a queue item with dependencies
	item := Item{
		Title:     "Dependent Feature",
		Priority:  1,
		Slug:      "dependent",
		DependsOn: []string{"prereq"},
	}
	addRes := mgr.Add(item)
	if addRes.IsFatal() {
		t.Fatalf("setup: Add: %v", addRes.Errors[0].Message)
	}

	// Make complete dir unreadable to trigger ReadDir failure in completedSlugs()
	if err := os.Chmod(completeDir, 0o000); err != nil {
		t.Fatalf("setup: chmod complete dir: %v", err)
	}
	defer os.Chmod(completeDir, 0o755) // Restore for cleanup

	// Next() should fail when completedSlugs() fails to scan complete dir
	nextRes := mgr.Next()
	if !nextRes.IsFatal() {
		t.Fatal("expected Fatal result when complete dir is unreadable")
	}
	if len(nextRes.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if nextRes.Errors[0].Code != result.CodeQueueCompletedScanFailed {
		t.Errorf("error code = %q, want %q", nextRes.Errors[0].Code, result.CodeQueueCompletedScanFailed)
	}
}

func TestCompletedSlugs_UnreadableFile_ReturnsWarning(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create complete directory with an unreadable file
	completeDir := filepath.Join(dir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadablePath := filepath.Join(completeDir, "IMPL-broken.yaml")
	if err := os.WriteFile(unreadablePath, []byte("feature_slug: broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadablePath, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadablePath, 0o644)

	// Also add a readable file to verify it still gets processed
	readablePath := filepath.Join(completeDir, "IMPL-readable.yaml")
	if err := os.WriteFile(readablePath, []byte("feature_slug: readable\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// completedSlugs is unexported, so test via Next() behavior
	// Add an item that depends on "readable"
	item := Item{Title: "Dep", Priority: 1, Slug: "dep", DependsOn: []string{"readable"}}
	addRes := mgr.Add(item)
	if addRes.IsFatal() {
		t.Fatalf("Add: %v", addRes.Errors[0].Message)
	}

	nextRes := mgr.Next()
	// Should still find "dep" since "readable" is resolved, but result should be partial
	// because the unreadable file produced a warning.
	if nextRes.IsFatal() {
		t.Fatalf("Next should succeed: %v", nextRes.Errors[0].Message)
	}
	if nextRes.GetData().Slug != "dep" {
		t.Errorf("slug = %q, want %q", nextRes.GetData().Slug, "dep")
	}
	// Fix 1: Next() must propagate completedSlugs() warnings as a partial result.
	if !nextRes.IsPartial() {
		t.Error("Next() should return a partial result when completedSlugs() has warnings")
	}
	if len(nextRes.Errors) == 0 {
		t.Error("Next() partial result should carry warning errors")
	}
	if nextRes.Errors[0].Code != result.CodeQueueCorruptedFile {
		t.Errorf("warning code = %q, want %q", nextRes.Errors[0].Code, result.CodeQueueCorruptedFile)
	}
}

func TestUpdateStatus_InvalidStatus(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "Test", Priority: 1, Slug: "test"}
	if addRes := mgr.Add(item); addRes.IsFatal() {
		t.Fatalf("Add: %v", addRes.Errors[0].Message)
	}

	updateRes := mgr.UpdateStatus("test", "banana")
	if !updateRes.IsFatal() {
		t.Fatal("expected Fatal result for invalid status")
	}
	if len(updateRes.Errors) == 0 {
		t.Fatal("expected errors")
	}
	if updateRes.Errors[0].Code != result.CodeQueueStatusUpdateFailed {
		t.Errorf("error code = %q, want %q", updateRes.Errors[0].Code, result.CodeQueueStatusUpdateFailed)
	}
	if !strings.Contains(updateRes.Errors[0].Message, "banana") {
		t.Errorf("error message should mention invalid status, got: %s", updateRes.Errors[0].Message)
	}
}

func TestUpdateStatus_AllValidStatuses(t *testing.T) {
	for _, status := range []string{"queued", "in_progress", "complete", "blocked"} {
		t.Run(status, func(t *testing.T) {
			dir := t.TempDir()
			mgr := NewManager(dir)

			item := Item{Title: "Test", Priority: 1, Slug: "test"}
			if addRes := mgr.Add(item); addRes.IsFatal() {
				t.Fatalf("Add: %v", addRes.Errors[0].Message)
			}

			updateRes := mgr.UpdateStatus("test", status)
			if updateRes.IsFatal() {
				t.Fatalf("UpdateStatus(%q) failed: %v", status, updateRes.Errors[0].Message)
			}
			if updateRes.GetData().NewStatus != status {
				t.Errorf("NewStatus = %q, want %q", updateRes.GetData().NewStatus, status)
			}
		})
	}
}

// TestAdd_CollisionReturnsFatal verifies Fix 2: Add() returns a fatal error when
// a queue item with the same priority+slug already exists on disk.
func TestAdd_CollisionReturnsFatal(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "My Feature", Priority: 1, Slug: "my-feature"}
	res1 := mgr.Add(item)
	if res1.IsFatal() {
		t.Fatalf("first Add should succeed: %v", res1.Errors[0].Message)
	}

	res2 := mgr.Add(item)
	if !res2.IsFatal() {
		t.Fatal("second Add with same priority+slug should return fatal error")
	}
	if len(res2.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if res2.Errors[0].Code != result.CodeQueueAddFailed {
		t.Errorf("error code = %q, want %q", res2.Errors[0].Code, result.CodeQueueAddFailed)
	}
	if !strings.Contains(res2.Errors[0].Message, "already exists") {
		t.Errorf("error message should mention 'already exists', got: %s", res2.Errors[0].Message)
	}
}

// TestAdd_EmptyTitleAndSlug verifies Fix 3: Add() returns a fatal error when
// both Title and Slug are empty.
func TestAdd_EmptyTitleAndSlug(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Priority: 1} // no Title, no Slug
	res := mgr.Add(item)
	if !res.IsFatal() {
		t.Fatal("Add with empty title and slug should return fatal error")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if res.Errors[0].Code != result.CodeQueueAddFailed {
		t.Errorf("error code = %q, want %q", res.Errors[0].Code, result.CodeQueueAddFailed)
	}
}

// TestNext_CompleteDirFilenameFallback verifies Fix 5: when an IMPL file in
// docs/IMPL/complete/ has no feature_slug field (or an unparseable/empty body),
// Next() resolves the dependency via the filename fallback
// (strings.TrimPrefix(base, "IMPL-")).
func TestNext_CompleteDirFilenameFallback(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Write an IMPL file that has no feature_slug field — only the filename carries the slug.
	completeDir := filepath.Join(dir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// YAML has no feature_slug; slug must be derived from filename "IMPL-prereq.yaml".
	noSlugContent := "title: Prereq Feature\nstate: COMPLETE\n"
	if err := os.WriteFile(filepath.Join(completeDir, "IMPL-prereq.yaml"), []byte(noSlugContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add item that depends on "prereq" (resolved via filename fallback).
	item := Item{Title: "Dependent", Priority: 1, Slug: "dependent", DependsOn: []string{"prereq"}}
	if res := mgr.Add(item); res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next should succeed via filename fallback: %v", nextRes.Errors[0].Message)
	}
	if nextRes.GetData().Slug != "dependent" {
		t.Errorf("slug = %q, want %q", nextRes.GetData().Slug, "dependent")
	}
}

func TestUpdateStatus_SaveYAMLFails(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Add an item
	item := Item{
		Title:    "Test Feature",
		Priority: 1,
		Slug:     "test-feature",
	}
	addRes := mgr.Add(item)
	if addRes.IsFatal() {
		t.Fatalf("setup: Add: %v", addRes.Errors[0].Message)
	}

	// Get the file path and make it read-only
	filePath := addRes.GetData().FilePath
	if err := os.Chmod(filePath, 0o444); err != nil {
		t.Fatalf("setup: chmod file: %v", err)
	}
	defer os.Chmod(filePath, 0o644) // Restore for cleanup

	// Attempt to update status should fail
	updateRes := mgr.UpdateStatus("test-feature", "in_progress")
	if !updateRes.IsFatal() {
		t.Fatal("expected Fatal result when file is read-only")
	}
	if len(updateRes.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if updateRes.Errors[0].Code != result.CodeQueueStatusUpdateFailed {
		t.Errorf("error code = %q, want %q", updateRes.Errors[0].Code, result.CodeQueueStatusUpdateFailed)
	}
}
