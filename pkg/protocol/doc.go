// Package protocol provides YAML-only IMPL manifest parsing, validation, and extraction.
//
// # IMPL Manifest Format
//
// IMPL manifests are structured YAML documents:
//
//	title: Add User Authentication
//	feature_slug: add-user-authentication
//	verdict: SUITABLE
//
//	interface_contracts:
//	  - name: Authenticator
//	    definition: |
//	      type Authenticator interface {
//	        Login(username, password string) error
//	      }
//
//	file_ownership:
//	  - file: src/auth.go
//	    agent: A
//	    wave: 1
//	    action: new
//
//	waves:
//	  - number: 1
//	    agents:
//	      - letter: A
//	        task: Implement user authentication logic
//	        completion:
//	          status: complete
//	          files_changed:
//	            - src/auth.go
//
// See the scout-and-wave protocol spec for full format definition:
// https://github.com/blackwell-systems/scout-and-wave/tree/main/protocol
//
// # Loading and Saving
//
// Load reads a YAML manifest from disk:
//
//	manifest, err := protocol.Load("/path/to/IMPL-feature.yaml")
//	if err != nil {
//	    log.Fatalf("Load failed: %v", err)
//	}
//
//	fmt.Printf("Title: %s\n", manifest.Title)
//	fmt.Printf("Waves: %d\n", len(manifest.Waves))
//
// Save writes a manifest back to disk:
//
//	if err := protocol.Save(manifest, "/path/to/IMPL-feature.yaml"); err != nil {
//	    log.Fatalf("Save failed: %v", err)
//	}
//
// # Validation
//
// ValidateInvariants checks protocol invariants (I1–I6):
//
//	violations := protocol.ValidateInvariants(manifest)
//	if len(violations) > 0 {
//	    for _, v := range violations {
//	        fmt.Println("Invariant violation:", v)
//	    }
//	}
//
// # Validation Entry Points
//
// There are three public validation entry points. Choose based on what you have:
//
// FullValidate(path, opts) — primary entry point for CLI, web, and testing.
//
//	Loads the YAML file, optionally auto-fixes correctable issues (gate type
//	normalization, unknown-key stripping), then runs Validate and ValidateIMPLDoc.
//	Returns FullValidateData with counts, errors, warnings, and fix count.
//	Use this when you have a file path and want all checks.
//
// Validate(m) — structural and invariant checks on a parsed *IMPLManifest.
//
//	Runs: I1 disjoint ownership, I2 agent dependencies, I3 wave ordering,
//	I4 required fields, I5 file ownership completeness, I6 no cycles,
//	E9 merge state, SM01 state machine, agent IDs, gate types, worktree
//	names, verification fields, completion statuses, failure types, pre-mortem
//	risk, multi-repo consistency, schema, action enums, integration checklist,
//	file existence, known issue titles, agent complexity.
//	Does NOT detect unknown YAML keys (requires raw bytes).
//	Use when you already have a parsed manifest and need fast structural checks.
//
// ValidateIMPLDoc(path) — E16 typed-block validation on the raw file.
//
//	Reads raw lines; validates typed block content:
//	impl-file-ownership, impl-dep-graph, impl-wave-structure, impl-completion-report.
//	Also checks for out-of-band dep graphs (E16C) and agent ID format (I2).
//	Use when you need line-number-accurate block-level validation.
//
// ValidateBytes(data) — in-memory convenience wrapper.
//
//	Parses bytes, runs Validate + DetectUnknownKeys.
//	Does NOT run ValidateIMPLDoc (no file path for line-number reporting).
//	Use when you have raw YAML bytes (e.g., from an HTTP request body).
//
// # Per-Agent Context Extraction (E23)
//
// ExtractAgentContextFromManifest trims the manifest to only sections relevant to a single agent:
//
//	agentContext, err := protocol.ExtractAgentContextFromManifest(manifest, agentID)
//	// agentContext contains: agent task, interface contracts, file ownership, scaffolds, quality gates
//	// Other agents' tasks are omitted — reduces context waste
//
// # Architecture
//
// The protocol package provides a YAML-based implementation of the Scout-and-Wave
// protocol. All IMPL documents are now YAML manifests (*.yaml or *.yml).
// Markdown parsing support has been removed.
//
// See docs/protocol-parsing.md for manifest schema and architecture details.
package protocol
