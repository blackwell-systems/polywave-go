package dedup

import (
	"context"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// executorFunc adapts a function to the ToolExecutor interface.
type executorFunc func(context.Context, tools.ExecutionContext, map[string]interface{}) (string, error)

func (f executorFunc) Execute(ctx context.Context, execCtx tools.ExecutionContext, input map[string]interface{}) (string, error) {
	return f(ctx, execCtx, input)
}

// WithDedup returns a new Workshop where read_file results are deduped
// and write_file/edit_file calls invalidate the cache.
// The returned Cache can be queried for Stats() after agent completion.
func WithDedup(w tools.Workshop) (tools.Workshop, *Cache) {
	cache := New()
	wrapped := tools.NewWorkshop()

	for _, tool := range w.All() {
		t := tool // capture loop variable

		// Write-invalidating tools: write_file, edit_file. If new write tools are added
		// to pkg/tools/standard.go, add them here to ensure cache invalidation on writes.
		switch t.Name {
		case "read_file":
			inner := t.Executor
			t.Executor = executorFunc(func(ctx context.Context, execCtx tools.ExecutionContext, input map[string]interface{}) (string, error) {
				result, err := inner.Execute(ctx, execCtx, input)
				if err != nil {
					return result, err
				}
				if strings.HasPrefix(result, "error:") {
					return result, nil
				}
				// Extract path from input
				path, _ := input["file_path"].(string)
				if deduped, summary := cache.Check(path, []byte(result)); deduped {
					return summary, nil
				}
				return result, nil
			})

		case "write_file":
			inner := t.Executor
			t.Executor = executorFunc(func(ctx context.Context, execCtx tools.ExecutionContext, input map[string]interface{}) (string, error) {
				result, err := inner.Execute(ctx, execCtx, input)
				path, _ := input["file_path"].(string)
				cache.Invalidate(path)
				return result, err
			})

		case "edit_file":
			inner := t.Executor
			t.Executor = executorFunc(func(ctx context.Context, execCtx tools.ExecutionContext, input map[string]interface{}) (string, error) {
				result, err := inner.Execute(ctx, execCtx, input)
				path, _ := input["file_path"].(string)
				cache.Invalidate(path)
				return result, err
			})
		}

		wrapped.Register(t) //nolint:errcheck
	}

	return wrapped, cache
}
