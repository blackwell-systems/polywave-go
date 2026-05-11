package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

func TestDetectScaffolds_Success(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	manifest := &protocol.IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test",
		Verdict:     "SUITABLE",
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:       "C1",
				Definition: "type Foo struct { X int }",
				Location:   "pkg/a/a.go",
			},
			{
				Name:       "C2",
				Definition: "type Foo struct { X int }",
				Location:   "pkg/b/b.go",
			},
		},
		FileOwnership: []protocol.FileOwnership{},
		Waves:         []protocol.Wave{},
	}

	if saveRes := protocol.Save(context.Background(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("save: %v", saveRes.Errors)
	}

	result, err := DetectScaffolds(context.Background(), manifestPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ScaffoldsNeeded) != 1 {
		t.Fatalf("expected 1 scaffold, got %d", len(result.ScaffoldsNeeded))
	}
	if result.ScaffoldsNeeded[0].TypeName != "Foo" {
		t.Errorf("expected Foo, got %s", result.ScaffoldsNeeded[0].TypeName)
	}
}

func TestDetectScaffolds_FileNotFound(t *testing.T) {
	_, err := DetectScaffolds(context.Background(), "/nonexistent/IMPL-test.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "scaffold detection: load manifest:") {
		t.Errorf("expected wrapped error message, got: %v", err)
	}
}

func TestDetectScaffolds_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "IMPL-bad.yaml")
	if err := os.WriteFile(badFile, []byte("not: [valid: yaml: content"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := DetectScaffolds(context.Background(), badFile)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "scaffold detection: load manifest:") {
		t.Errorf("expected wrapped error message, got: %v", err)
	}
}
