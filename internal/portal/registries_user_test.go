package portal

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestListConfiguredRegistriesUsesDisabledSources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	t.Setenv("AGENTS_HOME", "")

	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":      &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/portal/disabled.json":      &fstest.MapFile{Data: []byte(`{}`)},
	}
	store, err := NewStore(presets, agentsyncOptions(filepath.Join(home, ".agents")))
	if err != nil {
		t.Fatal(err)
	}
	// Write only valid sources (org/repo placeholders are rejected by WriteRegistry).
	if err := store.WriteRegistry(&RegistrySkills{Skills: []RegistrySkill{
		{Name: "git-commit", Source: "github/awesome-copilot", Skill: "git-commit"},
		{Name: "find-skills", Source: "vercel-labs/skills", Skill: "find-skills"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRegistrySkillEnabled("git-commit", false); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRegistrySkillEnabled("find-skills", false); err != nil {
		t.Fatal(err)
	}

	// Skip GitHub skillCount (would need network); listable is set before list call
	// Mock github to avoid network: set invalid base so skillCount falls back but listable stays true
	prev := githubAPIBase
	githubAPIBase = "http://127.0.0.1:1"
	t.Cleanup(func() { githubAPIBase = prev })

	ps := &portalServer{store: store, agentsDir: filepath.Join(home, ".agents")}
	regs := ps.listConfiguredRegistries()
	var listable int
	for _, r := range regs {
		t.Logf("%+v", r)
		if r.Listable {
			listable++
		}
	}
	if listable < 2 {
		t.Fatalf("expected listable GitHub registries from disabled entries, got %d: %+v", listable, regs)
	}
	for _, r := range regs {
		if r.Source == "org/repo" {
			t.Fatal("org/repo placeholder must never appear in registry list")
		}
	}
}
