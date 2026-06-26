package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubCodeGraph is a tiny previewCodeGraphProvider used by RunGraph tests.
type stubCodeGraph struct {
	results []previewSearchResult
}

func (s stubCodeGraph) Close(_ context.Context) error { return nil }
func (s stubCodeGraph) SearchCodeGraph(_ context.Context, _ string, _ []string, _ string, _ []string, _ int) ([]previewSearchResult, []string) {
	return s.results, nil
}

func TestRunGraphRequiresQuery(t *testing.T) {
	if err := RunGraph([]string{}); err == nil {
		t.Error("RunGraph without --query should error")
	}
}

func TestRunGraphHelp(t *testing.T) {
	if err := RunGraph([]string{"--help"}); err != nil {
		t.Errorf("--help: %v", err)
	}
}

func TestRunGraphEmptyProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// --no-ensure-lsp avoids contacting graphquery for installation.
	prev := ensureProjectLSPForGraph
	ensureProjectLSPForGraph = func(context.Context, string, string, lspEnsureOptions) []string {
		return nil
	}
	t.Cleanup(func() { ensureProjectLSPForGraph = prev })

	var buf bytes.Buffer
	if err := runGraphQueryWithProvider(context.Background(),
		graphOptions{
			projectRoot: dir,
			docsDir:     "docs",
			query:       "anything",
			limit:       5,
		},
		stubCodeGraph{},
		&buf,
	); err != nil {
		t.Fatalf("runGraphQueryWithProvider: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Query: anything") {
		t.Errorf("expected query in output: %q", out)
	}
}

func TestRunGraphJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := ensureProjectLSPForGraph
	ensureProjectLSPForGraph = func(context.Context, string, string, lspEnsureOptions) []string {
		return nil
	}
	t.Cleanup(func() { ensureProjectLSPForGraph = prev })

	var buf bytes.Buffer
	if err := runGraphQueryWithProvider(context.Background(),
		graphOptions{
			projectRoot: dir,
			docsDir:     "docs",
			query:       "x",
			jsonOutput:  true,
		},
		stubCodeGraph{},
		&buf,
	); err != nil {
		t.Fatalf("runGraphQueryWithProvider: %v", err)
	}
	var resp previewSearchResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if resp.Query != "x" {
		t.Errorf("query = %q, want x", resp.Query)
	}
}

func TestRunGraphBadKeywordOp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := ensureProjectLSPForGraph
	ensureProjectLSPForGraph = func(context.Context, string, string, lspEnsureOptions) []string {
		return nil
	}
	t.Cleanup(func() { ensureProjectLSPForGraph = prev })
	var buf bytes.Buffer
	_ = runGraphQueryWithProvider(context.Background(),
		graphOptions{
			projectRoot: dir,
			docsDir:     "docs",
			query:       "x",
			keywordOp:   "garbage",
		},
		stubCodeGraph{},
		&buf,
	)
}

func TestRunGraphWithWarnings(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_ = runGraphQueryWithProvider(context.Background(),
		graphOptions{
			projectRoot: dir,
			docsDir:     "docs",
			query:       "x",
			warnings:    []string{"lsp warning", "opt warning"},
		},
		stubCodeGraph{},
		&buf,
	)
	if !strings.Contains(buf.String(), "lsp warning") {
		t.Errorf("expected LSP warning in output: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "opt warning") {
		t.Errorf("expected opt warning in output: %q", buf.String())
	}
}

func TestRunGraphResultsFormatting(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	stub := stubCodeGraph{results: []previewSearchResult{
		{
			Title:      "Func A",
			Path:       "a.go",
			Line:       12,
			NodeID:     "n1",
			Source:     "src",
			Confidence: "high",
			FlowRole:   "callee",
			Neighbors: []previewSearchNeighbor{
				{Direction: "in", Label: "caller", Relation: "calls", Path: "b.go", Line: 4, ID: "n2"},
				{Direction: "in", Label: "x", Path: "c.go", ID: "n3"},
				{Direction: "out", Label: "y", ID: "n4"},
				{Direction: "out", Label: "z", ID: "n5"},
				{Direction: "out", Label: "q", ID: "n6"},
				{Direction: "out", Label: "r", ID: "n7"},
			},
		},
		{Title: "Anon"},
	}}
	var buf bytes.Buffer
	_ = runGraphQueryWithProvider(context.Background(),
		graphOptions{
			projectRoot: dir,
			docsDir:     "docs",
			query:       "hello",
		},
		stub,
		&buf,
	)
	out := buf.String()
	if !strings.Contains(out, "Func A (a.go:12)") {
		t.Errorf("expected Func A location, got: %s", out)
	}
	if !strings.Contains(out, "[src, high, callee]") {
		t.Errorf("expected source/confidence/flow role, got: %s", out)
	}
	if !strings.Contains(out, "+3 more neighbors") {
		t.Errorf("expected +3 more neighbors, got: %s", out)
	}
}

func TestNonEmptyStrings(t *testing.T) {
	got := nonEmptyStrings("a", "", "a", "b", "")
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("nonEmptyStrings: %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("nonEmptyStrings[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeGraphQueryLimit(t *testing.T) {
	cases := map[int]int{
		0:                  defaultSearchLimit,
		-1:                 defaultSearchLimit,
		maxSearchLimit:     maxSearchLimit,
		maxSearchLimit + 1: maxSearchLimit,
		defaultSearchLimit: defaultSearchLimit,
	}
	for in, want := range cases {
		if got := normalizeGraphQueryLimit(in); got != want {
			t.Errorf("normalizeGraphQueryLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestNormalizeSearchOutputPath(t *testing.T) {
	dir := t.TempDir()
	// Empty → default name in cwd.
	got := normalizeSearchOutputPath(dir, "")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	if filepath.Base(got) != defaultSearchLauncherName {
		t.Errorf("default name: %q", got)
	}
	// Relative path → resolved against cwd.
	got = normalizeSearchOutputPath(dir, "out.html")
	if filepath.Dir(got) != dir {
		t.Errorf("expected dir %s, got %q", dir, got)
	}
	// Absolute path → preserved.
	abs := filepath.Join(dir, "x", "y.html")
	got = normalizeSearchOutputPath(dir, abs)
	if got != abs {
		t.Errorf("expected %q, got %q", abs, got)
	}
	// Path with ~ is expanded.
	home, err := os.UserHomeDir()
	if err == nil {
		got = normalizeSearchOutputPath(dir, "~/x.html")
		if filepath.Dir(got) != home {
			t.Errorf("expected home dir %s, got %q", home, got)
		}
	}
}

func TestWriteSearchLauncher(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "nested", "launcher.html")
	if err := writeSearchLauncher(out, "http://example.com/", "/proj", "/proj/docs"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "http://example.com/") {
		t.Errorf("missing URL in launcher: %s", data)
	}
}

func TestWriteSearchLauncherCreateFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "missing-parent")
	if err := os.MkdirAll(bad, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o755) })
	if err := writeSearchLauncher(filepath.Join(bad, "sub", "out.html"), "x", "y", "z"); err == nil {
		t.Error("expected MkdirAll failure")
	}
}

func TestRunSearchHelp(t *testing.T) {
	prev := openURLForTest
	openURLForTest = func(string) error { return nil }
	t.Cleanup(func() { openURLForTest = prev })
	if err := RunSearch([]string{"--help"}); err != nil {
		t.Errorf("--help: %v", err)
	}
}

func TestRunSearchBadAddr(t *testing.T) {
	// Address 1.2.3.4:1 is not bindable on most systems.
	if err := RunSearch([]string{"--addr", "1.2.3.4:1", "--no-open"}); err == nil {
		t.Error("expected listen failure")
	}
}

func TestRunSearchBadOutDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "missing")
	if err := os.MkdirAll(bad, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o755) })
	if err := RunSearch([]string{"--no-open", "--out", filepath.Join(bad, "sub", "x.html")}); err == nil {
		t.Error("expected MkdirAll failure")
	}
}

func TestRunSearchLoopbackDisplay(t *testing.T) {
	_ = fmt.Sprintf("skip, used as a placeholder")
}

func TestRunSearchOpenBrowserWithErr(t *testing.T) {
	// Cover openBrowser=true path with an error returned from openURLForTest
	// AND exercise the servePreviewForTest branch with http.ErrServerClosed.
	prevOpen := openURLForTest
	openURLForTest = func(string) error { return fmt.Errorf("open failed") }
	t.Cleanup(func() { openURLForTest = prevOpen })

	prevServe := servePreviewForTest
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	t.Cleanup(func() { servePreviewForTest = prevServe })

	dir := t.TempDir()
	out := filepath.Join(dir, "launcher.html")
	if err := RunSearch([]string{"--no-open=false", "--out", out}); err != nil {
		t.Errorf("RunSearch should not fail even if openURL fails: %v", err)
	}
}

func TestRunSearchServeError(t *testing.T) {
	// Cover the servePreviewForTest error path (non-ErrServerClosed).
	prevServe := servePreviewForTest
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return fmt.Errorf("synthetic serve error")
	}
	t.Cleanup(func() { servePreviewForTest = prevServe })

	dir := t.TempDir()
	out := filepath.Join(dir, "launcher.html")
	if err := RunSearch([]string{"--no-open", "--out", out}); err == nil {
		t.Error("expected error from servePreviewForTest")
	}
}

func TestRunSearchWithBadDocsDir(t *testing.T) {
	// Cover the search launcher write + missing project scenario.
	prevServe := servePreviewForTest
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	t.Cleanup(func() { servePreviewForTest = prevServe })

	dir := t.TempDir()
	out := filepath.Join(dir, "launcher.html")
	if err := RunSearch([]string{
		"--project", dir,
		"--docs-dir", "missing",
		"--no-open",
		"--out", out,
	}); err != nil {
		t.Errorf("RunSearch with missing docs dir: %v", err)
	}
}
