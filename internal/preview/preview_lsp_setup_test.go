package preview

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ngosangns/ns-workspace/internal/graphquery"
)

func TestLSPWrappersCallIntoGraphQuery(t *testing.T) {
	root := t.TempDir()

	d := GraphQueryLSPDetector{}
	detected := d.DetectedLanguages(root, "docs")
	if len(detected) != 0 {
		t.Errorf("DetectedLanguages empty project = %v, want empty", detected)
	}
	installIDs := d.DetectedInstallIDs(root, "docs")
	if len(installIDs) != 0 {
		t.Errorf("DetectedInstallIDs empty project = %v, want empty", installIDs)
	}

	warnings := ensureProjectLSP(context.Background(), root, "docs", graphquery.EnsureOptions{})
	_ = warnings

	results := installProjectLSPs(context.Background(), root, "docs", graphquery.EnsureOptions{})
	_ = results

	_ = lspListStatus(root, "docs")

	specs := lspLanguageSpecs()
	if len(specs) == 0 {
		t.Errorf("lspLanguageSpecs empty")
	}

	ids := lspSupportedIDs()
	if len(ids) == 0 {
		t.Errorf("lspSupportedIDs empty")
	}

	for _, spec := range specs {
		if _, ok := lspInstallSpecByID(spec.ID); !ok {
			t.Errorf("lspInstallSpecByID(%q) not found", spec.ID)
		}
	}
	_, ok := lspInstallSpecByID("nonexistent-language-id")
	if ok {
		t.Errorf("lspInstallSpecByID nonexistent should be not-found")
	}
	_, ok = lspInstallSpecByServerID("nonexistent-server-id")
	if ok {
		t.Errorf("lspInstallSpecByServerID nonexistent should be not-found")
	}
	if len(specs) > 0 {
		_, ok = lspInstallSpecByServerID(specs[0].ServerID)
		if !ok {
			t.Errorf("lspInstallSpecByServerID(%q) not found", specs[0].ServerID)
		}
	}

	if lspCacheEnv != graphquery.CacheEnv {
		t.Errorf("lspCacheEnv = %q, want %q", lspCacheEnv, graphquery.CacheEnv)
	}

	if lspSymbolModeCallable != graphquery.SymbolModeCallable {
		t.Errorf("lspSymbolModeCallable mismatch")
	}
	if lspSymbolModeDocument != graphquery.SymbolModeDocument {
		t.Errorf("lspSymbolModeDocument mismatch")
	}
	if lspSymbolModeSelector != graphquery.SymbolModeSelector {
		t.Errorf("lspSymbolModeSelector mismatch")
	}

	_ = lspCacheCommandDirs("gopls")
	_ = lspCommandSource("gopls", root)
	lang := lspLanguage{ID: "go", ServerID: "gopls", LanguageID: "go", Name: "Go"}
	warning := lspUnavailableWarning(lang, "binary not found")
	if !strings.Contains(warning, "Go") || !strings.Contains(warning, "binary not found") {
		t.Errorf("lspUnavailableWarning = %q", warning)
	}
}

func TestPrintLSPUsageWritesToStdout(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	printLSPUsage()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if !strings.Contains(text, "LSP commands") {
		t.Errorf("printLSPUsage missing header: %q", text)
	}
}

func TestRunLSPCallsRunLSPGraphQuery(t *testing.T) {
	if err := RunLSP([]string{"--help"}); err != nil {
		t.Errorf("RunLSP --help = %v, want nil", err)
	}

	var buf bytes.Buffer
	if err := runLSPList([]string{}, &buf); err != nil {
		t.Errorf("runLSPList no args = %v", err)
	}
	if buf.Len() == 0 {
		t.Errorf("runLSPList no args produced no output")
	}

	buf.Reset()
	if err := runLSPList([]string{"--json"}, &buf); err != nil {
		t.Errorf("runLSPList --json = %v", err)
	}

	// Bad command to graphquery.RunLSP returns error.
	if err := RunLSP([]string{"unknown-cmd"}); err == nil {
		t.Errorf("RunLSP unknown-cmd expected error")
	}
}

func TestCommandCandidates(t *testing.T) {
	root := t.TempDir()
	m := &previewLSPManager{root: root}
	cands := m.commandCandidates("gopls")
	if len(cands) == 0 {
		t.Error("expected at least node_modules/.bin")
	}
	cands2 := m.commandCandidates("typescript-language-server")
	if len(cands2) == 0 {
		t.Error("expected at least 1 candidate for non-gopls")
	}
	// Ensure unique output (no duplicate paths).
	seen := map[string]bool{}
	for _, c := range cands {
		if seen[c] {
			t.Errorf("duplicate candidate: %s", c)
		}
		seen[c] = true
	}
}

func TestServerForCachesByServerID(t *testing.T) {
	m := newPreviewLSPManager("/tmp")
	lang := lspLanguage{ID: "test", ServerID: "test", Command: "nonexistent_xyz"}
	// First call should fail because the command is not found.
	if _, err := m.ServerFor(lang); err == nil {
		t.Fatal("expected error for missing command")
	}
	// Manually inject a server for the ServerID and verify it's reused.
	srv := &previewLSPServer{root: "/tmp", lang: lang}
	m.mu.Lock()
	m.servers[lang.ServerID] = srv
	m.mu.Unlock()
	got, err := m.ServerFor(lang)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != srv {
		t.Errorf("expected cached server, got different pointer")
	}
}
