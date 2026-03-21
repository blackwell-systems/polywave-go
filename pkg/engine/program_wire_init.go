package engine

// This file wires together components from different wave agents.
// Agent A (Wave 1) defined launchParallelScoutsFunc as a stub.
// Agent B (Wave 1) implemented LaunchParallelScouts.
// This init connects them.

func init() {
	launchParallelScoutsFunc = LaunchParallelScouts
}
