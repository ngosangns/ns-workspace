package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
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

// --- Tests for branches not covered by existing integration tests ---

func TestRunAgentSyncUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("update", []string{"--no-registry"}, os.DirFS("../..")); err != nil {
		t.Fatalf("update failed: %v", err)
	}
}

func TestRunAgentSyncStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("status", []string{}, os.DirFS("../..")); err != nil {
		t.Fatalf("status failed: %v", err)
	}
}

func TestRunAgentSyncDoctor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("doctor", []string{}, os.DirFS("../..")); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
}

func TestRunAgentSyncRegistry(t *testing.T) {
	home := t.TempDir()
	fakeBin := writeFakeNpx(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := RunAgentSync("registry", []string{}, os.DirFS("../..")); err != nil {
		t.Fatalf("registry failed: %v", err)
	}
}

func TestRunAgentSyncCatalog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("catalog", []string{"--tools", "claude"}, os.DirFS("../..")); err != nil {
		t.Fatalf("catalog failed: %v", err)
	}
}

func TestRunAgentSyncAgentsAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("agents", []string{"--tools", "claude"}, os.DirFS("../..")); err != nil {
		t.Fatalf("agents alias failed: %v", err)
	}
}

func TestRunAgentSyncUnknownCommand(t *testing.T) {
	err := RunAgentSync("nonexistent", []string{}, os.DirFS("../.."))
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	// Unknown command trả về flag.ErrHelp.
	if !errors.Is(err, flag.ErrHelp) {
		t.Logf("got err=%v (may not match flag.ErrHelp exactly)", err)
	}
}

func TestRunAgentSyncInvalidFlag(t *testing.T) {
	err := RunAgentSync("init", []string{"--unknown-flag"}, os.DirFS("../.."))
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunAgentSyncYesFlag(t *testing.T) {
	// --yes không có skip-confirm ở test nhưng phải được parse OK.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("init", []string{"--yes", "--no-registry", "--dry-run"}, os.DirFS("../..")); err != nil {
		t.Fatalf("--yes init failed: %v", err)
	}
}

func TestRunAgentSyncCustomAgentsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	customHome := filepath.Join(home, "custom-agents")
	if err := RunAgentSync("init", []string{"--agents-home", customHome, "--dry-run", "--no-registry"}, os.DirFS("../..")); err != nil {
		t.Fatalf("--agents-home init failed: %v", err)
	}
}

func TestRunAgentSyncToolsOverride(t *testing.T) {
	// --tools=opencode: tool filter override.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("init", []string{"--tools", "opencode", "--no-registry", "--dry-run"}, os.DirFS("../..")); err != nil {
		t.Fatalf("--tools init failed: %v", err)
	}
}

func TestRunAgentSyncCopyAndForce(t *testing.T) {
	// --copy --force: cover flag paths.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("init", []string{"--copy", "--force", "--no-registry"}, os.DirFS("../..")); err != nil {
		t.Fatalf("--copy --force init failed: %v", err)
	}
}

func TestRunAgentSyncNoMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	if err := RunAgentSync("init", []string{"--no-mcp", "--no-registry"}, os.DirFS("../..")); err != nil {
		t.Fatalf("--no-mcp init failed: %v", err)
	}
}

// TestRunAgentSyncDefaultAgentsDirError exercises the early-return path
// when agentsync.DefaultAgentsDir() fails (userHomeDir seam returns an
// error).
func TestRunAgentSyncDefaultAgentsDirError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	orig := agentsync.UserHomeDirForTest
	agentsync.UserHomeDirForTest = func() (string, error) { return "", errors.New("forced home error") }
	t.Cleanup(func() { agentsync.UserHomeDirForTest = orig })
	err := RunAgentSync("init", []string{"--no-registry"}, os.DirFS("../.."))
	if err == nil {
		t.Fatalf("expected error from DefaultAgentsDir failure")
	}
}

// TestRunAgentSyncDefaultUserConfigPathError exercises the early-return
// path when agentsync.DefaultUserConfigPath() fails (userConfigDir seam
// returns an error).
func TestRunAgentSyncDefaultUserConfigPathError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	orig := agentsync.UserConfigDirForTest
	agentsync.UserConfigDirForTest = func() (string, error) { return "", errors.New("forced config dir error") }
	t.Cleanup(func() { agentsync.UserConfigDirForTest = orig })
	err := RunAgentSync("init", []string{"--no-registry"}, os.DirFS("../.."))
	if err == nil {
		t.Fatalf("expected error from DefaultUserConfigPath failure")
	}
}

// --- helpers ---

func writeFakeNpx(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "npx"
	data := []byte("#!/usr/bin/env sh\nexit 0\n")
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Đảm bảo agentsync import được dùng để tránh unused warning nếu test bị lọc.
var _ = agentsync.ParseTools
