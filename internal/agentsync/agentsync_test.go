package agentsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	mustExist(t, filepath.Join(home, ".agents", "skills", "commit", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "init", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "spawn-opencode", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".agents", "skills", "_shared", "CONVENTIONS.md"))
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
	if agent["includeMcpJson"] != true {
		t.Fatalf("agent includeMcpJson = %v, want true", agent["includeMcpJson"])
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
		ToolFilter: ParseTools("claude,qwen,gemini"),
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
	if qwen, _ := qwenParsed["general"]; qwen != nil {
		t.Fatalf("qwen settings should NOT have Gemini-style \"general\" key (field leak): %s", qwenSettings)
	}

	geminiSettings := readFile(t, filepath.Join(home, ".gemini", "settings.json"))
	geminiParsed := readJSONFile(t, filepath.Join(home, ".gemini", "settings.json"))
	general, _ := geminiParsed["general"].(map[string]any)
	if general == nil {
		t.Fatalf("gemini settings should have general.defaultApprovalMode=auto_edit: %s", geminiSettings)
	}
	if mode, _ := general["defaultApprovalMode"].(string); mode != "auto_edit" {
		t.Fatalf("gemini general.defaultApprovalMode = %q, want \"auto_edit\": %s", mode, geminiSettings)
	}
	if gemini, _ := geminiParsed["permissions"]; gemini != nil {
		t.Fatalf("gemini settings should NOT have Claude/Qwen-style \"permissions\" key (field leak): %s", geminiSettings)
	}
}

// TestProviderFullBypassConfig asserts every provider preset lands a
// full-bypass / auto-approve configuration in the native config file:
//
//   - Claude Code: `permissions.defaultMode: "bypassPermissions"`.
//   - Qwen Code: `permissions.defaultMode: "yolo"` + `confirmShellCommands:
//     false` + `confirmFileEdits: false`.
//   - Gemini CLI: `general.defaultApprovalMode: "auto_edit"` (Gemini's YOLO
//     mode is CLI-flag only; auto_edit is the closest settings.json option
//     that auto-approves edit tools).
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
		ToolFilter: ParseTools("claude,opencode,qwen,gemini,cline"),
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

	// Gemini CLI
	gemini := readJSONFile(t, filepath.Join(home, ".gemini", "settings.json"))
	general, _ := gemini["general"].(map[string]any)
	if general == nil {
		t.Fatalf("gemini settings missing general: %v", gemini)
	}
	if mode, _ := general["defaultApprovalMode"].(string); mode != "auto_edit" {
		t.Fatalf("gemini general.defaultApprovalMode = %q, want \"auto_edit\": %v", mode, gemini)
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
//   - OpenCode MCP docs (mcp.{name}: type "remote" + url).
//   - Qwen Code MCP docs (mcpServers.{name}: httpUrl for HTTP servers, no type).
//   - Gemini CLI MCP docs (mcpServers.{name}: httpUrl for HTTP servers, no type).
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
		ToolFilter: ParseTools("claude,opencode,qwen,gemini,cline"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Claude Code: mcpServers.{name}.{type,url} for HTTP servers and
	// {command,args} for stdio servers (both shapes pass through verbatim).
	assertClaudeMCPServers(t, readJSONFile(t, filepath.Join(home, ".claude", "settings.json")))

	// OpenCode: mcp.{name}.{type:"remote", url} for HTTP servers and
	// {command,args} for stdio servers (both shapes pass through verbatim).
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
// into OpenCode's config.json keeps the vendor-documented shape: HTTP
// servers keep `type:"remote"` + `url`, stdio servers keep `command` +
// `args`. The shared preset may contain both kinds of entries.
func assertOpenCodeMCPServers(t *testing.T, opencode map[string]any) {
	t.Helper()
	opencodeMCP, _ := opencode["mcp"].(map[string]any)
	if opencodeMCP == nil {
		t.Fatalf("opencode config missing mcp: %v", opencode)
	}
	for name, raw := range opencodeMCP {
		server := raw.(map[string]any)
		switch {
		case isHTTPServer(server):
			if server["type"] != "remote" {
				t.Fatalf("opencode mcp.%s expected type=remote, got %v: %v", name, server["type"], server)
			}
			if _, ok := server["url"].(string); !ok {
				t.Fatalf("opencode mcp.%s missing url: %v", name, server)
			}
		case isStdioServer(server):
			if cmd, _ := server["command"].(string); cmd == "" {
				t.Fatalf("opencode mcp.%s missing command: %v", name, server)
			}
			if _, ok := server["args"].([]any); !ok {
				t.Fatalf("opencode mcp.%s missing args: %v", name, server)
			}
		default:
			t.Fatalf("opencode mcp.%s has unrecognized shape (no url/url+type for HTTP, no command for stdio): %v", name, server)
		}
	}
}

// isHTTPServer reports whether an mcpServers entry is an HTTP server.
// It accepts any HTTP-flavoured transport signal:
//
//   - shared preset: `type:"http"` + `url`
//   - qwen / gemini output: `httpUrl` (no `type`)
//   - cline output: `url` (no `type`)
//   - opencode output: `type:"remote"` + `url`
//
// The presence of a URL field (`url` or `httpUrl`) is the canonical
// signal after the per-adapter transform; we also accept `type` values
// `http`, `sse`, or `remote` for completeness, covering raw preset
// entries and transport-renamed outputs.
func isHTTPServer(server map[string]any) bool {
	if _, hasURL := server["url"]; hasURL {
		return true
	}
	if _, hasHTTPURL := server["httpUrl"]; hasHTTPURL {
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

// TestProviderMCPServerShapeMatchesVendorDocs_QwenGeminiCline extends the
// provider shape assertions to Qwen, Gemini and Cline. It is split from
// TestProviderMCPServerShapeMatchesVendorDocs so the suite can be filtered
// to just those providers when running local debugging.
func TestProviderMCPServerShapeMatchesVendorDocs_QwenGeminiCline(t *testing.T) {
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
		ToolFilter: ParseTools("qwen,gemini,cline"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Qwen Code: mcpServers.{name}.httpUrl for HTTP servers (no type
	// field), {command,args} unchanged for stdio servers.
	assertQwenMCPServers(t, readJSONFile(t, filepath.Join(home, ".qwen", "settings.json")))

	// Gemini CLI: mcpServers.{name}.httpUrl, no type, no hooks key.
	gemini := readJSONFile(t, filepath.Join(home, ".gemini", "settings.json"))
	if _, ok := gemini["hooks"]; ok {
		t.Fatalf("gemini settings should NOT have hooks key (Gemini CLI ignores it): %v", gemini)
	}
	assertGeminiMCPServers(t, gemini)

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

// assertGeminiMCPServers asserts Gemini's MCP shape: HTTP servers use
// `httpUrl` (not `url` + `type`); stdio servers keep `command` + `args`
// verbatim. The `type` field is dropped across the board.
func assertGeminiMCPServers(t *testing.T, gemini map[string]any) {
	t.Helper()
	geminiServers, _ := gemini["mcpServers"].(map[string]any)
	if geminiServers == nil {
		t.Fatalf("gemini settings missing mcpServers: %v", gemini)
	}
	for name, raw := range geminiServers {
		server := raw.(map[string]any)
		if _, ok := server["type"]; ok {
			t.Fatalf("gemini mcpServers.%s should NOT have type field: %v", name, server)
		}
		switch {
		case isHTTPServer(server):
			if _, ok := server["httpUrl"].(string); !ok {
				t.Fatalf("gemini mcpServers.%s missing httpUrl: %v", name, server)
			}
		case isStdioServer(server):
			if cmd, _ := server["command"].(string); cmd == "" {
				t.Fatalf("gemini mcpServers.%s missing command: %v", name, server)
			}
			if _, ok := server["args"].([]any); !ok {
				t.Fatalf("gemini mcpServers.%s missing args: %v", name, server)
			}
		default:
			t.Fatalf("gemini mcpServers.%s has unrecognized shape: %v", name, server)
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
		{adapter: "gemini", urlKey: "httpUrl", expectTyp: nil},
		{adapter: "cline", urlKey: "url", expectTyp: nil},
		{adapter: "kimi", urlKey: "url", expectTyp: "http"},
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
			// Stdio entry should always keep command+args untouched.
			local, _ := out["local"].(map[string]any)
			if local == nil || local["command"] != "npx" {
				t.Fatalf("%s lost stdio command: %v", tc.adapter, local)
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
// the zcode adapter materializes the shared AGENTS.md into
// ~/.zcode/AGENTS.md and mirrors shared skills into
// ~/.zcode/skills/<name>/SKILL.md. There is no first-party
// user-level MCP config in this ZCode release, so no mcp.json
// assertion is made; the test confirms sibling providers are
// untouched to keep filter scoping honest.
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
	mustExist(t, filepath.Join(home, ".zcode", "skills", "cleanup", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".zcode", "skills", "commit", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".zcode", "skills", "execution", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".zcode", "skills", "init", "SKILL.md"))
	mustExist(t, filepath.Join(home, ".zcode", "skills", "_shared", "CONVENTIONS.md"))
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
	mustExist(t, filepath.Join(home, ".zcode", "skills", "execution", "SKILL.md"))
}
