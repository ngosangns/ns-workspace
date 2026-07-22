package agentsync

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// PortalDisabledPath is the preset key for portal-disabled skill and
// provider ids. Disabled entries live in this JSON file (not as //
// comments). Shape:
//
//	{
//	  "skills": ["spawn-kimi"],
//	  "providers": ["antigravity"]
//	}
//
// Missing file → everything enabled by default.
const PortalDisabledPath = "presets/portal/disabled.json"

// PortalTogglesPath is the legacy JSONC toggles file (// comments).
// Still read for migration when PortalDisabledPath is absent.
const PortalTogglesPath = "presets/portal/toggles.jsonc"

// PortalToggles holds enable/disable state for skills and providers.
// Only Disabled* maps are authoritative for sync/portal; Enabled* and
// order fields remain for legacy parse compatibility.
type PortalToggles struct {
	EnabledSkills     map[string]bool
	DisabledSkills    map[string]bool
	EnabledProviders  map[string]bool
	DisabledProviders map[string]bool
	SkillOrder        []string
	ProviderOrder     []string
}

// portalDisabledFile is the on-disk shape of PortalDisabledPath.
type portalDisabledFile struct {
	Skills    []string `json:"skills,omitempty"`
	Providers []string `json:"providers,omitempty"`
}

// loadPortalToggles reads disabled skills/providers from the overlay or
// embedded preset. Prefers disabled.json; falls back to legacy
// toggles.jsonc comments when the new file is missing.
func loadPortalToggles(ctx Context) (PortalToggles, error) {
	data, err := readPresetFile(ctx, PortalDisabledPath)
	if err == nil {
		return ParsePortalDisabled(data)
	}
	if !isMissingPreset(err) {
		return PortalToggles{}, err
	}

	// Legacy: toggles.jsonc with // commented keys.
	legacy, err := readPresetFile(ctx, PortalTogglesPath)
	if err != nil {
		if isMissingPreset(err) {
			return PortalToggles{}, nil
		}
		return PortalToggles{}, err
	}
	return ParsePortalToggles(legacy)
}

func isMissingPreset(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "file does not exist") || strings.Contains(msg, "no such file")
}

// loadDisabledSkills returns the set of skill top-level names disabled
// via portal disabled.json. Used by InstallPresetTree.
func loadDisabledSkills(ctx Context) map[string]bool {
	if ctx.DisabledSkills != nil {
		return ctx.DisabledSkills
	}
	t, err := loadPortalToggles(ctx)
	if err != nil {
		return map[string]bool{}
	}
	return t.DisabledSkills
}

// applyPortalToggles fills Options.DisabledProviders / DisabledSkills
// from the disabled preset when not already set by the caller.
func applyPortalToggles(ctx *Context) {
	if ctx.DisabledProviders != nil && ctx.DisabledSkills != nil {
		return
	}
	t, err := loadPortalToggles(*ctx)
	if err != nil {
		return
	}
	if ctx.DisabledSkills == nil {
		ctx.DisabledSkills = t.DisabledSkills
	}
	if ctx.DisabledProviders == nil {
		ctx.DisabledProviders = t.DisabledProviders
	}
}

// ParsePortalDisabled parses presets/portal/disabled.json.
func ParsePortalDisabled(data []byte) (PortalToggles, error) {
	var raw portalDisabledFile
	if err := UnmarshalJSONC(data, &raw); err != nil {
		return PortalToggles{}, fmt.Errorf("parse portal disabled: %w", err)
	}
	out := PortalToggles{
		EnabledSkills:     map[string]bool{},
		DisabledSkills:    map[string]bool{},
		EnabledProviders:  map[string]bool{},
		DisabledProviders: map[string]bool{},
	}
	for _, id := range raw.Skills {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out.DisabledSkills[id] = true
	}
	for _, id := range raw.Providers {
		id = strings.TrimSpace(strings.ToLower(id))
		if id == "" {
			continue
		}
		out.DisabledProviders[id] = true
	}
	return out, nil
}

// FormatPortalDisabled renders pure JSON for PortalDisabledPath.
func FormatPortalDisabled(disabledSkills, disabledProviders map[string]bool) ([]byte, error) {
	file := portalDisabledFile{
		Skills:    SortedIDs(disabledSkills),
		Providers: SortedIDs(disabledProviders),
	}
	// Normalize providers to lower-case in output.
	for i, id := range file.Providers {
		file.Providers[i] = strings.ToLower(id)
	}
	sort.Strings(file.Providers)
	// Drop empty slices so {} is valid when nothing is disabled.
	if len(file.Skills) == 0 {
		file.Skills = nil
	}
	if len(file.Providers) == 0 {
		file.Providers = nil
	}
	return encodeJSONIndent(file)
}

// ParsePortalToggles parses legacy JSONC toggles content (// comments).
// Kept for migration of existing overlays.
func ParsePortalToggles(data []byte) (PortalToggles, error) {
	var raw map[string]any
	if err := UnmarshalJSONC(data, &raw); err != nil {
		return PortalToggles{}, fmt.Errorf("parse portal toggles: %w", err)
	}
	out := PortalToggles{
		EnabledSkills:     map[string]bool{},
		DisabledSkills:    map[string]bool{},
		EnabledProviders:  map[string]bool{},
		DisabledProviders: map[string]bool{},
	}

	skillsBody, _ := extractObjectBody(string(data), "skills")
	providersBody, _ := extractObjectBody(string(data), "providers")

	if skillsBody != "" {
		fillBoolMapFromBody(skillsBody, out.EnabledSkills, out.DisabledSkills, &out.SkillOrder)
	} else if m, ok := raw["skills"].(map[string]any); ok {
		for k, v := range m {
			if boolish(v) {
				out.EnabledSkills[k] = true
			} else {
				out.DisabledSkills[k] = true
			}
		}
	}

	if providersBody != "" {
		fillBoolMapFromBody(providersBody, out.EnabledProviders, out.DisabledProviders, &out.ProviderOrder)
	} else if m, ok := raw["providers"].(map[string]any); ok {
		for k, v := range m {
			if boolish(v) {
				out.EnabledProviders[strings.ToLower(k)] = true
			} else {
				out.DisabledProviders[strings.ToLower(k)] = true
			}
		}
	}

	norm := map[string]bool{}
	for k, v := range out.DisabledProviders {
		norm[strings.ToLower(k)] = v
	}
	out.DisabledProviders = norm
	normE := map[string]bool{}
	for k, v := range out.EnabledProviders {
		normE[strings.ToLower(k)] = v
	}
	out.EnabledProviders = normE
	return out, nil
}

func fillBoolMapFromBody(body string, enabled, disabled map[string]bool, order *[]string) {
	var active map[string]any
	_ = UnmarshalJSONC([]byte(body), &active)
	if active == nil {
		active = map[string]any{}
	}
	for k, v := range active {
		if boolish(v) {
			enabled[k] = true
		} else {
			disabled[k] = true
		}
	}
	for k := range extractCommentedProperties(body) {
		if !enabled[k] {
			disabled[k] = true
		}
	}
	if order != nil {
		*order = objectKeyOrder(body, active)
	}
}

func boolish(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "yes"
	case float64:
		return t != 0
	default:
		return true
	}
}

// FormatPortalToggles is kept for tests that still exercise legacy JSONC.
// Prefer FormatPortalDisabled for new writes.
func FormatPortalToggles(knownSkills, knownProviders []string, disabledSkills, disabledProviders map[string]bool) ([]byte, error) {
	if disabledSkills == nil {
		disabledSkills = map[string]bool{}
	}
	if disabledProviders == nil {
		disabledProviders = map[string]bool{}
	}
	skillEnabled := map[string]any{}
	skillDisabled := map[string]any{}
	for _, id := range knownSkills {
		if disabledSkills[id] {
			skillDisabled[id] = true
		} else {
			skillEnabled[id] = true
		}
	}
	for id := range disabledSkills {
		if _, ok := skillEnabled[id]; !ok {
			if _, ok2 := skillDisabled[id]; !ok2 {
				skillDisabled[id] = true
			}
		}
	}

	provEnabled := map[string]any{}
	provDisabled := map[string]any{}
	for _, id := range knownProviders {
		id = strings.ToLower(id)
		if disabledProviders[id] {
			provDisabled[id] = true
		} else {
			provEnabled[id] = true
		}
	}
	for id := range disabledProviders {
		id = strings.ToLower(id)
		if _, ok := provEnabled[id]; !ok {
			if _, ok2 := provDisabled[id]; !ok2 {
				provDisabled[id] = true
			}
		}
	}

	skillsBlock, err := FormatCommentedObjectMap(skillEnabled, skillDisabled, knownSkills)
	if err != nil {
		return nil, err
	}
	providersBlock, err := FormatCommentedObjectMap(provEnabled, provDisabled, lowerAll(knownProviders))
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("  \"skills\": ")
	b.WriteString(indentBlock(string(skillsBlock), "  "))
	b.WriteString(",\n")
	b.WriteString("  \"providers\": ")
	b.WriteString(indentBlock(string(providersBlock), "  "))
	b.WriteString("\n}\n")
	return []byte(b.String()), nil
}

func lowerAll(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strings.ToLower(id)
	}
	return out
}

// indentBlock re-indents a JSONC object block (which already ends with
// newline) so it nests under a parent key. First line stays inline after
// the key; subsequent lines get extra prefix.
func indentBlock(block, prefix string) string {
	block = strings.TrimRight(block, "\n")
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return "{}"
	}
	if len(lines) == 1 {
		return lines[0]
	}
	var b strings.Builder
	b.WriteString(lines[0])
	b.WriteByte('\n')
	for i := 1; i < len(lines); i++ {
		b.WriteString(prefix)
		b.WriteString(lines[i])
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// IsSkillEnabled reports whether skill id is enabled given toggles.
// Default is enabled when not listed as disabled.
func (t PortalToggles) IsSkillEnabled(id string) bool {
	if t.DisabledSkills != nil && t.DisabledSkills[id] {
		return false
	}
	return true
}

// IsProviderEnabled reports whether provider id is enabled. Default true.
func (t PortalToggles) IsProviderEnabled(id string) bool {
	id = strings.ToLower(id)
	if t.DisabledProviders != nil && t.DisabledProviders[id] {
		return false
	}
	return true
}

// SortedIDs returns sorted keys of a bool map (only true values).
func SortedIDs(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

