package pipeline

import "sync"

// Registry maps step names to StepFunc implementations.
// It is safe for concurrent reads after initial population.
type Registry struct {
	mu    sync.RWMutex
	funcs map[string]StepFunc
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{funcs: make(map[string]StepFunc)}
}

// Register associates name with fn. It overwrites any previous registration.
func (r *Registry) Register(name string, fn StepFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.funcs[name] = fn
}

// Get retrieves the StepFunc for name. Returns (fn, true) if found, else (nil, false).
func (r *Registry) Get(name string) (StepFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.funcs[name]
	return fn, ok
}

// DefaultRegistry returns a Registry pre-populated with the standard Polywave steps.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("validate_invariants", StepValidateInvariants().Func)
	r.Register("create_worktrees", StepCreateWorktrees().Func)
	r.Register("run_quality_gates", StepRunQualityGates().Func)
	r.Register("merge_agents", StepMergeAgents().Func)
	r.Register("verify_build", StepVerifyBuild().Func)
	r.Register("cleanup", StepCleanup().Func)
	return r
}
