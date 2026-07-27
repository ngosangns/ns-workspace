package agentsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseProviderRules(t *testing.T) {
	data := []byte(`{
  "spawn-kimi": ["opencode", "claude"],
  "cleanup": "all",
  "fix": ["kiro-cli"],
  "empty": []
}`)
	rules, err := ParseProviderRules(data)
	if err != nil {
		t.Fatalf("ParseProviderRules: %v", err)
	}
	if !rules.Allows("missing", "opencode") {
		t.Fatal("missing key should allow all")
	}
	if !rules.Allows("cleanup", "grok") {
		t.Fatal("explicit all should allow")
	}
	if rules.Allows("empty", "grok") {
		t.Fatal("empty list should allow none")
	}
	if got := rules.ProvidersFor("empty"); got == nil || len(got) != 0 {
		t.Fatalf("ProvidersFor empty = %v, want []", got)
	}
	if !rules.Allows("spawn-kimi", "opencode") {
		t.Fatal("spawn-kimi should allow opencode")
	}
	if rules.Allows("spawn-kimi", "grok") {
		t.Fatal("spawn-kimi should deny grok")
	}
	if !rules.Allows("fix", "kiro") {
		t.Fatal("kiro-cli alias should match kiro")
	}
	if !rules.Allows("fix", "kiro-cli") {
		t.Fatal("kiro-cli should match via canonical")
	}
}

func TestFormatProviderRulesRoundtrip(t *testing.T) {
	rules := ProviderRules{
		"a": {IDs: map[string]bool{"opencode": true, "claude": true}},
		"b": {All: true},
	}
	data, err := FormatProviderRules(rules)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	got, err := ParseProviderRules(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !got.Allows("a", "opencode") || got.Allows("a", "grok") {
		t.Fatalf("roundtrip a: %+v", got["a"])
	}
	if _, ok := got["b"]; ok {
		t.Fatal("explicit all should be omitted from file")
	}
}

func TestProviderRulesSetProviders(t *testing.T) {
	r := ProviderRules{}
	r.SetProviders("x", []string{"all"})
	if _, ok := r["x"]; ok {
		t.Fatal("all should delete key")
	}
	r.SetProviders("x", []string{"OpenCode", "claude"})
	if !r.Allows("x", "opencode") || r.Allows("x", "grok") {
		t.Fatalf("set list: %+v", r["x"])
	}
	r.SetProviders("x", []string{})
	if !r["x"].IsNone() || r.Allows("x", "opencode") {
		t.Fatalf("empty slice should be none: %+v", r["x"])
	}
	r.SetProviders("x", nil)
	if _, ok := r["x"]; ok {
		t.Fatal("nil should delete (default all)")
	}
}

func TestProvidersFor(t *testing.T) {
	r := ProviderRules{
		"x": {IDs: map[string]bool{"claude": true, "opencode": true}},
		"n": {IDs: map[string]bool{}},
	}
	got := r.ProvidersFor("x")
	if len(got) != 2 || got[0] != "claude" || got[1] != "opencode" {
		t.Fatalf("ProvidersFor = %v", got)
	}
	if got := r.ProvidersFor("missing"); len(got) != 1 || got[0] != "all" {
		t.Fatalf("missing = %v", got)
	}
	if got := r.ProvidersFor("n"); got == nil || len(got) != 0 {
		t.Fatalf("none = %v, want []", got)
	}
}

func TestCanonicalAdapterID(t *testing.T) {
	cases := map[string]string{
		"OpenCode":       "opencode",
		"kiro-cli":       "kiro",
		"zcode-cli":      "zcode",
		"claude-app":     "claude-desktop",
		"claudedesktop":  "claude-desktop",
		"  claude  ":     "claude",
	}
	for in, want := range cases {
		if got := CanonicalAdapterID(in); got != want {
			t.Errorf("CanonicalAdapterID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateProviderRules(t *testing.T) {
	rules := ProviderRules{
		"x": {IDs: map[string]bool{"opencode": true, "not-a-real-provider": true}},
	}
	w := ValidateProviderRules(rules, "skills providers")
	if len(w) != 1 || !strings.Contains(w[0], "not-a-real-provider") {
		t.Fatalf("warnings = %v", w)
	}
}

func TestLinkSkillDirsAllowFilter(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	for _, name := range []string{"keep", "drop"} {
		if err := os.MkdirAll(filepath.Join(src, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, name, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		// Seed a stale mirror for drop so prune can remove it.
		if err := os.MkdirAll(filepath.Join(dst, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx := Context{Report: stdoutReporter{}, seenDirs: map[string]bool{}}
	op := LinkSkillDirs{
		SrcRoot: src,
		DstRoot: dst,
		Replace: true,
		Allow:   func(name string) bool { return name == "keep" },
	}
	if err := op.Apply(ctx); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "keep")); err != nil {
		t.Fatalf("keep missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "drop")); !os.IsNotExist(err) {
		t.Fatalf("drop should be pruned, err=%v", err)
	}
}

func TestFilterMCPManifest(t *testing.T) {
	fsys := fstest.MapFS{
		"presets/mcp/providers.json": &fstest.MapFile{Data: []byte(`{"only-opencode":["opencode"],"all-one":"all"}`)},
	}
	ctx := Context{Presets: fsys, Report: stdoutReporter{}, manifestCache: map[string]any{}}
	in := MCPManifest{MCPServers: map[string]any{
		"only-opencode": map[string]any{"url": "https://a"},
		"all-one":       map[string]any{"url": "https://b"},
		"unlisted":      map[string]any{"url": "https://c"},
	}}
	got := filterMCPManifest(ctx, "opencode", in)
	if len(got.MCPServers) != 3 {
		t.Fatalf("opencode should keep all, got %d", len(got.MCPServers))
	}
	got = filterMCPManifest(ctx, "claude", in)
	if _, ok := got.MCPServers["only-opencode"]; ok {
		t.Fatal("claude should drop only-opencode")
	}
	if len(got.MCPServers) != 2 {
		t.Fatalf("claude should keep 2, got %d: %v", len(got.MCPServers), got.MCPServers)
	}
}

func TestLoadProviderRulesMissing(t *testing.T) {
	ctx := Context{Presets: fstest.MapFS{}, Report: stdoutReporter{}, manifestCache: map[string]any{}}
	rules := LoadProviderRules(ctx, SkillsProvidersPath)
	if !rules.Allows("any", "opencode") {
		t.Fatal("missing file should allow all")
	}
}
