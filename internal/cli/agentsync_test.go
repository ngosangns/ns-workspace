package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAgentSyncDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")

	if err := RunAgentSync("init", []string{"--dry-run", "--no-registry"}, os.DirFS("../..")); err != nil {
		t.Fatalf("dry-run init failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .agents, stat err: %v", err)
	}
}

func TestRunAgentSyncConfigFlagOverridesPresets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")

	dir := t.TempDir()
	customAgents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(customAgents, []byte("# Custom AGENTS via --config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	body, _ := json.Marshal(map[string]string{
		"presets/agents/AGENTS.md": customAgents,
	})
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--config", cfgPath,
		"--no-registry",
		"--tools", "claude",
	}
	if err := RunAgentSync("init", args, os.DirFS("../..")); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	shared, err := os.ReadFile(filepath.Join(home, ".agents", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shared), "Custom AGENTS via --config") {
		t.Fatalf("expected user config to override shared AGENTS.md, got: %s", shared)
	}
}

func TestRunAgentSyncEmptyConfigDisablesOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")

	if err := RunAgentSync("init", []string{
		"--config", "",
		"--no-registry",
		"--tools", "claude",
	}, os.DirFS("../..")); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	shared, err := os.ReadFile(filepath.Join(home, ".agents", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shared), "Trigger Skills") {
		t.Fatalf("expected embedded AGENTS.md without overlay, got: %s", shared)
	}
}

func TestIsAgentSyncCommand(t *testing.T) {
	for _, cmd := range []string{"init", "update", "status", "doctor", "registry", "agents", "catalog"} {
		if !IsAgentSyncCommand(cmd) {
			t.Fatalf("%s should be an agentsync command", cmd)
		}
	}
	for _, cmd := range []string{"preview", "search", "graph", "lsp"} {
		if IsAgentSyncCommand(cmd) {
			t.Fatalf("%s should not be an agentsync command", cmd)
		}
	}
}
