// Package engine provides high-level entrypoints for Scout, Wave, Scaffold, and Chat operations.
//
// # Scout
//
// RunScout generates an IMPL doc from a user prompt:
//
//	opts := engine.RunScoutOpts{
//	    Prompt:     "Add user authentication to the API",
//	    RepoPath:   "/path/to/repo",
//	    ScoutModel: "claude-sonnet-4-6",
//	}
//	implPath, err := engine.RunScout(ctx, opts)
//	if err != nil {
//	    log.Fatalf("Scout failed: %v", err)
//	}
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
//	err := engine.RunScaffold(ctx, implPath, repoPath)
//	if err != nil {
//	    log.Fatalf("Scaffold failed: %v", err)
//	}
//
// The Scaffold Agent writes shared types that multiple wave agents depend on,
// then runs build verification to ensure the scaffold compiles.
//
// # Chat
//
// RunChat provides standalone chat with Claude (no IMPL doc):
//
//	opts := engine.ChatOpts{
//	    Model:      "claude-sonnet-4-6",
//	    RepoPath:   "/path/to/repo",
//	    SystemPrompt: "You are a helpful coding assistant",
//	}
//	err := engine.RunChat(ctx, opts, onChunk)
//	if err != nil {
//	    log.Fatalf("Chat failed: %v", err)
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
package engine
