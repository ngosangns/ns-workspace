package agentsync

import (
	"fmt"
	"path/filepath"
	"sort"
)

// opencodeMCPManifest rewrites the shared MCP servers into the shape
// OpenCode's `mcp.{name}` block expects. See transformOpenCodeMCPServer
// for the per-server contract (docs: https://opencode.ai/docs/mcp-servers/).
func opencodeMCPManifest(manifest MCPManifest) MCPManifest {
	out := MCPManifest{MCPServers: map[string]any{}}
	for name, value := range manifest.MCPServers {
		server, ok := value.(map[string]any)
		if !ok {
			out.MCPServers[name] = value
			continue
		}
		out.MCPServers[name] = transformOpenCodeMCPServer(server)
	}
	return out
}

// transformOpenCodeMCPServer maps one shared-shape MCP server entry into
// OpenCode's discriminated union:
//
//   - remote: {type:"remote", url, enabled, headers?}
//   - local:  {type:"local", command:[]string, enabled, environment?}
//
// Shared presets use type:"http"+url or command+args (string command).
// OpenCode requires type "remote"|"local", command as a single argv array
// (not command string + args), and enabled (schema-required in recent
// OpenCode builds for local entries). env is renamed to environment.
func transformOpenCodeMCPServer(server map[string]any) map[string]any {
	typ, _ := server["type"].(string)
	url, hasURL := server["url"].(string)
	_, hasCmdString := server["command"].(string)
	_, hasCmdArray := server["command"].([]any)

	// Preserve an explicit enabled value from the shared manifest; default to
	// true for entries that are selected for emission.
	enabled := true
	if v, ok := server["enabled"].(bool); ok {
		enabled = v
	}

	// URL-backed transports (shared type http/sse, already-remote, or
	// bare url without a local command) become type:"remote".
	if typ == "http" || typ == "sse" || typ == "remote" || (hasURL && url != "" && !hasCmdString && !hasCmdArray) {
		next := map[string]any{
			"type":    "remote",
			"enabled": enabled,
		}
		if hasURL && url != "" {
			next["url"] = url
		}
		if headers, ok := server["headers"]; ok {
			next["headers"] = headers
		}
		if oauth, ok := server["oauth"]; ok {
			next["oauth"] = oauth
		}
		return next
	}

	// Local/stdio: OpenCode wants command as argv array + type local.
	next := map[string]any{
		"type":    "local",
		"enabled": enabled,
	}
	switch cmd := server["command"].(type) {
	case []any:
		next["command"] = cmd
	case string:
		argv := []any{cmd}
		if args, ok := server["args"].([]any); ok {
			argv = append(argv, args...)
		}
		next["command"] = argv
	default:
		// Preserve unexpected command shapes so misconfig is visible.
		if v, ok := server["command"]; ok {
			next["command"] = v
		}
	}
	if env, ok := server["environment"]; ok {
		next["environment"] = env
	} else if env, ok := server["env"]; ok {
		next["environment"] = env
	}
	if cwd, ok := server["cwd"]; ok {
		next["cwd"] = cwd
	}
	return next
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
//   - opencode: remote = type "remote" + url + enabled; local/stdio =
//     type "local" + command argv array + enabled (command string+args
//     from the shared preset are folded into command[]). env → environment.
//     See transformOpenCodeMCPServer / https://opencode.ai/docs/mcp-servers/.
//   - qwen: HTTP servers use the field `httpUrl` (not `url` + `type`).
//     Strip the `type` key and rename `url` to `httpUrl`. SSE servers
//     keep `url`; stdio servers keep `command`+`args`.
//   - antigravity: remote HTTP/SSE/websocket use `serverUrl` (legacy
//     `url`/`httpUrl` are not supported). Drop `type`. Stdio keeps
//     `command`+`args`+`env`. Written to ~/.gemini/config/mcp_config.json
//     (not settings.json). See https://antigravity.google/docs/mcp.
//   - cline: HTTP servers use `url` (no `type` field); Cline docs do not
//     document a `type` discriminator, so the field is stripped. We
//     also set `trust: true` so Cline auto-approves MCP tool calls.
//   - kimi/kiro: keep the shared shape (uses `mcpServers` with `url`/
//     `command` per their docs).
//   - grok: does not use this JSON transform path. MCP is rendered as a
//     TOML managed block in ~/.grok/config.toml via grokMCPBlock
//     (GrokPlugin.ExtraOperations); do not set MCPPath for grok.
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
			out[name] = transformOpenCodeMCPServer(next)
			continue
		case "qwen":
			// Qwen requires HTTP servers to use the `httpUrl` field
			// (streamable HTTP transport) and does not document a `type`
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
		case "antigravity":
			// Antigravity CLI requires remote servers to use `serverUrl`.
			// Legacy `url` / `httpUrl` are rejected. Drop `type`; stdio
			// keeps command/args/env.
			// https://antigravity.google/docs/mcp
			// https://antigravity.google/docs/cli/gcli-migration
			typ, _ := next["type"].(string)
			delete(next, "type")
			if typ == "http" || typ == "sse" || typ == "remote" {
				if url, ok := next["url"].(string); ok && url != "" {
					next["serverUrl"] = url
					delete(next, "url")
				}
				if httpURL, ok := next["httpUrl"].(string); ok && httpURL != "" {
					next["serverUrl"] = httpURL
					delete(next, "httpUrl")
				}
			} else if url, ok := next["url"].(string); ok && url != "" {
				// Bare remote URL without type → still migrate to serverUrl.
				if _, hasCmd := next["command"]; !hasCmd {
					next["serverUrl"] = url
					delete(next, "url")
				}
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
		case "zcode":
			// ZCode does not yet ship a user-level MCP config file, but
			// when it does the shared {type:"http",url} / {command,args}
			// shape is what its plugin-cache .mcp.json already uses.
			// Pass the manifest through verbatim so the ZCode adapter is
			// ready the day a ~/.zcode/mcp.json target ships.
		case "kiro", "kiro-cli":
			// Kiro IDE/CLI honors per-server `disabled` (see
			// https://kiro.dev/docs/mcp/configuration/). When the user
			// toggles a server off in the Kiro panel it writes
			// `"disabled": true` into ~/.kiro/settings/mcp.json. Portal
			// enablement is separate (servers.disabled.json); any server
			// still present in the enabled catalog must load in Kiro, so
			// force disabled=false on every managed entry during sync.
			next["disabled"] = false
		default:
			// claude, kimi and other adapters: keep the shared shape.
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
// Returns "" when the catalog is empty so AppendManagedBlock can drop a
// previously written managed block (portal disable-all).
func codexMCPBlock(manifest MCPManifest) string {
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
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

// grokMCPBlock renders the TOML managed block that Grok Build expects
// in `~/.grok/config.toml` under [mcp_servers.<name>]. Shared JSON
// fields map as:
//
//   - HTTP/SSE: url (+ optional headers); type is dropped
//   - stdio: command, args, env
//
// Servers are sorted by name for stable diffs. Bare section keys are
// used when the name is a safe TOML identifier; otherwise the name is
// quoted.
func grokMCPBlock(manifest MCPManifest) string {
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := ""
	for _, name := range names {
		raw, ok := grokMCPLookup(manifest, name)
		if !ok {
			continue
		}
		server, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out += grokMCPSectionHeader(name)
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
		if headers, ok := server["headers"].(map[string]any); ok && len(headers) > 0 {
			out += "headers = { "
			// Sort header keys for stable output.
			hkeys := make([]string, 0, len(headers))
			for k := range headers {
				hkeys = append(hkeys, k)
			}
			sort.Strings(hkeys)
			for i, k := range hkeys {
				if i > 0 {
					out += ", "
				}
				out += fmt.Sprintf("%q = %q", k, fmt.Sprintf("%v", headers[k]))
			}
			out += " }\n"
		}
		if env, ok := server["env"].(map[string]any); ok && len(env) > 0 {
			out += "env = { "
			ekeys := make([]string, 0, len(env))
			for k := range env {
				ekeys = append(ekeys, k)
			}
			sort.Strings(ekeys)
			for i, k := range ekeys {
				if i > 0 {
					out += ", "
				}
				out += fmt.Sprintf("%q = %q", k, fmt.Sprintf("%v", env[k]))
			}
			out += " }\n"
		}
		out += "\n"
	}
	return out
}

// grokMCPSectionHeader returns a [mcp_servers.<name>] table header.
// Safe bare keys match Grok CLI server-name rules (letters, digits,
// hyphen, underscore); anything else is quoted.
func grokMCPSectionHeader(name string) string {
	if grokMCPBareKeyOK(name) {
		return fmt.Sprintf("[mcp_servers.%s]\n", name)
	}
	return fmt.Sprintf("[mcp_servers.%q]\n", name)
}

// grokMCPBareKeyOK reports whether name can be used as an unquoted
// TOML dotted key segment for [mcp_servers.<name>].
func grokMCPBareKeyOK(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return true
}

// grokMCPLookup is the same defensive lookup seam as codexMCPLookup,
// used by grokMCPBlock so tests can force a missing key mid-iteration.
var grokMCPLookup = func(manifest MCPManifest, name string) (any, bool) {
	v, ok := manifest.MCPServers[name]
	return v, ok
}
