package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// AddData holds the result payload for a successful Add operation.
type AddData struct {
	Slug     string
	FilePath string
}

// ListData holds the result payload for a successful List operation.
type ListData struct {
	Items []Item
	Count int
}

// UpdateStatusData holds the result payload for a successful UpdateStatus operation.
type UpdateStatusData struct {
	Slug      string
	NewStatus string
}

// Manager manages the IMPL queue directory (docs/IMPL/queue/).
type Manager struct {
	repoPath string
}

// NewManager creates a new Manager rooted at the given repoPath.
func NewManager(repoPath string) *Manager {
	return &Manager{repoPath: repoPath}
}

// queueDir returns the absolute path to docs/IMPL/queue/.
func (m *Manager) queueDir() string {
	return protocol.IMPLQueueDir(m.repoPath)
}

// completeDir returns the absolute path to docs/IMPL/complete/.
func (m *Manager) completeDir() string {
	return protocol.IMPLCompleteDir(m.repoPath)
}

// slugFromTitle generates a URL-safe slug from a title string.
// Converts to lowercase, replaces spaces/non-alphanumeric with hyphens,
// collapses multiple hyphens, and trims leading/trailing hyphens.
func slugFromTitle(title string) string {
	s := strings.ToLower(title)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "item"
	}
	return s
}

// Add writes item as YAML to docs/IMPL/queue/{priority:03d}-{slug}.yaml.
// Auto-generates slug from title if item.Slug is empty.
// Sets status to "queued" if empty.
// Returns a Result containing the slug and file path on success.
func (m *Manager) Add(item Item) result.Result[AddData] {
	if item.Slug == "" {
		item.Slug = slugFromTitle(item.Title)
	}
	if item.Status == "" {
		item.Status = "queued"
	}

	dir := m.queueDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result.NewFailure[AddData]([]result.SAWError{
			result.NewFatal(result.CodeQueueAddFailed, fmt.Sprintf("create queue dir: %s", err.Error())).WithCause(err),
		})
	}

	filename := fmt.Sprintf("%03d-%s.yaml", item.Priority, item.Slug)
	path := filepath.Join(dir, filename)

	if err := protocol.SaveYAML(path, &item); err != nil {
		return result.NewFailure[AddData]([]result.SAWError{
			result.NewFatal(result.CodeQueueAddFailed, err.Error()).WithCause(err),
		})
	}

	return result.NewSuccess(AddData{
		Slug:     item.Slug,
		FilePath: path,
	})
}

// List reads all YAML files in docs/IMPL/queue/, sorts by priority (ascending),
// and returns a Result containing the items slice. FilePath is populated on each item.
func (m *Manager) List() result.Result[ListData] {
	dir := m.queueDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result.NewSuccess(ListData{Items: []Item{}, Count: 0})
		}
		return result.NewFailure[ListData]([]result.SAWError{
			result.NewFatal(result.CodeQueueListFailed, fmt.Sprintf("read dir: %s", err.Error())).WithCause(err),
		})
	}

	var items []Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		item, err := protocol.LoadYAML[Item](path)
		if err != nil {
			return result.NewFailure[ListData]([]result.SAWError{
				result.NewFatal(result.CodeQueueListFailed, err.Error()).WithCause(err),
			})
		}
		item.FilePath = path
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
	})

	return result.NewSuccess(ListData{Items: items, Count: len(items)})
}

// implSlug is a minimal struct for extracting feature_slug from IMPL YAML files.
type implSlug struct {
	FeatureSlug string `yaml:"feature_slug"`
}

// completedSlugs returns the set of slugs that are "complete" by scanning
// both the queue directory and docs/IMPL/complete/.
func (m *Manager) completedSlugs() result.Result[map[string]bool] {
	completed := make(map[string]bool)

	// Check queue items for complete status
	listResult := m.List()
	if listResult.IsFatal() {
		return result.NewFailure[map[string]bool]([]result.SAWError{
			result.NewFatal(result.CodeQueueCompletedScanFailed, listResult.Errors[0].Message),
		})
	}
	for _, item := range listResult.GetData().Items {
		if item.Status == "complete" && item.Slug != "" {
			completed[item.Slug] = true
		}
	}

	// Check docs/IMPL/complete/ directory for IMPL YAML files.
	// IMPL files use feature_slug (not slug), so we extract that field.
	// We also derive slug from filename pattern IMPL-{slug}.yaml as fallback.
	completeDir := m.completeDir()
	entries, err := os.ReadDir(completeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result.NewSuccess(completed)
		}
		return result.NewFailure[map[string]bool]([]result.SAWError{
			result.NewFatal(result.CodeQueueCompletedScanFailed, fmt.Sprintf("read complete dir: %s", err.Error())).WithCause(err),
		})
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		// Try to extract feature_slug from file contents
		path := filepath.Join(completeDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// In-memory unmarshal: data already read above; cannot use LoadYAML (no file path).
		var is implSlug
		if err := yaml.Unmarshal(data, &is); err == nil && is.FeatureSlug != "" {
			completed[is.FeatureSlug] = true
			continue
		}

		// Fallback: derive slug from filename IMPL-{slug}.yaml
		base := strings.TrimSuffix(name, ".yaml")
		if strings.HasPrefix(base, "IMPL-") {
			completed[strings.TrimPrefix(base, "IMPL-")] = true
		}
	}

	return result.NewSuccess(completed)
}

// Next returns the highest-priority (lowest number) item whose status is
// "queued" AND all depends_on slugs have status "complete".
// Returns Fatal with Code "QUEUE_EMPTY" if no eligible item exists.
func (m *Manager) Next() result.Result[Item] {
	listResult := m.List()
	if listResult.IsFatal() {
		return result.NewFailure[Item](listResult.Errors)
	}
	items := listResult.GetData().Items

	completedResult := m.completedSlugs()
	if completedResult.IsFatal() {
		return result.NewFailure[Item](completedResult.Errors)
	}
	completed := completedResult.GetData()

	for i := range items {
		item := &items[i]
		if item.Status != "queued" {
			continue
		}
		eligible := true
		for _, dep := range item.DependsOn {
			if !completed[dep] {
				eligible = false
				break
			}
		}
		if eligible {
			return result.NewSuccess(*item)
		}
	}

	return result.NewFailure[Item]([]result.SAWError{
		result.NewFatal(result.CodeQueueEmpty, "no eligible items in queue"),
	})
}

// UpdateStatus updates the status field in the queue item's YAML file
// identified by slug. Returns a Result containing the slug and new status on success.
func (m *Manager) UpdateStatus(slug string, status string) result.Result[UpdateStatusData] {
	dir := m.queueDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result.NewFailure[UpdateStatusData]([]result.SAWError{
			result.NewFatal(result.CodeQueueStatusUpdateFailed, fmt.Sprintf("read dir: %s", err.Error())).WithCause(err),
		})
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		item, err := protocol.LoadYAML[Item](path)
		if err != nil {
			return result.NewFailure[UpdateStatusData]([]result.SAWError{
				result.NewFatal(result.CodeQueueStatusUpdateFailed, err.Error()).WithCause(err),
			})
		}
		if item.Slug == slug {
			item.Status = status
			if err := protocol.SaveYAML(path, &item); err != nil {
				return result.NewFailure[UpdateStatusData]([]result.SAWError{
					result.NewFatal(result.CodeQueueStatusUpdateFailed, err.Error()).WithCause(err),
				})
			}
			return result.NewSuccess(UpdateStatusData{
				Slug:      slug,
				NewStatus: status,
			})
		}
	}

	return result.NewFailure[UpdateStatusData]([]result.SAWError{
		result.NewFatal(result.CodeQueueStatusUpdateFailed, fmt.Sprintf("slug %q not found in queue", slug)),
	})
}
