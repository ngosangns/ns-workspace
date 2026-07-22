package agentsync

import (
	"fmt"
	"path/filepath"
	"sort"
)

// ClaudePlugin powers the ClaudeAdapter's extra script generation and
// caps. The plugin does not transform MCP servers (Claude accepts the
// shared shape verbatim).
type ClaudePlugin struct{}

// ExtendCapabilities adds the mcpScripts artifact so `agents` reports
// it. Subclasses may also add ArtifactRules / ArtifactCommands here.
func (ClaudePlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations returns no extras — ClaudeAdapter.Plan emits the
// generated mcp.commands.sh itself via Plan's body.
func (ClaudePlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns the generated helper script path.
func (ClaudePlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	return []string{filepath.Join(ctx.Options.AgentsDir, "generated", "claude", "mcp.commands.sh")}
}

// TransformMCPServers returns the manifest unchanged. Claude Code
// accepts the shared shape `{"type":"http","url":...}`.
func (ClaudePlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return manifest, nil
}

// OpenCodePlugin implements the OpenCode MCP rewrite: remote HTTP
// servers get type "remote"+url+enabled; local/stdio get type "local"
// with command as argv array + enabled. See transformOpenCodeMCPServer.
type OpenCodePlugin struct {
	ConfigPath string
}

// ExtendCapabilities adds ArtifactMCP so OpenCode shows up as
// MCP-capable in `agents`.
func (OpenCodePlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations is a no-op — OpenCodeAdapter.Plan owns the merge
// shape (the plugin does not own it).
func (OpenCodePlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns the canonical opencode.json path.
func (p OpenCodePlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	if p.ConfigPath == "" {
		return nil
	}
	return []string{p.ConfigPath}
}

// TransformMCPServers rewrites shared MCP entries into OpenCode's
// local/remote schema (type, command argv, enabled).
func (OpenCodePlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return opencodeMCPManifest(manifest), nil
}

// CodexPlugin implements Codex's TOML managed block via ExtraOperations
// and the MCP artifact flag via ExtendCapabilities. The actual TOML
// emission lives in codexMCPBlock (mcp.go) — the plugin just declares
// the artifact.
type CodexPlugin struct{}

// ExtendCapabilities adds ArtifactMCP for Codex so `agents` reports
// the MCP block target.
func (CodexPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations is a no-op — CodexAdapter.Plan owns the managed
// block emission.
func (CodexPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns the Codex config.toml path.
func (CodexPlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	return []string{filepath.Join(ctx.Home, ".codex", "config.toml")}
}

// TransformMCPServers returns the manifest unchanged. CodexAdapter
// renders the TOML block directly from the shared shape.
func (CodexPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return manifest, nil
}

// QwenPlugin / AntigravityPlugin / ClinePlugin are minimal per-provider
// overrides whose only job is to rewrite MCP servers into the
// vendor-specific shape. They use TransformMCPServers to drop or
// rename fields; the BaseAdapter handles the rest of the file fan-out.

// QwenPlugin rewrites HTTP servers to httpUrl and drops the type
// discriminator that Qwen's settings.json does not recognize.
type QwenPlugin struct{}

// ExtendCapabilities adds ArtifactMCP for the shared mcpServers path
// under ~/.qwen/settings.json.
func (QwenPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations returns no extras; the template method handles the
// file fan-out.
func (QwenPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (QwenPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers drops `type` and renames `url` to `httpUrl` for
// HTTP servers per Qwen docs. SSE keeps `url`; stdio keeps
// `command`+`args`.
func (QwenPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	transformed, err := transformMCPServersForAdapterImpl("qwen", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("qwen transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// AntigravityPlugin rewrites remote MCP servers to serverUrl and
// writes them into the standalone mcp_config.json profile (not
// settings.json). Skills/settings fan-out is handled by BaseAdapter.
// Docs: https://antigravity.google/docs/mcp
type AntigravityPlugin struct{}

// ExtendCapabilities adds ArtifactMCP for the shared mcpServers path.
func (AntigravityPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations returns no extras.
func (AntigravityPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (AntigravityPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers drops `type` and renames remote `url`/`httpUrl`
// to `serverUrl` per Antigravity MCP schema.
func (AntigravityPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	transformed, err := transformMCPServersForAdapterImpl("antigravity", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("antigravity transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// ClinePlugin drops the `type` field (Cline docs do not document it)
// and sets `trust: true` so Cline auto-approves MCP tool calls. The
// YOLO mode flag itself is stored by Cline in
// ~/.cline/data/settings/global-settings.json and cannot be set from
// cline_mcp_settings.json.
type ClinePlugin struct{}

// ExtendCapabilities adds ArtifactMCP for the shared mcpServers path.
func (ClinePlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations returns no extras.
func (ClinePlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (ClinePlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers drops `type` and sets `trust: true` per Cline
// docs.
func (ClinePlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	transformed, err := transformMCPServersForAdapterImpl("cline", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("cline transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// GrokPlugin powers the Grok Build adapter's TOML MCP managed block.
// Instruction and skills are file-linked by BaseAdapter from
// AdapterTargets; MCP lives in ~/.grok/config.toml under
// [mcp_servers.<name>] and cannot go through MergeJSON, so ExtraOperations
// emits AppendManagedBlock (same primitive Codex uses for config.toml).
type GrokPlugin struct{}

// ExtendCapabilities adds ArtifactMCP so `agents` reports the managed
// config.toml MCP target alongside instructions and skills.
func (GrokPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations appends a managed MCP block into ~/.grok/config.toml
// when MCP is enabled and the shared manifest has servers. Empty
// manifests skip the op (Codex-compatible: a previously written block
// is left in place if the preset becomes empty).
func (GrokPlugin) ExtraOperations(ctx Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	if ctx.NoMCP {
		return nil, nil
	}
	manifest, err := readMCPManifestHook(ctx)
	if err != nil {
		return nil, err
	}
	if len(manifest.MCPServers) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	return []Operation{AppendManagedBlock{
		Dst:           filepath.Join(ctx.Home, ".grok", "config.toml"),
		Label:         "mcp",
		Content:       grokMCPBlock(manifest),
		Replace:       true,
		CleanupTables: names,
	}}, nil
}

// ExtraStatusPaths returns the Grok config.toml path so status/doctor
// surface the managed MCP target.
func (GrokPlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	return []string{filepath.Join(ctx.Home, ".grok", "config.toml")}
}

// TransformMCPServers returns the manifest unchanged. Grok MCP is
// rendered to TOML by grokMCPBlock from the shared shape; there is no
// JSON mcpServers path for this adapter.
func (GrokPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return manifest, nil
}

// ZCodePlugin powers the ZCode adapter. ZCode discovers skills from
// ~/.agents/skills (and optional ~/.zcode/skills); this adapter does
// not mirror skills. There is no first-party user-level MCP config
// file yet. When a stable ~/.zcode/mcp.json target ships, the plugin's
// TransformMCPServers will become the dispatch point.
type ZCodePlugin struct{}

// ExtendCapabilities leaves capabilities to artifactsFromSpec
// (instruction link only). Skills live in shared ~/.agents/skills.
func (ZCodePlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	return caps
}

// ExtraOperations returns no extras; the template method handles the
// file fan-out.
func (ZCodePlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (ZCodePlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers returns the manifest unchanged. Reserved for the
// day ZCode ships a user-level mcp.json / mcpServers target so the
// shared preset can flow through without a rewrite. Other Claude-lineage
// agents that ZCode inherits from also accept the canonical shape.
func (ZCodePlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	transformed, err := transformMCPServersForAdapterImpl("zcode", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("zcode transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// compile-time interface checks. Every concrete plugin must satisfy
// AdapterPlugin so the BaseAdapter constructor can wire it directly.
var (
	_ AdapterPlugin = ClaudePlugin{}
	_ AdapterPlugin = OpenCodePlugin{}
	_ AdapterPlugin = CodexPlugin{}
	_ AdapterPlugin = QwenPlugin{}
	_ AdapterPlugin = AntigravityPlugin{}
	_ AdapterPlugin = ClinePlugin{}
	_ AdapterPlugin = GrokPlugin{}
	_ AdapterPlugin = ZCodePlugin{}
	_ AdapterPlugin = NoopPlugin{}
)
