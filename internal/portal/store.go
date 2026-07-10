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
	toggles, err := s.readToggles()
	if err != nil {
		return nil, err
	}
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
			Enabled:    toggles.IsSkillEnabled(id),
		})
	}

	// User-added skills (not in embedded).
	for _, rel := range s.config.EntriesUnder("presets/skills") {
		parts := strings.SplitN(rel, "/", 2)
		id := parts[0]
		if seen[id] || id == "_shared" || strings.HasPrefix(id, ".") {
			continue
		}
		seen[id] = true
		skills = append(skills, Skill{
			ID:         id,
			Name:       id,
			Source:     "overlay",
			Overridden: true,
			Enabled:    toggles.IsSkillEnabled(id),
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
	toggles, err := s.readToggles()
	if err != nil {
		return nil, err
	}
	return &Skill{
		ID:         id,
		Name:       id,
		Source:     source,
		Overridden: source == "overlay",
		Enabled:    toggles.IsSkillEnabled(id),
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

// SetSkillEnabled enables or disables a skill by rewriting portal toggles
// JSONC (disabled skills are // commented in the toggles preset).
func (s *Store) SetSkillEnabled(id string, enabled bool) error {
	return s.setToggle(func(disabledSkills, disabledProviders map[string]bool) {
		if enabled {
			delete(disabledSkills, id)
		} else {
			disabledSkills[id] = true
		}
	})
}

// ReadMCPs returns the shared MCP servers manifest with provenance metadata.
// Disabled servers stay in the file as // comments (never deleted) and appear
// in Items with Enabled=false for the portal list.
func (s *Store) ReadMCPs() (*MCPManifest, error) {
	key := "presets/mcp/servers.json"
	data, err := s.readEffective(key)
	if err != nil {
		return nil, err
	}
	enabled, disabled, order, err := agentsync.ParseMCPServersJSONC(data)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP servers JSONC: %w", err)
	}
	// Normalize file text so portal always shows commented disabled entries
	// even when the embedded source was pure JSON (no comments yet).
	content, err := agentsync.FormatMCPServersJSONC(enabled, disabled, order)
	if err != nil {
		content = data
	}
	items := buildMCPItems(enabled, disabled)
	return &MCPManifest{
		MCPServers:      MCPServers{MCPServers: enabled},
		DisabledServers: disabled,
		Items:           items,
		Content:         string(content),
		Overridden:      s.isOverridden(key),
		Source:          sourceLabel(s.isOverridden(key)),
	}, nil
}

// ReadMCPPreset returns the embedded MCP servers preset (enabled only).
func (s *Store) ReadMCPPreset() (*MCPServers, error) {
	key := "presets/mcp/servers.json"
	data, err := s.readEmbedded(key)
	if err != nil {
		return nil, err
	}
	enabled, _, _, err := agentsync.ParseMCPServersJSONC(data)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP preset JSON: %w", err)
	}
	return &MCPServers{MCPServers: enabled}, nil
}

// WriteMCPs updates the shared MCP servers manifest via overlay.
// Existing disabled (commented) servers are preserved unless the caller
// re-introduces them as live keys (then they become enabled). Entries are
// never hard-deleted from the overlay by this path — only commented out.
func (s *Store) WriteMCPs(servers *MCPServers) error {
	key := "presets/mcp/servers.json"
	var disabled map[string]any
	var order []string
	if prev, err := s.readEffective(key); err == nil {
		prevEnabled, prevDisabled, prevOrder, _ := agentsync.ParseMCPServersJSONC(prev)
		disabled = prevDisabled
		order = prevOrder
		// Anything that was previously active but missing from the new
		// payload is treated as disable (comment-out), not delete.
		if disabled == nil {
			disabled = map[string]any{}
		}
		for name, cfg := range prevEnabled {
			if _, keep := servers.MCPServers[name]; !keep {
				if _, already := disabled[name]; !already {
					disabled[name] = cfg
				}
			}
		}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	enabled := servers.MCPServers
	if enabled == nil {
		enabled = map[string]any{}
	}
	// Keys present in the write payload leave the disabled set (re-enabled).
	for name := range enabled {
		delete(disabled, name)
	}
	data, err := agentsync.FormatMCPServersJSONC(enabled, disabled, order)
	if err != nil {
		return err
	}
	return s.writeOverlay(key, data)
}

// WriteMCPsContent writes a full JSONC body for mcp/servers.json (supports
// // comments for disabled servers). Prefer this when the portal file editor
// shows the raw overlay text.
//
// Safety: if the previous overlay had disabled servers that are missing from
// both enabled and disabled sets in the new parse (parser loss / bad edit),
// those previous disabled entries are merged back so disable never silently
// hard-deletes config.
func (s *Store) WriteMCPsContent(content []byte) error {
	enabled, disabled, order, err := agentsync.ParseMCPServersJSONC(content)
	if err != nil {
		return fmt.Errorf("invalid MCP servers JSONC: %w", err)
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	// Merge any previously disabled entries that vanished from the payload.
	if prev, err := s.readEffective("presets/mcp/servers.json"); err == nil {
		_, prevDisabled, _, _ := agentsync.ParseMCPServersJSONC(prev)
		for name, cfg := range prevDisabled {
			if _, inEn := enabled[name]; inEn {
				continue
			}
			if _, inDis := disabled[name]; inDis {
				continue
			}
			disabled[name] = cfg
		}
	}
	// Re-format so disabled always render as // comments consistently.
	data, err := agentsync.FormatMCPServersJSONC(enabled, disabled, order)
	if err != nil {
		return err
	}
	return s.writeOverlay("presets/mcp/servers.json", data)
}

// SetMCPEnabled enables or disables one MCP server in the JSONC overlay.
// Disable comments the entry out (keeps config in file); enable uncomments it.
// Entries are never removed from the overlay by this method.
func (s *Store) SetMCPEnabled(name string, enabled bool) error {
	key := "presets/mcp/servers.json"
	data, err := s.readEffective(key)
	if err != nil {
		return err
	}
	active, disabled, order, err := agentsync.ParseMCPServersJSONC(data)
	if err != nil {
		return err
	}
	if active == nil {
		active = map[string]any{}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	if enabled {
		cfg, ok := disabled[name]
		if !ok {
			if _, already := active[name]; already {
				return nil
			}
			return fmt.Errorf("mcp server %q not found among disabled entries", name)
		}
		active[name] = cfg
		delete(disabled, name)
	} else {
		cfg, ok := active[name]
		if !ok {
			if _, already := disabled[name]; already {
				return nil // already commented out — not deleted
			}
			return fmt.Errorf("mcp server %q not found", name)
		}
		disabled[name] = cfg
		delete(active, name)
	}
	out, err := agentsync.FormatMCPServersJSONC(active, disabled, order)
	if err != nil {
		return err
	}
	return s.writeOverlay(key, out)
}

// ResetMCPs removes the user overlay for the MCP servers manifest.
func (s *Store) ResetMCPs() error {
	return s.removeOverlay("presets/mcp/servers.json")
}

// SetProviderEnabled enables or disables a provider adapter via toggles JSONC.
func (s *Store) SetProviderEnabled(id string, enabled bool) error {
	id = strings.ToLower(id)
	return s.setToggle(func(disabledSkills, disabledProviders map[string]bool) {
		if enabled {
			delete(disabledProviders, id)
		} else {
			disabledProviders[id] = true
		}
	})
}

// ProviderEnabled reports whether an adapter is currently enabled.
func (s *Store) ProviderEnabled(id string) bool {
	t, err := s.readToggles()
	if err != nil {
		return true
	}
	return t.IsProviderEnabled(id)
}

func sourceLabel(overridden bool) string {
	if overridden {
		return "overlay"
	}
	return "embedded"
}

// ReadRegistry returns the registry skills manifest.
func (s *Store) ReadRegistry() (*RegistrySkills, error) {
	key := "presets/registry/skills.json"
	data, err := s.readEffective(key)
	if err != nil {
		return nil, err
	}
	var reg RegistrySkills
	if err := agentsync.UnmarshalJSONC(data, &reg); err != nil {
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

func buildMCPItems(enabled, disabled map[string]any) []MCPServerItem {
	items := make([]MCPServerItem, 0, len(enabled)+len(disabled))
	names := make([]string, 0, len(enabled)+len(disabled))
	for n := range enabled {
		names = append(names, n)
	}
	for n := range disabled {
		if _, ok := enabled[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if cfg, ok := enabled[n]; ok {
			items = append(items, MCPServerItem{Name: n, Enabled: true, Config: cfg})
			continue
		}
		items = append(items, MCPServerItem{Name: n, Enabled: false, Config: disabled[n]})
	}
	return items
}

func (s *Store) readToggles() (agentsync.PortalToggles, error) {
	data, err := s.readEffective(agentsync.PortalTogglesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			return agentsync.PortalToggles{}, nil
		}
		// Missing optional file from embedded FS.
		if strings.Contains(err.Error(), "file does not exist") ||
			strings.Contains(err.Error(), "no such file") {
			return agentsync.PortalToggles{}, nil
		}
		return agentsync.PortalToggles{}, err
	}
	return agentsync.ParsePortalToggles(data)
}

// setToggle mutates disabled skill/provider maps and rewrites the toggles overlay.
func (s *Store) setToggle(mutate func(disabledSkills, disabledProviders map[string]bool)) error {
	t, err := s.readToggles()
	if err != nil {
		return err
	}
	disabledSkills := map[string]bool{}
	for k, v := range t.DisabledSkills {
		if v {
			disabledSkills[k] = true
		}
	}
	disabledProviders := map[string]bool{}
	for k, v := range t.DisabledProviders {
		if v {
			disabledProviders[k] = true
		}
	}
	mutate(disabledSkills, disabledProviders)

	knownSkills, err := s.knownSkillIDs()
	if err != nil {
		return err
	}
	knownProviders := s.knownProviderIDs()
	data, err := agentsync.FormatPortalToggles(knownSkills, knownProviders, disabledSkills, disabledProviders)
	if err != nil {
		return err
	}
	return s.writeOverlay(agentsync.PortalTogglesPath, data)
}

func (s *Store) knownSkillIDs() ([]string, error) {
	skills, err := s.ListSkillsWithoutToggle()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(skills))
	for _, sk := range skills {
		ids = append(ids, sk.ID)
	}
	return ids, nil
}

// ListSkillsWithoutToggle enumerates skill ids without loading toggles
// (avoids recursion from setToggle → knownSkillIDs → ListSkills).
func (s *Store) ListSkillsWithoutToggle() ([]Skill, error) {
	seen := map[string]bool{}
	var skills []Skill
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
		skills = append(skills, Skill{ID: id, Name: id})
	}
	for _, rel := range s.config.EntriesUnder("presets/skills") {
		parts := strings.SplitN(rel, "/", 2)
		id := parts[0]
		if seen[id] || id == "_shared" || strings.HasPrefix(id, ".") {
			continue
		}
		seen[id] = true
		skills = append(skills, Skill{ID: id, Name: id})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, nil
}

func (s *Store) knownProviderIDs() []string {
	// Keep in sync with agentsync adapter registry ids.
	return []string{
		"claude", "opencode", "grok", "kimi", "kiro", "qwen", "gemini", "codex", "cline", "zcode",
	}
}
