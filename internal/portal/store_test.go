package portal

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

func newTestStore(t *testing.T) (*Store, fs.FS, string) {
	t.Helper()
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.json")
	overlayDir := filepath.Join(tmp, "portal")

	fsys := fstest.MapFS{
		"presets/skills/commit/SKILL.md": &fstest.MapFile{Data: []byte("# commit\n")},
		"presets/skills/cleanup/SKILL.md": &fstest.MapFile{Data: []byte("# cleanup\n")},
		"presets/mcp/servers.json": &fstest.MapFile{Data: []byte(`{"mcpServers":{"context7":{"type":"http","url":"https://example.com/mcp"}}}`)},
		"presets/registry/skills.json": &fstest.MapFile{Data: []byte(`{"skills":[{"name":"find-skills","source":"vercel-labs/skills","skill":"find-skills"}]}`)},
		"presets/settings/claude.json": &fstest.MapFile{Data: []byte(`{"permissions":{"defaultMode":"bypassPermissions"},"env":{}}`)},
	}

	store := &Store{
		presets:    fsys,
		config:     agentsync.UserConfig{},
		overlayDir: overlayDir,
		configPath: configPath,
	}
	return store, fsys, tmp
}

func TestListSkills(t *testing.T) {
	store, _, _ := newTestStore(t)
	skills, err := store.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestReadWriteSkill(t *testing.T) {
	store, _, tmp := newTestStore(t)

	skill, err := store.ReadSkill("commit")
	if err != nil {
		t.Fatalf("ReadSkill error: %v", err)
	}
	if skill.Content != "# commit\n" {
		t.Fatalf("unexpected content: %q", skill.Content)
	}
	if skill.Overridden {
		t.Fatal("expected skill not overridden")
	}

	if err := store.WriteSkill("commit", []byte("# updated\n")); err != nil {
		t.Fatalf("WriteSkill error: %v", err)
	}

	skill, err = store.ReadSkill("commit")
	if err != nil {
		t.Fatalf("ReadSkill after write error: %v", err)
	}
	if skill.Content != "# updated\n" {
		t.Fatalf("unexpected content after write: %q", skill.Content)
	}
	if !skill.Overridden {
		t.Fatal("expected skill overridden")
	}

	data, err := os.ReadFile(filepath.Join(tmp, "config.json"))
	if err != nil {
		t.Fatalf("read config error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected config file to be written")
	}

	if err := store.ResetSkill("commit"); err != nil {
		t.Fatalf("ResetSkill error: %v", err)
	}

	skill, err = store.ReadSkill("commit")
	if err != nil {
		t.Fatalf("ReadSkill after reset error: %v", err)
	}
	if skill.Content != "# commit\n" {
		t.Fatalf("unexpected content after reset: %q", skill.Content)
	}
	if skill.Overridden {
		t.Fatal("expected skill not overridden after reset")
	}
}

func TestReadWriteMCPs(t *testing.T) {
	store, _, _ := newTestStore(t)

	manifest, err := store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs error: %v", err)
	}
	if len(manifest.Servers()) != 1 {
		t.Fatalf("expected 1 server, got %d", len(manifest.Servers()))
	}
	if manifest.Overridden {
		t.Fatal("expected MCP manifest not overridden initially")
	}

	preset, err := store.ReadMCPPreset()
	if err != nil {
		t.Fatalf("ReadMCPPreset error: %v", err)
	}
	if len(preset.MCPServers) != 1 {
		t.Fatalf("expected 1 preset server, got %d", len(preset.MCPServers))
	}

	manifest.Servers()["new"] = map[string]any{"type": "stdio", "command": "echo"}
	if err := store.WriteMCPs(&manifest.MCPServers); err != nil {
		t.Fatalf("WriteMCPs error: %v", err)
	}

	manifest, err = store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs after write error: %v", err)
	}
	if len(manifest.Servers()) != 2 {
		t.Fatalf("expected 2 servers after write, got %d", len(manifest.Servers()))
	}
	if !manifest.Overridden {
		t.Fatal("expected MCP manifest overridden after write")
	}

	if err := store.ResetMCPs(); err != nil {
		t.Fatalf("ResetMCPs error: %v", err)
	}
	manifest, err = store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs after reset error: %v", err)
	}
	if len(manifest.Servers()) != 1 {
		t.Fatalf("expected 1 server after reset, got %d", len(manifest.Servers()))
	}
	if manifest.Overridden {
		t.Fatal("expected MCP manifest not overridden after reset")
	}
}

func TestReadWriteClaudeSettings(t *testing.T) {
	store, _, _ := newTestStore(t)

	settings, err := store.ReadClaudeSettings()
	if err != nil {
		t.Fatalf("ReadClaudeSettings error: %v", err)
	}
	if settings.Overridden {
		t.Fatal("expected Claude settings not overridden initially")
	}
	if settings.Permissions["defaultMode"] != "bypassPermissions" {
		t.Fatalf("unexpected permissions: %+v", settings.Permissions)
	}

	settings.Env.AnthropicBaseURL = "https://router.example.com/anthropic"
	settings.Env.AnthropicModel = "anthropic/claude-opus-4"
	if err := store.WriteClaudeSettings(settings); err != nil {
		t.Fatalf("WriteClaudeSettings error: %v", err)
	}

	settings, err = store.ReadClaudeSettings()
	if err != nil {
		t.Fatalf("ReadClaudeSettings after write error: %v", err)
	}
	if !settings.Overridden {
		t.Fatal("expected Claude settings overridden after write")
	}
	if settings.Env.AnthropicBaseURL != "https://router.example.com/anthropic" {
		t.Fatalf("unexpected base url: %q", settings.Env.AnthropicBaseURL)
	}

	preset, err := store.ReadClaudeSettingsPreset()
	if err != nil {
		t.Fatalf("ReadClaudeSettingsPreset error: %v", err)
	}
	if preset.Env.AnthropicBaseURL != "" {
		t.Fatal("preset should not contain overlay env values")
	}

	if err := store.ResetClaudeSettings(); err != nil {
		t.Fatalf("ResetClaudeSettings error: %v", err)
	}
	settings, err = store.ReadClaudeSettings()
	if err != nil {
		t.Fatalf("ReadClaudeSettings after reset error: %v", err)
	}
	if settings.Overridden {
		t.Fatal("expected Claude settings not overridden after reset")
	}
	if settings.Env.AnthropicBaseURL != "" {
		t.Fatal("reset should remove overlay env values")
	}
}

func TestReadWriteRegistry(t *testing.T) {
	store, _, _ := newTestStore(t)

	reg, err := store.ReadRegistry()
	if err != nil {
		t.Fatalf("ReadRegistry error: %v", err)
	}
	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 registry skill, got %d", len(reg.Skills))
	}

	reg.Skills = append(reg.Skills, RegistrySkill{Name: "new", Source: "org/repo", Skill: "new"})
	if err := store.WriteRegistry(reg); err != nil {
		t.Fatalf("WriteRegistry error: %v", err)
	}

	reg, err = store.ReadRegistry()
	if err != nil {
		t.Fatalf("ReadRegistry after write error: %v", err)
	}
	if len(reg.Skills) != 2 {
		t.Fatalf("expected 2 registry skills after write, got %d", len(reg.Skills))
	}
}
