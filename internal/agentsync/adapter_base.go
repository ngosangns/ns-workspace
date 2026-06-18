package agentsync

import (
	"fmt"
	"os"
	"path/filepath"
)

// AdapterPlugin is the explicit behavior hook every adapter may
// implement. The default (NoopPlugin) returns zero values so plain
// adapters do not have to declare any methods. Concrete plugins embed
// or implement AdapterPlugin and override the methods they need.
//
// Each method's signature is:
//
//   - ExtendCapabilities(spec, caps): mutate caps after the template
//     method has computed the artifact list from AdapterSpec. Plugins
//     add extras like ArtifactMCP for OpenCode/Codex, or ArtifactRules
//     for Aider.
//   - ExtraOperations(ctx, spec, update): return extra Operation values
//     the template method appends after writing settings. Used by
//     Claude (generated mcp.commands.sh), OpenCode (merged config),
//     Codex (TOML managed block), Aider (conventions block), MiniMax
//     (config.json merge).
//   - ExtraStatusPaths(ctx, spec): return extra paths to include in
//     `status` output beyond the native paths the template method
//     already adds.
//   - TransformMCPServers(manifest): rewrite each MCP server entry
//     from the canonical shared shape into the shape the target
//     provider's native config expects. The default returns the input
//     unchanged.
type AdapterPlugin interface {
	ExtendCapabilities(spec AdapterSpec, caps AgentCapabilities) AgentCapabilities
	ExtraOperations(ctx Context, spec AdapterSpec, update bool) ([]Operation, error)
	ExtraStatusPaths(ctx Context, spec AdapterSpec) []string
	TransformMCPServers(manifest MCPManifest) (MCPManifest, error)
}

// NoopPlugin is the zero-behavior plugin used by adapters that have
// no provider-specific quirks. All methods return zero values; the
// template method picks up the defaults.
type NoopPlugin struct{}

// ExtendCapabilities returns caps unchanged.
func (NoopPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	return caps
}

// ExtraOperations returns no extras.
func (NoopPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (NoopPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers returns manifest unchanged.
func (NoopPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return manifest, nil
}

// BaseAdapter is the template-method base for every concrete adapter.
// Subclasses embed BaseAdapter and only override the methods they
// need (typically TransformMCPServers for providers that have their
// own per-server shape, and Plan when the native flow is not a plain
// JSON write).
//
// The base implementation derives capabilities from AdapterSpec,
// plans the common file/link/merge operations, and exposes the native
// paths.
type BaseAdapter struct {
	Spec   AdapterSpec
	Plugin AdapterPlugin
}

// Name returns the adapter id.
func (b *BaseAdapter) Name() string { return b.Spec.ID }

// Aliases returns the lowercased alias list.
func (b *BaseAdapter) Aliases() []string { return b.Spec.aliases() }

// Capabilities computes artifacts from the spec, then lets the plugin
// extend them.
func (b *BaseAdapter) Capabilities() AgentCapabilities {
	caps := AgentCapabilities{Tier: b.Spec.Tier, DocsURL: b.Spec.Docs, Notes: b.Spec.Notes}
	for _, kind := range artifactsFromSpec(b.Spec) {
		caps.Artifacts = append(caps.Artifacts, kind)
	}
	if b.Plugin == nil {
		return caps
	}
	return b.Plugin.ExtendCapabilities(b.Spec, caps)
}

// artifactsFromSpec derives the artifact kinds from the spec.
func artifactsFromSpec(spec AdapterSpec) []ArtifactKind {
	artifacts := []ArtifactKind{}
	t := spec.Targets
	if t.Instruction != "" {
		artifacts = append(artifacts, ArtifactInstructions)
	}
	if t.Skills != "" {
		artifacts = append(artifacts, ArtifactSkills)
	}
	if t.Subagents != "" {
		artifacts = append(artifacts, ArtifactSubagents)
	}
	if t.Settings != "" || (t.AgentConfigSrc != "" && t.AgentConfigDst != "") {
		artifacts = append(artifacts, ArtifactSettings)
	}
	if t.Settings != "" || t.HooksPath != "" {
		artifacts = append(artifacts, ArtifactHooks)
	}
	if t.MCPPath != "" {
		artifacts = append(artifacts, ArtifactMCP)
	}
	if spec.Manual {
		artifacts = append(artifacts, ArtifactRules, ArtifactCommands)
	}
	return artifacts
}

// Plan builds the common set of operations for a stable adapter:
// instruction/skills/subagents/settings/hooks links plus merged MCP.
// Subclasses override this when they need different topology.
func (b *BaseAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	replace := update || ctx.Force
	ops := []Operation{}
	if b.Spec.Manual {
		return b.manualPlan(ctx)
	}
	ops = append(ops, b.fileLinkOps(ctx, replace)...)
	extra, err := b.profileAndMcpOps(ctx, replace)
	if err != nil {
		return nil, err
	}
	ops = append(ops, extra...)
	if b.Plugin != nil {
		pluginExtra, err := b.Plugin.ExtraOperations(ctx, b.Spec, update)
		if err != nil {
			return nil, err
		}
		ops = append(ops, pluginExtra...)
	}
	return ops, nil
}

// manualPlan returns the single ManualStep emitted for manual /
// experimental adapters.
func (b *BaseAdapter) manualPlan(ctx Context) ([]Operation, error) {
	return []Operation{ManualStep{
		Agent: b.Spec.ID,
		Dst:   filepath.Join(ctx.Options.AgentsDir, "generated", b.Spec.ID, "README.md"),
		Text:  manualReadme(b.Spec),
	}}, nil
}

// fileLinkOps emits LinkOrCopy / LinkSkillDirs operations for the
// per-adapter file fan-out.
func (b *BaseAdapter) fileLinkOps(ctx Context, replace bool) []Operation {
	ops := []Operation{}
	t := b.Spec.Targets
	sourceAgents := filepath.Join(ctx.Options.AgentsDir, "AGENTS.md")
	sourceSkills := filepath.Join(ctx.Options.AgentsDir, "skills")
	sourceSubagents := filepath.Join(ctx.Options.AgentsDir, "agents")
	if t.Instruction != "" {
		ops = append(ops, LinkOrCopy{Src: sourceAgents, Dst: t.Instruction, Replace: replace})
	}
	if t.Skills != "" {
		ops = append(ops, LinkSkillDirs{SrcRoot: sourceSkills, DstRoot: t.Skills, Replace: replace})
	}
	if t.Subagents != "" {
		ops = append(ops, LinkSkillDirs{SrcRoot: sourceSubagents, DstRoot: t.Subagents, Replace: replace})
	}
	if t.AgentConfigSrc != "" && t.AgentConfigDst != "" {
		ops = append(ops, InstallPresetFile{Src: t.AgentConfigSrc, Dst: t.AgentConfigDst, Replace: replace})
	}
	return ops
}

// profileAndMcpOps emits ApplyAdapterSettings for profile-based
// providers and MergeJSON for the shared MCP servers.
func (b *BaseAdapter) profileAndMcpOps(ctx Context, replace bool) ([]Operation, error) {
	ops := []Operation{}
	profilePath, err := b.adapterSettingsProfile(ctx)
	if err != nil {
		return nil, err
	}
	if profilePath != "" {
		homeDir, err := adapterSettingsHomeDir()
		if err != nil {
			return nil, err
		}
		targetPath, err := resolveAdapterSettingsTarget(ctx, profilePath, homeDir)
		if err != nil {
			return nil, err
		}
		ops = append(ops, ApplyAdapterSettings{
			ProfilePath: profilePath,
			TargetPath:  targetPath,
			HomeDir:     homeDir,
			Replace:     replace,
		})
	} else {
		t := b.Spec.Targets
		if t.Settings != "" {
			ops = append(ops, LinkOrCopy{
				Src:     filepath.Join(ctx.Options.AgentsDir, "settings.json"),
				Dst:     t.Settings,
				Replace: replace,
			})
		}
		if t.HooksPath != "" && len(t.HooksKeyPath) > 0 {
			manifest, err := readSettingsManifest(ctx)
			if err != nil {
				return nil, err
			}
			ops = append(ops, MergeJSON{
				Dst:     t.HooksPath,
				KeyPath: t.HooksKeyPath,
				Values:  manifest.Hooks,
				Replace: replace,
			})
		}
	}
	if !ctx.NoMCP && b.Spec.Targets.MCPPath != "" && len(b.Spec.Targets.MCPKeyPath) > 0 {
		manifest, err := readMCPManifest(ctx)
		if err != nil {
			return nil, err
		}
		transformed, err := b.transformMCP(manifest)
		if err != nil {
			return nil, err
		}
		ops = append(ops, MergeJSON{
			Dst:     b.Spec.Targets.MCPPath,
			KeyPath:  b.Spec.Targets.MCPKeyPath,
			Values:  transformed,
			Replace: replace,
		})
	}
	return ops, nil
}

// transformMCP delegates to the plugin when one is set, otherwise
// falls back to the global dispatcher.
func (b *BaseAdapter) transformMCP(manifest MCPManifest) (map[string]any, error) {
	if b.Plugin != nil {
		transformed, err := b.Plugin.TransformMCPServers(manifest)
		if err != nil {
			return nil, err
		}
		return transformed.MCPServers, nil
	}
	return transformMCPServersForAdapter(b.Spec.ID, manifest)
}

// adapterSettingsProfile looks up the profile path for this adapter
// in presets/manifest.json.
func (b *BaseAdapter) adapterSettingsProfile(ctx Context) (string, error) {
	manifest, err := loadAdapterSettingsManifest(ctx)
	if err != nil {
		return "", err
	}
	return manifest[b.Spec.ID], nil
}

// StatusPaths returns the union of native paths and any extra paths
// the plugin surfaces.
func (b *BaseAdapter) StatusPaths(ctx Context) []string {
	paths := nativePaths(b.Spec, ctx.Home)
	if b.Spec.Manual {
		paths = append(paths, filepath.Join(ctx.Options.AgentsDir, "generated", b.Spec.ID, "README.md"))
	}
	if b.Plugin != nil {
		paths = append(paths, b.Plugin.ExtraStatusPaths(ctx, b.Spec)...)
	}
	return compact(paths)
}

// DoctorExecutables returns the executable names Doctor probes via
// LookPath.
func (b *BaseAdapter) DoctorExecutables() []string { return b.Spec.Executables }

// adapterSettingsHomeDir returns the user home directory used to
// resolve the relative target paths declared in adapter settings
// profiles.
func adapterSettingsHomeDir() (string, error) {
	return os.UserHomeDir()
}

// compile-time interface checks.
var _ Adapter = (*BaseAdapter)(nil)

// formatPathError wraps an error returned by a per-adapter Plan with
// the adapter id for friendlier log output.
func formatPathError(adapterID string, err error) error {
	return fmt.Errorf("adapter %s: %w", adapterID, err)
}
