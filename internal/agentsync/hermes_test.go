package agentsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTransformHermesMCPServer(t *testing.T) {
	t.Parallel()
	http := transformHermesMCPServer(map[string]any{
		"type": "http",
		"url":  "https://mcp.context7.com/mcp",
		"headers": map[string]any{
			"Authorization": "Bearer x",
		},
	})
	if http["url"] != "https://mcp.context7.com/mcp" {
		t.Fatalf("url = %v", http["url"])
	}
	if _, ok := http["type"]; ok {
		t.Fatalf("type should be dropped: %v", http)
	}
	if http["headers"] == nil {
		t.Fatalf("headers missing: %v", http)
	}

	stdio := transformHermesMCPServer(map[string]any{
		"command": "npx",
		"args":    []any{"-y", "chrome-devtools-mcp@latest"},
		"env":     map[string]any{"A": "1"},
	})
	if stdio["command"] != "npx" {
		t.Fatalf("command = %v", stdio["command"])
	}
	if _, ok := stdio["type"]; ok {
		t.Fatalf("stdio should not invent type: %v", stdio)
	}
	if stdio["env"] == nil {
		t.Fatalf("env missing: %v", stdio)
	}

	// url without command → remote
	remote := transformHermesMCPServer(map[string]any{"url": "https://example.com/mcp"})
	if remote["url"] != "https://example.com/mcp" {
		t.Fatalf("remote url = %v", remote["url"])
	}
}

func TestResolveHermesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", "")
	if got := resolveHermesHome(home, ""); got != filepath.Join(home, ".hermes") {
		t.Fatalf("default = %s", got)
	}
	explicit := filepath.Join(home, "custom")
	if got := resolveHermesHome(home, explicit); got != explicit {
		t.Fatalf("explicit = %s", got)
	}
	envHome := filepath.Join(home, "from-env")
	t.Setenv("HERMES_HOME", envHome)
	if got := resolveHermesHome(home, ""); got != envHome {
		t.Fatalf("env = %s want %s", got, envHome)
	}
	// explicit wins over env
	if got := resolveHermesHome(home, explicit); got != explicit {
		t.Fatalf("explicit over env = %s", got)
	}
}

func TestHermesInitWritesConfigYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("HERMES_HOME", "")

	agentsDir := filepath.Join(home, ".agents")
	manager := Manager{Presets: os.DirFS("../..")}

	overlay := filepath.Join(t.TempDir(), "servers.json")
	body := `{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`
	if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "cfg.json")
	cfgBody, _ := json.Marshal(map[string]string{MCPEnabledPath: overlay})
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed config with user model + a user-owned MCP server.
	hermesDir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := "model:\n  default: test-model\nmcp_servers:\n  user-server:\n    command: echo\n"
	if err := os.WriteFile(filepath.Join(hermesDir, "config.yaml"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	opt := Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("hermes"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	raw := readFile(t, filepath.Join(hermesDir, "config.yaml"))
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml: %v\n%s", err, raw)
	}

	// model preserved
	model, _ := doc["model"].(map[string]any)
	if model == nil || model["default"] != "test-model" {
		t.Fatalf("model not preserved: %v\n%s", doc["model"], raw)
	}

	skills, _ := doc["skills"].(map[string]any)
	dirs := asStringSlice(skills["external_dirs"])
	wantSkills := filepath.Join(agentsDir, "skills")
	found := false
	for _, d := range dirs {
		if filepath.Clean(d) == filepath.Clean(wantSkills) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("external_dirs missing %s: %v\n%s", wantSkills, dirs, raw)
	}

	mcp := asStringKeyedMap(doc["mcp_servers"])
	if mcp["user-server"] == nil {
		t.Fatalf("user MCP lost: %s", raw)
	}
	ctx7 := asStringKeyedMap(mcp["context7"])
	if ctx7["url"] != "https://mcp.context7.com/mcp" {
		t.Fatalf("context7: %v", ctx7)
	}
	if _, ok := ctx7["type"]; ok {
		t.Fatalf("context7 type should be dropped: %v", ctx7)
	}
	chrome := asStringKeyedMap(mcp["chrome-devtools"])
	if chrome["command"] != "npx" {
		t.Fatalf("chrome: %v", chrome)
	}

	// No skills mirror under ~/.hermes/skills from adapters
	// (Hermes uses external_dirs). Shared skills still install under agentsDir.
	if _, err := os.Stat(filepath.Join(agentsDir, "skills")); err != nil {
		t.Fatalf("shared skills missing: %v", err)
	}

	stamp := readFile(t, filepath.Join(agentsDir, hermesManagedMCPStamp))
	if !strings.Contains(stamp, "context7") || !strings.Contains(stamp, "chrome-devtools") {
		t.Fatalf("stamp: %s", stamp)
	}

	// Idempotent update
	opt.Command = "update"
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update: %v", err)
	}
	raw2 := readFile(t, filepath.Join(hermesDir, "config.yaml"))
	if !strings.Contains(raw2, "user-server") || !strings.Contains(raw2, "test-model") {
		t.Fatalf("update lost user data: %s", raw2)
	}
}

func TestHermesUpdateRemovesDisabledMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("HERMES_HOME", "")

	agentsDir := filepath.Join(home, ".agents")
	manager := Manager{Presets: os.DirFS("../..")}

	overlayAll := filepath.Join(t.TempDir(), "servers-all.json")
	allBody := `{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]},"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`
	if err := os.WriteFile(overlayAll, []byte(allBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "cfg.json")
	cfgBody, _ := json.Marshal(map[string]string{MCPEnabledPath: overlayAll})
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		t.Fatal(err)
	}

	opt := Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		ConfigPath: cfgPath,
		NoRegistry: true,
		ToolFilter: ParseTools("hermes"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	cfgFile := filepath.Join(home, ".hermes", "config.yaml")
	raw := readFile(t, cfgFile)
	if !strings.Contains(raw, "chrome-devtools") || !strings.Contains(raw, "context7") {
		t.Fatalf("init mcp missing: %s", raw)
	}

	// Shrink catalog (portal disable chrome-devtools).
	overlayOne := filepath.Join(t.TempDir(), "servers-one.json")
	if err := os.WriteFile(overlayOne, []byte(`{"mcpServers":{"context7":{"type":"http","url":"https://mcp.context7.com/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also put chrome in disabled overlay (stamp + disabled cleanup).
	overlayDisabled := filepath.Join(t.TempDir(), "servers-disabled.json")
	if err := os.WriteFile(overlayDisabled, []byte(`{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgBody2, _ := json.Marshal(map[string]string{
		MCPEnabledPath:  overlayOne,
		MCPDisabledPath: overlayDisabled,
	})
	if err := os.WriteFile(cfgPath, cfgBody2, 0o644); err != nil {
		t.Fatal(err)
	}

	opt.Command = "update"
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update: %v", err)
	}
	raw = readFile(t, cfgFile)
	if strings.Contains(raw, "chrome-devtools") {
		t.Fatalf("disabled MCP still present:\n%s", raw)
	}
	if !strings.Contains(raw, "context7") {
		t.Fatalf("enabled MCP missing:\n%s", raw)
	}

	// Clear all managed MCP.
	overlayNone := filepath.Join(t.TempDir(), "servers-none.json")
	if err := os.WriteFile(overlayNone, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgBody3, _ := json.Marshal(map[string]string{MCPEnabledPath: overlayNone})
	if err := os.WriteFile(cfgPath, cfgBody3, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Apply(opt, true); err != nil {
		t.Fatalf("update empty: %v", err)
	}
	raw = readFile(t, cfgFile)
	if strings.Contains(raw, "context7") || strings.Contains(raw, "chrome-devtools") {
		t.Fatalf("expected managed MCP cleared:\n%s", raw)
	}
	// external_dirs must remain
	if !strings.Contains(raw, "external_dirs") {
		t.Fatalf("external_dirs lost:\n%s", raw)
	}
}

func TestHermesHomeEnvOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	custom := filepath.Join(home, "my-hermes")
	t.Setenv("HERMES_HOME", custom)

	agentsDir := filepath.Join(home, ".agents")
	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		NoRegistry: true,
		NoMCP:      true,
		ToolFilter: ParseTools("hermes"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}
	raw := readFile(t, filepath.Join(custom, "config.yaml"))
	if !strings.Contains(raw, "external_dirs") {
		t.Fatalf("expected config under HERMES_HOME:\n%s", raw)
	}
	// Default ~/.hermes should not be required
	if _, err := os.Stat(filepath.Join(home, ".hermes", "config.yaml")); err == nil {
		t.Fatal("should not write default ~/.hermes when HERMES_HOME set")
	}
}

func TestHermesNoMCPSkipsMCPButKeepsExternalDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("HERMES_HOME", "")

	hermesDir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := "mcp_servers:\n  keep-me:\n    command: echo\n"
	if err := os.WriteFile(filepath.Join(hermesDir, "config.yaml"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	agentsDir := filepath.Join(home, ".agents")
	manager := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  agentsDir,
		NoRegistry: true,
		NoMCP:      true,
		ToolFilter: ParseTools("hermes"),
	}
	if err := manager.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}
	raw := readFile(t, filepath.Join(hermesDir, "config.yaml"))
	if !strings.Contains(raw, "keep-me") {
		t.Fatalf("NoMCP should not strip existing MCP: %s", raw)
	}
	if !strings.Contains(raw, "external_dirs") {
		t.Fatalf("external_dirs missing: %s", raw)
	}
	if strings.Contains(raw, "context7") {
		t.Fatalf("NoMCP should not add catalog MCP: %s", raw)
	}
}

func TestHermesPluginExtendAndStatus(t *testing.T) {
	t.Setenv("HERMES_HOME", "")
	p := HermesPlugin{Home: "/tmp/h"}
	caps := p.ExtendCapabilities(AdapterSpec{}, AgentCapabilities{Tier: TierStable})
	found := false
	for _, a := range caps.Artifacts {
		if a == ArtifactMCP {
			found = true
		}
	}
	if !found {
		t.Fatalf("ArtifactMCP missing: %v", caps.Artifacts)
	}
	paths := p.ExtraStatusPaths(Context{Home: "/home"}, AdapterSpec{})
	if len(paths) != 1 || paths[0] != filepath.Join("/tmp/h", "config.yaml") {
		t.Fatalf("status paths: %v", paths)
	}
	// empty Home falls back to ~/.hermes under Context.Home
	p2 := HermesPlugin{}
	ctx := Context{Home: "/home"}
	paths2 := p2.ExtraStatusPaths(ctx, AdapterSpec{})
	if paths2[0] != filepath.Join("/home", ".hermes", "config.yaml") {
		t.Fatalf("fallback status: %v", paths2)
	}
	m, err := p.TransformMCPServers(MCPManifest{MCPServers: map[string]any{"a": 1}})
	if err != nil || m.MCPServers["a"] != 1 {
		t.Fatalf("transform passthrough: %v %v", m, err)
	}
}

func TestAppendExternalDirOnce(t *testing.T) {
	t.Parallel()
	dirs := appendExternalDirOnce(nil, "/a/skills")
	dirs = appendExternalDirOnce(dirs, "/a/skills")
	dirs = appendExternalDirOnce(dirs, "/a/skills/")
	if len(dirs) != 1 {
		t.Fatalf("dirs = %v", dirs)
	}
	dirs = appendExternalDirOnce(dirs, "/b/skills")
	if len(dirs) != 2 {
		t.Fatalf("dirs = %v", dirs)
	}
}

func TestHermesInRegistry(t *testing.T) {
	reg := NewAdapterRegistry(RegistryOptions{Home: "/home"})
	a := reg.Lookup("hermes")
	if a == nil {
		t.Fatal("hermes adapter missing")
	}
	if a.Capabilities().Tier != TierStable {
		t.Fatalf("tier = %s", a.Capabilities().Tier)
	}
	if len(a.DoctorExecutables()) != 1 || a.DoctorExecutables()[0] != "hermes" {
		t.Fatalf("executables = %v", a.DoctorExecutables())
	}
}

func TestMergeHermesConfigDescribe(t *testing.T) {
	var lines []string
	ctx := Context{Report: captureReporter{lines: &lines}}
	op := MergeHermesConfig{Dst: "/x/config.yaml"}
	op.Describe(ctx)
	if op.Path() != "/x/config.yaml" {
		t.Fatal(op.Path())
	}
	if len(lines) == 0 || !strings.Contains(lines[0], "hermes config") {
		t.Fatalf("describe: %v", lines)
	}
}

// captureReporter implements StatusReporter for unit tests.
type captureReporter struct {
	lines *[]string
}

func (c captureReporter) Line(format string, args ...any) {
	*c.lines = append(*c.lines, fmt.Sprintf(format, args...))
}
