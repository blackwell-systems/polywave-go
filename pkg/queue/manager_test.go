package queue

import (
	"os"
	"path/filepath"
	"testing"

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
	if err := mgr.Add(item); err != nil {
		t.Fatalf("Add: %v", err)
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

func TestAdd_AutoSlug(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:    "My Cool Feature!",
		Priority: 5,
	}
	if err := mgr.Add(item); err != nil {
		t.Fatalf("Add: %v", err)
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
		if err := mgr.Add(item); err != nil {
			t.Fatalf("Add %q: %v", item.Title, err)
		}
	}

	listed, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
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

	listed, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
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
		if err := mgr.Add(item); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	next, err := mgr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if next == nil {
		t.Fatal("Next returned nil, want item")
	}
	if next.Slug != "high" {
		t.Errorf("slug = %q, want %q", next.Slug, "high")
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
		if err := mgr.Add(item); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	next, err := mgr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if next == nil {
		t.Fatal("Next returned nil, want item")
	}
	// B has unmet dep, so A should be returned even though B has higher priority
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
		if err := mgr.Add(item); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	next, err := mgr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if next != nil {
		t.Errorf("expected nil, got %+v", next)
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
		if err := mgr.Add(item); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	next, err := mgr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if next == nil {
		t.Fatal("Next returned nil, want item")
	}
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
	if err := mgr.Add(item); err != nil {
		t.Fatalf("Add: %v", err)
	}

	next, err := mgr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if next == nil {
		t.Fatal("Next returned nil, want item")
	}
	if next.Slug != "dependent" {
		t.Errorf("slug = %q, want %q", next.Slug, "dependent")
	}
}

func TestUpdateStatus(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "Test", Priority: 1, Slug: "test"}
	if err := mgr.Add(item); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := mgr.UpdateStatus("test", "in_progress"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	items, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0].Status != "in_progress" {
		t.Errorf("status = %q, want %q", items[0].Status, "in_progress")
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

	err := mgr.UpdateStatus("nonexistent", "complete")
	if err == nil {
		t.Fatal("expected error for nonexistent slug")
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
