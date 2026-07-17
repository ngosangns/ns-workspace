package portal

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

func TestWriteUnifiedMCPContent(t *testing.T) {
	store, _, _ := newTestStore(t)

	raw := `{
  "mcpServers": {
    "alpha": {"command": "a"},
    "beta": {"command": "b"}
  },
  "disabled": ["beta"]
}`
	if err := store.WriteMCPsContent([]byte(raw)); err != nil {
		t.Fatalf("WriteMCPsContent: %v", err)
	}
	manifest, err := store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs: %v", err)
	}
	if _, ok := manifest.Servers()["alpha"]; !ok {
		t.Fatal("alpha should be enabled")
	}
	if _, ok := manifest.Servers()["beta"]; ok {
		t.Fatal("beta should not be in enabled map")
	}
	if _, ok := manifest.DisabledServers["beta"]; !ok {
		t.Fatal("beta should be disabled")
	}
	if !strings.Contains(manifest.Content, `"disabled"`) || !strings.Contains(manifest.Content, "beta") {
		t.Fatalf("content should be unified catalog:\n%s", manifest.Content)
	}
	// Full replace: omitting alpha deletes it.
	if err := store.WriteMCPsContent([]byte(`{"mcpServers":{"beta":{"command":"b2"}},"disabled":["beta"]}`)); err != nil {
		t.Fatalf("replace content: %v", err)
	}
	manifest, err = store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs after replace: %v", err)
	}
	if _, ok := manifest.Servers()["alpha"]; ok {
		t.Fatal("alpha should be hard-deleted on full replace")
	}
	if _, ok := manifest.DisabledServers["beta"]; !ok {
		t.Fatal("beta should remain disabled")
	}
}

func TestDeleteMCP(t *testing.T) {
	store, _, _ := newTestStore(t)

	raw := `{
  "mcpServers": {
    "alpha": {"command": "a"},
    "beta": {"command": "b"}
  },
  "disabled": ["beta"]
}`
	if err := store.WriteMCPsContent([]byte(raw)); err != nil {
		t.Fatalf("WriteMCPsContent: %v", err)
	}
	if err := store.DeleteMCP("beta"); err != nil {
		t.Fatalf("DeleteMCP beta: %v", err)
	}
	manifest, err := store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs: %v", err)
	}
	if _, ok := manifest.DisabledServers["beta"]; ok {
		t.Fatal("beta should be removed from disabled")
	}
	if _, ok := manifest.Servers()["alpha"]; !ok {
		t.Fatal("alpha should remain")
	}
	if err := store.DeleteMCP("alpha"); err != nil {
		t.Fatalf("DeleteMCP alpha: %v", err)
	}
	manifest, err = store.ReadMCPs()
	if err != nil {
		t.Fatalf("ReadMCPs after alpha delete: %v", err)
	}
	if _, ok := manifest.Servers()["alpha"]; ok {
		t.Fatal("alpha should be gone")
	}
	if err := store.DeleteMCP("missing"); err == nil {
		t.Fatal("expected error for missing server")
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

	reg.Skills = append(reg.Skills, RegistrySkill{Name: "demo-skill", Source: "acme/demo-skills", Skill: "demo-skill"})
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

	// Placeholders must never be accepted.
	if err := store.WriteRegistry(&RegistrySkills{Skills: []RegistrySkill{
		{Name: "new", Source: "org/repo", Skill: "new"},
	}}); err == nil {
		t.Fatal("WriteRegistry must reject org/repo placeholder")
	}
	if err := store.UpsertRegistrySkill(RegistrySkill{Name: "x", Source: "owner/repo", Skill: "x"}); err == nil {
		t.Fatal("UpsertRegistrySkill must reject owner/repo placeholder")
	}
}
