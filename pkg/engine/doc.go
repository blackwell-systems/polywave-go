// Package engine provides high-level entrypoints for Scout, Wave, Scaffold, and Chat operations.
//
// # Scout
//
// RunScout generates an IMPL doc from a user prompt:
//
//	opts := engine.RunScoutOpts{
//	    Feature:    "Add user authentication to the API",
//	    RepoPath:   "/path/to/repo",
//	    ScoutModel: "claude-sonnet-4-6",
//	}
//	res := engine.RunScout(ctx, opts, nil)
//	if !res.OK() {
//	    log.Fatalf("Scout failed: %v", res.Errors())
//	}
//	implPath := res.Data().IMPLPath
//
// Scout analyzes the codebase, determines suitability, and generates:
//   - Wave/agent structure with file ownership
//   - Interface contracts
//   - Dependency graph
//   - Per-agent task prompts
//
// # Wave
//
// StartWave launches all agents for a given wave:
//
//	orch := orchestrator.New(config)
//	err := orch.StartWave(ctx, 1)
//	if err != nil {
//	    log.Fatalf("Wave 1 failed: %v", err)
//	}
//
// Agents execute in parallel using git worktree isolation. The orchestrator
// coordinates agent execution, publishes SSE events, and runs quality gates.
//
// # Scaffold
//
// RunScaffold executes the Scaffold Agent to materialize shared types/interfaces:
//
//	opts := engine.RunScaffoldOpts{
//	    ImplPath: implPath,
//	    RepoPath: repoPath,
//	}
//	res := engine.RunScaffold(opts)
//	if !res.OK() {
//	    log.Fatalf("Scaffold failed: %v", res.Errors())
//	}
//	data := res.Data() // ScaffoldData{IMPLPath, ScaffoldsFound}
//
// The Scaffold Agent writes shared types that multiple wave agents depend on,
// then runs build verification to ensure the scaffold compiles.
//
// # Chat
//
// RunChat provides standalone chat with Claude (no IMPL doc):
//
//	opts := engine.RunChatOpts{
//	    IMPLPath: implPath,
//	    RepoPath: "/path/to/repo",
//	    Message:  "What does this module do?",
//	}
//	res := engine.RunChat(ctx, opts, onChunk)
//	if !res.OK() {
//	    log.Fatalf("Chat failed: %v", res.Errors())
//	}
//
// # Multi-Wave Orchestration
//
// To run a full multi-wave feature:
//
//	// 1. Scout generates IMPL doc
//	implPath, err := engine.RunScout(ctx, scoutOpts)
//
//	// 2. Parse IMPL doc
//	doc, err := protocol.ParseIMPLDoc(implPath)
//
//	// 3. Run all waves in sequence
//	orch := orchestrator.New(config)
//	for waveNum := 1; waveNum <= len(doc.Waves); waveNum++ {
//	    err = orch.StartWave(ctx, waveNum)
//	    err = orch.MergeWave(ctx, waveNum)
//	    err = orch.RunVerification(ctx)
//	}
//
// See docs/reference/architecture.md for engine flow and examples/library-usage/ for complete examples.
//
// # Step Pipeline
//
// Step functions are the individual units of work inside PrepareWave and
// FinalizeWave. Each exported Step* function (e.g. StepVerifyCommits,
// StepRunGates, StepMergeAgents) follows a common signature:
//
//	func StepMyStep(ctx context.Context, opts FinalizeWaveOpts,
//	    manifest *protocol.IMPLManifest,
//	    onEvent EventCallback) (*StepResult, *protocol.MyStepData, error)
//
// Every step takes a context, the wave options struct (FinalizeWaveOpts or
// PrepareWaveOpts), and an EventCallback; steps that need the parsed IMPL doc
// also take a *protocol.IMPLManifest. Returns a *StepResult, an optional typed
// data pointer (e.g. *protocol.VerifyCommitsData), and an error. Steps emit
// events via EventCallback at start ("running") and at completion ("complete"
// or "failed"). All steps are nil-safe with respect to EventCallback — the
// callback may be nil and steps guard before calling.
//
// FinalizeWave assembles the finalize pipeline (~15 steps, including
// StepGoWorkRestore) and PrepareWave assembles the prepare pipeline (~14 steps,
// including StepGoWorkSetup). Both pipelines stop on the first fatal error and
// return a partial result so callers can inspect which step failed. Individual
// steps are exported and callable independently for testing, CLI integration
// (polywave-tools finalize-wave --step-by-step), or custom orchestration.
//
// EventCallback is the single extension point that separates CLI and web
// behavior without branching in the engine:
//   - CLI: passes a callback that prints step name and status to stdout
//   - Web app: passes a callback that publishes SSE events to connected browser clients
//   - Tests / programmatic use: may pass nil (all steps guard nil before calling)
//
// This pattern means the engine has no import of CLI or web packages; the caller
// injects behavior.
//
// To add a new step:
//  1. Add the step function to finalize_steps.go (for finalize pipeline) or
//     prepare.go (for prepare pipeline) following the signature pattern above.
//  2. Call emitStepEvent(onEvent, stepName, "running", "") at the start and
//     emitStepEvent(onEvent, stepName, "complete"|"failed", detail) at the end.
//  3. Insert the call site in FinalizeWave (finalize.go) at the correct pipeline position.
//  4. Write a doc comment following the pattern of existing steps: one sentence
//     stating what the step checks, one sentence stating its fatality (fatal or
//     non-fatal), and any relevant rule codes (E*, I*, M*, H*) in parentheses.
//
package engine
