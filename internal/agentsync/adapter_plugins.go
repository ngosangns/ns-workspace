package agentsync

import (
	"fmt"
	"path/filepath"
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

// OpenCodePlugin implements the OpenCode MCP rewrite: HTTP servers
// get `type: "remote"` and the path plugins layered on top of the
// preset provider config.
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

// TransformMCPServers rewrites `type:"http"` to `type:"remote"` for
// OpenCode's remote transport.
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

// AiderPlugin implements Aider's conventions managed block. Aider has
// no native MCP target so the plugin only reports the aider.conf.yml
// path under ExtraStatusPaths.
type AiderPlugin struct{}

// ExtendCapabilities adds ArtifactRules and ArtifactCommands so
// `agents` reflects that Aider only emits managed blocks.
func (AiderPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactRules, ArtifactCommands)
	return caps
}

// ExtraOperations is a no-op — AiderAdapter.Plan emits the
// conventions block.
func (AiderPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns the Aider config path.
func (AiderPlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	return []string{filepath.Join(ctx.Home, ".aider.conf.yml")}
}

// TransformMCPServers returns the manifest unchanged. Aider has no
// MCP target.
func (AiderPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return manifest, nil
}

// MiniMaxPlugin implements mmx-cli's default model + region presets.
// mmx-cli reads a single JSON config file at ~/.mmx/config.json; no
// skills/agents/MCP fan-out.
type MiniMaxPlugin struct {
	ConfigPath string
}

// GetConfigPath returns the mmx config path the adapter writes to.
// Renamed from ConfigPath so it does not collide with the field name
// on MiniMaxAdapter.ConfigPath.
func (p MiniMaxPlugin) GetConfigPath() string { return p.ConfigPath }

// ExtendCapabilities adds ArtifactSettings.
func (MiniMaxPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactSettings)
	return caps
}

// ExtraOperations is a no-op — MiniMaxAdapter.Plan emits the
// MergeJSON itself.
func (MiniMaxPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns the mmx config path.
func (p MiniMaxPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string {
	if p.ConfigPath == "" {
		return nil
	}
	return []string{p.ConfigPath}
}

// TransformMCPServers returns the manifest unchanged. mmx-cli does
// not consume the shared MCP servers file.
func (MiniMaxPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	return manifest, nil
}

// QwenPlugin / GeminiPlugin / ClinePlugin are minimal per-provider
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
	transformed, err := transformMCPServersForAdapter("qwen", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("qwen transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// GeminiPlugin is the Gemini-CLI counterpart of QwenPlugin: same
// httpUrl rewrite, no hooks at root. The shared mcpServers go into
// the same settings.json that holds general.defaultApprovalMode.
type GeminiPlugin struct{}

// ExtendCapabilities adds ArtifactMCP for the shared mcpServers path.
func (GeminiPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations returns no extras.
func (GeminiPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (GeminiPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers drops `type` and renames `url` to `httpUrl`.
func (GeminiPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	transformed, err := transformMCPServersForAdapter("gemini", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("gemini transform: %w", err)
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
	transformed, err := transformMCPServersForAdapter("cline", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("cline transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// QoderPlugin powers the Qoder CLI adapter. Qoder CLI stores MCP
// servers and the full-bypass permission mode in ~/.qoder/settings.json
// using a Claude-like schema, so the MCP servers keep the shared
// {type:"http",url} shape and only the per-provider settings profile
// (general.defaultPermissionMode=bypass_permissions) is layered on top.
type QoderPlugin struct{}

// ExtendCapabilities adds ArtifactMCP for the shared mcpServers path
// under ~/.qoder/settings.json.
func (QoderPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

// ExtraOperations returns no extras; the template method handles the
// file fan-out and the settings profile.
func (QoderPlugin) ExtraOperations(_ Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return nil, nil
}

// ExtraStatusPaths returns no extras.
func (QoderPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string { return nil }

// TransformMCPServers keeps the shared shape. Qoder CLI accepts the
// canonical {type:"http",url} entry for HTTP servers and
// command/args for stdio servers, matching the shared preset.
func (QoderPlugin) TransformMCPServers(manifest MCPManifest) (MCPManifest, error) {
	transformed, err := transformMCPServersForAdapter("qoder", manifest)
	if err != nil {
		return MCPManifest{}, fmt.Errorf("qoder transform: %w", err)
	}
	return MCPManifest{MCPServers: transformed}, nil
}

// compile-time interface checks. Every concrete plugin must satisfy
// AdapterPlugin so the BaseAdapter constructor can wire it directly.
var (
	_ AdapterPlugin = ClaudePlugin{}
	_ AdapterPlugin = OpenCodePlugin{}
	_ AdapterPlugin = CodexPlugin{}
	_ AdapterPlugin = AiderPlugin{}
	_ AdapterPlugin = MiniMaxPlugin{}
	_ AdapterPlugin = QwenPlugin{}
	_ AdapterPlugin = GeminiPlugin{}
	_ AdapterPlugin = ClinePlugin{}
	_ AdapterPlugin = QoderPlugin{}
	_ AdapterPlugin = NoopPlugin{}
)
