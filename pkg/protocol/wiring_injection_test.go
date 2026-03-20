package protocol

import (
	"strings"
	"testing"
)

func TestInjectWiringInstructions(t *testing.T) {
	tests := []struct {
		name      string
		doc       *IMPLManifest
		agentID   string
		waveNum   int
		wantEmpty bool
		wantHas   []string
		wantNot   []string
	}{
		{
			name: "agent with single wiring entry in current wave",
			doc: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{
						Symbol:             "RegisterHandler",
						DefinedIn:          "pkg/feature/handler.go",
						MustBeCalledFrom:   "cmd/main.go",
						Agent:              "A",
						Wave:               1,
						IntegrationPattern: "register",
					},
				},
			},
			agentID:   "A",
			waveNum:   1,
			wantEmpty: false,
			wantHas: []string{
				"## Wiring Obligations (E35)",
				"RegisterHandler",
				"pkg/feature/handler.go",
				"cmd/main.go",
				"register",
				"| Symbol | Defined In | Must Be Called From | Pattern |",
			},
		},
		{
			name: "agent with multiple wiring entries in current wave",
			doc: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{
						Symbol:             "InitDB",
						DefinedIn:          "pkg/db/init.go",
						MustBeCalledFrom:   "cmd/main.go",
						Agent:              "B",
						Wave:               2,
						IntegrationPattern: "call",
					},
					{
						Symbol:             "RegisterRoutes",
						DefinedIn:          "pkg/api/routes.go",
						MustBeCalledFrom:   "pkg/server/server.go",
						Agent:              "B",
						Wave:               2,
						IntegrationPattern: "append",
					},
				},
			},
			agentID:   "B",
			waveNum:   2,
			wantEmpty: false,
			wantHas: []string{
				"## Wiring Obligations (E35)",
				"InitDB",
				"RegisterRoutes",
				"pkg/db/init.go",
				"pkg/api/routes.go",
				"cmd/main.go",
				"pkg/server/server.go",
				"call",
				"append",
			},
		},
		{
			name: "agent with no wiring entries",
			doc: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{
						Symbol:             "SomeFunc",
						DefinedIn:          "pkg/other/func.go",
						MustBeCalledFrom:   "pkg/caller.go",
						Agent:              "C",
						Wave:               1,
						IntegrationPattern: "call",
					},
				},
			},
			agentID:   "A",
			waveNum:   1,
			wantEmpty: true,
		},
		{
			name: "agent in different wave - should not see entries",
			doc: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{
						Symbol:             "Wave1Func",
						DefinedIn:          "pkg/wave1/func.go",
						MustBeCalledFrom:   "pkg/main.go",
						Agent:              "A",
						Wave:               1,
						IntegrationPattern: "call",
					},
					{
						Symbol:             "Wave2Func",
						DefinedIn:          "pkg/wave2/func.go",
						MustBeCalledFrom:   "pkg/main.go",
						Agent:              "A",
						Wave:               2,
						IntegrationPattern: "call",
					},
				},
			},
			agentID:   "A",
			waveNum:   1,
			wantEmpty: false,
			wantHas: []string{
				"Wave1Func",
				"pkg/wave1/func.go",
			},
			wantNot: []string{
				"Wave2Func",
				"pkg/wave2/func.go",
			},
		},
		{
			name: "multiple agents, same wave - only show current agent",
			doc: &IMPLManifest{
				Wiring: []WiringDeclaration{
					{
						Symbol:             "AgentAFunc",
						DefinedIn:          "pkg/a/func.go",
						MustBeCalledFrom:   "pkg/main.go",
						Agent:              "A",
						Wave:               1,
						IntegrationPattern: "call",
					},
					{
						Symbol:             "AgentBFunc",
						DefinedIn:          "pkg/b/func.go",
						MustBeCalledFrom:   "pkg/main.go",
						Agent:              "B",
						Wave:               1,
						IntegrationPattern: "call",
					},
				},
			},
			agentID:   "A",
			waveNum:   1,
			wantEmpty: false,
			wantHas: []string{
				"AgentAFunc",
				"pkg/a/func.go",
			},
			wantNot: []string{
				"AgentBFunc",
				"pkg/b/func.go",
			},
		},
		{
			name: "empty wiring slice",
			doc: &IMPLManifest{
				Wiring: []WiringDeclaration{},
			},
			agentID:   "A",
			waveNum:   1,
			wantEmpty: true,
		},
		{
			name:      "nil wiring slice",
			doc:       &IMPLManifest{},
			agentID:   "A",
			waveNum:   1,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InjectWiringInstructions(tt.doc, tt.agentID, tt.waveNum)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty string, got:\n%s", got)
				}
				return
			}

			if got == "" {
				t.Fatal("expected non-empty string, got empty")
			}

			// Check for required content
			for _, want := range tt.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("output missing required content %q\ngot:\n%s", want, got)
				}
			}

			// Check for excluded content
			for _, notWant := range tt.wantNot {
				if strings.Contains(got, notWant) {
					t.Errorf("output contains unwanted content %q\ngot:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestInjectWiringInstructions_OutputFormat(t *testing.T) {
	doc := &IMPLManifest{
		Wiring: []WiringDeclaration{
			{
				Symbol:             "TestFunc",
				DefinedIn:          "pkg/test.go",
				MustBeCalledFrom:   "pkg/caller.go",
				Agent:              "X",
				Wave:               1,
				IntegrationPattern: "call",
			},
		},
	}

	got := InjectWiringInstructions(doc, "X", 1)

	// Verify it starts with proper markdown heading
	if !strings.HasPrefix(got, "\n## Wiring Obligations (E35)\n\n") {
		t.Errorf("expected output to start with wiring obligations heading")
	}

	// Verify table structure
	if !strings.Contains(got, "| Symbol | Defined In | Must Be Called From | Pattern |") {
		t.Errorf("expected markdown table header")
	}

	if !strings.Contains(got, "|--------|------------|---------------------|--------|") {
		t.Errorf("expected markdown table separator")
	}

	// Verify row format
	expectedRow := "| TestFunc | pkg/test.go | pkg/caller.go | call |"
	if !strings.Contains(got, expectedRow) {
		t.Errorf("expected table row:\n  %s\ngot:\n%s", expectedRow, got)
	}

	// Verify trailing newline for clean section separation
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected output to end with newline")
	}
}
