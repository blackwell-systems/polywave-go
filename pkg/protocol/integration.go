package protocol

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateIntegration scans a completed wave for unconnected exports (E25).
// It parses completion reports to find modified files, uses Go AST to extract
// new exported symbols, then searches the repo for usage. Returns gaps.
func ValidateIntegration(manifest *IMPLManifest, waveNum int, repoPath string) (*IntegrationReport, error) {
	report := &IntegrationReport{
		Wave: waveNum,
		Gaps: []IntegrationGap{},
	}

	// Find the wave
	var wave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			wave = &manifest.Waves[i]
			break
		}
	}
	if wave == nil {
		return report, fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	// Collect files changed per agent from completion reports
	for _, agent := range wave.Agents {
		cr, ok := manifest.CompletionReports[agent.ID]
		if !ok {
			continue
		}
		if cr.Status != "complete" {
			continue
		}

		// Combine FilesChanged and FilesCreated
		allFiles := make([]string, 0, len(cr.FilesChanged)+len(cr.FilesCreated))
		allFiles = append(allFiles, cr.FilesChanged...)
		allFiles = append(allFiles, cr.FilesCreated...)

		for _, relFile := range allFiles {
			isGo := strings.HasSuffix(relFile, ".go") && !strings.HasSuffix(relFile, "_test.go")
			isTSX := isTSXFile(relFile) && !strings.HasSuffix(relFile, ".test.tsx") && !strings.HasSuffix(relFile, ".spec.tsx")
			if !isGo && !isTSX {
				continue
			}

			absFile := filepath.Join(repoPath, relFile)
			var exports []exportInfo
			var err error
			if isTSX {
				exports, err = extractTSXExports(absFile)
			} else {
				exports, err = extractExports(absFile)
			}
			if err != nil {
				// Skip files that can't be parsed
				continue
			}

			for _, exp := range exports {
				category := ClassifyExport(exp.Name, exp.Kind)
				if !IsIntegrationRequired(exp.Name, category) {
					continue
				}

				// Search repo for references (excluding the defining file and test files)
				refs := searchReferences(repoPath, relFile, exp.Name)
				if len(refs) == 0 {
					suggestedCallers, _ := SuggestCallers(repoPath, filepath.Dir(relFile), exp.Name)
					gap := IntegrationGap{
						ExportName:    exp.Name,
						FilePath:      relFile,
						AgentID:       agent.ID,
						Category:      category,
						Severity:      classifySeverity(exp.Name, category),
						Reason:        fmt.Sprintf("exported %s %q has no call-sites outside its defining file", exp.Kind, exp.Name),
						SuggestedFix:  suggestFix(exp.Name, category, suggestedCallers),
						SearchResults: suggestedCallers,
					}
					report.Gaps = append(report.Gaps, gap)
				}
			}
		}
	}

	report.Valid = len(report.Gaps) == 0
	report.Summary = buildSummary(report)
	return report, nil
}

// exportInfo holds information about an exported symbol found in Go source.
type exportInfo struct {
	Name string // e.g. "ValidateIntegration"
	Kind string // "func", "type", "method", "field"
}

// extractExports parses a Go file and returns all exported symbols.
func extractExports(filePath string) ([]exportInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, err
	}

	var exports []exportInfo

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				kind := "func"
				if d.Recv != nil {
					kind = "method"
				}
				exports = append(exports, exportInfo{Name: d.Name.Name, Kind: kind})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						exports = append(exports, exportInfo{Name: s.Name.Name, Kind: "type"})
						// Extract exported struct fields
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								for _, name := range field.Names {
									if name.IsExported() {
										exports = append(exports, exportInfo{Name: name.Name, Kind: "field"})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return exports, nil
}

// searchReferences searches the repo for references to exportName,
// excluding the defining file and test files.
func searchReferences(repoPath, definingFile, exportName string) []string {
	var refs []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden directories and vendor
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		isGoRef := strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
		isTSXRef := isTSXFile(path) && !strings.HasSuffix(path, ".test.tsx") && !strings.HasSuffix(path, ".spec.tsx")
		if !isGoRef && !isTSXRef {
			return nil
		}

		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}
		if rel == definingFile {
			return nil
		}

		// Simple string search in file
		if fileContains(path, exportName) {
			refs = append(refs, rel)
		}
		return nil
	})
	if err != nil {
		return refs
	}
	return refs
}

// fileContains checks if a file contains the given string.
func fileContains(filePath, needle string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), needle) {
			return true
		}
	}
	return false
}

// classifySeverity determines the severity of an integration gap.
func classifySeverity(exportName string, category string) string {
	// Functions that build or create things are higher severity
	highSeverityPrefixes := []string{"New", "Build", "Register", "Run", "Start", "Init"}
	for _, prefix := range highSeverityPrefixes {
		if strings.HasPrefix(exportName, prefix) {
			return "error"
		}
	}
	// React callback/handler props not wired = warning (likely cross-agent dep)
	if category == "prop_pass" {
		return "warning"
	}
	if category == "function_call" {
		return "warning"
	}
	return "info"
}

// suggestFix generates a human-readable suggestion for wiring the export.
func suggestFix(exportName string, category string, suggestedCallers []string) string {
	if len(suggestedCallers) > 0 {
		return fmt.Sprintf("Add call to %s in %s", exportName, suggestedCallers[0])
	}
	switch category {
	case "function_call":
		return fmt.Sprintf("Find appropriate caller for %s and add invocation", exportName)
	case "type_usage":
		return fmt.Sprintf("Use type %s in appropriate location", exportName)
	default:
		return fmt.Sprintf("Wire %s into the codebase", exportName)
	}
}

// buildSummary generates a human-readable summary for an integration report.
func buildSummary(report *IntegrationReport) string {
	if report.Valid {
		return fmt.Sprintf("Wave %d: all exports are integrated (no gaps detected)", report.Wave)
	}

	errorCount := 0
	warningCount := 0
	for _, gap := range report.Gaps {
		switch gap.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}

	parts := []string{fmt.Sprintf("Wave %d: %d integration gap(s) detected", report.Wave, len(report.Gaps))}
	if errorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", errorCount))
	}
	if warningCount > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warningCount))
	}
	return strings.Join(parts, ", ")
}

// AppendIntegrationReport persists an IntegrationReport to the IMPL manifest
// under the given waveKey (e.g. "wave1"). Since the IntegrationReports field
// may not yet exist on IMPLManifest, this uses raw YAML manipulation.
func AppendIntegrationReport(manifestPath string, waveKey string, report *IntegrationReport) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("AppendIntegrationReport: failed to read manifest: %w", err)
	}

	// Parse into raw YAML node tree
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("AppendIntegrationReport: failed to parse YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("AppendIntegrationReport: unexpected YAML structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("AppendIntegrationReport: root is not a mapping")
	}

	// Marshal the report to a YAML node
	var reportNode yaml.Node
	reportBytes, err := yaml.Marshal(report)
	if err != nil {
		return fmt.Errorf("AppendIntegrationReport: failed to marshal report: %w", err)
	}
	if err := yaml.Unmarshal(reportBytes, &reportNode); err != nil {
		return fmt.Errorf("AppendIntegrationReport: failed to unmarshal report node: %w", err)
	}

	// Find or create integration_reports mapping
	var reportsValue *yaml.Node
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "integration_reports" {
			reportsValue = root.Content[i+1]
			break
		}
	}

	if reportsValue == nil {
		// Add integration_reports key
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "integration_reports", Tag: "!!str"}
		valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, keyNode, valueNode)
		reportsValue = valueNode
	}

	if reportsValue.Kind != yaml.MappingNode {
		return fmt.Errorf("AppendIntegrationReport: integration_reports is not a mapping")
	}

	// Add or replace the wave key
	found := false
	for i := 0; i < len(reportsValue.Content)-1; i += 2 {
		if reportsValue.Content[i].Value == waveKey {
			reportsValue.Content[i+1] = reportNode.Content[0]
			found = true
			break
		}
	}
	if !found {
		waveKeyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: waveKey, Tag: "!!str"}
		reportsValue.Content = append(reportsValue.Content, waveKeyNode, reportNode.Content[0])
	}

	// Write back
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("AppendIntegrationReport: failed to marshal output: %w", err)
	}
	if err := os.WriteFile(manifestPath, out, 0644); err != nil {
		return fmt.Errorf("AppendIntegrationReport: failed to write file: %w", err)
	}

	return nil
}
