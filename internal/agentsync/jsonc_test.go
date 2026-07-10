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

func TestPortalTogglesRoundTrip(t *testing.T) {
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
	if toggles.IsSkillEnabled("commit") != true {
		t.Fatal("commit should be enabled")
	}
	if toggles.IsSkillEnabled("spawn-kimi") != false {
		t.Fatalf("spawn-kimi should be disabled: %#v", toggles.DisabledSkills)
	}
	if toggles.IsProviderEnabled("claude") != true {
		t.Fatal("claude should be enabled")
	}
	if toggles.IsProviderEnabled("gemini") != false {
		t.Fatalf("gemini should be disabled: %#v", toggles.DisabledProviders)
	}
}
