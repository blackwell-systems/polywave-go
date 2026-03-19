package protocol

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Regex patterns for TSX parsing and goroutine detection.
var (
	exportDefaultFuncPattern  = regexp.MustCompile(`export\s+default\s+function\s+(\w+)`)
	exportDefaultConstPattern = regexp.MustCompile(`export\s+default\s+(\w+)`)

	goFuncPattern  = regexp.MustCompile(`\bgo\s+(func\(|[\w\.]+\()`)
	tickerPattern  = regexp.MustCompile(`time\.NewTicker|time\.Ticker`)
	contextPattern = regexp.MustCompile(`context\.Background\(\)`)
)

// PopulateIntegrationChecklist analyzes file_ownership and populates
// post_merge_checklist with integration tasks. Returns a new manifest
// copy. If post_merge_checklist already has items, appends new groups.
func PopulateIntegrationChecklist(m *IMPLManifest) (*IMPLManifest, error) {
	// Create a shallow copy of the manifest
	result := *m

	// Determine the repo root - use empty string for single-repo IMPLs
	// Files without repo: field are treated as relative to repoRoot=""
	repoRoot := ""
	for _, ownership := range m.FileOwnership {
		if ownership.Repo != "" {
			repoRoot = ownership.Repo
			break
		}
	}
	// For single-repo IMPLs, repoRoot stays ""

	// Detect patterns across all four categories
	apiItems := detectAPIHandlers(m.FileOwnership, repoRoot)
	reactItems := detectReactComponents(m.FileOwnership, repoRoot)
	cliItems := detectCLICommands(m.FileOwnership, repoRoot)
	bgItems := detectBackgroundServices(m.FileOwnership, repoRoot)

	// Build groups from detected items
	type groupSpec struct {
		title string
		items []ChecklistItem
	}
	groupSpecs := []groupSpec{
		{"API Route Registration", apiItems},
		{"React Navigation Wiring", reactItems},
		{"CLI Command Registration", cliItems},
		{"Background Service Initialization", bgItems},
	}

	// Copy existing post_merge_checklist or create new one
	var checklist PostMergeChecklist
	if m.PostMergeChecklist != nil {
		// Deep copy existing groups
		checklist.Groups = make([]ChecklistGroup, len(m.PostMergeChecklist.Groups))
		for i, g := range m.PostMergeChecklist.Groups {
			checklist.Groups[i] = ChecklistGroup{
				Title: g.Title,
				Items: make([]ChecklistItem, len(g.Items)),
			}
			copy(checklist.Groups[i].Items, g.Items)
		}
	}

	// For each detected group, merge idempotently
	for _, spec := range groupSpecs {
		if len(spec.items) == 0 {
			continue
		}

		// Find existing group with same title
		existingIdx := -1
		for i, g := range checklist.Groups {
			if g.Title == spec.title {
				existingIdx = i
				break
			}
		}

		if existingIdx == -1 {
			// Group doesn't exist yet - add it in full
			checklist.Groups = append(checklist.Groups, ChecklistGroup{
				Title: spec.title,
				Items: spec.items,
			})
		} else {
			// Group exists - only add items whose Description doesn't already exist
			for _, newItem := range spec.items {
				alreadyPresent := false
				for _, existingItem := range checklist.Groups[existingIdx].Items {
					if existingItem.Description == newItem.Description {
						alreadyPresent = true
						break
					}
				}
				if !alreadyPresent {
					checklist.Groups[existingIdx].Items = append(checklist.Groups[existingIdx].Items, newItem)
				}
			}
		}
	}

	// Only set PostMergeChecklist if we have groups
	if len(checklist.Groups) > 0 {
		result.PostMergeChecklist = &checklist
	} else if m.PostMergeChecklist != nil {
		// Preserve existing checklist even if we added nothing
		result.PostMergeChecklist = &checklist
	}

	return &result, nil
}

// detectAPIHandlers scans file_ownership for new API handlers and
// returns checklist items for route registration.
func detectAPIHandlers(ownership []FileOwnership, repoRoot string) []ChecklistItem {
	var items []ChecklistItem

	for _, f := range ownership {
		if f.Action != "new" {
			continue
		}

		// Match pkg/api/*_handler.go
		if !matchesAPIHandlerPattern(f.File) {
			continue
		}

		// Resolve file path
		filePath := resolveFilePath(f.File, f.Repo, repoRoot)

		// Parse the handler file
		funcs, err := parseHandlerFunctions(filePath)
		if err != nil || len(funcs) == 0 {
			continue
		}

		// Build description and command
		funcList := strings.Join(funcs, ", ")
		description := fmt.Sprintf("Register routes in server.go for %s", funcList)

		// Use first function for the grep command (representative)
		command := fmt.Sprintf("grep '%s' pkg/api/server.go", funcs[0])

		items = append(items, ChecklistItem{
			Description: description,
			Command:     command,
		})
	}

	return items
}

// detectReactComponents scans file_ownership for new React components
// and returns checklist items for App.tsx navigation wiring.
func detectReactComponents(ownership []FileOwnership, repoRoot string) []ChecklistItem {
	var items []ChecklistItem

	for _, f := range ownership {
		if f.Action != "new" {
			continue
		}

		// Match web/src/components/*.tsx
		if !matchesReactComponentPattern(f.File) {
			continue
		}

		// Resolve file path
		filePath := resolveFilePath(f.File, f.Repo, repoRoot)

		// Parse component name
		componentName, err := parseReactComponentName(filePath)
		if err != nil || componentName == "" {
			continue
		}

		description := fmt.Sprintf("Add %s to App.tsx navigation", componentName)
		command := fmt.Sprintf("grep '%s' web/src/App.tsx", componentName)

		items = append(items, ChecklistItem{
			Description: description,
			Command:     command,
		})
	}

	return items
}

// detectCLICommands scans file_ownership for new CLI commands and
// returns checklist items for main.go registration.
func detectCLICommands(ownership []FileOwnership, repoRoot string) []ChecklistItem {
	var items []ChecklistItem

	for _, f := range ownership {
		if f.Action != "new" {
			continue
		}

		// Match cmd/saw/*_cmd.go or cmd/*_cmd.go
		if !matchesCLICommandPattern(f.File) {
			continue
		}

		// Resolve file path
		filePath := resolveFilePath(f.File, f.Repo, repoRoot)

		// Parse CLI command name
		cmdName, err := parseCLICommandName(filePath)
		if err != nil || cmdName == "" {
			continue
		}

		description := fmt.Sprintf("Register %s in cmd/saw/main.go", cmdName)
		// cmdName is e.g. "populate-integration-checklist", newXCmd is camelCase
		newFuncName := commandNameToFuncName(cmdName)
		command := fmt.Sprintf("grep '%s' cmd/saw/main.go", newFuncName)

		items = append(items, ChecklistItem{
			Description: description,
			Command:     command,
		})
	}

	return items
}

// detectBackgroundServices scans new files for goroutine patterns and
// returns checklist items for service initialization.
func detectBackgroundServices(ownership []FileOwnership, repoRoot string) []ChecklistItem {
	var items []ChecklistItem

	for _, f := range ownership {
		if f.Action != "new" {
			continue
		}

		// Only check Go files
		if !strings.HasSuffix(f.File, ".go") {
			continue
		}

		// Resolve file path
		filePath := resolveFilePath(f.File, f.Repo, repoRoot)

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		text := string(content)

		// Check for goroutine patterns
		hasGoroutine := goFuncPattern.MatchString(text)
		hasTicker := tickerPattern.MatchString(text)
		hasContext := contextPattern.MatchString(text)

		if hasGoroutine || hasTicker || hasContext {
			filename := filepath.Base(f.File)
			description := fmt.Sprintf("Verify if %s needs initialization in Server constructor", filename)

			items = append(items, ChecklistItem{
				Description: description,
			})
		}
	}

	return items
}

// parseHandlerFunctions extracts handler function names from a Go file.
// Returns empty slice if file cannot be parsed or has no handler functions.
func parseHandlerFunctions(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, err
	}

	var handlers []string

	ast.Inspect(f, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if it's a method with receiver *Server
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			return true
		}

		// Check receiver type is *Server
		recv := funcDecl.Recv.List[0]
		starExpr, ok := recv.Type.(*ast.StarExpr)
		if !ok {
			return true
		}
		ident, ok := starExpr.X.(*ast.Ident)
		if !ok || ident.Name != "Server" {
			return true
		}

		// Check function name starts with "handle"
		name := funcDecl.Name.Name
		if strings.HasPrefix(name, "handle") {
			handlers = append(handlers, name)
		}

		return true
	})

	return handlers, nil
}

// parseReactComponentName extracts the default exported component name
// from a TSX file. Returns empty string if not found.
func parseReactComponentName(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	text := string(content)

	// Try "export default function ComponentName" first
	if m := exportDefaultFuncPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1], nil
	}

	// Fallback: "export default ComponentName"
	if m := exportDefaultConstPattern.FindStringSubmatch(text); len(m) > 1 {
		name := m[1]
		// Skip keywords that aren't component names
		if name != "function" && name != "class" && name != "const" && name != "let" {
			return name, nil
		}
	}

	return "", nil
}

// parseCLICommandName extracts the cobra command name from a Go file.
// Returns empty string if no command found.
func parseCLICommandName(filePath string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return "", err
	}

	// Walk AST looking for cobra.Command struct literal with Use: field
	var cmdName string

	ast.Inspect(f, func(n ast.Node) bool {
		if cmdName != "" {
			return false
		}

		// Look for composite literals that could be cobra.Command
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Check if it's a cobra.Command type
		sel, ok := cl.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		xIdent, ok := sel.X.(*ast.Ident)
		if !ok || xIdent.Name != "cobra" || sel.Sel.Name != "Command" {
			return true
		}

		// Extract the Use: field value
		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Use" {
				continue
			}
			val, ok := kv.Value.(*ast.BasicLit)
			if !ok {
				continue
			}
			// Strip quotes and extract command name (may include args like "cmd-name [flags]")
			use := strings.Trim(val.Value, `"`)
			// Take only the first word (command name without args)
			parts := strings.Fields(use)
			if len(parts) > 0 {
				cmdName = parts[0]
			}
		}

		return true
	})

	return cmdName, nil
}

// matchesAPIHandlerPattern returns true if the file path matches pkg/api/*_handler.go
func matchesAPIHandlerPattern(file string) bool {
	// Normalize separators
	file = filepath.ToSlash(file)
	// Must be in pkg/api/ directory
	if !strings.HasPrefix(file, "pkg/api/") {
		return false
	}
	// Must end with _handler.go
	base := filepath.Base(file)
	return strings.HasSuffix(base, "_handler.go")
}

// matchesReactComponentPattern returns true if the file path matches
// web/src/components/*.tsx but excludes ui/ subdirectory and test files.
func matchesReactComponentPattern(file string) bool {
	file = filepath.ToSlash(file)

	// Must be in web/src/components/
	if !strings.HasPrefix(file, "web/src/components/") {
		return false
	}

	// Must end with .tsx
	if !strings.HasSuffix(file, ".tsx") {
		return false
	}

	// Exclude components/ui/ directory (UI primitives)
	if strings.Contains(file, "/ui/") {
		return false
	}

	// Exclude test files
	base := filepath.Base(file)
	if strings.HasSuffix(base, "test.tsx") || strings.HasSuffix(base, "spec.tsx") {
		return false
	}

	// Only match direct children (not subdirectories) - per spec: *.tsx
	// Strip the prefix and check there's no slash in the remainder
	remainder := strings.TrimPrefix(file, "web/src/components/")
	if strings.Contains(remainder, "/") {
		return false
	}

	return true
}

// matchesCLICommandPattern returns true if the file path matches
// cmd/saw/*_cmd.go or cmd/*_cmd.go
func matchesCLICommandPattern(file string) bool {
	file = filepath.ToSlash(file)
	base := filepath.Base(file)
	if !strings.HasSuffix(base, "_cmd.go") {
		return false
	}

	// Must be in cmd/ tree
	return strings.HasPrefix(file, "cmd/")
}

// resolveFilePath resolves a file path relative to the appropriate repo root.
// If file has no repo (or repo is empty/"."), it's relative to repoRoot.
func resolveFilePath(file, fileRepo, defaultRepoRoot string) string {
	root := defaultRepoRoot
	if fileRepo != "" && fileRepo != "." {
		root = fileRepo
	}
	if root == "" {
		return file
	}
	return filepath.Join(root, file)
}

// commandNameToFuncName converts a kebab-case command name to its Go
// constructor function name (e.g., "populate-integration-checklist" -> "newPopulateIntegrationChecklistCmd").
func commandNameToFuncName(cmdName string) string {
	parts := strings.Split(cmdName, "-")
	result := "new"
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		result += strings.ToUpper(p[:1]) + p[1:]
	}
	result += "Cmd"
	return result
}
