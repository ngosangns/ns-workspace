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
			"presets/skills/minimax-cli/SKILL.md":  "/tmp/c",
			"presets/subagents/opencode-intern.md": "/tmp/d",
			"presets/opencode/opencode.json":       "/tmp/e",
		},
	}
	got := cfg.EntriesUnder("presets/skills")
	want := []string{"cleanup/SKILL.md", "init/SKILL.md", "minimax-cli/SKILL.md"}
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
	customSkill := filepath.Join(dir, "minimax-cli.md")
	if err := os.WriteFile(customSkill, []byte("# user skill override\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	body, _ := json.Marshal(map[string]string{
		"presets/opencode/opencode.json":      opencodeOverride,
		"presets/mcp/servers.json":            mcpOverride,
		"presets/skills/minimax-cli/SKILL.md": customSkill,
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

	// The custom skill should be installed in the shared home AND in the
	// opencode native skill dir (since opencode was selected).
	sharedSkill := filepath.Join(home, ".agents", "skills", "minimax-cli", "SKILL.md")
	content := readFile(t, sharedSkill)
	if !strings.Contains(content, "user skill override") {
		t.Fatalf("shared skill did not pick up user content: %q", content)
	}
	mustExist(t, filepath.Join(home, ".config", "opencode", "skill", "minimax-cli", "SKILL.md"))
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



func TestMiniMaxAdapterWritesDefaultConfig(t *testing.T) {
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
		ToolFilter: ParseTools("minimax"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("minimax init failed: %v", err)
	}

	mmxConfig := readJSONFile(t, filepath.Join(home, ".mmx", "config.json"))
	if mmxConfig["defaultTextModel"] != "MiniMax-M3" {
		t.Fatalf("expected defaultTextModel MiniMax-M3, got %+v", mmxConfig)
	}
	if mmxConfig["defaultVideoModel"] != "MiniMax-Hailuo-2.3" {
		t.Fatalf("expected defaultVideoModel from preset, got %+v", mmxConfig)
	}
	if mmxConfig["defaultMusicModel"] != "music-2.6" {
		t.Fatalf("expected defaultMusicModel from preset, got %+v", mmxConfig)
	}
	if mmxConfig["defaultSpeechModel"] != "speech-2.8-hd" {
		t.Fatalf("expected defaultSpeechModel from preset, got %+v", mmxConfig)
	}
	if mmxConfig["timeout"] != float64(1800) {
		t.Fatalf("expected timeout 1800s, got %+v", mmxConfig["timeout"])
	}
	if mmxConfig["sessionTimeout"] != float64(1800) {
		t.Fatalf("expected sessionTimeout 1800s, got %+v", mmxConfig["sessionTimeout"])
	}

	// mmx-cli has no skills / subagents / MCP user-level directories, so the
	// adapter should NOT link shared skills/agents into ~/.mmx.
	mustNotExist(t, filepath.Join(home, ".mmx", "skills"))
	mustNotExist(t, filepath.Join(home, ".mmx", "agents"))
}

func TestMiniMaxAdapterAliasMatches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	for _, alias := range []string{"minimax", "minimax-cli", "mmx"} {
		opt := Options{
			Command:    "init",
			AgentsDir:  filepath.Join(home, ".agents"),
			NoRegistry: true,
			ToolFilter: ParseTools(alias),
		}
		if err := manager.Apply(opt, false); err != nil {
			t.Fatalf("alias %q init failed: %v", alias, err)
		}
		mmxConfig := filepath.Join(home, ".mmx", "config.json")
		raw, err := os.ReadFile(mmxConfig)
		if err != nil {
			t.Fatalf("alias %q: expected %s to exist: %v", alias, mmxConfig, err)
		}
		if !strings.Contains(string(raw), "defaultTextModel") {
			t.Fatalf("alias %q: mmx config did not pick up preset: %s", alias, raw)
		}
		os.Remove(mmxConfig)
	}
}

func TestMiniMaxAdapterRespectsUserConfigOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	dir := t.TempDir()
	mmxOverride := filepath.Join(dir, "mmx-config.json")
	// User overlay replaces the entire preset, so all keys must be present
	// in the override file (including the timeout defaults the user wants
	// to inherit). Region is the new key the user adds on top.
	if err := os.WriteFile(mmxOverride, []byte(`{"defaultTextModel":"MiniMax-M2.7","region":"cn","defaultSpeechModel":"speech-2.8-hd","defaultVideoModel":"MiniMax-Hailuo-2.3","defaultMusicModel":"music-2.6","timeout":300,"sessionTimeout":1800}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ns-workspace.json")
	body, _ := json.Marshal(map[string]string{
		"presets/minimax/config.json": mmxOverride,
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
		ToolFilter: ParseTools("minimax"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("minimax init with overlay failed: %v", err)
	}
	mmxConfig := readJSONFile(t, filepath.Join(home, ".mmx", "config.json"))
	if mmxConfig["defaultTextModel"] != "MiniMax-M2.7" {
		t.Fatalf("overlay should override defaultTextModel, got %+v", mmxConfig)
	}
	if mmxConfig["region"] != "cn" {
		t.Fatalf("overlay should add region, got %+v", mmxConfig)
	}
}

func TestMiniMaxAdapterUpdateRewritesManagedKeys(t *testing.T) {
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
		ToolFilter: ParseTools("minimax"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	// Simulate a stale managed key the user might have renamed manually.
	mmxPath := filepath.Join(home, ".mmx", "config.json")
	patched := []byte(`{"defaultTextModel":"stale-old-model","defaultSpeechModel":"speech-2.8-hd","defaultVideoModel":"MiniMax-Hailuo-2.3","defaultMusicModel":"music-2.6","timeout":60,"sessionTimeout":300}`)
	if err := os.WriteFile(mmxPath, patched, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	mmxConfig := readJSONFile(t, mmxPath)
	if mmxConfig["defaultTextModel"] != "MiniMax-M3" {
		t.Fatalf("update did not restore preset value, got %+v", mmxConfig)
	}
	if mmxConfig["timeout"] != float64(1800) {
		t.Fatalf("update did not restore timeout, got %+v", mmxConfig["timeout"])
	}
}

func TestMiniMaxAdapterDoesNotPolluteSharedSettings(t *testing.T) {
	// Regression: when Targets.Settings was set on the minimax adapter, the
	// plan linked shared ~/.agents/settings.json into ~/.mmx/config.json.
	// The plugin's MergeJSON then read the symlink's content ({"hooks": {}}),
	// merged the minimax preset on top, and wrote the merged blob to a new
	// regular file at ~/.mmx/config.json. The shared settings were safe
	// because the symlink got renamed to a .bak-* during the write, but
	// ~/.mmx/config.json ended up carrying the shared "hooks" key — a
	// mmx-cli would then send a malformed config to the mmx daemon.
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
		ToolFilter: ParseTools("minimax"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	mmx := readJSONFile(t, filepath.Join(home, ".mmx", "config.json"))
	for _, leaked := range []string{"hooks", "mcpServers"} {
		if _, ok := mmx[leaked]; ok {
			t.Fatalf("~/.mmx/config.json leaked shared %q key: %+v", leaked, mmx)
		}
	}
	// Symlink should never have been created either: the plugin writes
	// directly to ~/.mmx/config.json, not via the shared settings file.
	if info, err := os.Lstat(filepath.Join(home, ".mmx", "config.json")); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("~/.mmx/config.json should not be a symlink")
		}
	}
}

func TestMiniMaxCatalogReportsStableWithSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	ctx, err := manager.context(Options{
		Command:    "agents",
		AgentsDir:  filepath.Join(home, ".agents"),
		ToolFilter: ParseTools("minimax"),
	})
	if err != nil {
		t.Fatal(err)
	}
	var found *specAdapter
	for _, a := range manager.adapters(ctx) {
		if a.Name() == "minimax" {
			sa, ok := a.(specAdapter)
			if !ok {
				t.Fatalf("minimax adapter is not a specAdapter: %T", a)
			}
			found = &sa
			break
		}
	}
	if found == nil {
		t.Fatalf("minimax adapter missing from Manager.adapters()")
	}
	caps := found.Capabilities()
	if caps.Tier != TierStable {
		t.Fatalf("minimax tier = %s, want stable", caps.Tier)
	}
	hasSettings := false
	for _, a := range caps.Artifacts {
		if a == ArtifactSettings {
			hasSettings = true
			break
		}
	}
	if !hasSettings {
		t.Fatalf("minimax capabilities missing settings artifact: %+v", caps.Artifacts)
	}
	// Aliases must be exposed for --tools minimax-cli / --tools mmx to work.
	aliases := found.Aliases()
	want := map[string]bool{"minimax-cli": true, "mmx": true}
	for _, alias := range aliases {
		delete(want, alias)
	}
	if len(want) > 0 {
		t.Fatalf("minimax missing aliases: %+v (got %v)", want, aliases)
	}
}

func TestMiniMaxDoctorReportsExecutableAndConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	// Drop a fake `mmx` binary on PATH so doctor reports it as found.
	binDir := t.TempDir()
	fakeMmx := filepath.Join(binDir, "mmx")
	if err := os.WriteFile(fakeMmx, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("minimax"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Status should surface ~/.mmx/config.json without --minimax-only filter.
	if err := manager.Status(Options{
		Command:    "status",
		AgentsDir:  opt.AgentsDir,
		NoRegistry: true,
		ToolFilter: ParseTools("minimax"),
	}); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".mmx", "config.json")); err != nil {
		t.Fatalf("expected ~/.mmx/config.json to exist for status: %v", err)
	}
}

func TestMiniMaxDryRunDoesNotWrite(t *testing.T) {
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
		ToolFilter: ParseTools("minimax"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("dry-run init failed: %v", err)
	}
	mustNotExist(t, filepath.Join(home, ".mmx"))
	mustNotExist(t, filepath.Join(home, ".agents"))
}
