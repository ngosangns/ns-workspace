package agentsync

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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
	mustExist(t, filepath.Join(home, ".agents", "skills", "cleanup", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "init", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "spawn-opencode", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "_shared", "CONVENTIONS.md"))
	mustExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustExist(t, filepath.Join(home, ".claude", "agents", "opencode-intern.md"))
	mustExist(t, filepath.Join(home, ".grok", "AGENTS.md"))
	mustNotExist(t, filepath.Join(home, ".grok", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".grok", "config.toml"))
	mustExist(t, filepath.Join(home, ".kiro", "steering", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".kiro", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".kiro", "settings", "mcp.json"))
	mustExist(t, filepath.Join(home, ".qwen", "settings.json"))
	mustExist(t, filepath.Join(home, ".gemini", "GEMINI.md"))
	mustExist(t, filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	mustExist(t, filepath.Join(home, ".gemini", "config", "mcp_config.json"))
	mustExist(t, filepath.Join(home, ".codex", "config.toml"))
	mustExist(t, filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"))
	mustExist(t, filepath.Join(home, ".config", "opencode", "opencode.json"))

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
	if !strings.Contains(opencode, `"type": "local"`) || !strings.Contains(opencode, `"enabled": true`) || !strings.Contains(opencode, "chrome-devtools") {
		t.Fatalf("opencode config should include local MCP with type/enabled and argv command: %s", opencode)
	}
	if strings.Contains(opencode, `"command": "npx"`) {
		t.Fatalf("opencode local MCP must use command argv array, not string: %s", opencode)
	}

	codex := readFile(t, filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(codex, "[mcp_servers.\"context7\"]") {
		t.Fatalf("codex config did not include MCP preset: %s", codex)
	}

	grok := readFile(t, filepath.Join(home, ".grok", "config.toml"))
	if !strings.Contains(grok, "ns-workspace mcp") || !strings.Contains(grok, "[mcp_servers.context7]") || !strings.Contains(grok, `url = "https://mcp.context7.com/mcp"`) {
		t.Fatalf("grok config did not include managed MCP preset: %s", grok)
	}
	if strings.Contains(grok, `type = "http"`) || strings.Contains(grok, `"type"`) {
		t.Fatalf("grok MCP block should drop shared type field: %s", grok)
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
		filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"),
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

func TestBuildPlanExposesSyncPhaseOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		ToolFilter: ParseTools("stable"),
	}

	plan, err := manager.BuildPlan(opt, false)
	if err != nil {
		t.Fatalf("build plan failed: %v", err)
	}

	want := []PlanPhaseName{PhaseCore, PhaseRegistryHelpers, PhaseRegistryInstall, PhaseMCP, PhaseAdapters}
	if got := planPhaseNames(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected plan phase order:\n got: %v\nwant: %v", got, want)
	}
	if plan.Mode != "init" || plan.AgentsDir != filepath.Join(home, ".agents") {
		t.Fatalf("unexpected plan metadata: %+v", plan)
	}
	if len(plan.Phases[0].Operations) == 0 || plan.Phases[0].Operations[0].Artifact != ArtifactDirectory {
		t.Fatalf("core phase should begin with directory creation, got %+v", plan.Phases[0].Operations)
	}
	mustNotExist(t, filepath.Join(home, ".agents"))
}

func TestUpdatePlanReadsPresetManifestsInsteadOfStaleSharedOutputs(t *testing.T) {
	home := t.TempDir()
	agentsHome := filepath.Join(home, ".agents")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	if err := os.MkdirAll(filepath.Join(agentsHome, "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsHome, "mcp", "servers.json"), []byte(`{"mcpServers":{"stale":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsHome, "settings.json"), []byte(`{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"stale"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := Manager{Presets: os.DirFS("../..")}
	plan, err := manager.BuildPlan(Options{
		Command:    "update",
		AgentsDir:  agentsHome,
		NoRegistry: true,
		ToolFilter: ParseTools("qwen"),
	}, true)
	if err != nil {
		t.Fatalf("build update plan failed: %v", err)
	}

	var foundMCP bool
	for _, phase := range plan.Phases {
		for _, planned := range phase.Operations {
			merge, ok := planned.Op.(MergeJSON)
			if !ok || planned.Owner != "qwen" || planned.Artifact != ArtifactMCP {
				continue
			}
			foundMCP = true
			if _, ok := merge.Values["stale"]; ok {
				t.Fatalf("update plan kept stale shared MCP manifest: %+v", merge.Values)
			}
			if _, ok := merge.Values["context7"]; !ok {
				t.Fatalf("update plan did not read embedded MCP preset: %+v", merge.Values)
			}
		}
	}
	if !foundMCP {
		t.Fatalf("qwen MCP merge operation not found in plan: %+v", plan.Phases)
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

// TestUpdateCopiesKiroSkillsInsteadOfSymlink ensures update replaces skill
// symlinks with real directories. Kiro IDE does not follow ~/.kiro/skills
// symlinks (github.com/kirodotdev/Kiro/issues/6401).
func TestUpdateCopiesKiroSkillsInsteadOfSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("init always copies on Windows; symlink→copy migration is Unix-only")
	}
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
		ToolFilter: ParseTools("kiro"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	skillDir := filepath.Join(home, ".kiro", "skills", "execution")
	info, err := os.Lstat(skillDir)
	if err != nil {
		t.Fatalf("lstat after init: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("init without --copy should create skill symlink, got mode %v", info.Mode())
	}

	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	info, err = os.Lstat(skillDir)
	if err != nil {
		t.Fatalf("lstat after update: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(skillDir)
		t.Fatalf("update should copy skills (not symlink); still link -> %s", target)
	}
	if !info.IsDir() {
		t.Fatalf("update skill path should be a real directory, got mode %v", info.Mode())
	}
	mustExist(t, filepath.Join(skillDir, "SKILL.md"))
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

	mustExist(t, filepath.Join(home, ".grok", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".grok", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".grok", "config.toml"))
	grok := readFile(t, filepath.Join(home, ".grok", "config.toml"))
	if !strings.Contains(grok, "[mcp_servers.context7]") || !strings.Contains(grok, "ns-workspace mcp") {
		t.Fatalf("grok selection should write managed MCP block: %s", grok)
	}
	mustNotExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustNotExist(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
}

func TestGrokNoMCPSkipsManagedBlock(t *testing.T) {
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
		NoMCP:      true,
		ToolFilter: ParseTools("grok"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".grok", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".grok", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".grok", "config.toml"))
}

func TestGrokUpdatePreservesUserConfigTOML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	grokDir := filepath.Join(home, ".grok")
	if err := os.MkdirAll(grokDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seed := "[cli]\nauto_update = true\n\n[ui]\npermission_mode = \"always-approve\"\n\n"
	if err := os.WriteFile(filepath.Join(grokDir, "config.toml"), []byte(seed), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "update",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("grok"),
	}
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got := readFile(t, filepath.Join(home, ".grok", "config.toml"))
	if !strings.Contains(got, "permission_mode = \"always-approve\"") || !strings.Contains(got, "auto_update = true") {
		t.Fatalf("user TOML sections should be preserved: %s", got)
	}
	if !strings.Contains(got, "ns-workspace mcp") || !strings.Contains(got, "[mcp_servers.context7]") {
		t.Fatalf("managed MCP block should be present after update: %s", got)
	}
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

func TestKiroAgentConfigLoadsSkillsAndSteering(t *testing.T) {
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
		ToolFilter: ParseTools("kiro"),
	}

	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	agent := readJSONFile(t, filepath.Join(home, ".kiro", "agents", "ns-full.json"))
	if agent["name"] != "ns-full" {
		t.Fatalf("agent name = %v, want ns-full", agent["name"])
	}
	if !reflect.DeepEqual(agent["tools"], []any{"*"}) {
		t.Fatalf("agent tools = %v, want [*]", agent["tools"])
	}
	if !reflect.DeepEqual(agent["allowedTools"], []any{"@builtin", "@*"}) {
		t.Fatalf("agent allowedTools = %v, want [@builtin @*]", agent["allowedTools"])
	}
	if agent["model"] != "gpt-5.6-terra" {
		t.Fatalf("agent model = %v, want gpt-5.6-terra", agent["model"])
	}
	if agent["includeMcpJson"] != true {
		t.Fatalf("agent includeMcpJson = %v, want true", agent["includeMcpJson"])
	}
	toolsSettings, ok := agent["toolsSettings"].(map[string]any)
	if !ok || len(toolsSettings) == 0 {
		t.Fatalf("agent toolsSettings missing or empty: %v", agent["toolsSettings"])
	}
	shellSettings, _ := toolsSettings["shell"].(map[string]any)
	if shellSettings == nil {
		t.Fatalf("agent toolsSettings.shell missing: %v", toolsSettings)
	}

	resources, ok := agent["resources"].([]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("agent resources missing or empty: %v", agent["resources"])
	}
	hasSkillResource, hasSteeringResource := false, false
	for _, r := range resources {
		rs, _ := r.(string)
		if strings.Contains(rs, "skill://") && strings.Contains(rs, ".kiro/skills") {
			hasSkillResource = true
		}
		if strings.Contains(rs, "file://") && strings.Contains(rs, ".kiro/steering") {
			hasSteeringResource = true
		}
	}
	if !hasSkillResource {
		t.Fatalf("agent resources missing skill URI: %v", resources)
	}
	if !hasSteeringResource {
		t.Fatalf("agent resources missing steering URI: %v", resources)
	}

	mcp := readJSONFile(t, filepath.Join(home, ".kiro", "settings", "mcp.json"))
	servers, ok := mcp["mcpServers"].(map[string]any)
	if !ok || len(servers) == 0 {
		t.Fatalf("kiro mcp.json missing mcpServers: %v", mcp)
	}
	for name, server := range servers {
		s, ok := server.(map[string]any)
		if !ok {
			t.Fatalf("mcp server %q is not an object: %v", name, server)
		}
		hasHTTP := s["type"] == "http" && s["url"] != nil
		hasCommand := s["command"] != nil
		if !hasHTTP && !hasCommand {
			t.Fatalf("mcp server %q missing http or command config: %v", name, s)
		}
	}
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

func TestAdapterCatalogInvariants(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	ctx, err := manager.context(Options{
		Command:    "agents",
		AgentsDir:  filepath.Join(home, ".agents"),
		ToolFilter: ParseTools("all"),
	})
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	aliases := map[string]string{}
	validTiers := map[SupportTier]bool{TierStable: true, TierManual: true, TierExperimental: true, TierCatalog: true}
	for _, adapter := range manager.adapters(ctx) {
		name := adapter.Name()
		if name == "" {
			t.Fatalf("adapter has empty name: %#v", adapter)
		}
		if names[name] {
			t.Fatalf("duplicate adapter name: %s", name)
		}
		names[name] = true
		caps := adapter.Capabilities()
		if !validTiers[caps.Tier] {
			t.Fatalf("%s has invalid tier %q", name, caps.Tier)
		}
		if caps.Tier == TierStable && len(caps.Artifacts) == 0 {
			t.Fatalf("stable adapter %s must expose at least one artifact", name)
		}
		if aliased, ok := adapter.(interface{ Aliases() []string }); ok {
			for _, alias := range aliased.Aliases() {
				alias = strings.ToLower(alias)
				if owner := aliases[alias]; owner != "" {
					t.Fatalf("alias %q is used by both %s and %s", alias, owner, name)
				}
				aliases[alias] = name
			}
		}
		if caps.Tier != TierStable {
			for _, path := range adapter.StatusPaths(ctx) {
				wantPrefix := filepath.Join(ctx.Options.AgentsDir, "generated")
				if !strings.HasPrefix(path, wantPrefix) {
					t.Fatalf("%s should only write generated guidance, got %s", name, path)
				}
			}
		}
	}
}

func TestRegistryCommandArgsStayAlignedWithScriptCommand(t *testing.T) {
	skill := RegistrySkill{Name: "taste-skill", Source: "leonxlnx/taste-skill", Skill: "design-taste-frontend"}
	wantArgs := []string{"--yes", "skills", "add", "leonxlnx/taste-skill", "--skill", "design-taste-frontend", "--global", "--agent", "universal", "--yes", "--copy"}
	if got := registryCommandArgs(skill, true, true); !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("unexpected registry args:\n got: %v\nwant: %v", got, wantArgs)
	}
	line := registryCommand(skill, true, true, "/tmp/ns-agents")
	for _, part := range []string{"AGENTS_HOME=/tmp/ns-agents", "npx --yes skills add leonxlnx/taste-skill", "--skill design-taste-frontend", "--global", "--agent universal", "--yes", "--copy"} {
		if !strings.Contains(line, part) {
			t.Fatalf("registry command %q missing %q", line, part)
		}
	}
}

func TestValidateRegistrySource(t *testing.T) {
	for _, bad := range []string{
		"org/repo",
		"https://github.com/org/repo.git",
		"git@github.com:org/repo.git",
		"owner/repo",
		"user/repo",
		"example/repo",
		"your-org/your-repo",
		"foo/bar",
		"nopath",
		"",
	} {
		if err := validateRegistrySource(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	for _, ph := range []string{"org/repo", "ORG/REPO", "https://github.com/org/repo.git", "owner/repo", "foo/bar"} {
		if !IsPlaceholderRegistrySource(ph) {
			t.Fatalf("IsPlaceholderRegistrySource(%q) = false, want true", ph)
		}
	}
	for _, good := range []string{"github/awesome-copilot", "vercel-labs/skills", "https://github.com/github/awesome-copilot.git", "2389-research/landing-page-design"} {
		if err := validateRegistrySource(good); err != nil {
			t.Fatalf("unexpected error for %q: %v", good, err)
		}
		if IsPlaceholderRegistrySource(good) {
			t.Fatalf("IsPlaceholderRegistrySource(%q) = true, want false", good)
		}
	}
}

func TestSanitizeRegistrySkillsDropsPlaceholders(t *testing.T) {
	in := []RegistrySkill{
		{Name: "ok", Source: "github/awesome-copilot", Skill: "git-commit"},
		{Name: "bad", Source: "org/repo", Skill: "new"},
		{Name: "but", Skill: "but", Installer: installerButSkill},
	}
	got := SanitizeRegistrySkills(in)
	if len(got) != 2 {
		t.Fatalf("SanitizeRegistrySkills = %+v, want 2 entries (ok + but)", got)
	}
	for _, sk := range got {
		if sk.Source == "org/repo" || IsPlaceholderRegistrySource(sk.Source) && sk.Installer != installerButSkill {
			t.Fatalf("placeholder leaked: %+v", sk)
		}
	}
}

func TestButSkillRegistryCommandUsesButCLI(t *testing.T) {
	skill := RegistrySkill{Name: "gitbutler", Skill: "but", Installer: installerButSkill}
	if skill.installerKind() != installerButSkill {
		t.Fatalf("installerKind = %q", skill.installerKind())
	}
	path := butSkillInstallPath("/tmp/ns-agents", skill)
	if path != "/tmp/ns-agents/skills/but" {
		t.Fatalf("butSkillInstallPath = %q", path)
	}
	line := registryCommand(skill, true, false, "/tmp/ns-agents")
	for _, part := range []string{"but skill install", "--path", "/tmp/ns-agents/skills/but", "--format none"} {
		if !strings.Contains(line, part) {
			t.Fatalf("but skill command %q missing %q", line, part)
		}
	}
	if strings.Contains(line, "npx") {
		t.Fatalf("but-skill entry must not use npx: %q", line)
	}
}

func TestInstallRegistrySkillsButSkillDryRun(t *testing.T) {
	home := t.TempDir()
	agents := filepath.Join(home, ".agents")
	if err := os.MkdirAll(filepath.Join(agents, "registry"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a minimal registry with only but-skill so npx is not required.
	manifest := `{"skills":[{"name":"gitbutler","skill":"but","installer":"but-skill"}]}`
	if err := os.WriteFile(filepath.Join(agents, "registry", "skills.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := &bufferReporter{}
	ctx := Context{
		Options: Options{
			AgentsDir: agents,
			DryRun:    true,
		},
		Report:        rep,
		manifestCache: map[string]any{},
	}
	if err := installRegistrySkills(ctx); err != nil {
		t.Fatalf("installRegistrySkills dry-run: %v", err)
	}
	joined := rep.joined()
	if !strings.Contains(joined, "but skill install") || !strings.Contains(joined, filepath.Join(agents, "skills", "but")) {
		t.Fatalf("dry-run should print but skill install path: %s", joined)
	}
}

func TestInstallOneRegistrySkillButSkill(t *testing.T) {
	if _, err := exec.LookPath("but"); err != nil {
		t.Skip("but CLI not installed")
	}
	home := t.TempDir()
	agents := filepath.Join(home, ".agents")
	if err := os.MkdirAll(filepath.Join(agents, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	rep := &bufferReporter{}
	ctx := Context{
		Options:       Options{AgentsDir: agents},
		Report:        rep,
		manifestCache: map[string]any{},
	}
	skill := RegistrySkill{Name: "gitbutler", Skill: "but", Installer: installerButSkill}
	if err := installOneRegistrySkill(ctx, skill, os.Environ(), ""); err != nil {
		t.Fatalf("installOneRegistrySkill but-skill: %v", err)
	}
	mustExist(t, filepath.Join(agents, "skills", "but", "SKILL.md"))
	body := readFile(t, filepath.Join(agents, "skills", "but", "SKILL.md"))
	if !strings.Contains(body, "name: but") && !strings.Contains(body, "GitButler") {
		snippet := body
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("installed but skill looks wrong: %s", snippet)
	}
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

func planPhaseNames(plan SyncPlan) []PlanPhaseName {
	names := make([]PlanPhaseName, 0, len(plan.Phases))
	for _, phase := range plan.Phases {
		names = append(names, phase.Name)
	}
	return names
}

func TestAdapterSettingsPerProviderNoFieldLeak(t *testing.T) {
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
		ToolFilter: ParseTools("claude,qwen,antigravity"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	claudeSettings := readFile(t, filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(claudeSettings, `"defaultMode": "bypassPermissions"`) {
		t.Fatalf("claude settings should have permissions.defaultMode=bypassPermissions: %s", claudeSettings)
	}

	qwenSettings := readFile(t, filepath.Join(home, ".qwen", "settings.json"))
	qwenParsed := readJSONFile(t, filepath.Join(home, ".qwen", "settings.json"))
	perms, _ := qwenParsed["permissions"].(map[string]any)
	if perms == nil {
		t.Fatalf("qwen settings should have permissions field (defaultMode yolo): %s", qwenSettings)
	}
	if mode, _ := perms["defaultMode"].(string); mode != "yolo" {
		t.Fatalf("qwen permissions.defaultMode = %q, want \"yolo\": %s", mode, qwenSettings)
	}
	if qwen, _ := qwenParsed["toolPermission"]; qwen != nil {
		t.Fatalf("qwen settings should NOT have Antigravity-style \"toolPermission\" key (field leak): %s", qwenSettings)
	}

	agySettings := readFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	agyParsed := readJSONFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	if mode, _ := agyParsed["toolPermission"].(string); mode != "always-proceed" {
		t.Fatalf("antigravity settings should have toolPermission=always-proceed: %s", agySettings)
	}
	if agy, _ := agyParsed["permissions"]; agy != nil {
		t.Fatalf("antigravity settings should NOT have Claude/Qwen-style \"permissions\" key (field leak): %s", agySettings)
	}
	if _, ok := agyParsed["mcpServers"]; ok {
		t.Fatalf("antigravity settings should NOT nest mcpServers (MCP is separate mcp_config.json): %s", agySettings)
	}
}

// TestProviderFullBypassConfig asserts every provider preset lands a
// full-bypass / auto-approve configuration in the native config file:
//
//   - Claude Code: `permissions.defaultMode: "bypassPermissions"`.
//   - Qwen Code: `permissions.defaultMode: "yolo"` + `confirmShellCommands:
//     false` + `confirmFileEdits: false`.
//   - Antigravity CLI: `toolPermission: "always-proceed"` +
//     `artifactReviewPolicy: "always-proceed"` in
//     ~/.gemini/antigravity-cli/settings.json.
//   - Cline: per-MCP-server `trust: true` (Cline stores YOLO mode in
//     `~/.cline/data/settings/global-settings.json`, which cannot be set
//     from cline_mcp_settings.json; per-server trust is the closest
//     equivalent shipped via the MCP preset path).
//   - OpenCode: `permission: "allow"` (already shipped via preset).
func TestProviderFullBypassConfig(t *testing.T) {
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
		ToolFilter: ParseTools("claude,opencode,qwen,antigravity,cline"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Claude Code
	claude := readJSONFile(t, filepath.Join(home, ".claude", "settings.json"))
	perms, _ := claude["permissions"].(map[string]any)
	if perms == nil {
		t.Fatalf("claude settings missing permissions: %v", claude)
	}
	if mode, _ := perms["defaultMode"].(string); mode != "bypassPermissions" {
		t.Fatalf("claude permissions.defaultMode = %q, want \"bypassPermissions\": %v", mode, claude)
	}

	// OpenCode
	opencode := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if perm, _ := opencode["permission"].(string); perm != "allow" {
		t.Fatalf("opencode permission = %q, want \"allow\": %v", perm, opencode)
	}

	// Qwen Code
	qwen := readJSONFile(t, filepath.Join(home, ".qwen", "settings.json"))
	qwenPerms, _ := qwen["permissions"].(map[string]any)
	if qwenPerms == nil {
		t.Fatalf("qwen settings missing permissions: %v", qwen)
	}
	if mode, _ := qwenPerms["defaultMode"].(string); mode != "yolo" {
		t.Fatalf("qwen permissions.defaultMode = %q, want \"yolo\": %v", mode, qwen)
	}
	if confirm, _ := qwenPerms["confirmShellCommands"].(bool); confirm {
		t.Fatalf("qwen permissions.confirmShellCommands = true, want false (full bypass): %v", qwenPerms)
	}
	if confirm, _ := qwenPerms["confirmFileEdits"].(bool); confirm {
		t.Fatalf("qwen permissions.confirmFileEdits = true, want false (full bypass): %v", qwenPerms)
	}

	// Antigravity CLI
	agy := readJSONFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	if mode, _ := agy["toolPermission"].(string); mode != "always-proceed" {
		t.Fatalf("antigravity toolPermission = %q, want \"always-proceed\": %v", mode, agy)
	}
	if policy, _ := agy["artifactReviewPolicy"].(string); policy != "always-proceed" {
		t.Fatalf("antigravity artifactReviewPolicy = %q, want \"always-proceed\": %v", policy, agy)
	}

	// Cline — trust flag on every shared MCP server
	cline := readJSONFile(t, filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"))
	clineServers, _ := cline["mcpServers"].(map[string]any)
	if len(clineServers) == 0 {
		t.Fatalf("cline settings missing mcpServers: %v", cline)
	}
	for name, raw := range clineServers {
		server, _ := raw.(map[string]any)
		if trust, _ := server["trust"].(bool); !trust {
			t.Fatalf("cline mcpServers.%s missing trust=true (full bypass): %v", name, server)
		}
	}
}

// TestProviderMCPServerShapeMatchesVendorDocs asserts the MCP server entries
// written into each provider's native config file use the field names and
// discriminator values documented by the tool's vendor.
//
// Verified against:
//
//   - Claude Code settings docs (mcpServers.{name}: type/url or command/args).
//   - OpenCode MCP docs (mcp.{name}: type "remote"+url+enabled, or type "local"+command[]+enabled).
//   - Qwen Code MCP docs (mcpServers.{name}: httpUrl for HTTP servers, no type).
//   - Antigravity MCP docs (mcp_config.json mcpServers.{name}: serverUrl for remote, no type).
//   - Cline MCP docs (mcpServers.{name}: url or command+args, no type field).
func TestProviderMCPServerShapeMatchesVendorDocs(t *testing.T) {
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
		ToolFilter: ParseTools("claude,opencode,qwen,antigravity,cline"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Claude Code: mcpServers.{name}.{type,url} for HTTP servers and
	// {command,args} for stdio servers (both shapes pass through verbatim).
	assertClaudeMCPServers(t, readJSONFile(t, filepath.Join(home, ".claude", "settings.json")))

	// OpenCode: mcp.{name}.{type:"remote",url,enabled} for HTTP servers and
	// {type:"local",command:[...],enabled} for stdio (shared command+args
	// folded into a single argv array per OpenCode schema).
	assertOpenCodeMCPServers(t, readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json")))
}

// assertClaudeMCPServers asserts that every mcpServers entry written into
// Claude Code's settings.json keeps the vendor-documented shape: HTTP
// servers keep `type:"http"` + `url`, stdio servers keep `command` + `args`.
// The shared preset may contain both kinds of entries.
func assertClaudeMCPServers(t *testing.T, claude map[string]any) {
	t.Helper()
	mcpServers, _ := claude["mcpServers"].(map[string]any)
	if mcpServers == nil {
		t.Fatalf("claude settings missing mcpServers: %v", claude)
	}
	for name, raw := range mcpServers {
		server := raw.(map[string]any)
		switch {
		case isHTTPServer(server):
			if server["type"] != "http" {
				t.Fatalf("claude mcpServers.%s expected type=http, got %v: %v", name, server["type"], server)
			}
			if _, ok := server["url"].(string); !ok {
				t.Fatalf("claude mcpServers.%s missing url: %v", name, server)
			}
		case isStdioServer(server):
			if cmd, _ := server["command"].(string); cmd == "" {
				t.Fatalf("claude mcpServers.%s missing command: %v", name, server)
			}
			if _, ok := server["args"].([]any); !ok {
				t.Fatalf("claude mcpServers.%s missing args: %v", name, server)
			}
			if tVal, ok := server["type"]; ok && tVal != nil {
				t.Fatalf("claude mcpServers.%s stdio should not have type field, got %v: %v", name, tVal, server)
			}
		default:
			t.Fatalf("claude mcpServers.%s has unrecognized shape (no url/url+type for HTTP, no command for stdio): %v", name, server)
		}
	}
}

// assertOpenCodeMCPServers asserts that every mcp.{name} entry written
// into OpenCode's config.json matches the vendor schema:
//
//   - remote: type "remote" + url + enabled
//   - local:  type "local" + command as argv array + enabled (no separate args)
//
// Shared presets still use command string + args; the opencode transform
// folds them into command[].
func assertOpenCodeMCPServers(t *testing.T, opencode map[string]any) {
	t.Helper()
	opencodeMCP, _ := opencode["mcp"].(map[string]any)
	if opencodeMCP == nil {
		t.Fatalf("opencode config missing mcp: %v", opencode)
	}
	for name, raw := range opencodeMCP {
		server := raw.(map[string]any)
		if enabled, ok := server["enabled"].(bool); !ok || !enabled {
			t.Fatalf("opencode mcp.%s expected enabled=true, got %v: %v", name, server["enabled"], server)
		}
		switch {
		case server["type"] == "remote" || isHTTPServer(server):
			if server["type"] != "remote" {
				t.Fatalf("opencode mcp.%s expected type=remote, got %v: %v", name, server["type"], server)
			}
			if _, ok := server["url"].(string); !ok {
				t.Fatalf("opencode mcp.%s missing url: %v", name, server)
			}
		case server["type"] == "local" || isStdioServer(server):
			if server["type"] != "local" {
				t.Fatalf("opencode mcp.%s expected type=local, got %v: %v", name, server["type"], server)
			}
			cmd, ok := server["command"].([]any)
			if !ok || len(cmd) == 0 {
				t.Fatalf("opencode mcp.%s expected command argv array, got %v: %v", name, server["command"], server)
			}
			if _, ok := server["args"]; ok {
				t.Fatalf("opencode mcp.%s must not keep separate args field after transform: %v", name, server)
			}
		default:
			t.Fatalf("opencode mcp.%s has unrecognized shape: %v", name, server)
		}
	}
}

// isHTTPServer reports whether an mcpServers entry is an HTTP server.
// It accepts any HTTP-flavoured transport signal:
//
//   - shared preset: `type:"http"` + `url`
//   - qwen output: `httpUrl` (no `type`)
//   - antigravity output: `serverUrl` (no `type`)
//   - cline output: `url` (no `type`)
//   - opencode output: `type:"remote"` + `url`
//
// The presence of a URL field (`url`, `httpUrl`, or `serverUrl`) is the
// canonical signal after the per-adapter transform; we also accept `type`
// values `http`, `sse`, or `remote` for completeness, covering raw preset
// entries and transport-renamed outputs.
func isHTTPServer(server map[string]any) bool {
	if _, hasURL := server["url"]; hasURL {
		return true
	}
	if _, hasHTTPURL := server["httpUrl"]; hasHTTPURL {
		return true
	}
	if _, hasServerURL := server["serverUrl"]; hasServerURL {
		return true
	}
	if typ, ok := server["type"].(string); ok {
		switch typ {
		case "http", "sse", "remote":
			return true
		}
	}
	return false
}

// isStdioServer reports whether an mcpServers entry is a stdio server
// (i.e. has a `command` field). Stdio servers do not carry a `url`.
func isStdioServer(server map[string]any) bool {
	_, hasCmd := server["command"]
	return hasCmd
}

func TestAdapterSettingsBuildsCorrectContent(t *testing.T) {
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
		ToolFilter: ParseTools("claude"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	claudeSettings := readFile(t, filepath.Join(home, ".claude", "settings.json"))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(claudeSettings), &parsed); err != nil {
		t.Fatalf("claude settings is not valid JSON: %v", err)
	}
	if _, ok := parsed["permissions"]; !ok {
		t.Fatalf("claude settings missing permissions key: %s", claudeSettings)
	}
	if perms, ok := parsed["permissions"].(map[string]any); ok {
		if mode, ok := perms["defaultMode"].(string); !ok || mode != "bypassPermissions" {
			t.Fatalf("claude settings permissions.defaultMode = %v, want bypassPermissions", mode)
		}
	}
}

func TestAdapterSettingsHonorsMergeStrategy(t *testing.T) {
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
		ToolFilter: ParseTools("claude"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	claudeSettings := readFile(t, filepath.Join(home, ".claude", "settings.json"))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(claudeSettings), &parsed); err != nil {
		t.Fatalf("claude settings is not valid JSON: %v", err)
	}

	// Verify hooks field exists (from default preset, merge-deep strategy)
	if _, ok := parsed["hooks"]; !ok {
		t.Fatalf("claude settings missing hooks key (should come from default preset): %s", claudeSettings)
	}

	// Verify permissions field exists (from claude preset, replace strategy)
	perms, ok := parsed["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("claude settings missing or invalid permissions key: %s", claudeSettings)
	}
	if mode, ok := perms["defaultMode"].(string); !ok || mode != "bypassPermissions" {
		t.Fatalf("claude settings permissions.defaultMode = %v, want bypassPermissions", mode)
	}

	// Verify mcpServers field exists (from shared MCP manifest, merge-shallow strategy)
	if _, ok := parsed["mcpServers"]; !ok {
		t.Fatalf("claude settings missing mcpServers key (should come from shared MCP manifest): %s", claudeSettings)
	}
}

// TestProviderMCPServerShapeMatchesVendorDocs_QwenAntigravityCline extends the
// provider shape assertions to Qwen, Antigravity and Cline. It is split from
// TestProviderMCPServerShapeMatchesVendorDocs so the suite can be filtered
// to just those providers when running local debugging.
func TestProviderMCPServerShapeMatchesVendorDocs_QwenAntigravityCline(t *testing.T) {
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
		ToolFilter: ParseTools("qwen,antigravity,cline"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Qwen Code: mcpServers.{name}.httpUrl for HTTP servers (no type
	// field), {command,args} unchanged for stdio servers.
	assertQwenMCPServers(t, readJSONFile(t, filepath.Join(home, ".qwen", "settings.json")))

	// Antigravity CLI: MCP lives in ~/.gemini/config/mcp_config.json with
	// serverUrl for remote servers (not nested in settings.json).
	agySettings := readJSONFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	if _, ok := agySettings["mcpServers"]; ok {
		t.Fatalf("antigravity settings should NOT nest mcpServers: %v", agySettings)
	}
	assertAntigravityMCPServers(t, readJSONFile(t, filepath.Join(home, ".gemini", "config", "mcp_config.json")))

	// Cline: mcpServers.{name}.{url} for HTTP servers, {command,args}
	// for stdio servers; the `type` field is stripped on every entry,
	// and `trust:true` is set on every entry (so Cline auto-approves
	// MCP tool calls without per-tool confirmation prompts).
	assertClineMCPServers(t, readJSONFile(t, filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json")))
}

// assertQwenMCPServers asserts Qwen's MCP shape: HTTP servers use
// `httpUrl` (not `url` + `type`); stdio servers keep `command` + `args`
// verbatim. The `type` field is dropped across the board because Qwen
// does not document that discriminator.
func assertQwenMCPServers(t *testing.T, qwen map[string]any) {
	t.Helper()
	qwenServers, _ := qwen["mcpServers"].(map[string]any)
	if qwenServers == nil {
		t.Fatalf("qwen settings missing mcpServers: %v", qwen)
	}
	for name, raw := range qwenServers {
		server := raw.(map[string]any)
		if _, ok := server["type"]; ok {
			t.Fatalf("qwen mcpServers.%s should NOT have type field: %v", name, server)
		}
		switch {
		case isHTTPServer(server):
			if _, ok := server["httpUrl"].(string); !ok {
				t.Fatalf("qwen mcpServers.%s missing httpUrl: %v", name, server)
			}
		case isStdioServer(server):
			if cmd, _ := server["command"].(string); cmd == "" {
				t.Fatalf("qwen mcpServers.%s missing command: %v", name, server)
			}
			if _, ok := server["args"].([]any); !ok {
				t.Fatalf("qwen mcpServers.%s missing args: %v", name, server)
			}
		default:
			t.Fatalf("qwen mcpServers.%s has unrecognized shape: %v", name, server)
		}
	}
}

// assertAntigravityMCPServers asserts Antigravity's MCP shape: remote
// servers use `serverUrl` (not `url`/`httpUrl`/`type`); stdio servers keep
// `command` + `args` verbatim. The `type` field is dropped across the board.
// https://antigravity.google/docs/mcp
func assertAntigravityMCPServers(t *testing.T, agy map[string]any) {
	t.Helper()
	servers, _ := agy["mcpServers"].(map[string]any)
	if servers == nil {
		t.Fatalf("antigravity mcp_config missing mcpServers: %v", agy)
	}
	for name, raw := range servers {
		server := raw.(map[string]any)
		if _, ok := server["type"]; ok {
			t.Fatalf("antigravity mcpServers.%s should NOT have type field: %v", name, server)
		}
		if _, ok := server["httpUrl"]; ok {
			t.Fatalf("antigravity mcpServers.%s should NOT have legacy httpUrl: %v", name, server)
		}
		switch {
		case isHTTPServer(server):
			if _, ok := server["serverUrl"].(string); !ok {
				t.Fatalf("antigravity mcpServers.%s missing serverUrl: %v", name, server)
			}
			if _, ok := server["url"]; ok {
				t.Fatalf("antigravity mcpServers.%s should NOT keep url after serverUrl rewrite: %v", name, server)
			}
		case isStdioServer(server):
			if cmd, _ := server["command"].(string); cmd == "" {
				t.Fatalf("antigravity mcpServers.%s missing command: %v", name, server)
			}
			if _, ok := server["args"].([]any); !ok {
				t.Fatalf("antigravity mcpServers.%s missing args: %v", name, server)
			}
		default:
			t.Fatalf("antigravity mcpServers.%s has unrecognized shape: %v", name, server)
		}
	}
}

// assertClineMCPServers asserts Cline's MCP shape: HTTP servers keep
// `url` (no `type`); stdio servers keep `command` + `args`. The `type`
// field is dropped on every entry and `trust:true` is set on every
// entry so Cline auto-approves MCP tool calls.
func assertClineMCPServers(t *testing.T, cline map[string]any) {
	t.Helper()
	clineServers, _ := cline["mcpServers"].(map[string]any)
	if clineServers == nil {
		t.Fatalf("cline settings missing mcpServers: %v", cline)
	}
	for name, raw := range clineServers {
		server := raw.(map[string]any)
		if _, ok := server["type"]; ok {
			t.Fatalf("cline mcpServers.%s should NOT have type field: %v", name, server)
		}
		if trust, ok := server["trust"].(bool); !ok || !trust {
			t.Fatalf("cline mcpServers.%s should have trust=true (full bypass): %v", name, server)
		}
		switch {
		case isHTTPServer(server):
			if _, ok := server["url"].(string); !ok {
				t.Fatalf("cline mcpServers.%s missing url: %v", name, server)
			}
		case isStdioServer(server):
			if cmd, _ := server["command"].(string); cmd == "" {
				t.Fatalf("cline mcpServers.%s missing command: %v", name, server)
			}
			if _, ok := server["args"].([]any); !ok {
				t.Fatalf("cline mcpServers.%s missing args: %v", name, server)
			}
		default:
			t.Fatalf("cline mcpServers.%s has unrecognized shape: %v", name, server)
		}
	}
}

// TestTransformMCPServersForAdapter directly exercises the per-provider MCP
// server transform so future schema changes are caught with a focused unit
// test, independent of the full init pipeline.
func TestTransformMCPServersForAdapter(t *testing.T) {
	manifest := MCPManifest{MCPServers: map[string]any{
		"http":  map[string]any{"type": "http", "url": "https://example.com/mcp"},
		"sse":   map[string]any{"type": "sse", "url": "https://example.com/sse"},
		"local": map[string]any{"command": "npx", "args": []any{"-y", "my-mcp"}},
	}}

	cases := []struct {
		adapter   string
		urlKey    string
		expectTyp any
	}{
		{adapter: "claude", urlKey: "url", expectTyp: "http"},
		{adapter: "opencode", urlKey: "url", expectTyp: "remote"},
		{adapter: "qwen", urlKey: "httpUrl", expectTyp: nil},
		{adapter: "antigravity", urlKey: "serverUrl", expectTyp: nil},
		{adapter: "cline", urlKey: "url", expectTyp: nil},
		{adapter: "kimi", urlKey: "url", expectTyp: "http"},
		{adapter: "kiro", urlKey: "url", expectTyp: "http"},
	}
	for _, tc := range cases {
		t.Run(tc.adapter, func(t *testing.T) {
			out, err := transformMCPServersForAdapter(tc.adapter, manifest)
			if err != nil {
				t.Fatalf("transform failed: %v", err)
			}
			http, _ := out["http"].(map[string]any)
			if http == nil {
				t.Fatalf("%s transform dropped http server: %v", tc.adapter, out)
			}
			if tc.expectTyp == nil {
				if _, ok := http["type"]; ok {
					t.Fatalf("%s http server should have no type, got %v", tc.adapter, http["type"])
				}
			} else if http["type"] != tc.expectTyp {
				t.Fatalf("%s http server type = %v, want %v", tc.adapter, http["type"], tc.expectTyp)
			}
			if _, ok := http[tc.urlKey].(string); !ok {
				t.Fatalf("%s http server missing %s: %v", tc.adapter, tc.urlKey, http)
			}
			if tc.adapter == "kiro" {
				// Managed servers must be force-enabled so Kiro panel
				// toggles (disabled:true) do not stick after portal sync.
				for _, name := range []string{"http", "sse", "local"} {
					srv, _ := out[name].(map[string]any)
					if srv == nil {
						t.Fatalf("kiro dropped %s: %v", name, out)
					}
					if srv["disabled"] != false {
						t.Fatalf("kiro %s disabled = %v, want false", name, srv["disabled"])
					}
				}
			}
			if tc.adapter == "opencode" {
				if http["enabled"] != true {
					t.Fatalf("opencode remote missing enabled=true: %v", http)
				}
				local, _ := out["local"].(map[string]any)
				if local == nil || local["type"] != "local" || local["enabled"] != true {
					t.Fatalf("opencode local shape wrong: %v", local)
				}
				cmd, ok := local["command"].([]any)
				if !ok || len(cmd) < 2 || cmd[0] != "npx" || cmd[1] != "-y" {
					t.Fatalf("opencode local command argv wrong: %v", local["command"])
				}
				if _, ok := local["args"]; ok {
					t.Fatalf("opencode local must not keep args: %v", local)
				}
			} else {
				// Stdio entry should keep command+args for non-OpenCode adapters.
				local, _ := out["local"].(map[string]any)
				if local == nil || local["command"] != "npx" {
					t.Fatalf("%s lost stdio command: %v", tc.adapter, local)
				}
			}
		})
	}
}

// TestPresetSkillsOverrideProviderTargetSkills asserts that every skill
// shipped in a preset overrides the matching skill already present in a
// provider's native target directory, even on a plain `init` (no --force).
//
// Before the fix, init mode passed Replace=false down to each per-entry
// linkOrCopy, so a stale provider-target skill with the same name was
// "skip existing" and the preset version never won. This test seeds a
// conflicting skill in ~/.claude/skills and ~/.kiro/skills and confirms the
// preset content replaces it.
func TestPresetSkillsOverrideProviderTargetSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	// Seed a conflicting "execution" skill in two provider targets. This is a
	// skill the preset also ships, so the preset must win.
	stale := "STALE PROVIDER SKILL\n"
	conflicts := []string{
		filepath.Join(home, ".claude", "skills", "execution", "SKILL.md"),
		filepath.Join(home, ".kiro", "skills", "execution", "SKILL.md"),
	}
	for _, path := range conflicts {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		CopyMode:   true, // copy so we can read the materialized content directly
		ToolFilter: ParseTools("all"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	presetSkill := readFile(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	for _, path := range conflicts {
		got := readFile(t, path)
		if got == stale {
			t.Fatalf("provider target %s still holds stale skill; preset did not override it", path)
		}
		if got != presetSkill {
			t.Fatalf("provider target %s does not match preset skill:\n got: %q\nwant: %q", path, got, presetSkill)
		}
	}
}

// TestZCodeAdapterMaterializesNativeLayout verifies that selecting
// the zcode adapter links shared AGENTS.md into ~/.zcode/AGENTS.md
// and relies on ~/.agents/skills for skill discovery (no native
// skills mirror). Sibling providers stay untouched.
func TestZCodeAdapterMaterializesNativeLayout(t *testing.T) {
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
		ToolFilter: ParseTools("zcode"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	mustExist(t, filepath.Join(home, ".zcode", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".zcode", "skills", "execution", "SKILL.md"))
	// Selecting only zcode must not touch sibling providers.
	mustNotExist(t, filepath.Join(home, ".claude", "CLAUDE.md"))
	mustNotExist(t, filepath.Join(home, ".kiro", "AGENTS.md"))
}

// TestZCodeAliasResolvesAdapter confirms the zcode-cli alias selects
// the same adapter as the canonical id.
func TestZCodeAliasResolvesAdapter(t *testing.T) {
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
		ToolFilter: ParseTools("zcode-cli"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init via zcode-cli alias failed: %v", err)
	}
	mustExist(t, filepath.Join(home, ".zcode", "AGENTS.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".zcode", "skills", "execution", "SKILL.md"))
}

func TestCleanupManagedLinksRemovesSharedSymlinksOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	agents := filepath.Join(home, ".agents")
	sharedSkill := filepath.Join(agents, "skills", "execution")
	if err := os.MkdirAll(sharedSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedSkill, "SKILL.md"), []byte("# shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stale managed mirrors that should be removed.
	legacyGrok := filepath.Join(home, ".grok", "skills")
	legacyOC := filepath.Join(home, ".config", "opencode", "skill")
	legacyZ := filepath.Join(home, ".zcode", "skills")
	legacyClineData := filepath.Join(home, ".cline", "data", "skills")
	for _, dir := range []string{legacyGrok, legacyOC, legacyZ, legacyClineData} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(sharedSkill, filepath.Join(dir, "execution")); err != nil {
			t.Fatal(err)
		}
	}
	// User-owned real dir must survive cleanup.
	userOwned := filepath.Join(legacyGrok, "my-local-skill")
	if err := os.MkdirAll(userOwned, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userOwned, "SKILL.md"), []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "update",
		AgentsDir:  agents,
		NoRegistry: true,
		ToolFilter: ParseTools("grok,opencode,zcode,cline"),
	}
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	mustNotExist(t, filepath.Join(legacyGrok, "execution"))
	mustNotExist(t, filepath.Join(legacyOC, "execution"))
	mustNotExist(t, filepath.Join(legacyZ, "execution"))
	mustNotExist(t, filepath.Join(legacyClineData, "execution"))
	mustExist(t, filepath.Join(userOwned, "SKILL.md"))
	// Cline now mirrors to ~/.cline/skills
	mustExist(t, filepath.Join(home, ".cline", "skills", "execution", "SKILL.md"))
	// OpenCode/Grok/ZCode must not re-create native skill mirrors.
	mustNotExist(t, filepath.Join(home, ".config", "opencode", "skill", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".grok", "skills", "execution", "SKILL.md"))
	mustNotExist(t, filepath.Join(home, ".zcode", "skills", "execution", "SKILL.md"))
}

// TestReadMCPManifestInitPrefersOverlayOverDisk asserts that when the portal
// has written an MCP enabled overlay, init mode reads it instead of the stale
// materialized ~/.agents/mcp/servers.json so toggles take effect immediately.
func TestReadMCPManifestInitPrefersOverlayOverDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	agentsDir := filepath.Join(home, ".agents")
	if err := os.MkdirAll(filepath.Join(agentsDir, "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Stale materialized file still lists chrome-devtools.
	if err := os.WriteFile(
		filepath.Join(agentsDir, "mcp", "servers.json"),
		[]byte(`{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	// Portal overlay removed chrome-devtools after the user disabled it.
	overlayPath := filepath.Join(t.TempDir(), "servers.json")
	if err := os.WriteFile(
		overlayPath,
		[]byte(`{"mcpServers":{"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	ctx, _ := newTestContextWithOverlay(t, map[string]string{
		MCPEnabledPath: overlayPath,
	})
	ctx.Options.AgentsDir = agentsDir

	got, err := readMCPManifest(ctx)
	if err != nil {
		t.Fatalf("readMCPManifest: %v", err)
	}
	if _, ok := got.MCPServers["chrome-devtools"]; ok {
		t.Fatalf("overlay should win over disk; chrome-devtools still present: %#v", got.MCPServers)
	}
	if _, ok := got.MCPServers["context7"]; !ok {
		t.Fatalf("overlay missing context7: %#v", got.MCPServers)
	}
}

// TestInitOpenCodeMCPReflectsPortalOverlay verifies the full init pipeline
// writes opencode.json MCP entries from the portal overlay, not stale disk.
func TestInitOpenCodeMCPReflectsPortalOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	agentsDir := filepath.Join(home, ".agents")
	if err := os.MkdirAll(filepath.Join(agentsDir, "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(agentsDir, "mcp", "servers.json"),
		[]byte(`{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(t.TempDir(), "servers.json")
	if err := os.WriteFile(
		overlayPath,
		[]byte(`{"mcpServers":{"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "cfg.json")
	cfgBody, err := json.Marshal(map[string]string{MCPEnabledPath: overlayPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		t.Fatal(err)
	}

	manager := Manager{Presets: os.DirFS("../..")}
	if err := manager.Apply(Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("opencode"),
	}, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	opencode := readFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if strings.Contains(opencode, "chrome-devtools") {
		t.Fatalf("opencode should not include portal-disabled chrome-devtools: %s", opencode)
	}
	if !strings.Contains(opencode, "context7") {
		t.Fatalf("opencode should include context7 from overlay: %s", opencode)
	}
}

// TestTransformOpenCodePreservesExplicitEnabledFalse asserts an explicit
// enabled:false in the shared manifest is not forced back to true.
func TestTransformOpenCodePreservesExplicitEnabledFalse(t *testing.T) {
	remote := transformOpenCodeMCPServer(map[string]any{
		"type":    "http",
		"url":     "https://example.com/mcp",
		"enabled": false,
	})
	if remote["enabled"] != false {
		t.Fatalf("remote enabled=false should be preserved: %#v", remote)
	}
	local := transformOpenCodeMCPServer(map[string]any{
		"command": "npx",
		"args":    []any{"pkg"},
		"enabled": false,
	})
	if local["enabled"] != false {
		t.Fatalf("local enabled=false should be preserved: %#v", local)
	}
}
