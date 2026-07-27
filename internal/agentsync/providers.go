package agentsync

import (
	"fmt"
	"sort"
	"strings"
)

// SkillsProvidersPath is the central allowlist for which providers may
// receive each skill (preset or registry) during per-adapter skill mirror.
// Shape:
//
//	{
//	  "spawn-kimi": ["opencode", "claude"],
//	  "cleanup": "all"
//	}
//
// Missing key / empty file → allow all providers (backward compatible).
const SkillsProvidersPath = "presets/skills/providers.json"

// MCPProvidersPath is the central allowlist for which providers may
// receive each MCP server during per-adapter MCP write/transform.
// Same shape as SkillsProvidersPath.
const MCPProvidersPath = "presets/mcp/providers.json"

// ProviderRules maps item id (skill top-level name or MCP server name)
// to its allow rule. Missing keys default to all providers.
type ProviderRules map[string]ProviderAllow

// ProviderAllow is one item's allowed-providers rule.
type ProviderAllow struct {
	// All is true when the item is allowed on every provider.
	All bool
	// IDs are canonical lower-case adapter ids when All is false.
	IDs map[string]bool
}

// IsAll reports whether the rule allows every provider.
// Missing map keys default to all; a stored empty-ID rule means none.
func (a ProviderAllow) IsAll() bool {
	return a.All
}

// IsNone reports whether the rule allows no providers (explicit empty list).
func (a ProviderAllow) IsNone() bool {
	return !a.All && len(a.IDs) == 0
}

// Allows reports whether adapterID may receive this item.
func (a ProviderAllow) Allows(adapterID string) bool {
	if a.All {
		return true
	}
	if len(a.IDs) == 0 {
		return false
	}
	return a.IDs[CanonicalAdapterID(adapterID)]
}

// Allows reports whether itemID may be synced to adapterID.
// Missing item rules default to all.
func (r ProviderRules) Allows(itemID, adapterID string) bool {
	if r == nil {
		return true
	}
	rule, ok := r[itemID]
	if !ok {
		return true
	}
	return rule.Allows(adapterID)
}

// ProvidersFor returns the public list form for one item:
//   - ["all"] when unrestricted (missing key / All)
//   - [] when explicitly none
//   - sorted canonical adapter ids for a partial allowlist
func (r ProviderRules) ProvidersFor(itemID string) []string {
	if r == nil {
		return []string{"all"}
	}
	rule, ok := r[itemID]
	if !ok || rule.IsAll() {
		return []string{"all"}
	}
	if rule.IsNone() {
		return []string{}
	}
	out := make([]string, 0, len(rule.IDs))
	for id, on := range rule.IDs {
		if on {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// SetProviders updates the rule for itemID from a public providers list.
//   - ["all"] / list containing "all" → delete key (default all)
//   - [] → store explicit none (allow no providers)
//   - ["opencode", ...] → store those ids
func (r ProviderRules) SetProviders(itemID string, providers []string) {
	if r == nil {
		return
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return
	}
	if isAllProvidersList(providers) {
		delete(r, itemID)
		return
	}
	// Explicit empty array → none (key present, no ids).
	if providers != nil && len(providers) == 0 {
		r[itemID] = ProviderAllow{IDs: map[string]bool{}}
		return
	}
	ids := map[string]bool{}
	for _, p := range providers {
		p = CanonicalAdapterID(p)
		if p == "" || p == "all" {
			continue
		}
		ids[p] = true
	}
	if len(ids) == 0 {
		// Nil or only blanks after filter → treat as none.
		r[itemID] = ProviderAllow{IDs: map[string]bool{}}
		return
	}
	r[itemID] = ProviderAllow{IDs: ids}
}

// CanonicalAdapterID lowercases and maps known aliases to the registry
// canonical id (e.g. kiro-cli → kiro).
func CanonicalAdapterID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "kiro-cli":
		return "kiro"
	case "zcode-cli":
		return "zcode"
	case "claude-app", "claudedesktop":
		return "claude-desktop"
	default:
		return id
	}
}

// KnownAdapterIDs returns the set of canonical adapter ids plus aliases
// accepted in providers.json values. Built from a temporary registry so
// doctor/portal validation stays in sync with NewAdapterRegistry.
func KnownAdapterIDs() map[string]bool {
	reg := NewAdapterRegistry(RegistryOptions{})
	out := map[string]bool{}
	for _, id := range reg.Ids() {
		out[strings.ToLower(id)] = true
		out[CanonicalAdapterID(id)] = true
	}
	out["all"] = true
	return out
}

// LoadProviderRules reads providers.json at presetKey (overlay-aware).
// Missing file → empty rules (everything allowed).
func LoadProviderRules(ctx Context, presetKey string) ProviderRules {
	if ctx.Presets == nil {
		return ProviderRules{}
	}
	data, err := readPresetFile(ctx, presetKey)
	if err != nil {
		return ProviderRules{}
	}
	rules, err := ParseProviderRules(data)
	if err != nil {
		return ProviderRules{}
	}
	return rules
}

// ParseProviderRules parses a providers.json document.
func ParseProviderRules(data []byte) (ProviderRules, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return ProviderRules{}, nil
	}
	var raw map[string]any
	if err := UnmarshalJSONC(data, &raw); err != nil {
		return nil, fmt.Errorf("parse provider rules: %w", err)
	}
	out := ProviderRules{}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		rule, err := parseProviderAllow(value)
		if err != nil {
			return nil, fmt.Errorf("parse provider rules %q: %w", key, err)
		}
		if rule.IsAll() {
			// Persist explicit "all" as absent so default stays compact.
			continue
		}
		out[key] = rule
	}
	return out, nil
}

func parseProviderAllow(value any) (ProviderAllow, error) {
	switch v := value.(type) {
	case nil:
		return ProviderAllow{All: true}, nil
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "" || s == "all" || s == "*" {
			return ProviderAllow{All: true}, nil
		}
		return ProviderAllow{IDs: map[string]bool{CanonicalAdapterID(s): true}}, nil
	case []any:
		// Explicit empty array → none (allow no providers).
		if len(v) == 0 {
			return ProviderAllow{IDs: map[string]bool{}}, nil
		}
		ids := map[string]bool{}
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return ProviderAllow{}, fmt.Errorf("provider id must be string, got %T", item)
			}
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "" {
				continue
			}
			if s == "all" || s == "*" {
				return ProviderAllow{All: true}, nil
			}
			ids[CanonicalAdapterID(s)] = true
		}
		// Array of blanks only → none.
		return ProviderAllow{IDs: ids}, nil
	default:
		return ProviderAllow{}, fmt.Errorf("expected string or array, got %T", value)
	}
}

// FormatProviderRules renders pure JSON for a providers.json file.
func FormatProviderRules(rules ProviderRules) ([]byte, error) {
	file := map[string]any{}
	if rules != nil {
		keys := make([]string, 0, len(rules))
		for k, rule := range rules {
			if rule.IsAll() {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			rule := rules[k]
			ids := make([]string, 0, len(rule.IDs))
			for id, on := range rule.IDs {
				if on {
					ids = append(ids, id)
				}
			}
			sort.Strings(ids)
			// Empty ids → [] (none). Non-empty → partial allowlist.
			file[k] = ids
		}
	}
	return encodeJSONIndent(file)
}

// isAllProvidersList reports whether providers means unrestricted "all".
// An explicit empty slice means none, not all.
func isAllProvidersList(providers []string) bool {
	if providers == nil {
		return true
	}
	for _, p := range providers {
		s := strings.ToLower(strings.TrimSpace(p))
		if s == "all" || s == "*" {
			return true
		}
	}
	return false
}

// filterMCPManifest drops servers that are not allowed for adapterID.
func filterMCPManifest(ctx Context, adapterID string, manifest MCPManifest) MCPManifest {
	if len(manifest.MCPServers) == 0 {
		return manifest
	}
	rules := LoadProviderRules(ctx, MCPProvidersPath)
	out := make(map[string]any, len(manifest.MCPServers))
	for name, server := range manifest.MCPServers {
		if rules.Allows(name, adapterID) {
			out[name] = server
		}
	}
	return MCPManifest{MCPServers: out}
}

// filterMCPServersMap drops servers not allowed for adapterID.
func filterMCPServersMap(ctx Context, adapterID string, servers map[string]any) map[string]any {
	if len(servers) == 0 {
		return servers
	}
	rules := LoadProviderRules(ctx, MCPProvidersPath)
	out := make(map[string]any, len(servers))
	for name, server := range servers {
		if rules.Allows(name, adapterID) {
			out[name] = server
		}
	}
	return out
}

// ValidateProviderRules reports unknown provider ids found in rules.
// Returns sorted warning strings (empty when clean).
func ValidateProviderRules(rules ProviderRules, label string) []string {
	if len(rules) == 0 {
		return nil
	}
	known := KnownAdapterIDs()
	var warnings []string
	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, item := range keys {
		rule := rules[item]
		if rule.IsAll() {
			continue
		}
		ids := make([]string, 0, len(rule.IDs))
		for id := range rule.IDs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if !known[id] {
				warnings = append(warnings, fmt.Sprintf("%s %q: unknown provider %q", label, item, id))
			}
		}
	}
	return warnings
}
