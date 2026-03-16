package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsIntegrationRequired_BuildPrefix(t *testing.T) {
	if !IsIntegrationRequired("BuildWaveConstraints", "function_call") {
		t.Error("expected true for Build prefix")
	}
	if !IsIntegrationRequired("CreateServer", "function_call") {
		t.Error("expected true for Create prefix")
	}
	if !IsIntegrationRequired("NewObserver", "function_call") {
		t.Error("expected true for New prefix")
	}
	if !IsIntegrationRequired("RegisterHandler", "function_call") {
		t.Error("expected true for Register prefix")
	}
	if !IsIntegrationRequired("SetupRoutes", "function_call") {
		t.Error("expected true for Setup prefix")
	}
	if !IsIntegrationRequired("InitConfig", "function_call") {
		t.Error("expected true for Init prefix")
	}
	if !IsIntegrationRequired("RunMigration", "function_call") {
		t.Error("expected true for Run prefix")
	}
	if !IsIntegrationRequired("StartServer", "function_call") {
		t.Error("expected true for Start prefix")
	}
	if !IsIntegrationRequired("WireRoutes", "function_call") {
		t.Error("expected true for Wire prefix")
	}
}

func TestIsIntegrationRequired_Suffixes(t *testing.T) {
	if !IsIntegrationRequired("AuthHandler", "function_call") {
		t.Error("expected true for Handler suffix")
	}
	if !IsIntegrationRequired("LoggingMiddleware", "function_call") {
		t.Error("expected true for Middleware suffix")
	}
	if !IsIntegrationRequired("ConnectionFactory", "function_call") {
		t.Error("expected true for Factory suffix")
	}
	if !IsIntegrationRequired("QueryBuilder", "function_call") {
		t.Error("expected true for Builder suffix")
	}
}

func TestIsIntegrationRequired_GetPrefix(t *testing.T) {
	if IsIntegrationRequired("GetConfig", "function_call") {
		t.Error("expected false for Get prefix")
	}
	if IsIntegrationRequired("IsValid", "function_call") {
		t.Error("expected false for Is prefix")
	}
	if IsIntegrationRequired("HasPermission", "function_call") {
		t.Error("expected false for Has prefix")
	}
	if IsIntegrationRequired("String", "function_call") {
		t.Error("expected false for String prefix")
	}
	if IsIntegrationRequired("FormatOutput", "function_call") {
		t.Error("expected false for Format prefix")
	}
	if IsIntegrationRequired("ValidateInput", "function_call") {
		t.Error("expected false for Validate prefix")
	}
}

func TestIsIntegrationRequired_FieldInit(t *testing.T) {
	if !IsIntegrationRequired("SomeField", "field_init") {
		t.Error("expected true for field_init category")
	}
}

func TestIsIntegrationRequired_None(t *testing.T) {
	if IsIntegrationRequired("Anything", "none") {
		t.Error("expected false for none category")
	}
}

func TestIsIntegrationRequired_ConstVar(t *testing.T) {
	if IsIntegrationRequired("MaxRetries", "const") {
		t.Error("expected false for const category")
	}
	if IsIntegrationRequired("DefaultTimeout", "var") {
		t.Error("expected false for var category")
	}
}

func TestClassifyExport_Func(t *testing.T) {
	if got := ClassifyExport("BuildServer", "func"); got != "function_call" {
		t.Errorf("expected function_call, got %s", got)
	}
	if got := ClassifyExport("ServeHTTP", "method"); got != "function_call" {
		t.Errorf("expected function_call for method, got %s", got)
	}
}

func TestClassifyExport_Type(t *testing.T) {
	if got := ClassifyExport("Server", "type"); got != "type_usage" {
		t.Errorf("expected type_usage, got %s", got)
	}
}

func TestClassifyExport_Field(t *testing.T) {
	if got := ClassifyExport("Config", "field"); got != "field_init" {
		t.Errorf("expected field_init, got %s", got)
	}
}

func TestClassifyExport_VarConst(t *testing.T) {
	if got := ClassifyExport("MaxRetries", "var"); got != "none" {
		t.Errorf("expected none for var, got %s", got)
	}
	if got := ClassifyExport("Version", "const"); got != "none" {
		t.Errorf("expected none for const, got %s", got)
	}
}

func TestClassifyExport_Unknown(t *testing.T) {
	if got := ClassifyExport("Something", "unknown"); got != "none" {
		t.Errorf("expected none for unknown kind, got %s", got)
	}
}

func TestSuggestCallers_FindsImporters(t *testing.T) {
	// Create a temp directory with a mini Go module that has two packages,
	// where pkg B imports pkg A.
	tmpDir := t.TempDir()

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package A (the export source)
	pkgADir := filepath.Join(tmpDir, "pkga")
	if err := os.MkdirAll(pkgADir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgADir, "a.go"), []byte(`package pkga

func BuildSomething() {}
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package B that imports A
	pkgBDir := filepath.Join(tmpDir, "pkgb")
	if err := os.MkdirAll(pkgBDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgBDir, "b.go"), []byte(`package pkgb

import "example.com/test/pkga"

var _ = pkga.BuildSomething
`), 0644); err != nil {
		t.Fatal(err)
	}

	callers, err := SuggestCallers(tmpDir, "example.com/test/pkga", "BuildSomething")
	if err != nil {
		t.Fatalf("SuggestCallers error: %v", err)
	}

	if len(callers) == 0 {
		t.Fatal("expected at least one caller, got none")
	}

	// Verify we found pkgb/b.go
	found := false
	for _, c := range callers {
		if filepath.Base(c) == "b.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find b.go in callers, got: %v", callers)
	}
}

func TestSuggestCallers_EmptyRepo(t *testing.T) {
	// Create a temp directory with a single package, no importers.
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/empty\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "mypkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "a.go"), []byte(`package mypkg

func DoSomething() {}
`), 0644); err != nil {
		t.Fatal(err)
	}

	callers, err := SuggestCallers(tmpDir, "example.com/empty/mypkg", "DoSomething")
	if err != nil {
		t.Fatalf("SuggestCallers error: %v", err)
	}

	if len(callers) != 0 {
		t.Errorf("expected no callers, got: %v", callers)
	}
}
