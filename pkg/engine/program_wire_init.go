package engine

// This file wires together components from different wave agents.
// Agent A (Wave 1) defined launchParallelScoutsFunc as a stub.
// Agent B (Wave 1) implemented LaunchParallelScouts.
// This init connects them.

// init wires launchParallelScoutsFunc to its concrete implementation.
//
// Why init() and not a constructor parameter?
// Wave-agent compilation independence: critic.go (which provides
// LaunchParallelScouts) may not be compiled in all binary variants
// (e.g., slim builds that exclude the critic package). Using a function
// variable with init() wiring keeps the dependency optional at link time,
// so the main engine package compiles without critic.go if needed.
//
// How to override in tests:
// Set launchParallelScoutsFunc to a stub before calling the function
// under test:
//
//	launchParallelScoutsFunc = func(ctx context.Context, opts LaunchParallelScoutsOpts) result.Result[LaunchParallelScoutsResult] {
//	    return result.NewSuccess(LaunchParallelScoutsResult{...})
//	}
//
// TODO: Replace with explicit constructor injection when the compilation
// independence constraint is lifted (i.e., critic.go is always compiled).
func init() {
	launchParallelScoutsFunc = LaunchParallelScouts
}
