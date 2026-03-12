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
