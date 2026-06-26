package agentsync

import (
	"fmt"
	"path/filepath"
	"sort"
)

// opencodeMCPManifest rewrites the shared MCP servers into the shape
// OpenCode's `mcp.{name}` block expects: `type: "http"` becomes
// `type: "remote"`. Kept as a thin wrapper so the opencode plugin can
// keep a dedicated call site; the actual per-provider logic now lives
// in TransformMCPServers on the plugin interface.
func opencodeMCPManifest(manifest MCPManifest) MCPManifest {
	out := MCPManifest{MCPServers: map[string]any{}}
	for name, value := range manifest.MCPServers {
		server, ok := value.(map[string]any)
		if !ok {
			out.MCPServers[name] = value
			continue
		}
		next := map[string]any{}
		for key, serverValue := range server {
			next[key] = serverValue
		}
		// OpenCode uses "remote" for URL-backed MCP servers; shared
		// presets keep "http" for other agent config formats.
		if typ, _ := next["type"].(string); typ == "http" {
			next["type"] = "remote"
		}
		out.MCPServers[name] = next
	}
	return out
}

// transformMCPServersForAdapterImpl is the seam used by adapter_plugins
// to delegate to transformMCPServersForAdapter. Tests can override the
// variable to inject custom errors and cover error branches in plugin
// TransformMCPServers methods that always succeed in production.
var transformMCPServersForAdapterImpl = transformMCPServersForAdapter

// transformMCPServersForAdapter rewrites each MCP server entry from the
// canonical shared shape ({"type": "http", "url": "..."}) into the shape
// that the target provider's native config expects.
//
// Per-provider rules (verified against each tool's official docs):
//
//   - claude: keeps the shared shape verbatim. Claude Code's mcpServers
//     accepts `type: "http"` + `url` for HTTP servers, and `command`/`args`
//     for stdio servers.
//   - opencode: rewrites `type: "http"` to `type: "remote"` (OpenCode's
//     remote transport uses the literal "remote").
//   - qwen: HTTP servers use the field `httpUrl` (not `url` + `type`).
//     Strip the `type` key and rename `url` to `httpUrl`. SSE servers
//     keep `url`; stdio servers keep `command`+`args`.
//   - gemini: same HTTP shape as Qwen (`httpUrl` + no `type`). Gemini's
//     settings.json groups everything under `mcpServers` with the same
//     field semantics as Qwen.
//   - cline: HTTP servers use `url` (no `type` field); Cline docs do not
//     document a `type` discriminator, so the field is stripped. We
//     also set `trust: true` so Cline auto-approves MCP tool calls.
//   - kimi/kiro: keep the shared shape (uses `mcpServers` with `url`/
//     `command` per their docs).
//
// Adapters without a specific override fall back to the shared shape.
//
// This function is the single dispatch point used by the inline merge
// path in specAdapter.Plan and by buildAdapterSettings. Concrete
// AdapterPlugin implementations also expose the same logic via
// TransformMCPServers; the per-plugin method is preferred when an
// adapter has its own plugin instance.
func transformMCPServersForAdapter(adapterID string, manifest MCPManifest) (map[string]any, error) {
	out := map[string]any{}
	for name, value := range manifest.MCPServers {
		server, ok := value.(map[string]any)
		if !ok {
			out[name] = value
			continue
		}
		next := map[string]any{}
		for key, serverValue := range server {
			next[key] = serverValue
		}
		switch adapterID {
		case "opencode":
			// OpenCode uses "remote" for URL-backed MCP servers; shared
			// presets keep "http" for other agent config formats.
			if typ, _ := next["type"].(string); typ == "http" {
				next["type"] = "remote"
			}
		case "qwen", "gemini":
			// Qwen and Gemini both require HTTP servers to use the `httpUrl`
			// field (streamable HTTP transport) and do not document a `type`
			// discriminator. Drop `type` and rename `url` to `httpUrl`.
			typ, _ := next["type"].(string)
			if typ == "http" {
				delete(next, "type")
				if url, ok := next["url"].(string); ok {
					next["httpUrl"] = url
					delete(next, "url")
				}
			} else {
				// SSE and stdio servers: leave `url`/`command` intact. Drop
				// `type` if present so the native config doesn't see an
				// undocumented key.
				delete(next, "type")
			}
		case "cline":
			// Cline docs document `url` (HTTP/SSE) or `command`+`args`
			// (stdio). The shared `type` field is not part of Cline's
			// schema and is stripped to keep the native file clean.
			// We set `trust: true` so Cline auto-approves MCP tool calls
			// without per-tool confirmation prompts (equivalent to
			// toggling "Always allow" in the IDE). The YOLO mode flag
			// itself is managed by Cline in `~/.cline/data/settings/
			// global-settings.json` and cannot be set from
			// cline_mcp_settings.json, so this is the closest equivalent
			// to a full bypass we can ship from the MCP preset path.
			delete(next, "type")
			next["trust"] = true
		default:
			// claude, kimi, kiro and other adapters: keep the shared shape.
		}
		out[name] = next
	}
	return out, nil
}

// mcpCommandScript renders a `claude mcp add-json ...` helper script
// that the user can run to register MCP servers against Claude Code's
// user-scope store. lineBuilder formats one server entry.
func mcpCommandScript(ctx Context, agentID string, lineBuilder func(name string, server string) string) (string, error) {
	manifest, err := readMCPManifest(ctx)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	script := "#!/usr/bin/env sh\nset -eu\n\n"
	script += "# Generated by ns-workspace: register MCP servers for " + agentID + "\n"
	script += "# Usage: sh " + filepath.Join(ctx.Options.AgentsDir, "generated", agentID, "mcp.commands.sh") + "\n\n"
	for _, name := range names {
		raw, ok := manifest.MCPServers[name]
		if !ok {
			continue
		}
		serverJSON, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		payload, err := encodeJSONInline(serverJSON)
		if err != nil {
			return "", err
		}
		script += lineBuilder(name, payload)
	}
	return script, nil
}

// codexMCPBlock renders the TOML managed block that Codex expects in
// `~/.codex/config.toml`. Servers are sorted by name for stable diffs.
func codexMCPBlock(manifest MCPManifest) string {
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := "[mcp_servers]\n"
	for _, name := range names {
		raw, ok := codexMCPLookup(manifest, name)
		if !ok {
			continue
		}
		server, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out += fmt.Sprintf("[mcp_servers.%q]\n", name)
		if cmd, ok := server["command"].(string); ok && cmd != "" {
			out += fmt.Sprintf("command = %q\n", cmd)
		}
		if args, ok := server["args"].([]any); ok {
			out += "args = ["
			for i, arg := range args {
				if i > 0 {
					out += ", "
				}
				if s, ok := arg.(string); ok {
					out += fmt.Sprintf("%q", s)
				}
			}
			out += "]\n"
		}
		if url, ok := server["url"].(string); ok && url != "" {
			out += fmt.Sprintf("url = %q\n", url)
		}
		if env, ok := server["env"].(map[string]any); ok {
			out += "env = { "
			first := true
			for k, v := range env {
				if !first {
					out += ", "
				}
				out += fmt.Sprintf("%q = %q", k, fmt.Sprintf("%v", v))
				first = false
			}
			out += " }\n"
		}
		out += "\n"
	}
	return out
}

// codexMCPLookup is a thin seam over the manifest map lookup so tests
// can simulate a key disappearing mid-iteration (the production map is
// never mutated between name collection and lookup, but the defensive
// !ok check is part of the public contract).
var codexMCPLookup = func(manifest MCPManifest, name string) (any, bool) {
	v, ok := manifest.MCPServers[name]
	return v, ok
}
