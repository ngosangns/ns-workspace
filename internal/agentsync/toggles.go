package agentsync

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// PortalTogglesPath is the preset key for portal enable/disable state.
// Disabled skills and providers are stored as // comments so the file
// remains human-readable JSONC.
const PortalTogglesPath = "presets/portal/toggles.jsonc"

// PortalToggles models presets/portal/toggles.jsonc.
//
// Shape:
//
//	{
//	  "skills": {
//	    "commit": true,
//	    // "spawn-kimi": true
//	  },
//	  "providers": {
//	    "claude": true
//	    // "gemini": true
//	  }
//	}
//
// Presence of a live key means enabled. Commented keys are disabled.
// Skills/providers absent from both maps default to enabled.
type PortalToggles struct {
	// EnabledSkills maps skill id → true for currently enabled skills
	// that appear as live keys. Empty map means "no explicit list".
	EnabledSkills map[string]bool
	// DisabledSkills maps skill id → true for portal-disabled skills.
	DisabledSkills map[string]bool
	// EnabledProviders / DisabledProviders mirror the same for adapters.
	EnabledProviders  map[string]bool
	DisabledProviders map[string]bool
	// SkillOrder / ProviderOrder preserve file order for rewrites.
	SkillOrder    []string
	ProviderOrder []string
}

// loadPortalToggles reads toggles from the user overlay or embedded
// preset. Missing file → empty toggles (everything enabled by default).
func loadPortalToggles(ctx Context) (PortalToggles, error) {
	data, err := readPresetFile(ctx, PortalTogglesPath)
	if err != nil {
		// Optional file: treat missing as empty toggles.
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			return PortalToggles{}, nil
		}
		// readPresetFile wraps errors as "preset path: %w"
		if strings.Contains(err.Error(), "file does not exist") ||
			strings.Contains(err.Error(), "no such file") {
			return PortalToggles{}, nil
		}
		return PortalToggles{}, err
	}
	return ParsePortalToggles(data)
}

// loadDisabledSkills returns the set of skill top-level names disabled
// via portal toggles. Used by InstallPresetTree.
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
// from the toggles preset when not already set by the caller.
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

// ParsePortalToggles parses JSONC toggles content.
func ParsePortalToggles(data []byte) (PortalToggles, error) {
	// Structure after strip: { "skills": {...}, "providers": {...} }
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

	// Normalize provider keys to lower-case.
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
	// Active keys via strip+unmarshal of body as object.
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
	// Commented keys = disabled
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

// FormatPortalToggles renders toggles JSONC with disabled entries commented.
// knownSkills / knownProviders are the full catalogs; only those listed
// will appear (enabled as live keys, disabled as comments).
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
	// Keep orphan disabled skills that are no longer in known list.
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

	// Nest under top-level object with 2-space indent for children.
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

// SortedIDs returns sorted keys of a bool map.
func SortedIDs(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
