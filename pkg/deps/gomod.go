package deps

import (
	"bufio"
	"os"
	"strings"
)

// ParseGoModReplace reads goModPath and returns the set of module paths
// declared in replace directives that point to a local directory
// (i.e. the replacement is a relative or absolute filesystem path, not
// a versioned module). Returns an empty map (not nil) on any error.
//
// Example go.mod fragment handled:
//
//	replace github.com/blackwell-systems/scout-and-wave-go => ../scout-and-wave-go
//
// The returned map key is the original module path (before "=>").
func ParseGoModReplace(goModPath string) map[string]struct{} {
	result := make(map[string]struct{})

	f, err := os.Open(goModPath)
	if err != nil {
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inReplaceBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect start of replace block: "replace ("
		if line == "replace (" {
			inReplaceBlock = true
			continue
		}

		// Detect end of replace block
		if inReplaceBlock && line == ")" {
			inReplaceBlock = false
			continue
		}

		// Handle single-line replace: "replace A => B"
		if !inReplaceBlock && strings.HasPrefix(line, "replace ") {
			directive := strings.TrimPrefix(line, "replace ")
			if mod, ok := parseReplaceDirective(directive); ok {
				result[mod] = struct{}{}
			}
			continue
		}

		// Inside a replace block, each line is a directive: "A => B"
		if inReplaceBlock {
			if mod, ok := parseReplaceDirective(line); ok {
				result[mod] = struct{}{}
			}
		}
	}

	// On scanner error, return what we have (defensive)
	return result
}

// parseReplaceDirective parses a single replace directive of the form "A => B"
// (with optional version on left side: "A v1.0.0 => B").
// Returns the original module path and true if the replacement is local
// (no "@" version suffix on the right side). Returns "", false otherwise.
func parseReplaceDirective(directive string) (string, bool) {
	parts := strings.SplitN(directive, "=>", 2)
	if len(parts) != 2 {
		return "", false
	}

	// Left side: module path, optionally followed by a version
	// e.g. "github.com/foo/bar" or "github.com/foo/bar v1.2.3"
	leftFields := strings.Fields(strings.TrimSpace(parts[0]))
	if len(leftFields) == 0 {
		return "", false
	}
	modulePath := leftFields[0]

	// Right side: replacement — local if it doesn't contain "@"
	replacement := strings.TrimSpace(parts[1])
	if replacement == "" {
		return "", false
	}

	// Versioned replacements always include "@vX.Y.Z"; local dirs do not.
	replacementModule := strings.Fields(replacement)[0]
	if strings.Contains(replacementModule, "@") {
		return "", false
	}

	return modulePath, true
}
