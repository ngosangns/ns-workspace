package preview

import "context"

// Knowledge is a thin public façade over the preview "knowledge core"
// (scanSpecProject + buildPreviewSearchResponse). It lets other packages
// (notably internal/kbmcp) read the scanned project snapshot and run searches
// without depending on the package's private symbols, keeping a single
// read/search contract instead of duplicating logic.
type Knowledge struct {
	projectRoot string
	docsDir     string
	project     specProject
}

// KnowledgeDocument is a public, read-only view of a single scanned document.
// It mirrors the fields of the internal specDocument that downstream consumers
// (e.g. MCP tools) need: identity, location, format and OKF metadata.
type KnowledgeDocument struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Path        string   `json:"path"`
	Language    string   `json:"language,omitempty"`
	Format      string   `json:"format,omitempty"`
	Category    string   `json:"category"`
	Status      string   `json:"status,omitempty"`
	Version     string   `json:"version,omitempty"`
	Compliance  string   `json:"compliance,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Timestamp   string   `json:"timestamp,omitempty"`
	Raw         string   `json:"raw,omitempty"`
}

// OpenKnowledge scans the project at projectRoot (with docs directory docsDir)
// and returns a snapshot façade. The projectRoot is normalized the same way the
// preview/search commands normalize it, so subsequent searches stay consistent
// with the scanned snapshot. Returns an error when the docs directory is
// missing or invalid (delegated to scanSpecProject).
func OpenKnowledge(projectRoot, docsDir string) (*Knowledge, error) {
	root := normalizePreviewProjectRoot(projectRoot)
	project, err := scanSpecProject(root, docsDir)
	if err != nil {
		return nil, err
	}
	return &Knowledge{projectRoot: root, docsDir: docsDir, project: project}, nil
}

// ProjectRoot returns the normalized project root used for the snapshot.
func (k *Knowledge) ProjectRoot() string {
	return k.projectRoot
}

// DocsRoot returns the absolute docs root directory for the snapshot.
func (k *Knowledge) DocsRoot() string {
	return k.project.Summary.DocsRoot
}

// Name returns the project name from the scanned summary.
func (k *Knowledge) Name() string {
	return k.project.Summary.Name
}

// Documents returns a public view of every document in the snapshot, sorted by
// path (the order produced by scanSpecProject).
func (k *Knowledge) Documents() []KnowledgeDocument {
	docs := make([]KnowledgeDocument, 0, len(k.project.Documents))
	for _, doc := range k.project.Documents {
		docs = append(docs, toKnowledgeDocument(doc))
	}
	return docs
}

// Document looks up a single document by its id (its docs-root-relative path).
// The second return value is false when no document matches.
func (k *Knowledge) Document(id string) (KnowledgeDocument, bool) {
	for _, doc := range k.project.Documents {
		if doc.ID == id {
			return toKnowledgeDocument(doc), true
		}
	}
	return KnowledgeDocument{}, false
}

// Search runs the shared preview search pipeline against the snapshot and
// returns the exact same response that buildPreviewSearchResponse produces for
// the given query. Code-graph results are omitted (codeGraph is nil) because
// the façade targets the docs knowledge base; all other panels (docs semantic,
// docs graph, code semantic) match the preview/search contract.
func (k *Knowledge) Search(ctx context.Context, query, mode, keywordOperator string, limit int) previewSearchResponse {
	return buildPreviewSearchResponse(ctx, k.project, nil, k.projectRoot, query, mode, keywordOperator, limit)
}

func toKnowledgeDocument(doc specDocument) KnowledgeDocument {
	return KnowledgeDocument{
		ID:          doc.ID,
		Title:       doc.Title,
		Path:        doc.Path,
		Language:    doc.Language,
		Format:      doc.Format,
		Category:    doc.Category,
		Status:      doc.Status,
		Version:     doc.Version,
		Compliance:  doc.Compliance,
		Priority:    doc.Priority,
		Description: doc.Description,
		Type:        doc.Type,
		Tags:        doc.Tags,
		Timestamp:   doc.Timestamp,
		Raw:         doc.Raw,
	}
}
