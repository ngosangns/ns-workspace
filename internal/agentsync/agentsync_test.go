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

func TestRegistryManifestIncludesMiniMaxCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	manager := Manager{Presets: os.DirFS("../..")}
	ctx, err := manager.context(Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("stable"),
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := readRegistryManifest(ctx)
	if err != nil {
		t.Fatalf("readRegistryManifest: %v", err)
	}

	var found bool
	for _, s := range manifest.Skills {
		if s.Name == "minimax-cli" {
			found = true
			if s.Source != "MiniMax-AI/cli" {
				t.Fatalf("minimax-cli source = %q, want MiniMax-AI/cli", s.Source)
			}
			if s.Skill != "mmx-cli" {
				t.Fatalf("minimax-cli skill = %q, want mmx-cli", s.Skill)
			}
		}
	}
	if !found {
		t.Fatalf("minimax-cli not in registry manifest: %+v", manifest.Skills)
	}

	// writeRegistryHelpers materializes the install.sh — read it back and
	// confirm the new entry made it through the same code path that runs in
	// real `init` (with `--no-registry` set so npx never actually fires).
	if err := writeRegistryHelpers(ctx, true); err != nil {
		t.Fatalf("writeRegistryHelpers: %v", err)
	}
	script := readFile(t, filepath.Join(ctx.Options.AgentsDir, "registry", "install.sh"))
	for _, part := range []string{
		"npx --yes skills add MiniMax-AI/cli",
		"--skill mmx-cli",
		"--global",
		"--agent universal",
	} {
		if !strings.Contains(script, part) {
			t.Fatalf("install script missing %q. Got:\n%s", part, script)
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
	if !strings.Contains(claudeSettings, "permissions") {
		t.Fatalf("claude settings should have permissions field: %s", claudeSettings)
	}

	qwenSettings := readFile(t, filepath.Join(home, ".qwen", "settings.json"))
	if strings.Contains(qwenSettings, "permissions") {
		t.Fatalf("qwen settings should NOT have permissions field (field leak): %s", qwenSettings)
	}

	geminiSettings := readFile(t, filepath.Join(home, ".gemini", "settings.json"))
	if strings.Contains(geminiSettings, "permissions") {
		t.Fatalf("gemini settings should NOT have permissions field (field leak): %s", geminiSettings)
	}
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

