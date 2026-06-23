package preview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDoc is a small helper writing a doc at a docs-root-relative path.
func writeDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadFrontmatterMap(t *testing.T) {
	// No leading delimiter → no block, no error.
	if _, ok, err := readFrontmatterMap("# Title\n\nbody"); ok || err != nil {
		t.Fatalf("expected no block, got ok=%v err=%v", ok, err)
	}
	// Unterminated block → block detected, error.
	if _, ok, err := readFrontmatterMap("---\ntype: module\n"); !ok || err == nil {
		t.Fatalf("expected unterminated error, got ok=%v err=%v", ok, err)
	}
	// Invalid YAML → error.
	if _, ok, err := readFrontmatterMap("---\n: : :\n---\n"); !ok || err == nil {
		t.Fatalf("expected invalid YAML error, got ok=%v err=%v", ok, err)
	}
	// Valid block → type readable; ISO timestamp (auto-typed to time.Time) is
	// still surfaced as a non-empty string via fmString.
	fm, ok, err := readFrontmatterMap("---\ntype: module\ntimestamp: 2026-06-23T00:00:00Z\n---\n\n# Body")
	if !ok || err != nil {
		t.Fatalf("expected valid block, got ok=%v err=%v", ok, err)
	}
	if fmString(fm, "type") != "module" {
		t.Errorf("type = %q, want module", fmString(fm, "type"))
	}
	if fmString(fm, "timestamp") == "" {
		t.Errorf("timestamp should be non-empty (time.Time formatted)")
	}
}

func TestValidateOKFBundle(t *testing.T) {
	root := t.TempDir()
	// Conformant doc.
	writeDoc(t, root, "modules/preview.md", "---\ntype: module\ntitle: Preview\ndescription: d\ntimestamp: 2026-06-23T00:00:00Z\n---\n\n# Preview\n")
	// Missing frontmatter entirely.
	writeDoc(t, root, "bad/no-fm.md", "# No frontmatter\n\nbody\n")
	// Frontmatter present but empty type.
	writeDoc(t, root, "bad/no-type.md", "---\ntitle: X\n---\n\n# X\n")
	// Reserved index.md is exempt from the type requirement.
	writeDoc(t, root, "index.md", "# Listing\n\n* [Preview](modules/preview.md)\n")

	report, err := validateOKFBundle(root, false)
	if err != nil {
		t.Fatalf("validateOKFBundle: %v", err)
	}
	if report.Checked != 4 {
		t.Errorf("checked = %d, want 4", report.Checked)
	}
	if report.Conformant {
		t.Errorf("bundle with missing frontmatter/type must not be conformant")
	}
	if countErrorDocs(report) != 2 {
		t.Errorf("expected 2 docs with errors, got %d", countErrorDocs(report))
	}
	// The reserved index.md must not appear as an error.
	for _, issue := range report.Issues {
		if issue.Path == "index.md" && len(issue.Errors) > 0 {
			t.Errorf("reserved index.md should be exempt, got errors: %v", issue.Errors)
		}
	}
}

func TestValidateOKFBundleStrictPromotesWarnings(t *testing.T) {
	root := t.TempDir()
	// type present but missing recommended title/description/timestamp.
	writeDoc(t, root, "m/a.md", "---\ntype: module\n---\n\n# A\n")

	lenient, _ := validateOKFBundle(root, false)
	if !lenient.Conformant {
		t.Errorf("lenient mode: missing recommended keys are warnings, should stay conformant")
	}
	strict, _ := validateOKFBundle(root, true)
	if strict.Conformant {
		t.Errorf("strict mode: missing recommended keys must fail conformance")
	}
}

func TestRegenerateIndexes(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "modules/preview.md", "---\ntype: module\ntitle: Preview\ndescription: Preview module.\n---\n\n# Preview\n")
	writeDoc(t, root, "modules/harness.md", "---\ntype: module\ntitle: Harness\ndescription: Harness module.\n---\n\n# Harness\n")
	writeDoc(t, root, "features/web.md", "---\ntype: feature\ntitle: Web\ndescription: Web feature.\n---\n\n# Web\n")

	written, err := regenerateIndexes(root, false)
	if err != nil {
		t.Fatalf("regenerateIndexes: %v", err)
	}
	// One index per directory with docs: modules/, features/, and root.
	if len(written) != 3 {
		t.Fatalf("expected 3 index files, got %d: %v", len(written), written)
	}

	modIndex, err := os.ReadFile(filepath.Join(root, "modules", "index.md"))
	if err != nil {
		t.Fatalf("read modules/index.md: %v", err)
	}
	body := string(modIndex)
	if !strings.Contains(body, "## module") {
		t.Errorf("modules index missing `## module` group:\n%s", body)
	}
	if !strings.Contains(body, "[Harness](harness.md)") || !strings.Contains(body, "[Preview](preview.md)") {
		t.Errorf("modules index missing entries:\n%s", body)
	}
	// Entries are sorted by title (Harness before Preview).
	if strings.Index(body, "Harness") > strings.Index(body, "Preview") {
		t.Errorf("entries should be sorted by title:\n%s", body)
	}

	// Root index links subdirectories with derived descriptions.
	rootIndex, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatalf("read index.md: %v", err)
	}
	if !strings.Contains(string(rootIndex), "modules/index.md") {
		t.Errorf("root index should link modules/index.md:\n%s", string(rootIndex))
	}
}

func TestRegenerateIndexesDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "modules/preview.md", "---\ntype: module\ntitle: Preview\n---\n\n# Preview\n")

	written, err := regenerateIndexes(root, true)
	if err != nil {
		t.Fatalf("regenerateIndexes dry-run: %v", err)
	}
	if len(written) == 0 {
		t.Errorf("dry-run should still report intended files")
	}
	if _, err := os.Stat(filepath.Join(root, "modules", "index.md")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not write index.md")
	}
}
