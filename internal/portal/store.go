package portal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

// Store reads and writes effective presets by combining the embedded preset
// FS with a user-config overlay persisted under the user's config directory.
type Store struct {
	presets    fs.FS
	config     agentsync.UserConfig
	overlayDir string
	configPath string
}

// NewStore loads the user overlay config and prepares the overlay directory.
func NewStore(presets fs.FS, opt agentsync.Options) (*Store, error) {
	manager := agentsync.Manager{Presets: presets}
	cfg, err := manager.LoadUserConfig(opt)
	if err != nil {
		return nil, err
	}
	configPath := opt.ConfigPath
	if configPath == "" {
		configPath, err = agentsync.DefaultUserConfigPath()
		if err != nil {
			return nil, err
		}
	}
	overlayDir := filepath.Join(filepath.Dir(configPath), "portal")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		return nil, fmt.Errorf("create portal overlay dir: %w", err)
	}
	return &Store{
		presets:    presets,
		config:     cfg,
		overlayDir: overlayDir,
		configPath: configPath,
	}, nil
}

// effectivePath returns the absolute path used to persist an overlay for the
// given preset key, e.g. "presets/skills/commit/SKILL.md".
func (s *Store) effectivePath(presetKey string) string {
	key := agentsync.NormalizePresetKey(presetKey)
	return filepath.Join(s.overlayDir, filepath.FromSlash(key))
}

// readEmbedded reads a file from the embedded preset FS.
func (s *Store) readEmbedded(presetKey string) ([]byte, error) {
	key := agentsync.NormalizePresetKey(presetKey)
	return fs.ReadFile(s.presets, key)
}

// readEffective returns the effective content for a preset path, preferring
// the user overlay and falling back to the embedded preset FS.
func (s *Store) readEffective(presetKey string) ([]byte, error) {
	if user, ok := s.config.Lookup(presetKey); ok {
		return os.ReadFile(user)
	}
	return s.readEmbedded(presetKey)
}

// isOverridden reports whether the preset path has a user overlay.
func (s *Store) isOverridden(presetKey string) bool {
	_, ok := s.config.Lookup(presetKey)
	return ok
}

// writeOverlay persists content as a user overlay and updates the overlay map.
func (s *Store) writeOverlay(presetKey string, content []byte) error {
	key := agentsync.NormalizePresetKey(presetKey)
	effPath := s.effectivePath(key)
	if err := os.MkdirAll(filepath.Dir(effPath), 0o755); err != nil {
		return fmt.Errorf("create overlay parent dir: %w", err)
	}
	if err := os.WriteFile(effPath, content, 0o644); err != nil {
		return fmt.Errorf("write overlay file: %w", err)
	}
	return s.updateConfigEntry(key, effPath)
}

// removeOverlay deletes a user overlay entry and file.
func (s *Store) removeOverlay(presetKey string) error {
	key := agentsync.NormalizePresetKey(presetKey)
	effPath := s.effectivePath(key)
	_ = os.Remove(effPath)
	return s.updateConfigEntry(key, "")
}

// updateConfigEntry adds or removes a key from the user config JSON.
func (s *Store) updateConfigEntry(key, value string) error {
	raw := map[string]string{}
	if data, err := os.ReadFile(s.configPath); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse user config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read user config: %w", err)
	}
	if value == "" {
		delete(raw, key)
	} else {
		raw[key] = value
	}
	if len(raw) == 0 {
		_ = os.Remove(s.configPath)
		s.config = agentsync.UserConfig{}
		return nil
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(s.configPath, data, 0o644); err != nil {
		return fmt.Errorf("write user config: %w", err)
	}
	// Reload config so subsequent reads reflect the change.
	manager := agentsync.Manager{Presets: s.presets}
	opt := agentsync.Options{ConfigPath: s.configPath}
	cfg, err := manager.LoadUserConfig(opt)
	if err != nil {
		return fmt.Errorf("reload user config: %w", err)
	}
	s.config = cfg
	return nil
}

// ListSkills returns all skills under presets/skills.
func (s *Store) ListSkills() ([]Skill, error) {
	seen := map[string]bool{}
	var skills []Skill

	// Embedded skills.
	entries, err := fs.ReadDir(s.presets, "presets/skills")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if id == "_shared" || strings.HasPrefix(id, ".") {
			continue
		}
		seen[id] = true
		skills = append(skills, Skill{
			ID:         id,
			Name:       id,
			Source:     "embedded",
			Overridden: s.isOverridden(skillPath(id)),
		})
	}

	// User-added skills (not in embedded).
	for _, rel := range s.config.EntriesUnder("presets/skills") {
		parts := strings.SplitN(rel, "/", 2)
		id := parts[0]
		if seen[id] {
			continue
		}
		seen[id] = true
		skills = append(skills, Skill{
			ID:         id,
			Name:       id,
			Source:     "overlay",
			Overridden: true,
		})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, nil
}

// ReadSkill returns a skill with its content.
func (s *Store) ReadSkill(id string) (*Skill, error) {
	key := skillPath(id)
	content, err := s.readEffective(key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("skill %q not found", id)
		}
		return nil, err
	}
	source := "embedded"
	if s.isOverridden(key) {
		source = "overlay"
	}
	return &Skill{
		ID:         id,
		Name:       id,
		Source:     source,
		Overridden: source == "overlay",
		Content:    string(content),
	}, nil
}

// WriteSkill updates a skill by writing an overlay.
func (s *Store) WriteSkill(id string, content []byte) error {
	return s.writeOverlay(skillPath(id), content)
}

// ResetSkill removes the user overlay for a skill.
func (s *Store) ResetSkill(id string) error {
	return s.removeOverlay(skillPath(id))
}

// ReadMCPs returns the shared MCP servers manifest with provenance metadata.
func (s *Store) ReadMCPs() (*MCPManifest, error) {
	key := "presets/mcp/servers.json"
	data, err := s.readEffective(key)
	if err != nil {
		return nil, err
	}
	var servers MCPServers
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("invalid MCP servers JSON: %w", err)
	}
	return &MCPManifest{
		MCPServers: servers,
		Overridden: s.isOverridden(key),
		Source:     sourceLabel(s.isOverridden(key)),
	}, nil
}

// ReadMCPPreset returns the embedded MCP servers preset.
func (s *Store) ReadMCPPreset() (*MCPServers, error) {
	key := "presets/mcp/servers.json"
	data, err := s.readEmbedded(key)
	if err != nil {
		return nil, err
	}
	var servers MCPServers
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("invalid MCP preset JSON: %w", err)
	}
	return &servers, nil
}

// WriteMCPs updates the shared MCP servers manifest via overlay.
func (s *Store) WriteMCPs(servers *MCPServers) error {
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.writeOverlay("presets/mcp/servers.json", data)
}

// ResetMCPs removes the user overlay for the MCP servers manifest.
func (s *Store) ResetMCPs() error {
	return s.removeOverlay("presets/mcp/servers.json")
}

func sourceLabel(overridden bool) string {
	if overridden {
		return "overlay"
	}
	return "embedded"
}

// ReadClaudeSettings returns the effective Claude Code settings with provenance metadata.
func (s *Store) ReadClaudeSettings() (*ClaudeSettings, error) {
	key := "presets/settings/claude.json"
	data, err := s.readEffective(key)
	if err != nil {
		return nil, err
	}
	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("invalid Claude settings JSON: %w", err)
	}
	settings.Overridden = s.isOverridden(key)
	settings.Source = sourceLabel(settings.Overridden)
	return &settings, nil
}

// ReadClaudeSettingsPreset returns the embedded Claude Code settings preset.
func (s *Store) ReadClaudeSettingsPreset() (*ClaudeSettings, error) {
	key := "presets/settings/claude.json"
	data, err := s.readEmbedded(key)
	if err != nil {
		return nil, err
	}
	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("invalid Claude preset JSON: %w", err)
	}
	settings.Source = "embedded"
	settings.Overridden = false
	return &settings, nil
}

// WriteClaudeSettings updates the Claude Code settings via overlay.
func (s *Store) WriteClaudeSettings(settings *ClaudeSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.writeOverlay("presets/settings/claude.json", data)
}

// ResetClaudeSettings removes the user overlay for Claude Code settings.
func (s *Store) ResetClaudeSettings() error {
	return s.removeOverlay("presets/settings/claude.json")
}

// ReadRegistry returns the registry skills manifest.
func (s *Store) ReadRegistry() (*RegistrySkills, error) {
	key := "presets/registry/skills.json"
	data, err := s.readEffective(key)
	if err != nil {
		return nil, err
	}
	var reg RegistrySkills
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("invalid registry skills JSON: %w", err)
	}
	return &reg, nil
}

// WriteRegistry updates the registry skills manifest via overlay.
func (s *Store) WriteRegistry(reg *RegistrySkills) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.writeOverlay("presets/registry/skills.json", data)
}

// UserOverlay returns the current overlay entries.
func (s *Store) UserOverlay() *UserOverlay {
	origin := s.config.Origin()
	if origin == "" {
		origin = s.configPath
	}
	return &UserOverlay{
		Origin:  origin,
		Entries: s.config.Entries(),
	}
}

func skillPath(id string) string {
	return fmt.Sprintf("presets/skills/%s/SKILL.md", id)
}
