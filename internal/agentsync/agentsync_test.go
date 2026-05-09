package agentsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCreatesStableAndManualAgentLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("all"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".agents", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".agents", "agents", "opencode-intern.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustExist(t, filepath.Join(home, ".claude", "agents", "opencode-intern.md"))
	mustExist(t, filepath.Join(home, ".qwen", "settings.json"))
	mustExist(t, filepath.Join(home, ".gemini", "settings.json"))
	mustExist(t, filepath.Join(home, ".codex", "config.toml"))
	mustExist(t, filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"))
	mustExist(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	mustExist(t, filepath.Join(home, ".agents", "generated", "cursor", "README.md"))

	qwen := readFile(t, filepath.Join(home, ".qwen", "settings.json"))
	if !strings.Contains(qwen, "context7") || !strings.Contains(qwen, "figma") {
		t.Fatalf("qwen settings did not include MCP preset: %s", qwen)
	}
	if !strings.Contains(qwen, "PreToolUse") || !strings.Contains(qwen, "graphify-out/graph.json") {
		t.Fatalf("qwen settings did not include shared hooks: %s", qwen)
	}

	opencode := readFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if !strings.Contains(opencode, "PreToolUse") || !strings.Contains(opencode, "context7") {
		t.Fatalf("opencode config did not include hooks and MCP presets: %s", opencode)
	}

	codex := readFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(codex, "[mcp_servers.\"context7\"]") {
		t.Fatalf("codex config did not include MCP preset: %s", codex)
	}
}

func TestStableToolSelectionSkipsManualAdapters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("stable"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustNotExist(t, filepath.Join(home, ".agents", "generated", "cursor", "README.md"))
}

func TestDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		DryRun:     true,
		NoRegistry: true,
		ToolFilter: ParseTools("all"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("dry-run init failed: %v", err)
	}
	mustNotExist(t, filepath.Join(home, ".agents"))
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
