package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunAgentSyncDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	if err := RunAgentSync("init", []string{"--dry-run", "--no-registry"}, os.DirFS("../..")); err != nil {
		t.Fatalf("dry-run init failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .agents, stat err: %v", err)
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
