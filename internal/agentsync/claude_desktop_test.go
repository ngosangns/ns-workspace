package agentsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestClaudeDesktopConfigDir covers the per-OS location of
// claude_desktop_config.json without having to run on that OS.
func TestClaudeDesktopConfigDir(t *testing.T) {
	orig := registryGOOS
	t.Cleanup(func() { registryGOOS = orig })

	registryGOOS = "darwin"
	if got, want := claudeDesktopConfigDir("/home", "/home/.config"), filepath.Join("/home", "Library", "Application Support", "Claude"); got != want {
		t.Fatalf("darwin dir = %q, want %q", got, want)
	}

	registryGOOS = "linux"
	if got, want := claudeDesktopConfigDir("/home", "/home/.config"), filepath.Join("/home", ".config", "Claude"); got != want {
		t.Fatalf("linux dir = %q, want %q", got, want)
	}

	registryGOOS = "windows"
	t.Setenv("APPDATA", filepath.Join("C:", "Users", "u", "AppData", "Roaming"))
	if got, want := claudeDesktopConfigDir("/home", "/home/.config"), filepath.Join("C:", "Users", "u", "AppData", "Roaming", "Claude"); got != want {
		t.Fatalf("windows dir = %q, want %q", got, want)
	}
	t.Setenv("APPDATA", "")
	if got, want := claudeDesktopConfigDir("/home", "/home/.config"), filepath.Join("/home", "AppData", "Roaming", "Claude"); got != want {
		t.Fatalf("windows fallback dir = %q, want %q", got, want)
	}
}

// TestClaudeDesktopRegistered asserts the adapter is in the default
// catalog, reachable by alias, and targets claude_desktop_config.json.
func TestClaudeDesktopRegistered(t *testing.T) {
	desktop := t.TempDir()
	reg := NewAdapterRegistry(RegistryOptions{Home: t.TempDir(), ClaudeDesktopDir: desktop})

	adapter := reg.Lookup("claude-desktop")
	if adapter == nil {
		t.Fatalf("claude-desktop not registered")
	}
	if reg.Lookup("claude-app") == nil {
		t.Fatalf("claude-app alias not resolvable")
	}
	// The alias must not shadow the Claude Code adapter.
	if claude := reg.Lookup("claude"); claude == nil || claude.Name() != "claude" {
		t.Fatalf("claude adapter shadowed by claude-desktop")
	}

	caps := adapter.Capabilities()
	if !containsArtifact(caps.Artifacts, ArtifactMCP) {
		t.Fatalf("expected MCP artifact, got %v", caps.Artifacts)
	}
	for _, kind := range []ArtifactKind{ArtifactInstructions, ArtifactSkills, ArtifactSubagents} {
		if containsArtifact(caps.Artifacts, kind) {
			t.Fatalf("claude-desktop should not claim %s: %v", kind, caps.Artifacts)
		}
	}
	if len(adapter.DoctorExecutables()) != 0 {
		t.Fatalf("GUI app has no CLI binary to probe: %v", adapter.DoctorExecutables())
	}

	want := filepath.Join(desktop, "claude_desktop_config.json")
	if paths := nativePaths(AdapterSpec{Targets: AdapterTargets{MCPPath: want}}, ""); len(paths) != 1 || paths[0] != want {
		t.Fatalf("native paths = %v, want [%s]", paths, want)
	}
}

func containsArtifact(list []ArtifactKind, want ArtifactKind) bool {
	for _, kind := range list {
		if kind == want {
			return true
		}
	}
	return false
}

// TestClaudeDesktopTransformMCPServers pins the stdio-only rewrite:
// remote servers go through the mcp-remote bridge, stdio servers keep
// their argv and lose the `type` discriminator Claude Desktop ignores.
func TestClaudeDesktopTransformMCPServers(t *testing.T) {
	manifest := MCPManifest{MCPServers: map[string]any{
		"remote": map[string]any{
			"type": "http",
			"url":  "https://mcp.example.com/mcp",
			"headers": map[string]any{
				"X-Trace":       "on",
				"Authorization": "Bearer token",
			},
			"env": map[string]any{"NODE_ENV": "production"},
		},
		"sse": map[string]any{
			"type": "sse",
			"url":  "https://sse.example.com/sse",
		},
		"local": map[string]any{
			"type":    "stdio",
			"command": "npx",
			"args":    []any{"-y", "some-mcp"},
			"env":     map[string]any{"TOKEN": "x"},
		},
		"scalar": "not-an-object",
	}}

	out, err := ClaudeDesktopPlugin{}.TransformMCPServers(manifest)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	remote := out.MCPServers["remote"].(map[string]any)
	if remote["command"] != "npx" {
		t.Fatalf("remote command = %v, want npx", remote["command"])
	}
	wantArgs := []any{
		"-y", "mcp-remote", "https://mcp.example.com/mcp",
		"--header", "Authorization: Bearer token",
		"--header", "X-Trace: on",
	}
	if !reflect.DeepEqual(remote["args"], wantArgs) {
		t.Fatalf("remote args = %v, want %v", remote["args"], wantArgs)
	}
	if _, ok := remote["url"]; ok {
		t.Fatalf("remote entry must not keep url: %v", remote)
	}
	if _, ok := remote["type"]; ok {
		t.Fatalf("remote entry must not keep type: %v", remote)
	}
	if env, ok := remote["env"].(map[string]any); !ok || env["NODE_ENV"] != "production" {
		t.Fatalf("remote env dropped: %v", remote["env"])
	}

	sse := out.MCPServers["sse"].(map[string]any)
	if !reflect.DeepEqual(sse["args"], []any{"-y", "mcp-remote", "https://sse.example.com/sse"}) {
		t.Fatalf("sse args = %v", sse["args"])
	}

	local := out.MCPServers["local"].(map[string]any)
	if local["command"] != "npx" || !reflect.DeepEqual(local["args"], []any{"-y", "some-mcp"}) {
		t.Fatalf("stdio entry rewritten: %v", local)
	}
	if _, ok := local["type"]; ok {
		t.Fatalf("stdio entry must not keep type: %v", local)
	}

	if out.MCPServers["scalar"] != "not-an-object" {
		t.Fatalf("non-object entry mangled: %v", out.MCPServers["scalar"])
	}
}

// TestClaudeDesktopMergePreservesExistingKeys applies the planned merge
// against a real claude_desktop_config.json that already holds unrelated
// preferences, which must survive the sync.
func TestClaudeDesktopMergePreservesExistingKeys(t *testing.T) {
	ctx, _ := newTestContext(t)
	desktop := t.TempDir()
	configPath := filepath.Join(desktop, "claude_desktop_config.json")
	existing := `{"preferences":{"sidebarMode":"epitaxy"},"mcpServers":{"user-added":{"command":"echo"}}}`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	reg := NewAdapterRegistry(RegistryOptions{Home: ctx.Home, XDGConfigHome: ctx.XDGConfigHome, ClaudeDesktopDir: desktop})
	adapter := reg.Lookup("claude-desktop")
	ops, err := adapter.Plan(ctx, false)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected a single MCP merge op, got %d: %v", len(ops), ops)
	}
	merge, ok := ops[0].(MergeJSON)
	if !ok || merge.Dst != configPath {
		t.Fatalf("unexpected op %#v", ops[0])
	}
	if err := merge.Apply(ctx); err != nil {
		t.Fatalf("apply: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	prefs, ok := got["preferences"].(map[string]any)
	if !ok || prefs["sidebarMode"] != "epitaxy" {
		t.Fatalf("unrelated preferences lost: %v", got["preferences"])
	}
	servers, ok := got["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing: %v", got)
	}
	if _, ok := servers["user-added"]; !ok {
		t.Fatalf("init merge dropped user server: %v", servers)
	}
	// Shared catalog entries land as stdio commands only.
	if len(servers) < 2 {
		t.Fatalf("shared servers not merged: %v", servers)
	}
	for name, raw := range servers {
		server, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, hasURL := server["url"]; hasURL {
			t.Fatalf("server %q kept a url Claude Desktop cannot read: %v", name, server)
		}
		if _, hasCmd := server["command"]; !hasCmd {
			t.Fatalf("server %q has no stdio command: %v", name, server)
		}
	}
}

// TestClaudeDesktopNoMCPSkips asserts --no-mcp leaves the desktop config
// untouched (the adapter has nothing else to write).
func TestClaudeDesktopNoMCPSkips(t *testing.T) {
	ctx, _ := newTestContext(t)
	ctx.Options.NoMCP = true
	ctx.NoMCP = true
	desktop := t.TempDir()
	reg := NewAdapterRegistry(RegistryOptions{Home: ctx.Home, XDGConfigHome: ctx.XDGConfigHome, ClaudeDesktopDir: desktop})
	ops, err := reg.Lookup("claude-desktop").Plan(ctx, false)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("expected no ops with --no-mcp, got %v", ops)
	}
}
