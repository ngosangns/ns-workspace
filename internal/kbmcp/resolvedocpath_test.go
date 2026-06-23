package kbmcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"
)

// TestResolveDocPath_NeverEscapesProperty is the property-based companion to the
// table-driven traversal tests.
//
// Property 15 (MCP chống path traversal — Validates: Requirements 8.2):
// for ANY id string, resolveDocPath either rejects it (err != nil) or returns
// an absolute path that lives strictly inside the docs root. It must never
// return a path outside docs root.
func TestResolveDocPath_NeverEscapesProperty(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	sep := string(os.PathSeparator)

	property := func(id string) bool {
		p, err := resolveDocPath(root, id)
		if err != nil {
			// Rejected ids are always acceptable: nothing is resolved.
			return true
		}
		// Accepted ids MUST resolve strictly inside the docs root.
		if !strings.HasPrefix(p, root+sep) {
			return false
		}
		// And the resolved path must not contain any unresolved upward escape.
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return false
		}
		return rel != ".." && !strings.HasPrefix(rel, ".."+sep) && !filepath.IsAbs(rel)
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatalf("resolveDocPath escaped docs root for some id: %v", err)
	}
}

// TestResolveDocPath_TraversalCorpusProperty stresses the guard with ids built
// from path-ish segments (including "..", separators and absolute markers) that
// random string generation rarely produces, so the traversal branch is
// exercised heavily. Same invariant as above (Property 15, Req 8.2).
func TestResolveDocPath_TraversalCorpusProperty(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	sep := string(os.PathSeparator)
	segments := []string{"..", ".", "a", "b", "nested", "doc.md", "/", "\\", "", " ", "/etc", "passwd"}

	pick := func(n uint8) string { return segments[int(n)%len(segments)] }

	property := func(a, b, c, d uint8) bool {
		id := strings.Join([]string{pick(a), pick(b), pick(c), pick(d)}, "/")
		p, err := resolveDocPath(root, id)
		if err != nil {
			return true
		}
		return strings.HasPrefix(p, root+sep)
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 5000}); err != nil {
		t.Fatalf("resolveDocPath escaped docs root for a traversal id: %v", err)
	}
}
