package agentsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// hermesManagedMCPStamp is the relative path under AgentsDir for the
// last set of MCP server names ns-workspace wrote into Hermes config.
const hermesManagedMCPStamp = "generated/hermes/managed-mcp.json"

// MergeHermesConfig merges managed fields into Hermes Agent config.yaml:
//
//   - skills.external_dirs: ensure ExternalSkillsDir is listed (idempotent)
//   - mcp_servers: set Enabled MCP entries; delete CleanupNames not enabled
//
// User-owned keys (model, memory, user-only MCP servers, other external_dirs)
// are preserved. YAML comments may be lost on rewrite (yaml.v3 limitation).
type MergeHermesConfig struct {
	Dst               string
	ExternalSkillsDir string
	MCPServers        map[string]any
	EnabledNames      []string
	CleanupNames      []string
	StampPath         string
	Replace           bool
}

// Apply merges managed Hermes config and updates the managed-MCP stamp.
func (op MergeHermesConfig) Apply(ctx Context) error {
	doc := map[string]any{}
	if data, err := os.ReadFile(op.Dst); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := yaml.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("invalid YAML in %s: %w", op.Dst, err)
			}
			if doc == nil {
				doc = map[string]any{}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	mergeHermesConfigDoc(doc, op.ExternalSkillsDir, op.MCPServers, op.EnabledNames, op.CleanupNames)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return fmt.Errorf("encode hermes config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode hermes config: %w", err)
	}
	if err := writeFileManaged(ctx, op.Dst, buf.Bytes(), op.Replace); err != nil {
		return err
	}
	if op.StampPath != "" {
		if err := writeHermesManagedStamp(ctx, op.StampPath, op.EnabledNames); err != nil {
			return err
		}
	}
	return nil
}

// Describe reports the hermes config path.
func (op MergeHermesConfig) Describe(ctx Context) {
	ctx.Report.Line("hermes config: %s", op.Dst)
}

// Path returns the destination config path.
func (op MergeHermesConfig) Path() string { return op.Dst }

// mergeHermesConfigDoc mutates doc in place.
// When servers is nil and cleanup is empty, mcp_servers is left unchanged
// (NoMCP path). Empty servers map with cleanup still removes managed keys.
func mergeHermesConfigDoc(doc map[string]any, externalSkills string, servers map[string]any, enabled, cleanup []string) {
	if doc == nil {
		return
	}
	externalSkills = strings.TrimSpace(externalSkills)
	if externalSkills != "" {
		skills := asStringKeyedMap(doc["skills"])
		dirs := asStringSlice(skills["external_dirs"])
		dirs = appendExternalDirOnce(dirs, externalSkills)
		skills["external_dirs"] = stringSliceToAny(dirs)
		doc["skills"] = skills
	}

	if servers == nil && len(cleanup) == 0 {
		return
	}

	mcp := asStringKeyedMap(doc["mcp_servers"])
	enabledSet := map[string]bool{}
	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name != "" {
			enabledSet[name] = true
		}
	}
	for _, name := range cleanup {
		name = strings.TrimSpace(name)
		if name == "" || enabledSet[name] {
			continue
		}
		delete(mcp, name)
	}
	if servers != nil {
		for name, val := range servers {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			mcp[name] = val
		}
	}
	doc["mcp_servers"] = mcp
}

func asStringKeyedMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		if m == nil {
			return map[string]any{}
		}
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out
	default:
		return map[string]any{}
	}
}

func asStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		out := make([]string, 0, len(s))
		for _, item := range s {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			str, ok := item.(string)
			if !ok {
				continue
			}
			str = strings.TrimSpace(str)
			if str != "" {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func stringSliceToAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

// appendExternalDirOnce adds path if no existing entry cleans to the same path.
func appendExternalDirOnce(dirs []string, path string) []string {
	want := filepath.Clean(path)
	for _, d := range dirs {
		if filepath.Clean(d) == want {
			return dirs
		}
	}
	return append(dirs, path)
}

type hermesManagedStamp struct {
	Servers []string `json:"servers"`
}

func loadHermesManagedStamp(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var stamp hermesManagedStamp
	if err := json.Unmarshal(data, &stamp); err != nil {
		return nil
	}
	out := make([]string, 0, len(stamp.Servers))
	for _, s := range stamp.Servers {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func writeHermesManagedStamp(ctx Context, path string, names []string) error {
	clean := uniqueStrings(names)
	payload, err := json.MarshalIndent(hermesManagedStamp{Servers: clean}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return writeFileManaged(ctx, path, payload, true)
}

// resolveHermesHome returns explicit, else HERMES_HOME env, else ~/.hermes.
func resolveHermesHome(home, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return filepath.Clean(ExpandPath(explicit))
	}
	if v := strings.TrimSpace(os.Getenv("HERMES_HOME")); v != "" {
		return filepath.Clean(ExpandPath(v))
	}
	return filepath.Join(home, ".hermes")
}

// transformHermesMCPServer maps one shared-shape MCP entry into Hermes
// config.yaml mcp_servers shape:
//
//   - remote: url (+ headers/auth/oauth); drop type
//   - stdio: command, args, env, cwd
func transformHermesMCPServer(server map[string]any) map[string]any {
	if server == nil {
		return map[string]any{}
	}
	url := firstStringValue(server, "url", "httpUrl", "serverUrl")
	cmd, hasCmd := server["command"].(string)
	typ, _ := server["type"].(string)

	// Remote HTTP/SSE (or bare url without a local command).
	if url != "" && (typ == "http" || typ == "sse" || typ == "remote" || !hasCmd || strings.TrimSpace(cmd) == "") {
		next := map[string]any{"url": url}
		if h, ok := server["headers"]; ok {
			next["headers"] = h
		}
		if a, ok := server["auth"]; ok {
			next["auth"] = a
		}
		if o, ok := server["oauth"]; ok {
			next["oauth"] = o
		}
		return next
	}

	next := map[string]any{}
	if hasCmd && strings.TrimSpace(cmd) != "" {
		next["command"] = cmd
	}
	if args, ok := server["args"]; ok {
		next["args"] = args
	}
	if env, ok := server["env"]; ok {
		next["env"] = env
	}
	if cwd, ok := server["cwd"]; ok {
		next["cwd"] = cwd
	}
	return next
}

// hermesMCPServers transforms a filtered manifest into Hermes-shaped maps.
func hermesMCPServers(manifest MCPManifest) (map[string]any, []string) {
	servers := make(map[string]any, len(manifest.MCPServers))
	names := make([]string, 0, len(manifest.MCPServers))
	for name, raw := range manifest.MCPServers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
		if m, ok := raw.(map[string]any); ok {
			servers[name] = transformHermesMCPServer(m)
		} else {
			servers[name] = raw
		}
	}
	sort.Strings(names)
	return servers, names
}
