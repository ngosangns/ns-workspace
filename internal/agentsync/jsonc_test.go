package agentsync

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripJSONC(t *testing.T) {
	src := []byte(`{
  // line comment
  "a": 1, /* block */
  "b": "http://x" // keep url
}`)
	got := StripJSONC(src)
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal stripped: %v\n%s", err, got)
	}
	if m["a"].(float64) != 1 || m["b"].(string) != "http://x" {
		t.Fatalf("unexpected map: %#v", m)
	}
}

func TestParseAndFormatMCPServersJSONC(t *testing.T) {
	src := []byte(`{
  "mcpServers": {
    "context7": {
      "type": "http",
      "url": "https://example.com"
    },
    // disabled by portal
    // "kimi": {
    //   "command": "npx",
    //   "args": ["-y", "kimi-for-claude"]
    // }
  }
}
`)
	enabled, disabled, _, err := ParseMCPServersJSONC(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := enabled["context7"]; !ok {
		t.Fatalf("expected context7 enabled, got %#v", enabled)
	}
	if _, ok := disabled["kimi"]; !ok {
		t.Fatalf("expected kimi disabled, got %#v", disabled)
	}

	out, err := FormatMCPServersJSONC(enabled, disabled, nil)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(string(out), `// "kimi"`) && !strings.Contains(string(out), `// "kimi":`) {
		// Format writes // "kimi": on one line
		if !strings.Contains(string(out), "kimi") || !strings.Contains(string(out), "//") {
			t.Fatalf("expected commented kimi in output:\n%s", out)
		}
	}
	en2, dis2, _, err := ParseMCPServersJSONC(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if _, ok := en2["context7"]; !ok {
		t.Fatal("context7 missing after round-trip")
	}
	if _, ok := dis2["kimi"]; !ok {
		t.Fatalf("kimi missing after round-trip:\n%s\ndisabled=%#v", out, dis2)
	}
}

func TestExtractMultipleCommentedMCPServers(t *testing.T) {
	// Matches the on-disk overlay shape written by the portal (no commas
	// between consecutive // property blocks).
	src := []byte(`{
  "mcpServers": {
    "context7": {
      "type": "http",
      "url": "https://example.com"
    },

    // disabled by portal (re-enable from UI)
    // "figma": {
    //   "type": "http",
    //   "url": "https://mcp.figma.com/mcp"
    // }
    // "local-figma": {
    //   "type": "http",
    //   "url": "http://127.0.0.1:3845/mcp"
    // }
  }
}
`)
	enabled, disabled, _, err := ParseMCPServersJSONC(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := enabled["context7"]; !ok {
		t.Fatalf("enabled missing context7: %v", enabled)
	}
	if _, ok := disabled["figma"]; !ok {
		t.Fatalf("disabled missing figma: %v", disabled)
	}
	if _, ok := disabled["local-figma"]; !ok {
		t.Fatalf("disabled missing local-figma: %v", disabled)
	}
}

func TestPortalDisabledRoundTrip(t *testing.T) {
	data, err := FormatPortalDisabled(
		map[string]bool{"spawn-kimi": true},
		map[string]bool{"gemini": true},
	)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if strings.Contains(string(data), "//") {
		t.Fatalf("disabled.json must be pure JSON without comments:\n%s", data)
	}
	if !strings.Contains(string(data), "spawn-kimi") || !strings.Contains(string(data), "gemini") {
		t.Fatalf("expected disabled ids in file:\n%s", data)
	}
	toggles, err := ParsePortalDisabled(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if toggles.IsSkillEnabled("commit") != true {
		t.Fatal("commit should be enabled by default")
	}
	if toggles.IsSkillEnabled("spawn-kimi") != false {
		t.Fatalf("spawn-kimi should be disabled: %#v", toggles.DisabledSkills)
	}
	if toggles.IsProviderEnabled("claude") != true {
		t.Fatal("claude should be enabled by default")
	}
	if toggles.IsProviderEnabled("gemini") != false {
		t.Fatalf("gemini should be disabled: %#v", toggles.DisabledProviders)
	}
}

func TestPortalTogglesLegacyRoundTrip(t *testing.T) {
	// Legacy JSONC format still parses for migration.
	data, err := FormatPortalToggles(
		[]string{"commit", "spawn-kimi"},
		[]string{"claude", "gemini"},
		map[string]bool{"spawn-kimi": true},
		map[string]bool{"gemini": true},
	)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(string(data), "spawn-kimi") || !strings.Contains(string(data), "//") {
		t.Fatalf("expected commented spawn-kimi:\n%s", data)
	}
	toggles, err := ParsePortalToggles(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if toggles.IsSkillEnabled("spawn-kimi") != false {
		t.Fatalf("spawn-kimi should be disabled: %#v", toggles.DisabledSkills)
	}
	if toggles.IsProviderEnabled("gemini") != false {
		t.Fatalf("gemini should be disabled: %#v", toggles.DisabledProviders)
	}
}

func TestMCPDisabledJSONRoundTrip(t *testing.T) {
	enabled := map[string]any{
		"context7": map[string]any{"type": "http", "url": "https://example.com"},
	}
	disabled := map[string]any{
		"kimi": map[string]any{"command": "npx", "args": []any{"-y", "kimi-for-claude"}},
	}
	enData, err := FormatMCPServersJSON(enabled, nil)
	if err != nil {
		t.Fatalf("format enabled: %v", err)
	}
	disData, err := FormatMCPDisabledJSON(disabled)
	if err != nil {
		t.Fatalf("format disabled: %v", err)
	}
	// Pure JSON: no comment lines (URLs may contain "//" inside strings).
	for _, line := range strings.Split(string(enData)+"\n"+string(disData), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			t.Fatalf("split files must not use // comments:\n%s", line)
		}
	}
	en2, _, _, err := ParseMCPServersJSONC(enData)
	if err != nil {
		t.Fatal(err)
	}
	dis2, err := ParseMCPDisabledJSON(disData)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := en2["context7"]; !ok {
		t.Fatal("context7 missing")
	}
	if _, ok := dis2["kimi"]; !ok {
		t.Fatal("kimi missing from disabled file")
	}
	if _, ok := en2["kimi"]; ok {
		t.Fatal("kimi must not be in enabled file")
	}
}
