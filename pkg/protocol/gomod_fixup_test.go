package protocol

import "testing"

func TestFixDeepReplacePaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "deep paths get corrected",
			input: `module example.com/myapp

go 1.25.0

replace (
	github.com/foo/bar => ../../../../bar
	github.com/foo/baz => ../../../../baz
)`,
			expected: `module example.com/myapp

go 1.25.0

replace (
	github.com/foo/bar => ../bar
	github.com/foo/baz => ../baz
)`,
		},
		{
			name: "correct paths unchanged",
			input: `module example.com/myapp

replace github.com/foo/bar => ../bar
`,
			expected: `module example.com/myapp

replace github.com/foo/bar => ../bar
`,
		},
		{
			name: "no replace block unchanged",
			input: `module example.com/myapp

go 1.25.0
`,
			expected: `module example.com/myapp

go 1.25.0
`,
		},
		{
			name: "mixed deep and correct",
			input: `replace (
	github.com/a/b => ../b
	github.com/c/d => ../../../../../../d
)`,
			expected: `replace (
	github.com/a/b => ../b
	github.com/c/d => ../d
)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixDeepReplacePaths(tt.input)
			if got != tt.expected {
				t.Errorf("fixDeepReplacePaths():\ngot:\n%s\nwant:\n%s", got, tt.expected)
			}
		})
	}
}
