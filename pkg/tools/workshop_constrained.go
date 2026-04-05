package tools

// Middleware constructor variables. These reference the real implementations
// from constraint_middleware.go (Agent A), bash_constraints.go (Agent B),
// and role_middleware.go (Agent C). They are package-level variables so that:
// 1. They resolve at init time when all wave files are merged
// 2. Tests can verify composition logic independently
//
// After wave merge, these point to the real middleware constructors.
var (
	ownershipMiddlewareFn func(toolName string, c Constraints) Middleware
	freezeMiddlewareFn    func(toolName string, c Constraints) Middleware
	rolePathMiddlewareFn  func(toolName string, c Constraints) Middleware
)

func init() {
	RegisterConstraintMiddleware()
}

// writableTools are the tools that can modify files and need constraint middleware.
var writableTools = map[string]bool{
	"write_file": true,
	"edit_file":  true,
}

// WithConstraints returns a new Workshop with all constraint middleware applied
// based on the Constraints config. Returns the wrapped Workshop and a
// CommitTracker (non-nil only when c.TrackCommits is true).
//
// Middleware application order (outermost to innermost):
// 1. RolePathMiddleware (I6) - broadest filter first
// 2. FreezeMiddleware (I2) - frozen paths before ownership
// 3. OwnershipMiddleware (I1) - file-level ownership
// 4. BashConstraintMiddleware (I5+I1) - bash-specific (only on bash tool)
//
// If Constraints is zero-value, returns the original Workshop unchanged
// (backward compatible).
func WithConstraints(w Workshop, c Constraints) (Workshop, *CommitTracker) {
	if isZeroConstraints(c) {
		return w, nil
	}

	var tracker *CommitTracker
	if c.TrackCommits {
		tracker = &CommitTracker{}
	}

	wrapped := NewWorkshop()
	for _, tool := range w.All() {
		t := tool // copy

		switch {
		case t.Name == "bash":
			// TODO(I5): Bash constraint enforcement (commit tracking) is deferred.
			// The bashConstraintMiddlewareFn stub was a permanent passthrough and
			// has been removed. Re-introduce real I5 enforcement here when needed.

		case writableTools[t.Name]:
			// Write/edit tools get the constraint middleware stack.
			// Apply builds outermost-first, so list order = execution order:
			// RolePath runs first, then Freeze, then Ownership (innermost).
			var middlewares []Middleware
			if len(c.AllowedPathPrefixes) > 0 {
				middlewares = append(middlewares, rolePathMiddlewareFn(t.Name, c))
			}
			if len(c.FrozenPaths) > 0 && c.FreezeTime != nil {
				middlewares = append(middlewares, freezeMiddlewareFn(t.Name, c))
			}
			if len(c.OwnedFiles) > 0 {
				middlewares = append(middlewares, ownershipMiddlewareFn(t.Name, c))
			}
			if len(middlewares) > 0 {
				t.Executor = Apply(t.Executor, middlewares...)
			}

		default:
			// Read-only tools (read_file, glob, grep, list_directory): no constraints
		}

		if res := wrapped.Register(t); res.IsFatal() {
			panic("WithConstraints: duplicate tool name: " + t.Name)
		}
	}

	return wrapped, tracker
}

// ConstrainedTools creates a Workshop with StandardTools and all constraints applied.
// Convenience function combining StandardTools + WithConstraints.
func ConstrainedTools(workDir string, c Constraints) (Workshop, *CommitTracker) {
	return WithConstraints(StandardTools(workDir), c)
}

// isZeroConstraints returns true if the Constraints struct has no enforcement
// configured, meaning the workshop should pass through unchanged.
func isZeroConstraints(c Constraints) bool {
	return len(c.OwnedFiles) == 0 &&
		len(c.FrozenPaths) == 0 &&
		c.FreezeTime == nil &&
		!c.TrackCommits &&
		len(c.AllowedPathPrefixes) == 0
}
