package agentsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateRemovesDisabledMCPFromJSONProviders locks the portal-disable →
// update path for providers that merge MCP as JSON (kiro, kimi, cline,
// opencode, claude, qwen, antigravity).
func TestUpdateRemovesDisabledMCPFromJSONProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	agentsDir := filepath.Join(home, ".agents")
	manager := Manager{Presets: os.DirFS("../..")}

	overlayAll := filepath.Join(t.TempDir(), "servers-all.json")
	allBody := `{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`
	if err := os.WriteFile(overlayAll, []byte(allBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "cfg.json")
	cfgBody, _ := json.Marshal(map[string]string{MCPEnabledPath: overlayAll})
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		t.Fatal(err)
	}

	opt := Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("kiro,kimi,cline,opencode,claude,qwen,antigravity"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	paths := []string{
		filepath.Join(home, ".kiro", "settings", "mcp.json"),
		filepath.Join(home, ".kimi", "mcp.json"),
		filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
		filepath.Join(home, ".config", "opencode", "opencode.json"),
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".qwen", "settings.json"),
		filepath.Join(home, ".gemini", "config", "mcp_config.json"),
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s after init: %v", p, err)
		}
		if !strings.Contains(string(b), "chrome-devtools") {
			t.Fatalf("%s missing chrome-devtools after init: %s", p, b)
		}
	}

	// Portal disable: keep only context7 in the enabled overlay.
	overlayOne := filepath.Join(t.TempDir(), "servers-one.json")
	if err := os.WriteFile(overlayOne, []byte(`{"mcpServers":{"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgBody2, _ := json.Marshal(map[string]string{MCPEnabledPath: overlayOne})
	if err := os.WriteFile(cfgPath, cfgBody2, 0o644); err != nil {
		t.Fatal(err)
	}

	opt.Command = "update"
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update: %v", err)
	}

	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s after update: %v", p, err)
		}
		s := string(b)
		if strings.Contains(s, "chrome-devtools") {
			t.Errorf("disabled MCP still present in %s:\n%s", p, s)
		}
		if !strings.Contains(s, "context7") {
			t.Errorf("enabled MCP missing in %s:\n%s", p, s)
		}
	}
}

// TestUpdateRemovesDisabledMCPFromGrokAndCodex covers TOML managed blocks:
// shrink catalog (portal disable), orphan tables outside markers, and
// clear-all when the enabled catalog is empty.
func TestUpdateRemovesDisabledMCPFromGrokAndCodex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	agentsDir := filepath.Join(home, ".agents")
	manager := Manager{Presets: os.DirFS("../..")}

	overlayAll := filepath.Join(t.TempDir(), "servers-all.json")
	allBody := `{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`
	if err := os.WriteFile(overlayAll, []byte(allBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "cfg.json")
	cfgBody, _ := json.Marshal(map[string]string{MCPEnabledPath: overlayAll})
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		t.Fatal(err)
	}

	opt := Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("grok,codex"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Duplicate a managed server outside the markers (vendor/user copy).
	for _, p := range []string{
		filepath.Join(home, ".grok", "config.toml"),
		filepath.Join(home, ".codex", "config.toml"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		extra := "[mcp_servers.chrome-devtools]\ncommand = \"stale\"\n\n" + string(b)
		if err := os.WriteFile(p, []byte(extra), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	overlayOne := filepath.Join(t.TempDir(), "servers-one.json")
	if err := os.WriteFile(overlayOne, []byte(`{"mcpServers":{"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Portal disable overlay: server no longer in enabled catalog (and may
	// only exist as an orphan outside the managed block, like live Grok).
	overlayDisabled := filepath.Join(t.TempDir(), "servers-disabled.json")
	if err := os.WriteFile(overlayDisabled, []byte(`{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"serena":{"command":"uvx","args":["serena"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgBody2, _ := json.Marshal(map[string]string{
		MCPEnabledPath:  overlayOne,
		MCPDisabledPath: overlayDisabled,
	})
	if err := os.WriteFile(cfgPath, cfgBody2, 0o644); err != nil {
		t.Fatal(err)
	}

	// Leave a disabled-only orphan outside the managed block (never in the
	// current markers) — matches ~/.grok/config.toml after older updates.
	for _, p := range []string{
		filepath.Join(home, ".grok", "config.toml"),
		filepath.Join(home, ".codex", "config.toml"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		orphan := "[mcp_servers.serena]\ncommand = \"uvx\"\nargs = [\"serena\"]\n\n" + string(b)
		if err := os.WriteFile(p, []byte(orphan), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	opt.Command = "update"
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update: %v", err)
	}

	for _, p := range []string{
		filepath.Join(home, ".grok", "config.toml"),
		filepath.Join(home, ".codex", "config.toml"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if strings.Contains(s, "chrome-devtools") {
			t.Errorf("disabled MCP still present in %s:\n%s", p, s)
		}
		if strings.Contains(s, "serena") {
			t.Errorf("portal-disabled orphan MCP still present in %s:\n%s", p, s)
		}
		if !strings.Contains(s, "context7") {
			t.Errorf("enabled MCP missing in %s:\n%s", p, s)
		}
	}

	// Disable all MCPs — managed block must be cleared, not left stale.
	overlayNone := filepath.Join(t.TempDir(), "servers-none.json")
	if err := os.WriteFile(overlayNone, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgBody3, _ := json.Marshal(map[string]string{MCPEnabledPath: overlayNone})
	if err := os.WriteFile(cfgPath, cfgBody3, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update empty: %v", err)
	}
	for _, p := range []string{
		filepath.Join(home, ".grok", "config.toml"),
		filepath.Join(home, ".codex", "config.toml"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if strings.Contains(s, "context7") || strings.Contains(s, "chrome-devtools") || strings.Contains(s, "ns-workspace mcp") {
			t.Errorf("expected managed MCP cleared when all disabled in %s:\n%s", p, s)
		}
	}
}

func TestRemoveManagedBlockAndExtractNames(t *testing.T) {
	begin := "# >>> ns-workspace mcp >>>"
	end := "# <<< ns-workspace mcp <<<"
	current := "top = true\n\n" + begin + "\n[mcp_servers.foo]\ncommand = \"a\"\n\n[mcp_servers.\"bar-baz\"]\nurl = \"https://x\"\n" + end + "\n\nbottom = true\n"
	names := extractManagedBlockMCPNames(current, begin, end)
	if len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if !got["foo"] || !got["bar-baz"] {
		t.Fatalf("names = %v", names)
	}
	cleared := removeManagedBlock(current, begin, end)
	if strings.Contains(cleared, "ns-workspace mcp") || strings.Contains(cleared, "mcp_servers") {
		t.Fatalf("block not removed: %s", cleared)
	}
	if !strings.Contains(cleared, "top = true") || !strings.Contains(cleared, "bottom = true") {
		t.Fatalf("user content lost: %s", cleared)
	}
	if removeManagedBlock("no block", begin, end) != "no block" {
		t.Fatal("missing block should be unchanged")
	}
}

func TestExtractNonMCPFromManagedBlockPreservesProjects(t *testing.T) {
	begin := "# >>> ns-workspace mcp >>>"
	end := "# <<< ns-workspace mcp <<<"
	// Live-like Codex layout: end marker after user [projects.*] tables.
	current := begin + "\n[mcp_servers]\n[mcp_servers.\"chrome-devtools\"]\ncommand = \"npx\"\n\n[projects.\"/tmp/a\"]\ntrust_level = \"trusted\"\n" + end + "\n"
	trapped := extractNonMCPFromManagedBlock(current, begin, end)
	if !strings.Contains(trapped, `[projects."/tmp/a"]`) {
		t.Fatalf("expected projects preserved, got %q", trapped)
	}
	if strings.Contains(trapped, "mcp_servers") || strings.Contains(trapped, "chrome-devtools") {
		t.Fatalf("MCP tables should not be in trapped content: %q", trapped)
	}
	// Replace block then re-inject.
	block := begin + "\n[mcp_servers.context7]\nurl = \"https://x\"\n" + end + "\n"
	next := replaceManagedBlock(current, begin, end, block)
	next = injectAfterManagedBlock(next, end, trapped)
	if !strings.Contains(next, "context7") {
		t.Fatalf("new MCP missing: %s", next)
	}
	if strings.Contains(next, "chrome-devtools") {
		t.Fatalf("old MCP still present: %s", next)
	}
	if !strings.Contains(next, `[projects."/tmp/a"]`) {
		t.Fatalf("projects lost after rewrite: %s", next)
	}
	// projects must sit outside the new markers
	start := strings.Index(next, begin)
	stop := strings.Index(next, end)
	inner := next[start:stop]
	if strings.Contains(inner, "projects.") {
		t.Fatalf("projects still inside managed markers:\n%s", next)
	}
}
