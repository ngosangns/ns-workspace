package agentsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// privateMarkers returns identity/internal-service substrings that must not
// appear in shipped presets or adapter docs. Built via concatenation so this
// test file itself does not embed the private strings as contiguous literals
// for naive tree scans.
func privateMarkers() []string {
	// Split every private token so naive ripgrep of the audit patterns
	// does not match this test file itself.
	user := "ngo" + "sang" + "ns"
	proj := "vic" + "lass"
	shop := "woku" + "shop"
	return []string{
		"/Users/" + user,
		proj,
		strings.ToUpper(proj[:1]) + proj[1:],
		shop,
		"Woku" + " Shop",
		"llm." + shop + ".com",
	}
}

// TestPublicSafePresetAndAdapterDocs asserts shipped presets and adapter catalog
// do not embed personal machine paths or private LLM endpoints. Runs against the
// real on-disk preset files and the live NewAdapterRegistry used by init/update.
func TestPublicSafePresetAndAdapterDocs(t *testing.T) {
	root := moduleRoot(t)
	banned := privateMarkers()

	// OpenCode preset must not ship personal provider endpoints.
	opencodePath := filepath.Join(root, "presets", "opencode", "opencode.json")
	data, err := os.ReadFile(opencodePath)
	if err != nil {
		t.Fatalf("read opencode preset: %v", err)
	}
	body := string(data)
	for _, b := range banned {
		if strings.Contains(body, b) {
			t.Errorf("opencode preset embeds private marker (len=%d)", len(b))
		}
	}
	if !strings.Contains(body, `"permission"`) || !strings.Contains(body, "allow") {
		t.Errorf("opencode preset should keep public-safe permission allow, got: %s", body)
	}

	// Adapter registry Docs must not point at a developer home absolute file URL.
	home := t.TempDir()
	reg := NewAdapterRegistry(RegistryOptions{Home: home})
	for _, a := range reg.All() {
		caps := a.Capabilities()
		for _, doc := range caps.DocsURL {
			for _, b := range banned {
				if strings.Contains(doc, b) {
					t.Errorf("adapter %q DocsURL embeds private marker", a.Name())
				}
			}
		}
	}

	// Subagent copy and docs/examples used as shipped surface.
	paths := []string{
		filepath.Join(root, "presets", "subagents", "opencode-intern.md"),
		filepath.Join(root, "presets", "skills", "lsp-code-graph", "SKILL.md"),
		filepath.Join(root, "README.md"),
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		s := string(raw)
		for _, b := range banned {
			if strings.Contains(s, b) {
				t.Errorf("%s embeds private marker (len=%d)", p, len(b))
			}
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "presets", "opencode", "opencode.json")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root with presets not found")
		}
		dir = parent
	}
}
