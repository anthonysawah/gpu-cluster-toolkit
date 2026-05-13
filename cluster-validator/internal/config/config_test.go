package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nodes.yaml")
	contents := `
nodes:
  - name: gpu-node-01
    endpoint: http://localhost:8081/echo
  - name: gpu-node-02
    endpoint: http://localhost:8082/echo
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(cfg.Nodes))
	}
	if cfg.Nodes[0].Name != "gpu-node-01" {
		t.Errorf("expected first node name gpu-node-01, got %s", cfg.Nodes[0].Name)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestValidate_EmptyNodes(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty nodes list, got nil")
	}
}

func TestValidate_DuplicateName(t *testing.T) {
	cfg := &Config{
		Nodes: []Node{
			{Name: "a", Endpoint: "http://x"},
			{Name: "a", Endpoint: "http://y"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestValidate_MissingEndpoint(t *testing.T) {
	cfg := &Config{
		Nodes: []Node{{Name: "a"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing endpoint, got nil")
	}
}