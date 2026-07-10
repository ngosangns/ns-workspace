package agentsync

import (
	"fmt"
	"io/fs"
	"strings"
)

// Options carries CLI flags and runtime paths for an agentsync run.
//
// The zero value is not useful; callers construct it from CLI flags via
// internal/cli before passing to Manager.Apply/Status/Doctor/etc.
type Options struct {
	Command    string
	AgentsDir  string
	ConfigPath string
	DryRun     bool
	Yes        bool
	Force      bool
	CopyMode   bool
	NoMCP      bool
	NoRegistry bool
	ToolFilter map[string]bool
	// DisabledProviders is filled from presets/portal/toggles.jsonc during
	// context setup. Keys are lower-case adapter ids. When set, selected()
	// returns false even if --tools would otherwise include the adapter.
	DisabledProviders map[string]bool
	// DisabledSkills is filled from presets/portal/toggles.jsonc. Top-level
	// skill directory names that should be skipped by InstallPresetTree.
	DisabledSkills map[string]bool
}

// Manager owns the preset FS and exposes the high-level
// Apply/Status/Doctor/Catalog/InstallRegistrySkills entry points.
//
// Construction is cheap; one Manager can drive many sequential runs
// because each call to Apply/Status creates its own Context.
type Manager struct {
	Presets fs.FS
}

// MCPManifest is the shared shape `presets/mcp/servers.json` ships in.
// Per-provider plugins rewrite the per-server entries through
// AdapterPlugin.TransformMCPServers so each native config file gets the
// field names and discriminators its vendor CLI expects.
type MCPManifest struct {
	MCPServers map[string]any `json:"mcpServers"`
}

// SettingsManifest models the cross-cutting settings preset
// (`presets/settings/default.json`). Provider-specific presets are merged
// into the same file via AdapterSettingsProfile.
type SettingsManifest struct {
	Hooks map[string]any `json:"hooks"`
}

// RegistryManifest models `presets/registry/skills.json` — third-party
// skills installed during update/registry. Default installer is
// `npx --yes skills add ... --agent universal`; entries may opt into
// other installers (e.g. GitButler `but skill install`).
type RegistryManifest struct {
	Skills []RegistrySkill `json:"skills"`
}

// RegistrySkill is one entry inside RegistryManifest.
//
// Installer selects how the skill is materialised:
//
//   - "" / "npx-skills": `npx --yes skills add <source> --skill <skill>`
//   - "but-skill": `but skill install --path <agents>/skills/<skill>`
//     (official GitButler CLI skill; see https://docs.gitbutler.com/commands/but-skill)
//
// Source is required for npx-skills and ignored for but-skill.
type RegistrySkill struct {
	Name      string `json:"name"`
	Source    string `json:"source,omitempty"`
	Skill     string `json:"skill"`
	Installer string `json:"installer,omitempty"`
}

const (
	registryAgentTarget = "universal"

	// installerNpxSkills is the default registry installer (skills.sh / npx).
	installerNpxSkills = "npx-skills"
	// installerButSkill installs via GitButler CLI (`but skill install`).
	installerButSkill = "but-skill"
)

// installerKind returns the effective installer for this entry.
func (s RegistrySkill) installerKind() string {
	switch strings.TrimSpace(s.Installer) {
	case installerButSkill:
		return installerButSkill
	case "", installerNpxSkills:
		return installerNpxSkills
	default:
		return strings.TrimSpace(s.Installer)
	}
}

// ArtifactKind enumerates the kinds of materializable artifacts the
// agentsync pipeline can produce. Tests and status output use it to
// report what each adapter handles.
type ArtifactKind string

const (
	ArtifactDirectory    ArtifactKind = "directory"
	ArtifactInstructions ArtifactKind = "instructions"
	ArtifactSkills       ArtifactKind = "skills"
	ArtifactSubagents    ArtifactKind = "subagents"
	ArtifactSettings     ArtifactKind = "settings"
	ArtifactHooks        ArtifactKind = "hooks"
	ArtifactMCP          ArtifactKind = "mcp"
	ArtifactRules        ArtifactKind = "rules"
	ArtifactCommands     ArtifactKind = "commands"
)

// SupportTier classifies adapter maturity. Stable adapters run by
// default; Manual adapters only emit helper scripts under
// ~/.agents/generated/<id>/; Experimental adapters are off by default.
type SupportTier string

const (
	TierStable       SupportTier = "stable"
	TierManual       SupportTier = "manual"
	TierExperimental SupportTier = "experimental"
	TierCatalog      SupportTier = "catalog"
)

// AgentCapabilities is the public description returned by every
// Adapter implementation. The Manager aggregates these for `agents` and
// `doctor` commands.
type AgentCapabilities struct {
	Tier      SupportTier
	DocsURL   []string
	Artifacts []ArtifactKind
	Notes     string
}

// StatusReporter is the minimal interface Manager uses to emit status
// lines. Production code passes stdoutReporter; tests inject a buffer.
type StatusReporter interface {
	Line(format string, args ...any)
}

type stdoutReporter struct{}

// Line writes one status line to stdout. stdoutReporter is the default
// reporter used by Manager.context when no override is supplied.
func (stdoutReporter) Line(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// Context carries the resolved runtime state for one Apply/Status/Doctor
// call. It embeds Options so callers can read either path; XDGConfigHome
// and Home are pre-resolved from the environment.
type Context struct {
	Options
	Home          string
	XDGConfigHome string
	Presets       fs.FS
	UserConfig    UserConfig
	Report        StatusReporter
	Update        bool
	manifestCache map[string]any
	seenDirs      map[string]bool
}
