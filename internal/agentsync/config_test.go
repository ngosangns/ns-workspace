package agentsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadUserConfigReturnsZeroWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	// Isolate from the real user config dir (e.g. ~/Library/Application Support/ns-workspace).
	t.Setenv("NS_WORKSPACE_CONFIG", filepath.Join(dir, "no-such.json"))
	cfg, err := loadUserConfig(Options{ConfigPath: filepath.Join(dir, "no-such.json")})
	if err != nil {
		t.Fatalf("loadUserConfig: %v", err)
	}
	if cfg.HasOverlay() {
		t.Fatalf("expected zero config, got overlay: %+v", cfg.Entries())
	}
	if cfg.Origin() != "" {
		t.Fatalf("expected empty origin, got %q", cfg.Origin())
	}
}

func TestLoadUserConfigReadsExplicitFlag(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "user-agents.md")
	if err := os.WriteFile(source, []byte("# custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	body, _ := json.Marshal(map[string]string{
		"presets/agents/AGENTS.md": source,
	})
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadUserConfig(Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("loadUserConfig: %v", err)
	}
	if cfg.Origin() != cfgPath {
		t.Fatalf("origin = %q, want %q", cfg.Origin(), cfgPath)
	}
	if got, ok := cfg.Lookup("presets/agents/AGENTS.md"); !ok || got != source {
		t.Fatalf("lookup AGENTS.md = %q, %v; want %q, true", got, ok, source)
	}
}

func TestLoadUserConfigRejectsInvalidKeys(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "x.md")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "bad.json")
	cases := []map[string]string{
		{"not-a-presets-path": source},
		{"/absolute/leading/slash": source},
		{"presets/x/y": ""},
		{"presets/x/y": "relative/path"},
		{"presets/x/y": filepath.Join(dir, "missing-file.md")},
		{"presets/x/y": dir},
	}
	for i, body := range cases {
		data, _ := json.Marshal(body)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := loadUserConfig(Options{ConfigPath: cfgPath})
		if err == nil {
			t.Fatalf("case %d: expected error for %v", i, body)
		}
	}
}

func TestUserConfigEntriesUnderReturnsRelativeNames(t *testing.T) {
	cfg := UserConfig{
		entries: map[string]string{
			"presets/skills/init/SKILL.md":         "/tmp/a",
			"presets/skills/cleanup/SKILL.md":      "/tmp/b",
			"presets/skills/custom-skill/SKILL.md": "/tmp/c",
			"presets/subagents/opencode-intern.md": "/tmp/d",
			"presets/opencode/opencode.json":       "/tmp/e",
		},
	}
	got := cfg.EntriesUnder("presets/skills")
	want := []string{"cleanup/SKILL.md", "custom-skill/SKILL.md", "init/SKILL.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EntriesUnder skills = %v, want %v", got, want)
	}
	if got := cfg.EntriesUnder("presets/agents"); len(got) != 0 {
		t.Fatalf("EntriesUnder agents = %v, want empty", got)
	}
}

func TestUserConfigNormalizesBackslashes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(source, []byte(`{"permission":"allow"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	// JSON keys with Windows-style backslashes should still be matched when
	// callers use forward slashes.
	body := []byte(`{"presets\\opencode\\opencode.json": "` + source + `"}`)
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadUserConfig(Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("loadUserConfig: %v", err)
	}
	if got, ok := cfg.Lookup("presets/opencode/opencode.json"); !ok || got != source {
		t.Fatalf("expected normalized lookup to find entry, got %q, %v", got, ok)
	}
}

func TestUserConfigOverlayOverridesOpenCodeManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	// Build a user config that swaps the opencode permission and adds a
	// custom MCP server. Also adds a brand-new skill the embedded presets
	// do not ship.
	dir := t.TempDir()
	opencodeOverride := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(opencodeOverride, []byte(`{"permission":"allow","timeout":300000}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mcpOverride := filepath.Join(dir, "servers.json")
	if err := os.WriteFile(mcpOverride, []byte(`{"mcpServers":{"my-custom":{"type":"http","url":"https://example.com/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	customSkill := filepath.Join(dir, "custom-skill.md")
	if err := os.WriteFile(customSkill, []byte("# user skill override\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	body, _ := json.Marshal(map[string]string{
		"presets/opencode/opencode.json":       opencodeOverride,
		"presets/mcp/servers.json":             mcpOverride,
		"presets/skills/custom-skill/SKILL.md": customSkill,
	})
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("opencode"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// OpenCode native config should reflect the overlay.
	nativeOC := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if nativeOC["permission"] != "allow" {
		t.Fatalf("opencode permission not preserved: %+v", nativeOC)
	}
	if _, ok := nativeOC["timeout"]; !ok {
		t.Fatalf("opencode config missing user timeout: %+v", nativeOC)
	}
	// User MCP server flows into the opencode config (opencode-specific key
	// is "mcp", not "mcpServers").
	if _, ok := nativeOC["mcp"].(map[string]any)["my-custom"]; !ok {
		t.Fatalf("opencode config missing user MCP: %+v", nativeOC["mcp"])
	}

	// The custom skill is installed in the shared home. OpenCode discovers
	// ~/.agents/skills natively, so there is no native skill mirror.
	sharedSkill := filepath.Join(home, ".agents", "skills", "custom-skill", "SKILL.md")
	content := readFile(t, sharedSkill)
	if !strings.Contains(content, "user skill override") {
		t.Fatalf("shared skill did not pick up user content: %q", content)
	}
	mustNotExist(t, filepath.Join(home, ".config", "opencode", "skill", "custom-skill", "SKILL.md"))
}

func TestUserConfigOverlayIsAbsentByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("NS_WORKSPACE_CONFIG", filepath.Join(t.TempDir(), "no-such-config.json"))

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("opencode"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	nativeOC := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	// Without overlay the opencode config should match the embedded preset:
	// permission "allow" and the standard MCP servers but no "timeout" key.
	if _, ok := nativeOC["timeout"]; ok {
		t.Fatalf("embedded opencode config should not include timeout: %+v", nativeOC)
	}
	if nativeOC["permission"] != "allow" {
		t.Fatalf("embedded opencode config permission changed: %+v", nativeOC)
	}
}

func TestUserConfigOverlayAdditionIsInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	dir := t.TempDir()
	customSkill := filepath.Join(dir, "user-skill.md")
	if err := os.WriteFile(customSkill, []byte("# user added skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	body, _ := json.Marshal(map[string]string{
		"presets/skills/custom-only/SKILL.md": customSkill,
	})
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("claude"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	sharedSkill := filepath.Join(home, ".agents", "skills", "custom-only", "SKILL.md")
	content := readFile(t, sharedSkill)
	if !strings.Contains(content, "user added skill") {
		t.Fatalf("user skill did not get installed: %q", content)
	}
	mustExist(t, filepath.Join(home, ".claude", "skills", "custom-only", "SKILL.md"))
}
