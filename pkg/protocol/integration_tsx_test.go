package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTSXExports(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantLen  int
		wantHas  map[string]string // name -> kind
	}{
		{
			name: "exported default function",
			content: `import React from 'react'
export default function MyComponent({ title }: Props) {
  return <div>{title}</div>
}`,
			wantLen: 1,
			wantHas: map[string]string{"MyComponent": "func"},
		},
		{
			name: "props interface with callback",
			content: `interface LiveRailProps {
  slug: string
  onRescout?: () => void
  onClose: () => void
}

export default function LiveRail({ slug, onRescout, onClose }: LiveRailProps) {
  return <div />
}`,
			wantLen: 5, // LiveRail(func) + LiveRailProps(type via export check) + slug, onRescout, onClose (props)
			wantHas: map[string]string{
				"LiveRail": "func",
				"onRescout": "prop",
				"onClose":   "prop",
				"slug":      "prop",
			},
		},
		{
			name: "exported named function and const",
			content: `export function helper() {}
export const API_URL = '/api'
export type Config = { debug: boolean }`,
			wantLen: 3,
			wantHas: map[string]string{
				"helper":  "func",
				"API_URL": "func", // export const detected as func by regex
				"Config":  "type",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "component.tsx")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			exports, err := extractTSXExports(path)
			if err != nil {
				t.Fatalf("extractTSXExports: %v", err)
			}

			if len(exports) < len(tc.wantHas) {
				t.Errorf("got %d exports, want at least %d; exports: %+v", len(exports), len(tc.wantHas), exports)
			}

			exportMap := map[string]string{}
			for _, e := range exports {
				exportMap[e.Name] = e.Kind
			}

			for name, kind := range tc.wantHas {
				got, ok := exportMap[name]
				if !ok {
					t.Errorf("missing export %q", name)
				} else if got != kind {
					t.Errorf("export %q: got kind %q, want %q", name, got, kind)
				}
			}
		})
	}
}

func TestSymbolFoundViaTSXProp(t *testing.T) {
	cases := []struct {
		name    string
		content string
		symbol  string
		want    bool
	}{
		{
			name: "prop passed as JSX attribute",
			content: `<LiveRail
  slug={selectedSlug}
  onRescout={() => setLiveView('scout')}
  onClose={handleClose}
/>`,
			symbol: "onRescout",
			want:   true,
		},
		{
			name: "prop in TODO comment only",
			content: `<LiveRail
  slug={selectedSlug}
  // TODO(integration): wire onRescout={() => setLiveView('scout')} after Agent E adds prop
  onClose={handleClose}
/>`,
			symbol: "onRescout",
			want:   false,
		},
		{
			name: "prop in interface definition only (not usage)",
			content: `interface LiveRailProps {
  slug: string
  onRescout?: () => void
  onClose: () => void
}`,
			symbol: "onRescout",
			want:   false,
		},
		{
			name: "prop called with optional chaining",
			content: `function LiveRail({ onRescout }: Props) {
  return <button onClick={() => onRescout?.()}>Rescout</button>
}`,
			symbol: "onRescout",
			want:   true,
		},
		{
			name:    "symbol not present at all",
			content: `<LiveRail slug={slug} onClose={close} />`,
			symbol:  "onRescout",
			want:    false,
		},
		{
			name: "prop accessed via props object",
			content: `function LiveRail(props: Props) {
  if (props.onRescout) props.onRescout()
}`,
			symbol: "onRescout",
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "App.tsx")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := symbolFoundViaTSXProp(path, tc.symbol)
			if err != nil {
				t.Fatalf("symbolFoundViaTSXProp: %v", err)
			}
			if got != tc.want {
				t.Errorf("symbolFoundViaTSXProp(%q): got %v, want %v", tc.symbol, got, tc.want)
			}
		})
	}
}

func TestWiringValidation_TSX(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		symbol    string
		wantValid bool
	}{
		{
			name: "prop wired in JSX — PASS",
			content: `<LiveRail
  slug={slug}
  onRescout={() => setLiveView('scout')}
/>`,
			symbol:    "onRescout",
			wantValid: true,
		},
		{
			name: "prop only in TODO — FAIL",
			content: `<LiveRail
  slug={slug}
  // TODO: wire onRescout
/>`,
			symbol:    "onRescout",
			wantValid: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "App.tsx")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			manifest := &IMPLManifest{
				Wiring: []WiringDeclaration{
					{
						Symbol:             tc.symbol,
						DefinedIn:          "web/src/components/LiveRail.tsx",
						MustBeCalledFrom:   "App.tsx",
						Agent:              "E",
						Wave:               1,
						IntegrationPattern: "prop-pass",
					},
				},
			}

			res := ValidateWiringDeclarations(manifest, dir)
			if res.IsFatal() {
				t.Fatalf("ValidateWiringDeclarations: %+v", res.Errors)
			}
			result := res.GetData()
			if result.Valid != tc.wantValid {
				t.Errorf("Valid: got %v, want %v; gaps: %+v", result.Valid, tc.wantValid, result.Gaps)
			}
		})
	}
}

func TestIsIntegrationRequired_Prop(t *testing.T) {
	cases := []struct {
		name string
		prop string
		want bool
	}{
		{"callback prop on*", "onRescout", true},
		{"callback prop onChange", "onChange", true},
		{"handler prop", "handleSubmit", true},
		{"data prop", "slug", false},
		{"data prop title", "title", false},
		{"boolean prop", "compact", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsIntegrationRequired(tc.prop, "prop_pass")
			if got != tc.want {
				t.Errorf("IsIntegrationRequired(%q, prop_pass): got %v, want %v", tc.prop, got, tc.want)
			}
		})
	}
}
