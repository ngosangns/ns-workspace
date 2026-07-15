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
	"gopkg.in/yaml.v3"
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
		name, desc := s.skillMeta(id)
		skills = append(skills, Skill{
			ID:          id,
			Name:        name,
			Description: desc,
			Source:      "embedded",
			Overridden:  s.isOverridden(skillPath(id)),
			Enabled:     toggles.IsSkillEnabled(id),
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
		name, desc := s.skillMeta(id)
		skills = append(skills, Skill{
			ID:          id,
			Name:        name,
			Description: desc,
			Source:      "overlay",
			Overridden:  true,
			Enabled:     toggles.IsSkillEnabled(id),
		})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, nil
}

// skillMeta reads name/description from SKILL.md frontmatter when present.
func (s *Store) skillMeta(id string) (name, description string) {
	name = id
	data, err := s.readEffective(skillPath(id))
	if err != nil {
		return name, ""
	}
	n, d := parseSkillFrontmatter(data)
	if n != "" {
		name = n
	}
	return name, d
}

// parseSkillFrontmatter extracts name and description from YAML frontmatter.
func parseSkillFrontmatter(content []byte) (name, description string) {
	text := string(content)
	if !strings.HasPrefix(strings.TrimSpace(text), "---") {
		return "", ""
	}
	// Find opening --- (allow leading whitespace/BOM-less).
	start := strings.Index(text, "---")
	if start < 0 {
		return "", ""
	}
	rest := text[start+3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", ""
	}
	block := strings.TrimSpace(rest[:end])
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		return "", ""
	}
	return strings.TrimSpace(meta.Name), strings.TrimSpace(meta.Description)
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
	name, desc := parseSkillFrontmatter(content)
	if name == "" {
		name = id
	}
	return &Skill{
		ID:          id,
		Name:        name,
		Description: desc,
		Source:      source,
		Overridden:  source == "overlay",
		Enabled:     toggles.IsSkillEnabled(id),
		Content:     string(content),
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

// SetSkillEnabled enables or disables a skill by rewriting
// presets/portal/disabled.json (skill id list).
func (s *Store) SetSkillEnabled(id string, enabled bool) error {
	return s.setDisabled(func(disabledSkills, disabledProviders map[string]bool) {
		if enabled {
			delete(disabledSkills, id)
		} else {
			disabledSkills[id] = true
		}
	})
}

// ReadMCPs returns the shared MCP servers manifest with provenance metadata.
// Portal exposes one editable Content document (all servers + disabled list).
// On disk, enabled/disabled may still be split for agentsync materialize.
func (s *Store) ReadMCPs() (*MCPManifest, error) {
	enabled, disabled, order, err := s.loadMCPSplit()
	if err != nil {
		return nil, err
	}
	content, err := formatUnifiedMCPContent(enabled, disabled, order)
	if err != nil {
		return nil, err
	}
	items := buildMCPItems(enabled, disabled)
	overridden := s.isOverridden(agentsync.MCPEnabledPath) || s.isOverridden(agentsync.MCPDisabledPath)
	return &MCPManifest{
		MCPServers:      MCPServers{MCPServers: enabled},
		DisabledServers: disabled,
		Items:           items,
		Content:         string(content),
		Overridden:      overridden,
		Source:          sourceLabel(overridden),
	}, nil
}

// ReadMCPPreset returns the embedded MCP servers preset (enabled only).
func (s *Store) ReadMCPPreset() (*MCPServers, error) {
	key := agentsync.MCPEnabledPath
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

// WriteMCPs replaces the full MCP catalog: every entry in servers is enabled.
// Omitted names are removed (not moved to disabled). Prefer WriteMCPsContent
// or WriteMCPCatalog when the caller also needs disabled entries.
func (s *Store) WriteMCPs(servers *MCPServers) error {
	next := map[string]any{}
	order := make([]string, 0)
	if servers != nil && servers.MCPServers != nil {
		next = servers.MCPServers
		order = sortedMapKeys(next)
	}
	return s.writeMCPSplit(next, map[string]any{}, order)
}

// WriteMCPCatalog replaces the full catalog (enabled + disabled maps).
// Names may only appear in one map; enabled wins if duplicated.
func (s *Store) WriteMCPCatalog(enabled, disabled map[string]any, order []string) error {
	if enabled == nil {
		enabled = map[string]any{}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	for name := range enabled {
		delete(disabled, name)
	}
	if len(order) == 0 {
		order = append(sortedMapKeys(enabled), sortedMapKeys(disabled)...)
	}
	return s.writeMCPSplit(enabled, disabled, order)
}

// WriteMCPsContent writes the single portal MCP source document.
// Accepted shapes (pure JSON or JSONC):
//
//	{ "mcpServers": { ...all... }, "disabled": ["name"] }
//	{ "mcpServers": { ...all... }, "disabledServers": { "name": {...} } }
//	{ "mcpServers": { ...all enabled... } }  // full replace; nothing disabled
//
// Legacy // commented properties inside mcpServers are treated as disabled.
// The document is authoritative: names not present are hard-deleted.
func (s *Store) WriteMCPsContent(content []byte) error {
	enabled, disabled, order, err := parseUnifiedMCPContent(content)
	if err != nil {
		return fmt.Errorf("invalid MCP servers JSON: %w", err)
	}
	return s.WriteMCPCatalog(enabled, disabled, order)
}

// SetMCPEnabled enables or disables one MCP server.
// Disable moves the entry into servers.disabled.json; enable moves it back
// into servers.json. Entries are never hard-deleted by this method.
func (s *Store) SetMCPEnabled(name string, enabled bool) error {
	active, disabled, order, err := s.loadMCPSplit()
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
				return nil
			}
			return fmt.Errorf("mcp server %q not found", name)
		}
		disabled[name] = cfg
		delete(active, name)
	}
	return s.writeMCPSplit(active, disabled, order)
}

// ResetMCPs removes the user overlays for enabled and disabled MCP files.
func (s *Store) ResetMCPs() error {
	if err := s.removeOverlay(agentsync.MCPEnabledPath); err != nil {
		return err
	}
	return s.removeOverlay(agentsync.MCPDisabledPath)
}

// SetProviderEnabled enables or disables a provider adapter via disabled.json.
func (s *Store) SetProviderEnabled(id string, enabled bool) error {
	id = strings.ToLower(id)
	return s.setDisabled(func(disabledSkills, disabledProviders map[string]bool) {
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

// ReadRegistry returns the registry skills manifest with disabled entries
// from skills.disabled.json included for the portal list.
func (s *Store) ReadRegistry() (*RegistrySkills, error) {
	enabled, disabled, err := s.loadRegistrySplit()
	if err != nil {
		return nil, err
	}
	overridden := s.isOverridden(agentsync.RegistryEnabledPath) ||
		s.isOverridden(agentsync.RegistryDisabledPath)
	items := buildRegistryItems(enabled, disabled)
	return &RegistrySkills{
		Skills:         enabled,
		DisabledSkills: disabled,
		Items:          items,
		Overridden:     overridden,
		Source:         sourceLabel(overridden),
	}, nil
}

// WriteRegistry updates the enabled registry skills via overlay.
// Skills that were previously enabled but are missing from reg are moved
// into skills.disabled.json (not hard-deleted). Skills re-listed leave the
// disabled file.
func (s *Store) WriteRegistry(reg *RegistrySkills) error {
	prevEnabled, disabled, err := s.loadRegistrySplit()
	if err != nil {
		return err
	}
	disabledByName := registryByName(disabled)
	next := reg.Skills
	if next == nil {
		next = []RegistrySkill{}
	}
	nextNames := map[string]bool{}
	for _, sk := range next {
		nextNames[sk.Name] = true
		delete(disabledByName, sk.Name)
	}
	for _, sk := range prevEnabled {
		if nextNames[sk.Name] {
			continue
		}
		if _, already := disabledByName[sk.Name]; !already {
			disabledByName[sk.Name] = sk
		}
	}
	return s.writeRegistrySplit(next, registryValues(disabledByName))
}

// SetRegistrySkillEnabled moves a registry skill between skills.json and
// skills.disabled.json by name.
func (s *Store) SetRegistrySkillEnabled(name string, enabled bool) error {
	active, disabled, err := s.loadRegistrySplit()
	if err != nil {
		return err
	}
	activeBy := registryByName(active)
	disabledBy := registryByName(disabled)
	if enabled {
		sk, ok := disabledBy[name]
		if !ok {
			if _, already := activeBy[name]; already {
				return nil
			}
			return fmt.Errorf("registry skill %q not found among disabled entries", name)
		}
		activeBy[name] = sk
		delete(disabledBy, name)
	} else {
		sk, ok := activeBy[name]
		if !ok {
			if _, already := disabledBy[name]; already {
				return nil
			}
			return fmt.Errorf("registry skill %q not found", name)
		}
		disabledBy[name] = sk
		delete(activeBy, name)
	}
	return s.writeRegistrySplit(registryValues(activeBy), registryValues(disabledBy))
}

// ResetRegistry removes enabled and disabled registry overlays.
func (s *Store) ResetRegistry() error {
	if err := s.removeOverlay(agentsync.RegistryEnabledPath); err != nil {
		return err
	}
	return s.removeOverlay(agentsync.RegistryDisabledPath)
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

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatUnifiedMCPContent builds the single portal MCP source document.
func formatUnifiedMCPContent(enabled, disabled map[string]any, order []string) ([]byte, error) {
	if enabled == nil {
		enabled = map[string]any{}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	all := make(map[string]any, len(enabled)+len(disabled))
	seen := map[string]bool{}
	orderedNames := make([]string, 0, len(enabled)+len(disabled))
	for _, name := range order {
		if cfg, ok := enabled[name]; ok {
			all[name] = cfg
			if !seen[name] {
				orderedNames = append(orderedNames, name)
				seen[name] = true
			}
			continue
		}
		if cfg, ok := disabled[name]; ok {
			all[name] = cfg
			if !seen[name] {
				orderedNames = append(orderedNames, name)
				seen[name] = true
			}
		}
	}
	for _, name := range sortedMapKeys(enabled) {
		if seen[name] {
			continue
		}
		all[name] = enabled[name]
		orderedNames = append(orderedNames, name)
		seen[name] = true
	}
	for _, name := range sortedMapKeys(disabled) {
		if seen[name] {
			continue
		}
		all[name] = disabled[name]
		orderedNames = append(orderedNames, name)
		seen[name] = true
	}

	// Build ordered mcpServers object via intermediate JSON for stable key order.
	type doc struct {
		MCPServers map[string]any `json:"mcpServers"`
		Disabled   []string       `json:"disabled,omitempty"`
	}
	// json.Marshal map is random order; rebuild with ordered keys by
	// marshaling manually for mcpServers.
	disabledNames := sortedMapKeys(disabled)
	var b strings.Builder
	b.WriteString("{\n  \"mcpServers\": {\n")
	for i, name := range orderedNames {
		raw, err := json.MarshalIndent(all[name], "    ", "  ")
		if err != nil {
			return nil, err
		}
		b.WriteString("    ")
		key, _ := json.Marshal(name)
		b.Write(key)
		b.WriteString(": ")
		b.Write(raw)
		if i < len(orderedNames)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  }")
	if len(disabledNames) > 0 {
		b.WriteString(",\n  \"disabled\": [\n")
		for i, name := range disabledNames {
			key, _ := json.Marshal(name)
			b.WriteString("    ")
			b.Write(key)
			if i < len(disabledNames)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("  ]")
	}
	b.WriteString("\n}\n")
	_ = doc{} // keep type for clarity of schema
	return []byte(b.String()), nil
}

// parseUnifiedMCPContent parses the single portal MCP source document.
func parseUnifiedMCPContent(data []byte) (enabled, disabled map[string]any, order []string, err error) {
	// First recover // commented server props as disabled (legacy JSONC).
	// Then parse the top-level object for disabled / disabledServers.
	var top struct {
		MCPServers      map[string]any `json:"mcpServers"`
		Disabled        []string       `json:"disabled"`
		DisabledServers map[string]any `json:"disabledServers"`
	}
	// Use JSONC so comments are stripped from structure parse, but also
	// collect commented keys under mcpServers via ParseMCPServersJSONC.
	activeFromJSONC, commented, order, err := agentsync.ParseMCPServersJSONC(data)
	if err != nil {
		// Fallback: allow documents that only use disabledServers without
		// a classic mcpServers-only shape — try plain UnmarshalJSONC.
		if uerr := agentsync.UnmarshalJSONC(data, &top); uerr != nil {
			return nil, nil, nil, err
		}
	} else {
		if uerr := agentsync.UnmarshalJSONC(data, &top); uerr != nil {
			return nil, nil, nil, uerr
		}
		if top.MCPServers == nil {
			top.MCPServers = activeFromJSONC
		}
	}
	if top.MCPServers == nil {
		top.MCPServers = map[string]any{}
	}

	// Catalog starts as every key under mcpServers + disabledServers + comments.
	all := make(map[string]any, len(top.MCPServers)+len(top.DisabledServers)+len(commented))
	for name, cfg := range top.MCPServers {
		all[name] = cfg
	}
	for name, cfg := range top.DisabledServers {
		if _, ok := all[name]; !ok {
			all[name] = cfg
		}
	}
	for name, cfg := range commented {
		if _, ok := all[name]; !ok {
			all[name] = cfg
		}
	}

	disabledSet := map[string]bool{}
	for _, name := range top.Disabled {
		disabledSet[name] = true
	}
	for name := range top.DisabledServers {
		disabledSet[name] = true
	}
	for name := range commented {
		// Only mark commented as disabled if not actively present in mcpServers.
		if _, active := top.MCPServers[name]; !active {
			disabledSet[name] = true
		}
	}

	enabled = map[string]any{}
	disabled = map[string]any{}
	if len(order) == 0 {
		order = sortedMapKeys(all)
	} else {
		// Ensure order covers all keys.
		seen := map[string]bool{}
		for _, n := range order {
			seen[n] = true
		}
		for _, n := range sortedMapKeys(all) {
			if !seen[n] {
				order = append(order, n)
			}
		}
	}
	for name, cfg := range all {
		if disabledSet[name] {
			disabled[name] = cfg
		} else {
			enabled[name] = cfg
		}
	}
	// Disabled names listed but missing config → error for clarity.
	for name := range disabledSet {
		if _, ok := all[name]; !ok {
			return nil, nil, nil, fmt.Errorf("disabled server %q has no config in mcpServers", name)
		}
	}
	return enabled, disabled, order, nil
}

func buildRegistryItems(enabled, disabled []RegistrySkill) []RegistrySkillItem {
	items := make([]RegistrySkillItem, 0, len(enabled)+len(disabled))
	for _, sk := range enabled {
		items = append(items, RegistrySkillItem{RegistrySkill: sk, Enabled: true})
	}
	enabledNames := map[string]bool{}
	for _, sk := range enabled {
		enabledNames[sk.Name] = true
	}
	for _, sk := range disabled {
		if enabledNames[sk.Name] {
			continue
		}
		items = append(items, RegistrySkillItem{RegistrySkill: sk, Enabled: false})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func registryByName(skills []RegistrySkill) map[string]RegistrySkill {
	out := map[string]RegistrySkill{}
	for _, sk := range skills {
		if sk.Name == "" {
			continue
		}
		out[sk.Name] = sk
	}
	return out
}

func registryValues(m map[string]RegistrySkill) []RegistrySkill {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]RegistrySkill, 0, len(names))
	for _, n := range names {
		out = append(out, m[n])
	}
	return out
}

func (s *Store) readToggles() (agentsync.PortalToggles, error) {
	// Prefer disabled.json; fall back to legacy toggles.jsonc.
	data, err := s.readEffective(agentsync.PortalDisabledPath)
	if err == nil {
		return agentsync.ParsePortalDisabled(data)
	}
	if !isMissingFile(err) {
		return agentsync.PortalToggles{}, err
	}
	legacy, err := s.readEffective(agentsync.PortalTogglesPath)
	if err != nil {
		if isMissingFile(err) {
			return agentsync.PortalToggles{}, nil
		}
		return agentsync.PortalToggles{}, err
	}
	return agentsync.ParsePortalToggles(legacy)
}

func isMissingFile(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "file does not exist") || strings.Contains(msg, "no such file")
}

// setDisabled mutates disabled skill/provider maps and rewrites disabled.json.
func (s *Store) setDisabled(mutate func(disabledSkills, disabledProviders map[string]bool)) error {
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

	data, err := agentsync.FormatPortalDisabled(disabledSkills, disabledProviders)
	if err != nil {
		return err
	}
	// Drop legacy toggles overlay so the new file is the single source of truth.
	_ = s.removeOverlay(agentsync.PortalTogglesPath)
	return s.writeOverlay(agentsync.PortalDisabledPath, data)
}

func (s *Store) readMCPDisabled() (map[string]any, error) {
	data, err := s.readEffective(agentsync.MCPDisabledPath)
	if err != nil {
		if isMissingFile(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return agentsync.ParseMCPDisabledJSON(data)
}

func (s *Store) loadMCPSplit() (enabled, disabled map[string]any, order []string, err error) {
	data, err := s.readEffective(agentsync.MCPEnabledPath)
	if err != nil {
		return nil, nil, nil, err
	}
	enabled, legacyDisabled, order, err := agentsync.ParseMCPServersJSONC(data)
	if err != nil {
		return nil, nil, nil, err
	}
	disabled, err = s.readMCPDisabled()
	if err != nil {
		return nil, nil, nil, err
	}
	for name, cfg := range legacyDisabled {
		if _, ok := enabled[name]; ok {
			continue
		}
		if _, ok := disabled[name]; !ok {
			disabled[name] = cfg
		}
	}
	return enabled, disabled, order, nil
}

func (s *Store) writeMCPSplit(enabled, disabled map[string]any, order []string) error {
	if enabled == nil {
		enabled = map[string]any{}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	// Enabled keys must not remain in the disabled file.
	for name := range enabled {
		delete(disabled, name)
	}
	enData, err := agentsync.FormatMCPServersJSON(enabled, order)
	if err != nil {
		return err
	}
	if err := s.writeOverlay(agentsync.MCPEnabledPath, enData); err != nil {
		return err
	}
	if len(disabled) == 0 {
		return s.removeOverlay(agentsync.MCPDisabledPath)
	}
	disData, err := agentsync.FormatMCPDisabledJSON(disabled)
	if err != nil {
		return err
	}
	return s.writeOverlay(agentsync.MCPDisabledPath, disData)
}

func (s *Store) loadRegistrySplit() (enabled, disabled []RegistrySkill, err error) {
	data, err := s.readEffective(agentsync.RegistryEnabledPath)
	if err != nil {
		return nil, nil, err
	}
	var reg RegistrySkills
	if err := agentsync.UnmarshalJSONC(data, &reg); err != nil {
		return nil, nil, fmt.Errorf("invalid registry skills JSON: %w", err)
	}
	enabled = reg.Skills
	if enabled == nil {
		enabled = []RegistrySkill{}
	}

	disData, err := s.readEffective(agentsync.RegistryDisabledPath)
	if err != nil {
		if isMissingFile(err) {
			return enabled, []RegistrySkill{}, nil
		}
		return nil, nil, err
	}
	var dis RegistrySkills
	if err := agentsync.UnmarshalJSONC(disData, &dis); err != nil {
		return nil, nil, fmt.Errorf("invalid registry disabled JSON: %w", err)
	}
	disabled = dis.Skills
	if disabled == nil {
		disabled = []RegistrySkill{}
	}
	// Enabled names win if present in both.
	enNames := map[string]bool{}
	for _, sk := range enabled {
		enNames[sk.Name] = true
	}
	filtered := disabled[:0]
	for _, sk := range disabled {
		if enNames[sk.Name] {
			continue
		}
		filtered = append(filtered, sk)
	}
	return enabled, filtered, nil
}

func (s *Store) writeRegistrySplit(enabled, disabled []RegistrySkill) error {
	if enabled == nil {
		enabled = []RegistrySkill{}
	}
	if disabled == nil {
		disabled = []RegistrySkill{}
	}
	enData, err := json.MarshalIndent(RegistrySkills{Skills: enabled}, "", "  ")
	if err != nil {
		return err
	}
	enData = append(enData, '\n')
	if err := s.writeOverlay(agentsync.RegistryEnabledPath, enData); err != nil {
		return err
	}
	if len(disabled) == 0 {
		return s.removeOverlay(agentsync.RegistryDisabledPath)
	}
	disData, err := json.MarshalIndent(RegistrySkills{Skills: disabled}, "", "  ")
	if err != nil {
		return err
	}
	disData = append(disData, '\n')
	return s.writeOverlay(agentsync.RegistryDisabledPath, disData)
}
