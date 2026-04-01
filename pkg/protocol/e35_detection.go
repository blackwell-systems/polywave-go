package protocol

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// E35Gap represents a same-package function call where the caller file is not
// owned by the agent that owns the function definition.
type E35Gap struct {
	Agent        string   `json:"agent"`         // Agent that owns the function definition
	FunctionName string   `json:"function_name"` // Name of the function/method
	DefinedIn    string   `json:"defined_in"`    // File where function is defined
	CalledFrom   []string `json:"called_from"`   // Files where function is called (not owned by agent)
	Package      string   `json:"package"`       // Go package path
}

// DetectE35Gaps scans for E35 violations: functions defined by one agent but
// called from files not owned by that agent, within the same package.
// Returns structured E35Gap results.
//
// Detection algorithm:
// 1. Build map of agent -> owned files for waveNum
// 2. For each agent's Go files, extract function declarations
// 3. For each function, find all files in same package
// 4. Check if any same-package file (not owned by agent) calls the function
// 5. Report gaps with file:line locations
func DetectE35Gaps(m *IMPLManifest, waveNum int, repoRoot string) ([]E35Gap, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is nil")
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("repoRoot is empty")
	}

	// Build agent file ownership map for this wave
	agentFiles := make(map[string]map[string]bool)
	for _, fo := range m.FileOwnership {
		if fo.Wave != waveNum {
			continue
		}
		if agentFiles[fo.Agent] == nil {
			agentFiles[fo.Agent] = make(map[string]bool)
		}
		agentFiles[fo.Agent][fo.File] = true
	}

	// Build agent -> package -> [functions] map
	type funcDef struct {
		name   string
		file   string
		pkg    string
		pkgDir string // absolute path to package directory
		agent  string
	}
	var allFuncs []funcDef

	// Extract function declarations from each agent's Go files
	for agentID, files := range agentFiles {
		for file := range files {
			if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
				continue
			}

			absPath := filepath.Join(repoRoot, file)
			funcs, pkgName, pkgDir, err := extractFunctions(absPath)
			if err != nil {
				// File might not exist yet (Scout planned new file) or not parseable
				continue
			}

			// Skip functions that already existed at HEAD — they are being modified,
			// not newly introduced. Only new functions require caller ownership (E35).
			existingAtHEAD := existingFunctionsInHEAD(repoRoot, file)

			for _, fn := range funcs {
				if existingAtHEAD[fn] {
					continue
				}
				allFuncs = append(allFuncs, funcDef{
					name:   fn,
					file:   file,
					pkg:    pkgName,
					pkgDir: pkgDir,
					agent:  agentID,
				})
			}
		}
	}

	// For each function, find call sites in same package
	var gaps []E35Gap
	for _, fn := range allFuncs {
		// Find all Go files in the same package directory
		pkgFiles, err := filepath.Glob(filepath.Join(fn.pkgDir, "*.go"))
		if err != nil {
			continue
		}

		var callSites []string
		for _, pkgFile := range pkgFiles {
			// Skip test files
			if strings.HasSuffix(pkgFile, "_test.go") {
				continue
			}

			// Convert to relative path
			relPath, err := filepath.Rel(repoRoot, pkgFile)
			if err != nil {
				continue
			}

			// Skip if this agent owns the file
			if agentFiles[fn.agent][relPath] {
				continue
			}

			// Check if this file calls the function
			calls, err := findCallSites(pkgFile, fn.name)
			if err != nil {
				continue
			}

			for _, line := range calls {
				callSites = append(callSites, fmt.Sprintf("%s:%d", relPath, line))
			}
		}

		// If function is called from files not owned by the agent, record gap
		if len(callSites) > 0 {
			gaps = append(gaps, E35Gap{
				Agent:        fn.agent,
				FunctionName: fn.name,
				DefinedIn:    fn.file,
				CalledFrom:   callSites,
				Package:      fn.pkg,
			})
		}
	}

	// Also detect test file cascades
	testGaps, err := detectTestCascades(m, waveNum, repoRoot)
	if err == nil {
		gaps = append(gaps, testGaps...)
	}
	// Note: we don't fail on test cascade detection errors — they're advisory

	return gaps, nil
}

// detectTestCascades finds test files that reference changed interfaces but
// are not owned by any agent in the wave. This prevents post-merge test
// compilation failures when interface signatures change.
//
// Detection algorithm:
// 1. For each agent's owned files, identify interface/type definitions
// 2. For each interface found, locate test files in same package
// 3. Check if test file calls interface methods (Parse, Detect, etc.)
// 4. If test file NOT owned by any agent, report as cascade candidate
func detectTestCascades(m *IMPLManifest, waveNum int, repoRoot string) ([]E35Gap, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is nil")
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("repoRoot is empty")
	}

	// Build agent file ownership map
	agentFiles := make(map[string]map[string]bool)
	allOwnedFiles := make(map[string]bool)
	for _, fo := range m.FileOwnership {
		if fo.Wave != waveNum {
			continue
		}
		if agentFiles[fo.Agent] == nil {
			agentFiles[fo.Agent] = make(map[string]bool)
		}
		agentFiles[fo.Agent][fo.File] = true
		allOwnedFiles[fo.File] = true
	}

	// Build interface/type definitions from owned files
	type interfaceDef struct {
		name   string
		file   string
		pkg    string
		pkgDir string
		agent  string
	}
	var interfaces []interfaceDef

	for agentID, files := range agentFiles {
		for file := range files {
			if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
				continue
			}

			absPath := filepath.Join(repoRoot, file)
			ifaces, pkgName, pkgDir, err := extractInterfaces(absPath)
			if err != nil {
				continue
			}

			for _, iface := range ifaces {
				interfaces = append(interfaces, interfaceDef{
					name:   iface,
					file:   file,
					pkg:    pkgName,
					pkgDir: pkgDir,
					agent:  agentID,
				})
			}
		}
	}

	// For each interface, find test files in same package that reference it
	var gaps []E35Gap
	for _, iface := range interfaces {
		testFiles, err := filepath.Glob(filepath.Join(iface.pkgDir, "*_test.go"))
		if err != nil {
			continue
		}

		var orphanedTests []string
		for _, testFile := range testFiles {
			relPath, err := filepath.Rel(repoRoot, testFile)
			if err != nil {
				continue
			}

			// Skip if test file is owned by an agent
			if allOwnedFiles[relPath] {
				continue
			}

			// Check if test file references the interface
			refs, err := findInterfaceReferences(testFile, iface.name)
			if err != nil || len(refs) == 0 {
				continue
			}

			for _, line := range refs {
				orphanedTests = append(orphanedTests, fmt.Sprintf("%s:%d", relPath, line))
			}
		}

		if len(orphanedTests) > 0 {
			gaps = append(gaps, E35Gap{
				Agent:        iface.agent,
				FunctionName: iface.name,
				DefinedIn:    iface.file,
				CalledFrom:   orphanedTests,
				Package:      iface.pkg,
			})
		}
	}

	return gaps, nil
}

// extractInterfaces parses a Go file and returns interface/type names
func extractInterfaces(absPath string) ([]string, string, string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return nil, "", "", err
	}

	pkgName := f.Name.Name
	pkgDir := filepath.Dir(absPath)

	var interfaces []string
	for _, decl := range f.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
						interfaces = append(interfaces, typeSpec.Name.Name)
					}
					// Also detect struct types (for signature changes like Parse method)
					if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						interfaces = append(interfaces, typeSpec.Name.Name)
					}
				}
			}
		}
	}

	return interfaces, pkgName, pkgDir, nil
}

// findInterfaceReferences searches a test file for references to an interface
// (e.g., "parser := &CargoLockParser{}" or "var p LockFileParser")
func findInterfaceReferences(absPath, interfaceName string) ([]int, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, data, 0)
	if err != nil {
		return nil, err
	}

	var lines []int
	ast.Inspect(f, func(n ast.Node) bool {
		// Detect type references in variable declarations, assignments, composites
		switch node := n.(type) {
		case *ast.CompositeLit:
			if ident, ok := node.Type.(*ast.Ident); ok && ident.Name == interfaceName {
				pos := fset.Position(node.Pos())
				lines = append(lines, pos.Line)
			}
		case *ast.ValueSpec:
			if node.Type != nil {
				if ident, ok := node.Type.(*ast.Ident); ok && ident.Name == interfaceName {
					pos := fset.Position(node.Pos())
					lines = append(lines, pos.Line)
				}
			}
		}
		return true
	})

	return lines, nil
}

// extractFunctions parses a Go file and returns:
// - list of exported and unexported function/method names
// - package name
// - absolute path to package directory
// - error if file cannot be parsed
func extractFunctions(absPath string) ([]string, string, string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return nil, "", "", err
	}

	pkgName := f.Name.Name
	pkgDir := filepath.Dir(absPath)

	var funcs []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Include both exported and unexported functions
			funcs = append(funcs, d.Name.Name)
		}
	}

	return funcs, pkgName, pkgDir, nil
}

// existingFunctionsInHEAD returns the set of function names present in the file
// at HEAD (before this wave's changes). If the file doesn't exist at HEAD (it's
// a new file planned by the Scout), returns an empty set — all functions are new.
//
// This is used to avoid false-positive E35 gaps: a function that already existed
// and is merely being modified does not require new caller ownership.
func existingFunctionsInHEAD(repoRoot, relPath string) map[string]bool {
	cmd := exec.Command("git", "-C", repoRoot, "show", "HEAD:"+relPath)
	out, err := cmd.Output()
	if err != nil {
		// File doesn't exist at HEAD — it's a new file, all functions are new.
		return map[string]bool{}
	}

	fset := token.NewFileSet()
	f, parseErr := parser.ParseFile(fset, relPath, out, 0)
	if parseErr != nil {
		return map[string]bool{}
	}

	existing := make(map[string]bool)
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			existing[fn.Name.Name] = true
		}
	}
	return existing
}

// findCallSites parses a Go file and returns line numbers where the specified
// function is called. Handles both plain calls (Symbol(args)) and selector
// calls (pkg.Symbol(args) or obj.Symbol(args)).
func findCallSites(absPath, funcName string) ([]int, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, data, 0)
	if err != nil {
		return nil, err
	}

	var lines []int
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		var called string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			called = fn.Name
		case *ast.SelectorExpr:
			called = fn.Sel.Name
		}

		if called == funcName {
			pos := fset.Position(call.Pos())
			lines = append(lines, pos.Line)
		}

		return true
	})

	return lines, nil
}
