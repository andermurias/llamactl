package modelmanager_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andermurias/llamactl/internal/modelmanager"
)

// minimalYAML is a reduced llama-swap.yaml for testing round-trips.
const minimalYAML = `# test config
healthCheckTimeout: 120

groups:
  llm:
    swap: true
    exclusive: false
    members:
      - existing-model

models:
  existing-model:
    cmd: |
      llama-server
      --model /models/existing.gguf
      --port ${PORT}
    useModelName: "existing-model"
    ttl: 300
`

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "llama-swap.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}

// TestAddModelToConfig verifies that a new model appears in models: and groups:.
func TestAddModelToConfig(t *testing.T) {
	path := writeTempYAML(t, minimalYAML)

	cfg := modelmanager.ModelConfig{
		Cmd:          "/bin/mlx_lm.server\n--model mlx-community/new-model\n--port ${PORT}",
		UseModelName: "mlx-community/new-model",
		TTL:          300,
	}
	if err := modelmanager.AddModelToConfig(path, "new-model", "llm", cfg); err != nil {
		t.Fatalf("AddModelToConfig: %v", err)
	}

	// Read back and check
	exists, err := modelmanager.ModelExistsInConfig(path, "new-model")
	if err != nil {
		t.Fatalf("ModelExistsInConfig: %v", err)
	}
	if !exists {
		t.Error("new-model not found in config after add")
	}

	// Original model must still be there
	exists, _ = modelmanager.ModelExistsInConfig(path, "existing-model")
	if !exists {
		t.Error("existing-model disappeared after add")
	}

	// Check content contains the new model key
	data, _ := os.ReadFile(path)
	if string(data) == "" {
		t.Fatal("file is empty after write")
	}
}

// TestAddDuplicateModelErrors ensures adding a duplicate returns an error.
func TestAddDuplicateModelErrors(t *testing.T) {
	path := writeTempYAML(t, minimalYAML)

	cfg := modelmanager.ModelConfig{Cmd: "echo", TTL: 60}
	err := modelmanager.AddModelToConfig(path, "existing-model", "llm", cfg)
	if err == nil {
		t.Error("expected error adding duplicate model, got nil")
	}
}

// TestRemoveModelFromConfig verifies removal of a model from both sections.
func TestRemoveModelFromConfig(t *testing.T) {
	path := writeTempYAML(t, minimalYAML)

	removed, group, err := modelmanager.RemoveModelFromConfig(path, "existing-model")
	if err != nil {
		t.Fatalf("RemoveModelFromConfig: %v", err)
	}
	if removed == nil {
		t.Fatal("expected removed config, got nil")
	}
	if group != "llm" {
		t.Errorf("expected group=llm, got %q", group)
	}
	if removed.TTL != 300 {
		t.Errorf("expected TTL=300, got %d", removed.TTL)
	}

	// Model should be gone
	exists, _ := modelmanager.ModelExistsInConfig(path, "existing-model")
	if exists {
		t.Error("existing-model still present after remove")
	}
}

// TestRemoveNonexistentErrors checks that removing a missing model fails cleanly.
func TestRemoveNonexistentErrors(t *testing.T) {
	path := writeTempYAML(t, minimalYAML)
	_, _, err := modelmanager.RemoveModelFromConfig(path, "ghost-model")
	if err == nil {
		t.Error("expected error removing non-existent model")
	}
}

// TestDisabledStore verifies round-trip of the disabled store.
func TestDisabledStore(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "disabled.yaml")

	// Empty store should be returned when file doesn't exist
	store, err := modelmanager.LoadDisabledStore(storePath)
	if err != nil {
		t.Fatalf("LoadDisabledStore (missing): %v", err)
	}
	if len(store.Disabled) != 0 {
		t.Errorf("expected empty store, got %d entries", len(store.Disabled))
	}

	// Add an entry and save
	store.Disabled["test-model"] = modelmanager.DisabledEntry{
		Config: modelmanager.ModelConfig{Cmd: "echo", TTL: 60},
		Group:  "llm",
		HFID:   "owner/test-model",
		Type:   modelmanager.ModelTypeMLX,
	}
	if err := modelmanager.SaveDisabledStore(storePath, store); err != nil {
		t.Fatalf("SaveDisabledStore: %v", err)
	}

	// Reload and verify
	store2, err := modelmanager.LoadDisabledStore(storePath)
	if err != nil {
		t.Fatalf("LoadDisabledStore (reload): %v", err)
	}
	e, ok := store2.Disabled["test-model"]
	if !ok {
		t.Fatal("test-model not found after reload")
	}
	if e.Group != "llm" {
		t.Errorf("expected group=llm, got %q", e.Group)
	}
	if e.Type != modelmanager.ModelTypeMLX {
		t.Errorf("expected type mlx, got %q", e.Type)
	}
}

// TestManagerDisableEnable exercises the full disable → enable round-trip.
func TestManagerDisableEnable(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "llama-swap.yaml")
	disabledPath := filepath.Join(dir, "disabled.yaml")
	modelsDir := dir

	if err := os.WriteFile(configPath, []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := modelmanager.NewManager(configPath, disabledPath, modelsDir)

	// Disable
	if err := mgr.Disable("existing-model"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	exists, _ := modelmanager.ModelExistsInConfig(configPath, "existing-model")
	if exists {
		t.Error("model still in config after disable")
	}

	disabled, _ := mgr.ListDisabled()
	if _, ok := disabled["existing-model"]; !ok {
		t.Error("model not in disabled list after disable")
	}

	// Enable
	if err := mgr.Enable("existing-model"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	exists, _ = modelmanager.ModelExistsInConfig(configPath, "existing-model")
	if !exists {
		t.Error("model not back in config after enable")
	}

	disabled, _ = mgr.ListDisabled()
	if _, ok := disabled["existing-model"]; ok {
		t.Error("model still in disabled list after enable")
	}
}

// TestNormalizeHFID checks URL → ID normalisation.
func TestNormalizeHFID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://huggingface.co/google/gemma-3-12b-it", "google/gemma-3-12b-it"},
		{"google/gemma-3-12b-it", "google/gemma-3-12b-it"},
		{"https://huggingface.co/google/gemma/", "google/gemma"},
	}
	for _, c := range cases {
		got := modelmanager.NormalizeHFID(c.in)
		if got != c.want {
			t.Errorf("NormalizeHFID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestDeriveModelID checks ID derivation from HF identifiers.
func TestDeriveModelID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"mlx-community/gemma-3-12b-it-4bit", "gemma-3-12b-it-mlx"},
		{"mlx-community/phi-4-8bit", "phi-4-mlx"},
		{"google/gemma-3-12b-it", "gemma-3-12b-it"},
		{"meta-llama/Llama-3.2-3B-Instruct", "llama-3.2-3b-instruct"},
	}
	for _, c := range cases {
		got := modelmanager.DeriveModelID(c.in)
		if got != c.want {
			t.Errorf("DeriveModelID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
