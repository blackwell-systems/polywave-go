package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverLintGate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		want    string
		wantErr bool
	}{
		{
			name:  "no docs/IMPL directory returns empty",
			setup: func(t *testing.T, dir string) {},
			want:  "",
		},
		{
			name: "IMPL present but state=COMPLETE is skipped",
			setup: func(t *testing.T, dir string) {
				implDir := filepath.Join(dir, "docs", "IMPL")
				if err := os.MkdirAll(implDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeYAML(t, filepath.Join(implDir, "IMPL-done.yaml"), `
title: done feature
feature_slug: done
state: COMPLETE
quality_gates:
  level: standard
  gates:
    - type: lint
      command: "golangci-lint run"
      required: true
`)
			},
			want: "",
		},
		{
			name: "active IMPL with lint gate returns command",
			setup: func(t *testing.T, dir string) {
				implDir := filepath.Join(dir, "docs", "IMPL")
				if err := os.MkdirAll(implDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeYAML(t, filepath.Join(implDir, "IMPL-active.yaml"), `
title: active feature
feature_slug: active
state: WAVE_PENDING
quality_gates:
  level: standard
  gates:
    - type: build
      command: "go build ./..."
      required: true
    - type: lint
      command: "go vet ./..."
      required: true
`)
			},
			want: "go vet ./...",
		},
		{
			name: "active IMPL without lint gate falls back to saw.config.json",
			setup: func(t *testing.T, dir string) {
				implDir := filepath.Join(dir, "docs", "IMPL")
				if err := os.MkdirAll(implDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeYAML(t, filepath.Join(implDir, "IMPL-nolint.yaml"), `
title: no lint feature
feature_slug: nolint
state: WAVE_EXECUTING
quality_gates:
  level: quick
  gates:
    - type: build
      command: "go build ./..."
      required: true
`)
				writeJSON(t, filepath.Join(dir, "saw.config.json"), `{"lint_command": "eslint ."}`)
			},
			want: "eslint .",
		},
		{
			name: "neither IMPL lint gate nor config returns empty",
			setup: func(t *testing.T, dir string) {
				implDir := filepath.Join(dir, "docs", "IMPL")
				if err := os.MkdirAll(implDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeYAML(t, filepath.Join(implDir, "IMPL-nolint.yaml"), `
title: no lint
feature_slug: nolint
state: WAVE_PENDING
quality_gates:
  level: quick
  gates:
    - type: build
      command: "go build ./..."
      required: true
`)
			},
			want: "",
		},
		{
			name: "NOT_SUITABLE state is skipped",
			setup: func(t *testing.T, dir string) {
				implDir := filepath.Join(dir, "docs", "IMPL")
				if err := os.MkdirAll(implDir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeYAML(t, filepath.Join(implDir, "IMPL-unsuitable.yaml"), `
title: unsuitable
feature_slug: unsuitable
state: NOT_SUITABLE
quality_gates:
  level: standard
  gates:
    - type: lint
      command: "should-not-find-this"
      required: true
`)
			},
			want: "",
		},
		{
			name: "malformed YAML is skipped gracefully",
			setup: func(t *testing.T, dir string) {
				implDir := filepath.Join(dir, "docs", "IMPL")
				if err := os.MkdirAll(implDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(implDir, "IMPL-bad.yaml"), []byte(":::\nbad: [yaml"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			got, err := DiscoverLintGate(dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DiscoverLintGate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("DiscoverLintGate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
