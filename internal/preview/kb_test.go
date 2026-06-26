package preview

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeKBDoc(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunKBNoArgs(t *testing.T) {
	if err := RunKB(nil); err == nil {
		t.Error("RunKB with no args should error")
	}
}

func TestRunKBUnknown(t *testing.T) {
	if err := RunKB([]string{"wat"}); err == nil {
		t.Error("RunKB with unknown subcommand should error")
	}
}

func TestRunKBHelp(t *testing.T) {
	if err := RunKB([]string{"help"}); err != nil {
		t.Errorf("help subcommand: %v", err)
	}
}

func TestRunKBValidateText(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "ok.md"),
		"---\ntitle: OK\ndescription: d\ntimestamp: 2026-06-23T00:00:00Z\ntype: module\n---\n# OK")
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err != nil {
		t.Errorf("validate conformant bundle should succeed: %v", err)
	}
}

func TestRunKBValidateJSON(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "missing.md"), "no frontmatter")
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs", "--json"}); err == nil {
		t.Error("expected non-nil error for non-conformant bundle")
	}
}

func TestRunKBValidateMissingDocs(t *testing.T) {
	dir := t.TempDir()
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "nope"}); err == nil {
		t.Error("missing docs should error")
	}
}

func TestRunKBValidateStrict(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"), "---\ntype: module\n---\nx")
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err != nil {
		t.Errorf("non-strict should pass: %v", err)
	}
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs", "--strict"}); err == nil {
		t.Error("strict should fail when recommended keys are missing")
	}
}

func TestRunKBValidateReservedFiles(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "index.md"), "# Index\n")
	writeKBDoc(t, filepath.Join(docs, "log.md"), "# Log\n")
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err != nil {
		t.Errorf("reserved files exempt: %v", err)
	}
}

func TestRunKBValidateHelpFlag(t *testing.T) {
	dir := t.TempDir()
	if err := RunKB([]string{"validate", "--help", "--project", dir}); err != nil {
		t.Errorf("--help should return nil: %v", err)
	}
}

func TestRunKBValidateInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "bad.md"), "---\n[unclosed\n---\nx")
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err == nil {
		t.Error("invalid YAML should produce error")
	}
}

func TestRunKBValidateUnterminatedFM(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "u.md"), "---\ntype: module\n")
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err == nil {
		t.Error("unterminated frontmatter should produce error")
	}
}

func TestRunKBValidateReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(docs, "r.md")
	writeKBDoc(t, path, "---\ntype: m\n---\nx")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err == nil {
		t.Error("read error should produce error")
	}
}

func TestRunKBIndex(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	sub := filepath.Join(docs, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"),
		"---\ntitle: A\ndescription: alpha\ntype: module\n---\n# A")
	writeKBDoc(t, filepath.Join(sub, "b.md"),
		"---\ntitle: B\ndescription: beta\ntype: module\n---\n# B")
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "docs"}); err != nil {
		t.Fatalf("index should succeed: %v", err)
	}
	for _, p := range []string{filepath.Join(docs, "index.md"), filepath.Join(sub, "index.md")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected index at %s: %v", p, err)
		}
	}
}

func TestRunKBIndexDryRun(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"), "---\ntitle: A\ntype: m\n---\n# A")
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "docs", "--dry-run"}); err != nil {
		t.Errorf("dry-run should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(docs, "index.md")); err == nil {
		t.Error("dry-run should not write index.md")
	}
}

func TestRunKBIndexHelpFlag(t *testing.T) {
	if err := RunKB([]string{"index", "--help"}); err != nil {
		t.Errorf("--help should return nil: %v", err)
	}
}

func TestRunKBIndexMissingDocs(t *testing.T) {
	dir := t.TempDir()
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "nope"}); err == nil {
		t.Error("missing docs should error")
	}
}

func TestRunKBIndexWriteFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"), "---\ntitle: A\ntype: m\n---\n# A")
	if err := os.Chmod(docs, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(docs, 0o755) })
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "docs"}); err == nil {
		t.Error("write failure should error")
	}
}

func TestReadFrontmatterMapCases(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantHasBlk bool
		wantErr    bool
		wantVal    string
	}{
		{"empty", "", false, false, ""},
		{"no frontmatter", "# hi", false, false, ""},
		{"unterminated", "---\ntitle: x", true, true, ""},
		{"bad yaml", "---\n:bad\n---\nx", true, true, ""},
		{"ok", "---\ntitle: T\n---\nx", true, false, "T"},
		{"empty yaml body", "---\n\n---\nx", true, false, ""},
		{"BOM", "\ufeff---\ntitle: BOM\n---\nx", true, false, "BOM"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm, has, err := readFrontmatterMap(c.raw)
			if has != c.wantHasBlk {
				t.Errorf("hasBlock = %v, want %v", has, c.wantHasBlk)
			}
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if c.wantVal != "" {
				if got := fmString(fm, "title"); got != c.wantVal {
					t.Errorf("title = %q, want %q", got, c.wantVal)
				}
			}
		})
	}
}

func TestFMStringVariants(t *testing.T) {
	fm := map[string]any{
		"s":   "  spaced  ",
		"e":   "",
		"n":   nil,
		"int": 42,
	}
	if v := fmString(fm, "s"); v != "spaced" {
		t.Errorf("string not trimmed: %q", v)
	}
	if v := fmString(fm, "e"); v != "" {
		t.Errorf("empty string: %q", v)
	}
	if v := fmString(fm, "n"); v != "" {
		t.Errorf("nil: %q", v)
	}
	if v := fmString(fm, "int"); v != "42" {
		t.Errorf("int format: %q", v)
	}
	if v := fmString(fm, "missing"); v != "" {
		t.Errorf("missing key: %q", v)
	}
}

func TestPrintKBValidateText(t *testing.T) {
	report := kbValidateReport{
		DocsRoot:   "/x",
		Checked:    2,
		Conformant: false,
		Issues: []kbValidateIssue{
			{Path: "a.md", Errors: []string{"e1"}},
			{Path: "b.md", Warnings: []string{"w1"}},
		},
	}
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printKBValidateText(report)
	w.Close()
	os.Stdout = old
	buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "ERROR  a.md: e1") {
		t.Errorf("expected ERROR for a.md, got %q", out)
	}
	if !strings.Contains(out, "warn   b.md: w1") {
		t.Errorf("expected warn for b.md, got %q", out)
	}
	if !strings.Contains(out, "NOT conformant") {
		t.Errorf("expected NOT conformant, got %q", out)
	}
	// Conformant case
	report = kbValidateReport{DocsRoot: "/x", Checked: 1, Conformant: true}
	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w
	printKBValidateText(report)
	w.Close()
	os.Stdout = old
	buf.ReadFrom(r)
	out = buf.String()
	if !strings.Contains(out, "OKF-conformant") {
		t.Errorf("expected OKF-conformant, got %q", out)
	}
}

func TestCountErrorDocs(t *testing.T) {
	r := kbValidateReport{Issues: []kbValidateIssue{
		{Errors: []string{"a"}},
		{Warnings: []string{"b"}},
		{},
	}}
	if got := countErrorDocs(r); got != 1 {
		t.Errorf("countErrorDocs = %d, want 1", got)
	}
}

func TestValidateOKFBundleAdd(t *testing.T) {
	r := &kbValidateReport{}
	r.add("x", nil, nil)
	if len(r.Issues) != 0 {
		t.Errorf("expected no issue, got %v", r.Issues)
	}
	r2 := &kbValidateReport{Conformant: true}
	r2.add("x", nil, []string{"w"})
	if len(r2.Issues) != 1 || r2.Issues[0].Warnings[0] != "w" {
		t.Errorf("warning-only not added: %+v", r2.Issues)
	}
	if !r2.Conformant {
		t.Error("warnings should not flip Conformant")
	}
	r3 := &kbValidateReport{Conformant: true}
	r3.add("x", []string{"e"}, nil)
	if r3.Conformant {
		t.Error("errors should flip Conformant to false")
	}
}

func TestDeriveDirDescription(t *testing.T) {
	if got := deriveDirDescription(nil); got != "" {
		t.Errorf("empty: %q", got)
	}
	got := deriveDirDescription([]indexEntry{{group: "module", desc: "alpha"}})
	if got != "alpha" {
		t.Errorf("single doc: %q", got)
	}
	got = deriveDirDescription([]indexEntry{
		{group: "module", desc: "a"},
		{group: "module", desc: "b"},
	})
	if !strings.Contains(got, "documents") {
		t.Errorf("multi docs: %q", got)
	}
	got = deriveDirDescription([]indexEntry{
		{group: "Subdirectories"},
		{group: "module", desc: "x"},
	})
	if got != "x" {
		t.Errorf("subdir ignored: %q", got)
	}
	got = deriveDirDescription([]indexEntry{{group: "Subdirectories"}})
	if got != "" {
		t.Errorf("no real docs: %q", got)
	}
}

func TestIndexDirTitle(t *testing.T) {
	root := "/root"
	if got := indexDirTitle(root, root); got != "Docs Index" {
		t.Errorf("root title = %q", got)
	}
	if got := indexDirTitle("/root/sub", root); !strings.Contains(got, "Sub Index") {
		t.Errorf("sub title = %q", got)
	}
}

func TestCollectIndexEntriesSkipsReadFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "x.md")
	if err := os.WriteFile(p, []byte("---\ntitle: X\ntype: m\n---\n# X"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	entries, err := collectIndexEntries(dir, map[string]bool{}, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("read failure should be skipped, got %d entries", len(entries))
	}
}

func TestValidateOKFBundleSorts(t *testing.T) {
	root := t.TempDir()
	writeKBDoc(t, filepath.Join(root, "z.md"), "---\ntype: m\n---\nx")
	writeKBDoc(t, filepath.Join(root, "a.md"), "---\ntype: m\n---\nx")
	report, err := validateOKFBundle(root, false)
	if err != nil {
		t.Fatalf("validateOKFBundle: %v", err)
	}
	if len(report.Issues) < 2 {
		t.Fatalf("expected 2 issues, got %d", len(report.Issues))
	}
	if report.Issues[0].Path > report.Issues[1].Path {
		t.Errorf("issues not sorted: %v", report.Issues)
	}
}

func TestDirectoriesToIndex(t *testing.T) {
	root := t.TempDir()
	// md file at root level.
	if err := os.WriteFile(filepath.Join(root, "root.md"), []byte("# R"), 0o644); err != nil {
		t.Fatal(err)
	}
	// index.md must be skipped.
	if err := os.WriteFile(filepath.Join(root, "index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	// nested md file.
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	// non-md file must be skipped.
	if err := os.WriteFile(filepath.Join(root, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, err := directoriesToIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, d := range dirs {
		got[d] = true
	}
	if !got[root] {
		t.Errorf("expected root in dirs, got %v", dirs)
	}
	if !got[sub] {
		t.Errorf("expected sub in dirs, got %v", dirs)
	}
}

func TestDirectoriesToIndex_WalkError(t *testing.T) {
	// Pass a non-existent root to trigger WalkDir error.
	dirs, err := directoriesToIndex("/nonexistent/path/zzz-abc-123")
	if err == nil {
		t.Errorf("expected error from WalkDir, got dirs=%v", dirs)
	}
	if dirs != nil {
		t.Errorf("expected nil dirs on error, got %v", dirs)
	}
}

func TestRunKBValidateParseError(t *testing.T) {
	// Pass an unknown flag to trigger ParseError in fs.Parse (non-help).
	if err := RunKB([]string{"validate", "--unknown-flag"}); err == nil {
		t.Error("expected parse error for unknown flag")
	}
}

func TestRunKBValidateNotADirectory(t *testing.T) {
	// docs path exists but is a file, not a directory → !info.IsDir() branch.
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.WriteFile(docs, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RunKB([]string{"validate", "--project", dir, "--docs-dir", "docs"}); err == nil {
		t.Error("expected error when docs path is a file")
	}
}

func TestRunKBIndexMissingDocsExtra(t *testing.T) {
	// Covers runKBIndex error path when docs dir missing.
	dir := t.TempDir()
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "nope"}); err == nil {
		t.Error("expected error for missing docs in index")
	}
}

func TestRunKBIndexWritesIndex(t *testing.T) {
	// Covers runKBIndex happy path and regenerateIndexes.
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs", "sub")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"),
		"---\ntitle: A\ndescription: da\ntimestamp: 2026-06-23T00:00:00Z\ntype: module\n---\n# A")
	writeKBDoc(t, filepath.Join(docs, "b.md"),
		"---\ntitle: B\ndescription: db\ntimestamp: 2026-06-23T00:00:00Z\ntype: module\n---\n# B")
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "docs"}); err != nil {
		t.Errorf("index should succeed: %v", err)
	}
	// Verify index.md was created.
	if _, err := os.Stat(filepath.Join(docs, "index.md")); err != nil {
		t.Errorf("index.md should exist: %v", err)
	}
	// Run again to exercise the regenerate-existing path.
	if err := RunKB([]string{"index", "--project", dir, "--docs-dir", "docs"}); err != nil {
		t.Errorf("second index should succeed: %v", err)
	}
}

func TestRunKBIndexParseError(t *testing.T) {
	// Pass unknown flag to trigger ParseError in runKBIndex.
	if err := RunKB([]string{"index", "--unknown-flag"}); err == nil {
		t.Error("expected parse error for unknown flag")
	}
}

func TestRunKBNoArgsExtra(t *testing.T) {
	// No subcommand → prints usage, returns error.
	if err := RunKB([]string{}); err == nil {
		t.Error("no subcommand should error (usage printed)")
	}
}

func TestRunKBUnknownSubcommand(t *testing.T) {
	if err := RunKB([]string{"bogus"}); err == nil {
		t.Error("unknown subcommand should error")
	}
}

func TestRegenerateIndexesDryRun(t *testing.T) {
	// Cover the dryRun=true branch (no file writes).
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"),
		"---\ntitle: A\ndescription: da\ntimestamp: 2026-06-23T00:00:00Z\ntype: module\n---\n# A")
	written, err := regenerateIndexes(docs, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) == 0 {
		t.Error("expected written list to be non-empty")
	}
	// Verify no file was written.
	if _, err := os.Stat(filepath.Join(docs, "index.md")); err == nil {
		t.Error("dryRun should not write index.md")
	}
}

func TestRegenerateIndexesWriteError(t *testing.T) {
	// Force WriteFile error by making the directory read-only.
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKBDoc(t, filepath.Join(docs, "a.md"),
		"---\ntitle: A\ndescription: da\ntimestamp: 2026-06-23T00:00:00Z\ntype: module\n---\n# A")
	// Make docs read-only so WriteFile fails when writing index.md.
	if err := os.Chmod(docs, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(docs, 0o755) })
	if _, err := regenerateIndexes(docs, false); err == nil {
		t.Error("expected write error")
	}
}

func TestCollectIndexEntries_ReadDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	// Make dir unreadable.
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := collectIndexEntries(dir, map[string]bool{}, map[string]string{}); err == nil {
		t.Error("expected error when dir is unreadable")
	}
}
