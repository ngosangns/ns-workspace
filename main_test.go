package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitCreatesSharedAndNativeLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")

	if err := run([]string{"init", "--no-registry"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".agents", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".agents", "agents", "opencode-intern.md"))
	mustExist(t, filepath.Join(home, ".agents", "registry", "skills.json"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "spawn-opencode", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "spawn-sub-agent", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "settings.json"))
	mustExist(t, filepath.Join(home, ".agents", "mcp", "servers.json"))
	mustExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustExist(t, filepath.Join(home, ".claude", "agents", "opencode-intern.md"))
	mustExist(t, filepath.Join(home, ".kimi", "mcp.json"))
	mustExist(t, filepath.Join(home, ".kiro", "steering", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".kiro", "settings", "mcp.json"))
	mustExist(t, filepath.Join(home, ".qwen", "settings.json"))
	mustExist(t, filepath.Join(home, ".gemini", "settings.json"))
	mustExist(t, filepath.Join(home, ".codex", "config.toml"))
	mustExist(t, filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"))
	mustExist(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	mustExist(t, filepath.Join(home, ".agents", "generated", "cursor", "README.md"))
	mustExist(t, filepath.Join(home, ".agents", "generated", "antigravity", "README.md"))
	mustExist(t, filepath.Join(home, ".agents", "generated", "trae", "README.md"))

	data, err := os.ReadFile(filepath.Join(home, ".qwen", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "context7") || !strings.Contains(string(data), "figma") {
		t.Fatalf("qwen settings did not include MCP preset: %s", data)
	}
	if !strings.Contains(string(data), "PreToolUse") || !strings.Contains(string(data), "graphify-out/graph.json") {
		t.Fatalf("qwen settings did not include shared hooks: %s", data)
	}

	kiro, err := os.ReadFile(filepath.Join(home, ".kiro", "settings", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kiro), "context7") || !strings.Contains(string(kiro), "figma") {
		t.Fatalf("kiro settings did not include MCP preset: %s", kiro)
	}

	opencode, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(opencode), "PreToolUse") || !strings.Contains(string(opencode), `"type": "remote"`) {
		t.Fatalf("opencode config should include remote MCP presets without unsupported hooks: %s", opencode)
	}

	codex, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codex), "[mcp_servers.\"context7\"]") {
		t.Fatalf("codex config did not include MCP preset: %s", codex)
	}
}

func TestUpdateBacksUpAndOverridesSharedAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")

	if err := run([]string{"init", "--no-registry"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	agents := filepath.Join(home, ".agents", "AGENTS.md")
	if err := os.WriteFile(agents, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"update", "--no-registry"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	data, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "local edit") {
		t.Fatalf("update did not override managed AGENTS.md")
	}
	matches, err := filepath.Glob(filepath.Join(home, ".agents", "AGENTS.md.bak-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one AGENTS.md backup, got %d", len(matches))
	}
}

func TestDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")

	if err := run([]string{"init", "--dry-run", "--no-registry"}); err != nil {
		t.Fatalf("dry-run init failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .agents, stat err: %v", err)
	}
}

func TestRegistryCommandBootstrapsCustomAgentsHome(t *testing.T) {
	home := t.TempDir()
	agentsHome := filepath.Join(home, "custom-agents")
	fakeBin := writeFakeNpx(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := run([]string{"registry", "--agents-home", agentsHome}); err != nil {
		t.Fatalf("registry failed: %v", err)
	}

	readme := mustRead(t, filepath.Join(agentsHome, "registry", "README.md"))
	if !strings.Contains(readme, agentsHome) || strings.Contains(readme, "~/.agents") {
		t.Fatalf("registry README did not use custom agents home: %s", readme)
	}
	script := mustRead(t, filepath.Join(agentsHome, "registry", "install.sh"))
	if !strings.Contains(script, "AGENTS_HOME="+agentsHome) || strings.Contains(script, "~/.agents") {
		t.Fatalf("registry install script did not use custom agents home: %s", script)
	}
	npxEnv := mustRead(t, filepath.Join(home, "npx-agents-home.log"))
	if !strings.Contains(npxEnv, agentsHome) {
		t.Fatalf("registry install did not pass custom agents home to npx: %s", npxEnv)
	}
	mustExist(t, filepath.Join(agentsHome, "registry", "skills.json"))
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to be readable: %v", path, err)
	}
	return string(data)
}

func writeFakeNpx(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "npx"
	data := []byte("#!/usr/bin/env sh\nprintf '%s\\n' \"$AGENTS_HOME\" >> \"$HOME/npx-agents-home.log\"\nexit 0\n")
	if runtime.GOOS == "windows" {
		name = "npx.bat"
		data = []byte("@echo off\r\necho %AGENTS_HOME%>> \"%HOME%\\npx-agents-home.log\"\r\nexit /b 0\r\n")
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
