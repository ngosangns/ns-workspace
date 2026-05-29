package agentsync

import (
	"encoding/json"
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
	t.Setenv("KIRO_HOME", "")

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
	mustExist(t, filepath.Join(home, ".agents", "skills", "commit", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "spawn-opencode", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "spawn-sub-agent", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustExist(t, filepath.Join(home, ".claude", "agents", "opencode-intern.md"))
	mustExist(t, filepath.Join(home, ".grok", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".kiro", "steering", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".kiro", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".kiro", "settings", "mcp.json"))
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
	if strings.Contains(qwen, "PreToolUse") || strings.Contains(qwen, "graphify-out/graph.json") {
		t.Fatalf("qwen settings should not install graphify hooks: %s", qwen)
	}

	kiro := readFile(t, filepath.Join(home, ".kiro", "settings", "mcp.json"))
	if !strings.Contains(kiro, "context7") || !strings.Contains(kiro, "figma") {
		t.Fatalf("kiro settings did not include MCP preset: %s", kiro)
	}

	opencode := readFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if strings.Contains(opencode, "PreToolUse") || !strings.Contains(opencode, `"type": "remote"`) || !strings.Contains(opencode, "context7") || !strings.Contains(opencode, `"permission": "allow"`) {
		t.Fatalf("opencode config should include remote MCP presets with permission allow, without unsupported hooks: %s", opencode)
	}

	codex := readFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(codex, "[mcp_servers.\"context7\"]") {
		t.Fatalf("codex config did not include MCP preset: %s", codex)
	}
}

func TestInstalledSettingsDoNotInstallGraphifyHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

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

	settingsPaths := []string{
		filepath.Join(home, ".agents", "settings.json"),
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".qwen", "settings.json"),
		filepath.Join(home, ".gemini", "settings.json"),
	}
	for _, path := range settingsPaths {
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			commands := hookCommands(t, path)
			if len(commands) != 0 {
				t.Fatalf("expected no installed hook commands in %s, got %v", path, commands)
			}
			settings := readFile(t, path)
			if strings.Contains(settings, "graphify-out") || strings.Contains(settings, "GRAPH_REPORT") {
				t.Fatalf("settings should not reference graphify artifacts: %s", settings)
			}
		})
	}
}

func TestUpdateRewritesManagedPresetContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

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

	staleSharedSkill := filepath.Join(home, ".agents", "skills", "stale-local", "SKILL.md")
	staleNativeSkill := filepath.Join(home, ".claude", "skills", "stale-local", "SKILL.md")
	for _, path := range []string{staleSharedSkill, staleNativeSkill} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".qwen", "settings.json"), []byte(`{"mcpServers":{"stale":{}},"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"stale"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"stale":true,"mcp":{"stale":{}},"permission":"deny"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	mustNotExist(t, staleSharedSkill)
	mustNotExist(t, staleNativeSkill)
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".claude", "skills", "execution", "SKILL.md"))

	qwen := readJSONFile(t, filepath.Join(home, ".qwen", "settings.json"))
	if _, ok := qwen["mcpServers"].(map[string]any)["stale"]; ok {
		t.Fatalf("update kept stale qwen MCP preset: %v", qwen)
	}
	if hooks, ok := qwen["hooks"].(map[string]any); !ok || len(hooks) != 0 {
		t.Fatalf("update did not rewrite qwen hooks from preset: %v", qwen)
	}

	opencode := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if _, ok := opencode["stale"]; ok {
		t.Fatalf("update kept stale opencode root key: %v", opencode)
	}
	if _, ok := opencode["mcp"].(map[string]any)["stale"]; ok {
		t.Fatalf("update kept stale opencode MCP preset: %v", opencode)
	}
	if opencode["permission"] != "allow" {
		t.Fatalf("update did not rewrite opencode permission from preset: %v", opencode)
	}
}

func TestKiroCLISelectionUsesKiroAdapter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("kiro-cli"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".kiro", "steering", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".kiro", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".kiro", "settings", "mcp.json"))
	mustNotExist(t, filepath.Join(home, ".qwen", "settings.json"))
}

func TestGrokSelectionCreatesNativeSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("grok"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".grok", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustNotExist(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
}

func TestKiroHomeOverrideUsesKiroPresetPaths(t *testing.T) {
	home := t.TempDir()
	kiroHome := filepath.Join(home, "custom-kiro")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", kiroHome)

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("kiro"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(kiroHome, "steering", "AGENTS.md"))
	mustExist(t, filepath.Join(kiroHome, "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(kiroHome, "settings", "mcp.json"))
	mustNotExist(t, filepath.Join(home, ".kiro", "steering", "AGENTS.md"))
}

func TestStableToolSelectionSkipsManualAdapters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

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
	t.Setenv("KIRO_HOME", "")

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

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("invalid JSON in %s: %v", path, err)
	}
	return obj
}

func hookCommands(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON in %s: %v", path, err)
	}
	var commands []string
	for _, group := range settings.Hooks["PreToolUse"] {
		for _, hook := range group.Hooks {
			if hook.Type == "command" && hook.Command != "" {
				commands = append(commands, hook.Command)
			}
		}
	}
	return commands
}
