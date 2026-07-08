package agentsync

import (
	"path/filepath"
)

// SimpleAdapter covers providers that just file-link the shared
// content into a native directory (grok, kimi, kiro, manual /
// experimental). It uses BaseAdapter's template method without a
// plugin override.
type SimpleAdapter struct {
	BaseAdapter
}

// ProfileAdapter covers providers that have a settings profile in
// presets/adapters/<id>.json (claude, qwen, gemini, cline). Its only
// customization beyond the template method is how MCP servers are
// transformed before being merged into the native config.
type ProfileAdapter struct {
	BaseAdapter
}

// CodexAdapter writes the legacy MCP TOML managed block in addition to
// the shared file fan-out.
type CodexAdapter struct {
	BaseAdapter
}

// ClaudeAdapter appends a generated claude mcp add-json helper script.
type ClaudeAdapter struct {
	BaseAdapter
}

// OpenCodeAdapter merges the shared MCP manifest into the canonical
// opencode.json config file.
type OpenCodeAdapter struct {
	BaseAdapter
	ConfigPath string
}

// Plan overrides BaseAdapter.Plan so OpenCode does not file-link
// ~/.config/opencode/AGENTS.md separately — instead it merges
// everything into a single opencode.json file.
func (o *OpenCodeAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	replace := update || ctx.Force
	ops := []Operation{}
	configValues := map[string]any{}
	if !ctx.NoMCP {
		manifest, err := readMCPManifest(ctx)
		if err != nil {
			return nil, err
		}
		// Use BaseAdapter.transformMCP so the plugin transform runs.
		transformed, err := o.transformMCP(manifest)
		if err != nil {
			return nil, err
		}
		configValues["mcp"] = transformed
	}
	presetValues, err := readOpenCodeConfigValues(ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range presetValues {
		configValues[key] = value
	}
	if len(configValues) > 0 {
		ops = append(ops, MergeJSON{
			Dst:     o.ConfigPath,
			KeyPath: []string{},
			Values:  configValues,
			Replace: replace && !ctx.NoMCP,
		})
	}
	ops = append(ops, o.fileLinkOps(ctx, replace)...)
	return ops, nil
}

// Plan overrides BaseAdapter.Plan for Claude: add the generated
// mcp.commands.sh helper script.
func (c *ClaudeAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	ops, err := c.BaseAdapter.Plan(ctx, update)
	if err != nil {
		return nil, err
	}
	if !ctx.NoMCP {
		script, err := mcpCommandScript(ctx, "claude", func(name string, server string) string {
			return "claude mcp add-json " + shellWord(name) + " '" + shellSingleQuotePayload(server) + "' --scope user\n"
		})
		if err != nil {
			return nil, err
		}
		ops = append(ops, WriteFile{
			Dst:     filepath.Join(ctx.Options.AgentsDir, "generated", "claude", "mcp.commands.sh"),
			Data:    []byte(script),
			Replace: update || ctx.Force,
		})
	}
	return ops, nil
}

// Plan overrides BaseAdapter.Plan for Codex: emit a TOML managed
// block to ~/.codex/config.toml and drop the default MCP merge.
func (c *CodexAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	ops, err := c.BaseAdapter.Plan(ctx, update)
	if err != nil {
		return nil, err
	}
	ops = stripMCPOps(ops)
	if ctx.NoMCP {
		return ops, nil
	}
	manifest, err := readMCPManifest(ctx)
	if err != nil {
		return nil, err
	}
	if len(manifest.MCPServers) == 0 {
		return ops, nil
	}
	ops = append(ops, AppendManagedBlock{
		Dst:     filepath.Join(ctx.Home, ".codex", "config.toml"),
		Label:   "mcp",
		Content: codexMCPBlock(manifest),
		Replace: true,
	})
	_ = update
	return ops, nil
}

// stripMCPOps drops MergeJSON operations whose Dst is the canonical
// mcpServers path. Used by Codex and OpenCode adapters which merge MCP
// into a different file.
func stripMCPOps(ops []Operation) []Operation {
	out := ops[:0]
	for _, op := range ops {
		if mj, ok := op.(MergeJSON); ok {
			base := filepath.Base(mj.Dst)
			if base == "mcp.json" || base == "cline_mcp_settings.json" {
				continue
			}
		}
		out = append(out, op)
	}
	return out
}

// decodeJSONBytes parses raw JSON bytes into out.
func decodeJSONBytes(data []byte, out *map[string]any) error {
	return decodeJSON(data, out)
}

// compile-time interface checks. Each concrete adapter must satisfy
// Adapter so the factory can return it.
var (
	_ Adapter = (*SimpleAdapter)(nil)
	_ Adapter = (*ProfileAdapter)(nil)
	_ Adapter = (*CodexAdapter)(nil)
	_ Adapter = (*ClaudeAdapter)(nil)
	_ Adapter = (*OpenCodeAdapter)(nil)
)
