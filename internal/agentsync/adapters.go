package agentsync

import (
	"path/filepath"
	"strings"
)

// Adapter is the polymorphism contract every provider CLI integration
// must satisfy. Manager dispatches the high-level Apply / Status /
// Doctor / Catalog entry points through this interface; the concrete
// implementations live in adapter_concrete.go as embed-based
// subclasses of BaseAdapter.
type Adapter interface {
	Name() string
	Aliases() []string
	Capabilities() AgentCapabilities
	Plan(ctx Context, update bool) ([]Operation, error)
	StatusPaths(ctx Context) []string
	DoctorExecutables() []string
}

// AdapterTargets describes the per-adapter paths and merge keys the
// pipeline writes into. Empty fields mean the adapter does not produce
// that artifact; the corresponding merge step is skipped.
type AdapterTargets struct {
	Instruction  string
	Skills       string
	Subagents    string
	Settings     string
	HooksPath    string
	HooksKeyPath []string
	MCPPath      string
	MCPKeyPath   []string
}

// AdapterSpec is the data-driven half of an adapter: identity, native
// targets, docs, and tier. The behavior half is the optional Plugin
// (see adapter_base.go). Together they feed BaseAdapter's template
// methods.
type AdapterSpec struct {
	ID          string
	Aliases     []string
	Tier        SupportTier
	Docs        []string
	Notes       string
	Executables []string
	Targets     AdapterTargets
	Manual      bool
}

// aliases returns lowercased aliases for tool-filter matching. Empty
// strings are dropped so a len-1 list means "no aliases".
func (s AdapterSpec) aliases() []string {
	out := make([]string, 0, len(s.Aliases))
	for _, alias := range s.Aliases {
		a := strings.ToLower(strings.TrimSpace(alias))
		if a != "" {
			out = append(out, a)
		}
	}
	return out
}

// selected reports whether opt's ToolFilter activates this adapter.
// Selection rules (in order):
//
//  1. Adapter id is in the filter map.
//  2. Any alias is in the filter map.
//  3. The filter requested all (special key "all") or the tier name.
func selected(opt Options, adapter Adapter) bool {
	name := strings.ToLower(adapter.Name())
	if opt.ToolFilter["all"] || opt.ToolFilter[name] {
		return true
	}
	if aliased, ok := adapter.(interface{ Aliases() []string }); ok {
		for _, alias := range aliased.Aliases() {
			if opt.ToolFilter[strings.ToLower(alias)] {
				return true
			}
		}
	}
	tier := string(adapter.Capabilities().Tier)
	return opt.ToolFilter[tier]
}

// nativePaths returns the resolved native paths for an adapter,
// filtering out empty entries so callers can iterate without nil
// checks. Used by both Status and the dry-run renderer.
func nativePaths(spec AdapterSpec, homeDir string) []string {
	t := spec.Targets
	paths := []string{
		expandHome(homeDir, t.Instruction),
		expandHome(homeDir, t.Skills),
		expandHome(homeDir, t.Subagents),
		expandHome(homeDir, t.Settings),
		expandHome(homeDir, t.HooksPath),
		expandHome(homeDir, t.MCPPath),
	}
	return compact(paths)
}

// expandHome joins rel under homeDir when rel starts with "." or "..",
// otherwise returns rel unchanged (absolute paths pass through).
func expandHome(homeDir, rel string) string {
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(homeDir, rel)
}
