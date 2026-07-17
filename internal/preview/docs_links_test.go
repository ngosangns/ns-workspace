package preview

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// markdownLinkRE matches markdown links of the form [text](path) and captures the path.
var markdownLinkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// TestDocsPlanningLinksResolve walks the repo knowledge base and asserts every
// relative specs/planning/ (or other relative markdown) target referenced from
// docs/**/*.md exists on disk. This is the cleanup invariant for dead planning links.
func TestDocsPlanningLinksResolve(t *testing.T) {
	root, ok := previewModuleRoot(".")
	if !ok {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		root, ok = previewModuleRoot(wd)
		if !ok {
			t.Skip("module root not found; skip docs link check outside checkout")
		}
	}
	docsRoot := filepath.Join(root, "docs")
	if _, err := os.Stat(docsRoot); err != nil {
		t.Fatalf("docs root: %v", err)
	}

	var missing []string
	var checked int
	err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		for _, m := range markdownLinkRE.FindAllSubmatch(data, -1) {
			target := strings.TrimSpace(string(m[1]))
			// Strip optional title: path "title"
			if i := strings.Index(target, " "); i >= 0 {
				target = target[:i]
			}
			target = strings.Trim(target, `"'`)
			if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			// Only enforce relative local paths (including specs/planning).
			if strings.HasPrefix(target, "/") {
				// Bundle-relative from docs root.
				target = strings.TrimPrefix(target, "/")
				abs := filepath.Join(docsRoot, filepath.FromSlash(target))
				checked++
				if _, err := os.Stat(abs); err != nil {
					rel, _ := filepath.Rel(root, path)
					missing = append(missing, rel+" -> /"+target)
				}
				continue
			}
			// Ignore pure anchors after path
			if i := strings.Index(target, "#"); i >= 0 {
				target = target[:i]
			}
			if target == "" {
				continue
			}
			abs := filepath.Clean(filepath.Join(dir, filepath.FromSlash(target)))
			// Only check targets under docs/ (ignore accidental escapes outside).
			if !strings.HasPrefix(abs, docsRoot) {
				continue
			}
			checked++
			if _, err := os.Stat(abs); err != nil {
				rel, _ := filepath.Rel(root, path)
				missing = append(missing, rel+" -> "+target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}
	if checked == 0 {
		t.Fatal("expected to check at least one relative docs link")
	}
	if len(missing) > 0 {
		t.Fatalf("broken relative docs links (%d):\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
}

// TestQuartzImplementationRemoved asserts the production-dead Quartz stack is gone
// while the deprecation flag remains in the shipped Run path (covered by
// TestRunQuartzDirDeprecatedIgnored).
func TestQuartzImplementationRemoved(t *testing.T) {
	root, ok := previewModuleRoot(".")
	if !ok {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		root, ok = previewModuleRoot(wd)
		if !ok {
			t.Skip("module root not found")
		}
	}
	for _, rel := range []string{
		"internal/preview/quartz.go",
		"internal/preview/quartz_test.go",
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("expected %s to be removed", rel)
		}
	}
}
