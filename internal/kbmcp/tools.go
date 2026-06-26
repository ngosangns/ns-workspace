package kbmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ngosangns/ns-workspace/internal/preview"
)

// Default search parameters mirror the preview/search command so search_docs
// returns the exact same response that buildPreviewSearchResponse produces
// (single contract; see Property 17). "hybrid" mode and the "sum" keyword
// operator match graph.go / the HTTP /api/search handler defaults.
const (
	defaultSearchMode            = "hybrid"
	defaultSearchKeywordOperator = "sum"
	defaultSearchLimit           = 8
)

// listDocsArgs are the arguments for the list_docs tool. Both filters are
// optional; empty means "no filter".
type listDocsArgs struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

// lookupArgs are the arguments for the lookup_doc tool.
type lookupArgs struct {
	ID string `json:"id"`
}

// searchArgs are the arguments for the search_docs tool. Limit is optional and
// falls back to defaultSearchLimit when absent or non-positive.
type searchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// docSummary is the compact per-document view returned by list_docs: identity,
// location, format/type and tags, without the document body.
type docSummary struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Type  string   `json:"type,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	Path  string   `json:"path"`
}

// listDocsResult is the list_docs response: the project name, docs root, and
// the filtered document summaries.
type listDocsResult struct {
	Project  string       `json:"project"`
	DocsRoot string       `json:"docsRoot"`
	Count    int          `json:"count"`
	Docs     []docSummary `json:"docs"`
}

// toolDescriptor is a single MCP tool advertised via "tools/list". InputSchema
// is a JSON Schema object describing the tool's arguments.
type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// toolDescriptors returns the descriptors for every knowledge-base tool, matching
// the schema in the design document. The concrete handlers for these tools are
// implemented in tasks 5.3 (read tools) and 5.4 (modify_doc).
func toolDescriptors() []toolDescriptor {
	return []toolDescriptor{
		{
			Name:        "list_docs",
			Description: "List all documents in the knowledge base",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{"type": "string", "description": "filter by doc type (optional)"},
					"tag":  map[string]any{"type": "string", "description": "filter by tag (optional)"},
				},
			},
		},
		{
			Name:        "lookup_doc",
			Description: "Get full content and metadata of a doc by id/path",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "search_docs",
			Description: "Search the knowledge base (docs + graph)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"limit": map[string]any{"type": "integer"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "modify_doc",
			Description: "Create or update a doc; path must be inside docs root",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":      map[string]any{"type": "string", "description": "doc path relative to docs root"},
					"content": map[string]any{"type": "string"},
				},
				"required": []string{"id", "content"},
			},
		},
	}
}

// The handlers below back the MCP tools:
//   - handleListDocs / handleLookupDoc / handleSearchDocs  → task 5.3 (this file)
//   - handleModifyDoc (+ resolveDocPath)                   → task 5.4
//
// Each accepts the raw JSON arguments from the tools/call request and returns a
// JSON-serializable result or an error. Errors are converted into JSON-RPC
// error responses by the dispatcher without crashing the server.

// handleListDocs opens the knowledge snapshot and returns the documents found
// in the docs root, optionally filtered by type and/or tag. Only documents
// scanned from the docs root are returned (Property 16, Req 7.3). The type
// filter matches case-insensitively; the tag filter matches when any of a
// document's tags equals the requested tag (case-insensitive).
func (s *Server) handleListDocs(args json.RawMessage) (any, error) {
	var in listDocsArgs
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}

	knowledge, err := preview.OpenKnowledge(s.projectRoot, s.docsDir)
	if err != nil {
		return nil, fmt.Errorf("open knowledge base: %w", err)
	}

	typeFilter := strings.ToLower(strings.TrimSpace(in.Type))
	tagFilter := strings.ToLower(strings.TrimSpace(in.Tag))

	docs := make([]docSummary, 0, len(knowledge.Documents()))
	for _, doc := range knowledge.Documents() {
		if typeFilter != "" && strings.ToLower(strings.TrimSpace(doc.Type)) != typeFilter {
			continue
		}
		if tagFilter != "" && !hasTag(doc.Tags, tagFilter) {
			continue
		}
		docs = append(docs, docSummary{
			ID:    doc.ID,
			Title: doc.Title,
			Type:  doc.Type,
			Tags:  doc.Tags,
			Path:  doc.Path,
		})
	}

	return listDocsResult{
		Project:  knowledge.Name(),
		DocsRoot: knowledge.DocsRoot(),
		Count:    len(docs),
		Docs:     docs,
	}, nil
}

// handleLookupDoc returns the full content and metadata of a single document
// identified by its docs-root-relative id. A missing or unknown id yields a
// clear error rather than a panic (Property 16, Req 7.4).
func (s *Server) handleLookupDoc(args json.RawMessage) (any, error) {
	var in lookupArgs
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, fmt.Errorf("lookup_doc requires a non-empty %q argument", "id")
	}

	knowledge, err := preview.OpenKnowledge(s.projectRoot, s.docsDir)
	if err != nil {
		return nil, fmt.Errorf("open knowledge base: %w", err)
	}

	doc, ok := knowledge.Document(id)
	if !ok {
		return nil, fmt.Errorf("document not found: %q", id)
	}
	return doc, nil
}

// handleSearchDocs runs the shared preview search pipeline against the snapshot
// so results match buildPreviewSearchResponse exactly for the same query
// (single contract, Property 17, Req 7.5). Mode and keyword operator use the
// preview/search defaults; limit defaults to defaultSearchLimit when not given.
func (s *Server) handleSearchDocs(ctx context.Context, args json.RawMessage) (any, error) {
	var in searchArgs
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, fmt.Errorf("search_docs requires a non-empty %q argument", "query")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	knowledge, err := preview.OpenKnowledge(s.projectRoot, s.docsDir)
	if err != nil {
		return nil, fmt.Errorf("open knowledge base: %w", err)
	}

	return knowledge.Search(ctx, query, defaultSearchMode, defaultSearchKeywordOperator, limit), nil
}

// modifyArgs are the arguments for the modify_doc tool: the docs-root-relative
// id of the document to create or update, and its new content.
type modifyArgs struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// modifyDocResult is the modify_doc response: success flag plus the
// docs-root-relative path that was written.
type modifyDocResult struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

// handleModifyDoc creates or updates a document inside the docs root. It
// resolves the docs root via the shared knowledge façade, validates that the
// requested id stays within that root (rejecting path traversal — Property 15,
// Req 8.2), creates any missing parent directories within the docs root
// (Req 8.3) and writes the content with 0o644 permissions. On success it
// returns {ok, path} (Req 8.1). Any rejection or I/O error is returned as a
// clear error and converted into a JSON-RPC error by the dispatcher; it never
// panics.
func (s *Server) handleModifyDoc(args json.RawMessage) (any, error) {
	var in modifyArgs
	if err := decodeArgs(args, &in); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, fmt.Errorf("modify_doc requires a non-empty %q argument", "id")
	}

	knowledge, err := preview.OpenKnowledge(s.projectRoot, s.docsDir)
	if err != nil {
		return nil, fmt.Errorf("open knowledge base: %w", err)
	}
	docsRoot := knowledge.DocsRoot()

	abs, err := resolveDocPath(docsRoot, id)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("create parent directory: %w", err)
	}
	if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
		return nil, fmt.Errorf("write doc: %w", err)
	}

	return modifyDocResult{OK: true, Path: id}, nil
}

// relFn is a test seam for filepath.Rel so tests can exercise the error branch
// (which is platform-specific and hard to trigger on Linux).
var relFn = filepath.Rel

// resolveDocPath joins a docs-root-relative id onto docsRoot and returns the
// absolute path only when the result stays strictly inside docsRoot. It is a
// security-critical guard against path traversal (Property 15, Req 8.2): it
// rejects empty ids, absolute ids, and any id whose cleaned target escapes the
// docs root (via "../" segments or otherwise). docsRoot is expected to be an
// absolute, existing directory (the façade's DocsRoot()); it is cleaned defensively.
func resolveDocPath(docsRoot, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("doc id must not be empty")
	}
	if filepath.IsAbs(id) {
		return "", fmt.Errorf("doc id must be relative to docs root: %q", id)
	}

	root := filepath.Clean(docsRoot)
	abs := filepath.Clean(filepath.Join(root, id))

	// Primary check: the cleaned target must be relative to the docs root with
	// no upward escape. filepath.Rel collapses ".." segments, so a traversal
	// that climbs above root yields a rel beginning with "..".
	rel, err := relFn(root, abs)
	if err != nil {
		return "", fmt.Errorf("path escapes docs root: %q", id)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes docs root: %q", id)
	}

	// Belt-and-suspenders: the absolute target must be prefixed by the docs root
	// plus a separator (so root itself and sibling dirs sharing a prefix are
	// rejected).
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes docs root: %q", id)
	}

	return abs, nil
}

// decodeArgs unmarshals the raw tools/call arguments into v. Absent arguments
// (nil/empty) are treated as an empty object so optional-only tools work
// without an "arguments" field. Malformed JSON yields a clear error.
func decodeArgs(args json.RawMessage, v any) error {
	if len(args) == 0 {
		return nil
	}
	if err := json.Unmarshal(args, v); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}

// hasTag reports whether tags contains target (compared case-insensitively).
// target is expected to already be lower-cased and trimmed by the caller.
func hasTag(tags []string, target string) bool {
	for _, tag := range tags {
		if strings.ToLower(strings.TrimSpace(tag)) == target {
			return true
		}
	}
	return false
}
