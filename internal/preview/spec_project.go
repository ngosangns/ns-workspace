package preview

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

type specProject struct {
	Summary   projectSummary `json:"summary"`
	Documents []specDocument `json:"documents"`
	Graph     specGraph      `json:"graph"`
}

type projectSummary struct {
	Name           string            `json:"name"`
	ProjectRoot    string            `json:"projectRoot"`
	DocsRoot       string            `json:"docsRoot"`
	AgentsPath     string            `json:"agentsPath"`
	AgentsFound    bool              `json:"agentsFound"`
	IndexFound     bool              `json:"indexFound"`
	SyncFound      bool              `json:"syncFound"`
	TotalSpecs     int               `json:"totalSpecs"`
	Categories     map[string]int    `json:"categories"`
	StatusCounts   map[string]int    `json:"statusCounts"`
	Compliance     map[string]int    `json:"compliance"`
	Sync           map[string]string `json:"sync"`
	Warnings       []string          `json:"warnings"`
	GeneratedTitle string            `json:"generatedTitle"`
}

type specDocument struct {
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
	Resource    string   `json:"resource,omitempty"`
	Raw         string   `json:"raw,omitempty"`
	HTML        string   `json:"html,omitempty"`
	SearchText  string   `json:"-"`
}

type specGraph struct {
	Nodes             []graphNode       `json:"nodes"`
	Edges             []graphEdge       `json:"edges"`
	Relationships     []graphRelation   `json:"relationships"`
	Constraints       []graphConstraint `json:"constraints,omitempty"`
	DependencyDiagram string            `json:"dependencyDiagram,omitempty"`
}

type graphNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type,omitempty"`
	Path     string `json:"path,omitempty"`
	SpecID   string `json:"specId,omitempty"`
	Category string `json:"category,omitempty"`
	Status   string `json:"status,omitempty"`
}

type graphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Label  string `json:"label,omitempty"`
	Type   string `json:"type,omitempty"`
	Origin string `json:"origin,omitempty"`
	Raw    string `json:"raw,omitempty"`
}

type graphRelation struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Description string `json:"description"`
	Section     string `json:"section,omitempty"`
}

type graphConstraint struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Description string `json:"description,omitempty"`
	Raw         string `json:"raw"`
}

type moduleMeta struct {
	Title       string
	Path        string
	Status      string
	Version     string
	Compliance  string
	Priority    string
	Description string
	Type        string
	Tags        []string
	Timestamp   string
	Resource    string
}

type semanticSpecRef struct {
	Raw              string
	Type             string
	Target           string
	Relation         string
	ExplicitRelation bool
	ValidRelation    bool
}

type metadataEntry struct {
	Key   string
	Value string
}

type htmlDocData struct {
	Meta      moduleMeta
	Entries   []metadataEntry
	Relations []htmlDocRelation
	Text      string
	BodyText  string
}

type htmlDocRelation struct {
	Target string
	Type   string
	Raw    string
}

const defaultSpecRelation = "references"

var (
	semanticSpecRefRE    = regexp.MustCompile(`@(doc|spec)/([^\s\)]+)`)
	allowedSpecRelations = map[string]bool{
		"references": true,
		"implements": true,
		"depends":    true,
		"blocks":     true,
		"blocked-by": true,
		"follows":    true,
		"related":    true,
		"supersedes": true,
		"verifies":   true,
		"provides":   true,
		"consumes":   true,
	}
)

func scanSpecProject(projectRoot, docsDir string) (specProject, error) {
	docsRoot := docsRoot(projectRoot, docsDir)
	info, err := os.Stat(docsRoot)
	if err != nil {
		return specProject{}, fmt.Errorf("docs directory not found: %s", docsRoot)
	}
	if !info.IsDir() {
		return specProject{}, fmt.Errorf("docs path is not a directory: %s", docsRoot)
	}

	indexPath := filepath.Join(docsRoot, "_index.md")
	syncPath := filepath.Join(docsRoot, "_sync.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")

	indexRaw := readOptional(indexPath)
	moduleMetaByPath := parseModuleTable(indexRaw)
	sync := parseSyncState(readOptional(syncPath))
	documents, err := scanSpecDocuments(docsRoot, moduleMetaByPath)
	if err != nil {
		return specProject{}, err
	}
	graph := parseSpecGraph(indexRaw, documents)
	summary := buildSummary(projectRoot, docsRoot, agentsPath, indexPath, syncPath, documents, sync)
	return specProject{Summary: summary, Documents: documents, Graph: graph}, nil
}

func docsRoot(projectRoot, docsDir string) string {
	docsDir = internalutil.ExpandPath(docsDir)
	if filepath.IsAbs(docsDir) {
		return filepath.Clean(docsDir)
	}
	return filepath.Join(projectRoot, docsDir)
}

func scanSpecDocuments(root string, table map[string]moduleMeta) ([]specDocument, error) {
	docs := []specDocument{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxSearchFileBytes {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		rawBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(rawBytes) {
			return nil
		}
		raw := string(rawBytes)
		format := documentFormatForPath(path)
		meta := mergeModuleMeta(parseDocumentMeta(rel, raw), table[rel])
		docs = append(docs, specDocument{
			ID:          rel,
			Title:       internalutil.FirstNonEmpty(meta.Title, titleFromDocument(raw, format), rel),
			Path:        rel,
			Language:    languageForPath(path),
			Format:      format,
			Category:    categoryFor(rel),
			Status:      meta.Status,
			Version:     meta.Version,
			Compliance:  meta.Compliance,
			Priority:    meta.Priority,
			Description: meta.Description,
			Type:        meta.Type,
			Tags:        meta.Tags,
			Timestamp:   meta.Timestamp,
			Resource:    meta.Resource,
			Raw:         raw,
			SearchText:  searchTextForDocument(raw, format),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})
	return docs, nil
}

func mergeModuleMeta(docMeta, tableMeta moduleMeta) moduleMeta {
	if tableMeta.Path == "" {
		return docMeta
	}
	return moduleMeta{
		Title:       internalutil.FirstNonEmpty(tableMeta.Title, docMeta.Title),
		Path:        internalutil.FirstNonEmpty(tableMeta.Path, docMeta.Path),
		Status:      internalutil.FirstNonEmpty(tableMeta.Status, docMeta.Status),
		Version:     internalutil.FirstNonEmpty(tableMeta.Version, docMeta.Version),
		Compliance:  internalutil.FirstNonEmpty(tableMeta.Compliance, docMeta.Compliance),
		Priority:    internalutil.FirstNonEmpty(tableMeta.Priority, docMeta.Priority),
		Description: internalutil.FirstNonEmpty(tableMeta.Description, docMeta.Description),
		Type:        internalutil.FirstNonEmpty(tableMeta.Type, docMeta.Type),
		Timestamp:   internalutil.FirstNonEmpty(tableMeta.Timestamp, docMeta.Timestamp),
		Resource:    internalutil.FirstNonEmpty(tableMeta.Resource, docMeta.Resource),
		Tags:        firstNonEmptyTags(tableMeta.Tags, docMeta.Tags),
	}
}

// firstNonEmptyTags returns the first non-empty tag slice from the supplied
// lists, or nil when none have entries.
func firstNonEmptyTags(lists ...[]string) []string {
	for _, list := range lists {
		if len(list) > 0 {
			return list
		}
	}
	return nil
}

func buildSummary(projectRoot, docsRoot, agentsPath, indexPath, syncPath string, docs []specDocument, sync map[string]string) projectSummary {
	categories := map[string]int{}
	status := map[string]int{}
	compliance := map[string]int{}
	for _, doc := range docs {
		categories[doc.Category]++
		if doc.Status != "" {
			status[doc.Status]++
		}
		if doc.Compliance != "" {
			compliance[doc.Compliance]++
		}
	}
	warnings := []string{}
	if !exists(indexPath) {
		warnings = append(warnings, "Missing docs/_index.md; using filesystem scan fallback.")
	}
	if !exists(syncPath) {
		warnings = append(warnings, "Missing docs/_sync.md; sync state is unavailable.")
	}
	if !exists(agentsPath) {
		warnings = append(warnings, "Missing project AGENTS.md.")
	}
	return projectSummary{
		Name:           filepath.Base(projectRoot),
		ProjectRoot:    projectRoot,
		DocsRoot:       docsRoot,
		AgentsPath:     agentsPath,
		AgentsFound:    exists(agentsPath),
		IndexFound:     exists(indexPath),
		SyncFound:      exists(syncPath),
		TotalSpecs:     len(docs),
		Categories:     categories,
		StatusCounts:   status,
		Compliance:     compliance,
		Sync:           sync,
		Warnings:       warnings,
		GeneratedTitle: "Docs Preview",
	}
}

func parseModuleTable(markdown string) map[string]moduleMeta {
	out := map[string]moduleMeta{}
	lines := strings.Split(markdown, "\n")
	inModules := false
	headers := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inModules = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), "Modules")
			continue
		}
		if !inModules || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := markdownTableCells(trimmed)
		if len(cells) == 0 || isMarkdownSeparatorRow(cells) {
			continue
		}
		if len(headers) == 0 {
			headers = normalizeHeaders(cells)
			continue
		}
		row := map[string]string{}
		for i, header := range headers {
			if i < len(cells) {
				row[header] = cells[i]
			}
		}
		path := extractMarkdownLinkTarget(row["spec file"])
		if path == "" {
			path = row["spec file"]
		}
		path = normalizeSpecPath(path)
		if path == "" {
			continue
		}
		out[path] = moduleMeta{
			Title:       stripMarkdown(row["module"]),
			Path:        path,
			Status:      stripMarkdown(row["status"]),
			Version:     stripMarkdown(row["version"]),
			Compliance:  stripMarkdown(row["compliance"]),
			Priority:    stripMarkdown(row["priority"]),
			Description: stripMarkdown(row["description"]),
		}
	}
	return out
}

// parseDocumentMeta resolves a document's metadata by merging two sources, with
// the OKF YAML frontmatter taking precedence over the legacy `## Meta` prose
// block. The merge follows three rules (Requirements 4.1, 4.2, 4.3):
//   - Frontmatter values win for any overlapping key.
//   - The `## Meta` prose only fills fields the frontmatter left empty.
//   - A malformed frontmatter block never panics: it logs a warning and falls
//     back entirely to the `## Meta` prose.
//
// When a document only has a `## Meta` block (no frontmatter), the result is
// identical to the previous behavior.
func parseDocumentMeta(rel, raw string) moduleMeta {
	if documentFormatForPath(rel) == "html" {
		meta := parseHTMLDocumentData(raw).Meta
		meta.Path = rel
		return meta
	}

	base := moduleMeta{Title: titleFromMarkdown(raw), Path: rel}

	// 1. YAML frontmatter (OKF) — highest precedence when present and valid.
	if fm, ok, err := parseFrontmatter(raw); ok {
		if err != nil {
			// Fail-open: malformed frontmatter falls back to `## Meta` (Req 4.3).
			fmt.Fprintf(os.Stderr, "preview: frontmatter parse failed for %s, falling back to ## Meta: %v\n", rel, err)
		} else {
			base = mergeFrontmatterMeta(base, fm)
		}
	}

	// 2. Legacy `## Meta` prose — only fills fields left empty by frontmatter.
	legacy := parseMetaSection(raw)
	base = fillEmptyMeta(base, legacy)

	return base
}

// parseMetaSection parses the legacy `## Meta` prose block into a moduleMeta. It
// operates only on the `## Meta` section (skipping any leading frontmatter) so
// it can be merged independently of the frontmatter. When there is no `## Meta`
// block, the prose scan falls back to the whole document, preserving the
// previous behavior for docs that declared metadata inline.
func parseMetaSection(raw string) moduleMeta {
	meta := moduleMeta{}
	block := metaSectionBlock(raw)
	for _, entry := range metadataEntries(block) {
		switch strings.ToLower(strings.TrimSpace(entry.Key)) {
		case "status":
			meta.Status = stripMarkdown(entry.Value)
		case "version":
			meta.Version = stripMarkdown(entry.Value)
		case "compliance":
			meta.Compliance = stripMarkdown(entry.Value)
		case "priority":
			meta.Priority = stripMarkdown(entry.Value)
		case "description":
			meta.Description = stripMarkdown(entry.Value)
		case "type":
			meta.Type = stripMarkdown(entry.Value)
		case "timestamp":
			meta.Timestamp = stripMarkdown(entry.Value)
		case "resource":
			meta.Resource = stripMarkdown(entry.Value)
		case "tags":
			meta.Tags = parseTagsValue(entry.Value)
		}
	}
	for _, line := range strings.Split(internalutil.FirstNonEmpty(block, raw), "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if strings.Contains(trimmed, "**Status**") {
			meta.Status = valueAfterColon(trimmed)
		}
		if strings.Contains(trimmed, "**Version**") {
			meta.Version = valueAfterColon(trimmed)
		}
		if strings.Contains(trimmed, "**Compliance**") {
			meta.Compliance = valueAfterColon(trimmed)
		}
		if strings.Contains(trimmed, "**Description**") {
			meta.Description = valueAfterColon(trimmed)
		}
		if strings.Contains(trimmed, "**Meta**") {
			meta.Status = internalutil.FirstNonEmpty(meta.Status, betweenAfter(trimmed, "Status"))
			meta.Version = internalutil.FirstNonEmpty(meta.Version, betweenAfter(trimmed, "Version"))
			meta.Compliance = internalutil.FirstNonEmpty(meta.Compliance, betweenAfter(trimmed, "Compliance"))
			meta.Description = internalutil.FirstNonEmpty(meta.Description, betweenAfter(trimmed, "Description"))
		}
	}
	return meta
}

// metaSectionBlock returns the content of the legacy `## Meta` prose section,
// skipping any leading YAML frontmatter so the section can be parsed
// independently of the frontmatter block. When no frontmatter is present this
// returns exactly what metadataBlock would for the `## Meta` section.
func metaSectionBlock(raw string) string {
	lines := strings.Split(raw, "\n")
	// Skip leading frontmatter if present so we only consider `## Meta`.
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				lines = lines[i+1:]
				break
			}
		}
	}
	inMeta := false
	var buf strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := headingLevel(trimmed)
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if level <= 2 && strings.EqualFold(title, "Meta") {
				inMeta = true
				continue
			}
			if inMeta && level <= 2 {
				break
			}
		}
		if inMeta {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

// mergeFrontmatterMeta overlays frontmatter values onto base; frontmatter wins
// for every overlapping key (Req 4.2). Title/Path on base are preserved.
func mergeFrontmatterMeta(base, fm moduleMeta) moduleMeta {
	base.Status = internalutil.FirstNonEmpty(fm.Status, base.Status)
	base.Version = internalutil.FirstNonEmpty(fm.Version, base.Version)
	base.Compliance = internalutil.FirstNonEmpty(fm.Compliance, base.Compliance)
	base.Priority = internalutil.FirstNonEmpty(fm.Priority, base.Priority)
	base.Description = internalutil.FirstNonEmpty(fm.Description, base.Description)
	base.Type = internalutil.FirstNonEmpty(fm.Type, base.Type)
	base.Timestamp = internalutil.FirstNonEmpty(fm.Timestamp, base.Timestamp)
	base.Resource = internalutil.FirstNonEmpty(fm.Resource, base.Resource)
	if len(fm.Tags) > 0 {
		base.Tags = fm.Tags
	}
	return base
}

// fillEmptyMeta fills only the fields left empty on base using values from fill.
// It never overrides a field base already has, so callers can use it to let a
// lower-precedence source (the `## Meta` prose) complete the frontmatter.
func fillEmptyMeta(base, fill moduleMeta) moduleMeta {
	base.Title = internalutil.FirstNonEmpty(base.Title, fill.Title)
	base.Status = internalutil.FirstNonEmpty(base.Status, fill.Status)
	base.Version = internalutil.FirstNonEmpty(base.Version, fill.Version)
	base.Compliance = internalutil.FirstNonEmpty(base.Compliance, fill.Compliance)
	base.Priority = internalutil.FirstNonEmpty(base.Priority, fill.Priority)
	base.Description = internalutil.FirstNonEmpty(base.Description, fill.Description)
	base.Type = internalutil.FirstNonEmpty(base.Type, fill.Type)
	base.Timestamp = internalutil.FirstNonEmpty(base.Timestamp, fill.Timestamp)
	base.Resource = internalutil.FirstNonEmpty(base.Resource, fill.Resource)
	if len(base.Tags) == 0 {
		base.Tags = fill.Tags
	}
	return base
}

// parseTagsValue normalizes a `## Meta` prose tags value (e.g. "a, b, c" or
// "[a, b]") into a []string, returning nil when no usable tags remain.
func parseTagsValue(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	out := []string{}
	for _, part := range strings.Split(value, ",") {
		tag := strings.Trim(strings.TrimSpace(stripMarkdown(part)), "\"'")
		if tag != "" {
			out = append(out, tag)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// frontmatterTags normalizes a YAML `tags` value that may be declared either as
// a single scalar string (`tags: preview`) or as a sequence (`tags: [a, b]`)
// into a flat []string. It is a permissive consumer: malformed shapes yield an
// empty slice instead of an error so frontmatter parsing never fails on tags.
type frontmatterTags []string

func (t *frontmatterTags) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		trimmed := strings.TrimSpace(value.Value)
		if trimmed == "" {
			*t = nil
			return nil
		}
		*t = frontmatterTags{trimmed}
	case yaml.SequenceNode:
		out := make(frontmatterTags, 0, len(value.Content))
		for _, item := range value.Content {
			if item == nil {
				continue
			}
			trimmed := strings.TrimSpace(item.Value)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		*t = out
	default:
		// Permissive consumer: unexpected shapes (mapping, alias, ...) are ignored.
		*t = nil
	}
	return nil
}

// frontmatterMeta is the intermediate struct used to decode the YAML frontmatter
// block. Only known keys are mapped; any unknown key in the frontmatter is
// silently ignored by yaml.Unmarshal (KnownFields is left disabled on purpose so
// the parser stays permissive per the OKF "permissive consumer" principle).
type frontmatterMeta struct {
	Type        string          `yaml:"type"`
	Description string          `yaml:"description"`
	Tags        frontmatterTags `yaml:"tags"`
	Timestamp   string          `yaml:"timestamp"`
	Resource    string          `yaml:"resource"`
	Status      string          `yaml:"status"`
	Version     string          `yaml:"version"`
	Compliance  string          `yaml:"compliance"`
	Priority    string          `yaml:"priority"`
}

// parseFrontmatter parses a leading YAML frontmatter block (delimited by `---`)
// into a moduleMeta. It returns ok=false (and no error) when the document does
// not begin with a frontmatter block, so the caller can fall back to `## Meta`
// prose. When the block exists but the YAML is malformed, it returns ok=true and
// a non-nil error so the caller can log a warning and still fall back.
func parseFrontmatter(raw string) (meta moduleMeta, ok bool, err error) {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return moduleMeta{}, false, nil
	}

	block := metadataBlock(raw)

	var fm frontmatterMeta
	if unmarshalErr := yaml.Unmarshal([]byte(block), &fm); unmarshalErr != nil {
		return moduleMeta{}, true, unmarshalErr
	}

	meta = moduleMeta{
		Type:        strings.TrimSpace(fm.Type),
		Description: strings.TrimSpace(fm.Description),
		Tags:        normalizeTags(fm.Tags),
		Timestamp:   strings.TrimSpace(fm.Timestamp),
		Resource:    strings.TrimSpace(fm.Resource),
		Status:      strings.TrimSpace(fm.Status),
		Version:     strings.TrimSpace(fm.Version),
		Compliance:  strings.TrimSpace(fm.Compliance),
		Priority:    strings.TrimSpace(fm.Priority),
	}
	return meta, true, nil
}

// normalizeTags converts the decoded frontmatter tags into a clean []string,
// returning nil when there are no usable tags.
func normalizeTags(tags frontmatterTags) []string {
	if len(tags) == 0 {
		return nil
	}
	return []string(tags)
}

func documentFormatForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "markdown"
	case ".html", ".htm":
		return "html"
	default:
		return "text"
	}
}

func titleFromDocument(raw, format string) string {
	if format == "html" {
		return parseHTMLDocumentData(raw).Meta.Title
	}
	return titleFromMarkdown(raw)
}

func searchTextForDocument(raw, format string) string {
	if format == "html" {
		return parseHTMLDocumentData(raw).BodyText
	}
	return raw
}

func parseSyncState(markdown string) map[string]string {
	out := map[string]string{}
	inCurrentSync := false
	for _, line := range strings.Split(markdown, "\n") {
		heading := strings.TrimSpace(line)
		if strings.HasPrefix(heading, "## ") {
			inCurrentSync = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(heading, "## ")), "Current Sync")
			continue
		}
		if !inCurrentSync {
			continue
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if !strings.Contains(trimmed, "**") || !strings.Contains(trimmed, ":") {
			continue
		}
		key := regexp.MustCompile(`\*\*([^*]+)\*\*`).FindStringSubmatch(trimmed)
		if len(key) != 2 {
			continue
		}
		out[key[1]] = valueAfterColon(trimmed)
	}
	return out
}

func parseSpecGraph(indexRaw string, docs []specDocument) specGraph {
	nodes := map[string]graphNode{}
	specByModule := map[string]specDocument{}
	docByNodeID := map[string]specDocument{}
	docByPath := map[string]specDocument{}
	dependencyDiagram := fencedBlockAfterHeading(indexRaw, "Dependency Graph")
	diagramLabels := parseMermaidNodeAliases(dependencyDiagram)
	diagramLabelSet := map[string]bool{}
	for _, label := range diagramLabels {
		diagramLabelSet[label] = true
	}
	for _, doc := range docs {
		docByPath[doc.Path] = doc
		docByNodeID[documentGraphID(doc, diagramLabelSet)] = doc
		name := moduleIDFromPath(doc.Path)
		if name == "" {
			continue
		}
		for _, alias := range specAliasesForDoc(name, doc) {
			specByModule[alias] = doc
		}
		nodeID := canonicalSpecNodeID(name, doc, diagramLabelSet)
		nodes[nodeID] = graphNode{ID: nodeID, Label: doc.Title, Type: "doc", Path: doc.Path, SpecID: doc.ID, Category: doc.Category, Status: doc.Status}
	}

	edges := []graphEdge{}
	for _, edge := range parseDependencyEdges(indexRaw) {
		edges = append(edges, edge)
		if _, ok := nodes[edge.From]; !ok {
			nodes[edge.From] = graphNode{ID: edge.From, Label: edge.From, Type: "external"}
		}
		if _, ok := nodes[edge.To]; !ok {
			nodes[edge.To] = graphNode{ID: edge.To, Label: edge.To, Type: "external"}
		}
	}
	for _, edge := range parseRelationshipEdges(indexRaw) {
		edge.From = canonicalGraphEndpoint(edge.From, specByModule, diagramLabelSet)
		edge.To = canonicalGraphEndpoint(edge.To, specByModule, diagramLabelSet)
		edges = append(edges, edge)
	}
	edges = append(edges, parseDocumentConnectionEdges(docs, docByPath, diagramLabelSet)...)
	for _, edge := range edges {
		if _, ok := nodes[edge.From]; !ok {
			nodes[edge.From] = graphNode{ID: edge.From, Label: edge.From, Type: "external"}
		}
		if _, ok := nodes[edge.To]; !ok {
			nodes[edge.To] = graphNode{ID: edge.To, Label: edge.To, Type: "external"}
		}
	}
	constraints := parseForbiddenDependencies(indexRaw)

	list := make([]graphNode, 0, len(nodes))
	for _, node := range nodes {
		if doc, ok := specByModule[node.ID]; ok {
			node.SpecID = doc.ID
			node.Path = doc.Path
			node.Type = internalutil.FirstNonEmpty(node.Type, "doc")
			node.Category = doc.Category
			node.Status = doc.Status
		} else if doc, ok := docByNodeID[node.ID]; ok {
			node.SpecID = doc.ID
			node.Path = doc.Path
			node.Type = internalutil.FirstNonEmpty(node.Type, "doc")
			node.Category = doc.Category
			node.Status = doc.Status
			node.Label = internalutil.FirstNonEmpty(node.Label, doc.Title)
		}
		list = append(list, node)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})
	dedupedEdges := dedupeEdges(edges)
	return specGraph{Nodes: list, Edges: dedupedEdges, Relationships: relationshipsFromEdges(dedupedEdges), Constraints: constraints, DependencyDiagram: dependencyDiagram}
}

func documentGraphID(doc specDocument, diagramLabelSet map[string]bool) string {
	if name := moduleIDFromPath(doc.Path); name != "" {
		return canonicalSpecNodeID(name, doc, diagramLabelSet)
	}
	return strings.TrimSuffix(doc.Path, filepath.Ext(doc.Path))
}

func specAliasesForDoc(name string, doc specDocument) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	add(name)
	add(strings.ReplaceAll(name, "-", "."))
	add(strings.ReplaceAll(name, ".", "-"))
	pathAlias := strings.TrimSuffix(doc.Path, filepath.Ext(doc.Path))
	add(pathAlias)
	add(strings.TrimPrefix(pathAlias, "modules/"))
	add(strings.ToLower(doc.Title))
	add(strings.ToLower(strings.ReplaceAll(doc.Title, " ", "-")))
	add(strings.ToLower(strings.ReplaceAll(doc.Title, " ", ".")))
	addKnownSpecAliases(doc.Path, add)
	return out
}

func addKnownSpecAliases(path string, add func(string)) {
	switch path {
	case "modules/mfe.md":
		add("ww")
		add("web wrappers")
	case "modules/turnstile-captcha.md":
		add("turnstile")
		add("common.libs.captcha")
		add("cloudflare turnstile api")
	case "modules/editorui/_overview.md":
		for _, alias := range []string{
			"editorui.common",
			"editorui.loader",
			"editorui.word",
			"editorui.geo",
			"editorui.diagram",
			"editorui.freedrawing",
			"editorui.math",
			"editorui.magh",
			"editorui.textbox",
			"editorui.image",
			"editorui.composer",
			"commontools",
			"classroomtools",
		} {
			add(alias)
		}
	}
}

func canonicalGraphEndpoint(value string, specs map[string]specDocument, diagramLabelSet map[string]bool) string {
	if doc, ok := specs[value]; ok {
		return canonicalSpecNodeID(moduleIDFromPath(doc.Path), doc, diagramLabelSet)
	}
	lower := strings.ToLower(value)
	if doc, ok := specs[lower]; ok {
		return canonicalSpecNodeID(moduleIDFromPath(doc.Path), doc, diagramLabelSet)
	}
	return value
}

func canonicalSpecNodeID(name string, doc specDocument, diagramLabelSet map[string]bool) string {
	for _, alias := range specAliasesForDoc(name, doc) {
		if diagramLabelSet[alias] {
			return alias
		}
	}
	return name
}

func parseDependencyEdges(markdown string) []graphEdge {
	block := fencedBlockAfterHeading(markdown, "Dependency Graph")
	aliases := parseMermaidNodeAliases(block)
	edges := []graphEdge{}
	for _, line := range strings.Split(block, "\n") {
		clean := cleanGraphLine(line)
		if clean == "" || strings.HasPrefix(clean, "%%") || strings.HasPrefix(strings.ToLower(clean), "flowchart ") || strings.HasPrefix(strings.ToLower(clean), "graph ") || strings.HasPrefix(strings.ToLower(clean), "subgraph ") || strings.EqualFold(clean, "end") {
			continue
		}
		if !strings.Contains(clean, "→") {
			if from, to, ok := parseMermaidEdge(clean, aliases); ok {
				edges = append(edges, graphEdge{From: from, To: to, Label: "depends", Type: "depends", Origin: "index"})
			}
			continue
		}
		parts := strings.SplitN(clean, "→", 2)
		from := cleanNodeName(parts[0])
		for _, target := range strings.Split(parts[1], ",") {
			to := cleanNodeName(target)
			if from != "" && to != "" {
				edges = append(edges, graphEdge{From: from, To: to, Label: "depends", Type: "depends", Origin: "index"})
			}
		}
	}
	return edges
}

func parseMermaidNodeAliases(block string) map[string]string {
	aliases := map[string]string{}
	re := regexp.MustCompile(`([A-Za-z][A-Za-z0-9_.-]*)\s*(?:@\{[^}]*\}\s*)?\[\s*"?([^"\]]+)"?\s*\]`)
	for _, match := range re.FindAllStringSubmatch(block, -1) {
		if len(match) != 3 {
			continue
		}
		alias := cleanNodeName(match[1])
		label := cleanNodeName(match[2])
		if alias != "" && label != "" {
			aliases[alias] = label
		}
	}
	return aliases
}

func parseMermaidEdge(line string, aliases map[string]string) (string, string, bool) {
	for _, arrow := range []string{"-.->", "-->", "==>"} {
		idx := strings.Index(line, arrow)
		if idx < 0 {
			continue
		}
		left := strings.TrimSpace(line[:idx])
		right := strings.TrimSpace(line[idx+len(arrow):])
		if labelIdx := strings.Index(left, " -- "); labelIdx >= 0 {
			left = strings.TrimSpace(left[:labelIdx])
		}
		from := resolveMermaidEndpoint(left, aliases)
		to := resolveMermaidEndpoint(right, aliases)
		return from, to, from != "" && to != ""
	}
	return "", "", false
}

func resolveMermaidEndpoint(value string, aliases map[string]string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ":::"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "["); idx >= 0 {
		alias := cleanNodeName(value[:idx])
		label := extractMermaidInlineLabel(value[idx:])
		if alias != "" && label != "" {
			aliases[alias] = label
			return label
		}
		return alias
	}
	value = strings.Fields(value)[0]
	id := cleanNodeName(value)
	if label, ok := aliases[id]; ok {
		return label
	}
	return id
}

func extractMermaidInlineLabel(value string) string {
	re := regexp.MustCompile(`\[\s*"?([^"\]]+)"?\s*\]`)
	match := re.FindStringSubmatch(value)
	if len(match) == 2 {
		return cleanNodeName(match[1])
	}
	return cleanNodeName(value)
}

func parseRelationshipEdges(markdown string) []graphEdge {
	lines := strings.Split(markdown, "\n")
	inMap := false
	section := ""
	out := []graphEdge{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inMap = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), "Relationship Map")
			continue
		}
		if !inMap {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			section = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") || !strings.Contains(trimmed, "→") {
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		leftRight := strings.SplitN(item, "→", 2)
		if len(leftRight) != 2 {
			continue
		}
		desc := ""
		toPart := leftRight[1]
		if idx := strings.Index(toPart, ":"); idx >= 0 {
			desc = strings.TrimSpace(toPart[idx+1:])
			toPart = toPart[:idx]
		}
		for _, from := range splitNodeList(leftRight[0]) {
			for _, to := range splitNodeList(toPart) {
				if from != "" && to != "" {
					relationType := relationTypeFromText(internalutil.FirstNonEmpty(section, desc, "related"))
					out = append(out, graphEdge{From: from, To: to, Label: internalutil.FirstNonEmpty(desc, relationType), Type: relationType, Origin: "relationship-map", Raw: item})
				}
			}
		}
	}
	return out
}

func parseDocumentConnectionEdges(docs []specDocument, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	out := []graphEdge{}
	for _, doc := range docs {
		if isSpecControlFile(doc.Path) {
			continue
		}
		from := documentGraphID(doc, diagramLabelSet)
		out = append(out, parseDocumentMetadataEdges(doc, from, docByPath, diagramLabelSet)...)
		out = append(out, parseDocumentContentEdges(doc, from, docByPath, diagramLabelSet)...)
	}
	return out
}

func isSpecControlFile(path string) bool {
	base := filepath.Base(path)
	return base == "_index.md" || base == "_sync.md"
}

func parseDocumentMetadataEdges(doc specDocument, from string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	if doc.Format == "html" {
		data := parseHTMLDocumentData(doc.Raw)
		edges := []graphEdge{}
		for _, entry := range data.Entries {
			if !isSpecLinkMetadataKey(entry.Key) {
				continue
			}
			relationType := relationTypeFromText(entry.Key)
			edges = append(edges, edgesFromSemanticReferences(doc.Path, from, entry.Value, "metadata", docByPath, diagramLabelSet)...)
			edges = append(edges, edgesFromMarkdownLinks(doc.Path, from, entry.Value, relationType, "metadata", docByPath, diagramLabelSet)...)
			edges = append(edges, edgesFromPlainDocPaths(doc.Path, from, entry.Value, relationType, "metadata", docByPath, diagramLabelSet)...)
		}
		for _, relation := range data.Relations {
			if target, ok := resolveSpecReference(doc.Path, relation.Target, docByPath, diagramLabelSet); ok && from != target {
				edges = append(edges, graphEdge{From: from, To: target, Label: relation.Type, Type: relation.Type, Origin: "metadata", Raw: internalutil.FirstNonEmpty(relation.Raw, relation.Target)})
			}
		}
		return dedupeEdges(edges)
	}
	meta := metadataBlock(doc.Raw)
	if meta == "" {
		return nil
	}
	edges := edgesFromMarkdownLinks(doc.Path, from, meta, "related", "metadata", docByPath, diagramLabelSet)
	edges = append(edges, edgesFromSemanticReferences(doc.Path, from, meta, "metadata", docByPath, diagramLabelSet)...)
	for _, entry := range metadataEntries(meta) {
		if !isSpecLinkMetadataKey(entry.Key) {
			continue
		}
		relationType := relationTypeFromText(entry.Key)
		edges = append(edges, edgesFromSemanticReferences(doc.Path, from, entry.Value, "metadata", docByPath, diagramLabelSet)...)
		edges = append(edges, edgesFromMarkdownLinks(doc.Path, from, entry.Value, relationType, "metadata", docByPath, diagramLabelSet)...)
		edges = append(edges, edgesFromPlainDocPaths(doc.Path, from, entry.Value, relationType, "metadata", docByPath, diagramLabelSet)...)
	}
	return dedupeEdges(edges)
}

func parseDocumentContentEdges(doc specDocument, from string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	content := contentWithoutMetadata(doc.Raw)
	if doc.Format == "html" {
		content = htmlContentWithoutMetadata(doc.Raw)
	}
	edges := edgesFromSemanticReferences(doc.Path, from, content, "inline", docByPath, diagramLabelSet)
	edges = append(edges, edgesFromMarkdownLinks(doc.Path, from, content, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	edges = append(edges, edgesFromPlainDocPaths(doc.Path, from, content, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	if doc.Format == "html" {
		edges = append(edges, edgesFromHTMLLinks(doc.Path, from, doc.Raw, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	}
	return dedupeEdges(edges)
}

func metadataBlock(raw string) string {
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		var buf strings.Builder
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) == "---" {
				return buf.String()
			}
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	inMeta := false
	var buf strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := headingLevel(trimmed)
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if level <= 2 && strings.EqualFold(title, "Meta") {
				inMeta = true
				continue
			}
			if inMeta && level <= 2 {
				break
			}
		}
		if inMeta {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

func contentWithoutMetadata(raw string) string {
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i, line := range lines[1:] {
			if strings.TrimSpace(line) == "---" {
				return strings.Join(lines[i+2:], "\n")
			}
		}
	}
	inMeta := false
	out := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := headingLevel(trimmed)
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if level <= 2 && strings.EqualFold(title, "Meta") {
				inMeta = true
				continue
			}
			if inMeta && level <= 2 {
				inMeta = false
			}
		}
		if !inMeta {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func htmlContentWithoutMetadata(raw string) string {
	return parseHTMLDocumentData(raw).BodyText
}

func parseHTMLDocumentData(raw string) htmlDocData {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		text := strings.Join(strings.Fields(raw), " ")
		return htmlDocData{Text: text, BodyText: text}
	}
	data := htmlDocData{}
	var textParts []string
	var bodyParts []string
	var firstHeading string
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.TextNode {
			return
		}
		if insideHTMLTag(node, "script") || insideHTMLTag(node, "style") {
			return
		}
		text := strings.TrimSpace(node.Data)
		if text == "" {
			return
		}
		textParts = append(textParts, text)
		if !insideHTMLTag(node, "doc-meta") {
			bodyParts = append(bodyParts, text)
		}
	})
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch strings.ToLower(node.Data) {
		case "doc-meta":
			data.Meta = htmlDocMeta(node)
			data.Entries = htmlDocMetadataEntries(node, data.Meta)
		case "doc-relation":
			relation := htmlRelationFromNode(node)
			if relation.Target != "" {
				data.Relations = append(data.Relations, relation)
			}
		case "a":
			if insideHTMLTag(node, "doc-meta") {
				relation := htmlRelationFromNode(node)
				if relation.Target != "" {
					data.Relations = append(data.Relations, relation)
				}
			}
		case "h1":
			if firstHeading == "" && !insideHTMLTag(node, "doc-meta") {
				firstHeading = compactAllWhitespace(htmlNodeText(node))
			}
		case "title":
			if data.Meta.Title == "" {
				data.Meta.Title = compactAllWhitespace(htmlNodeText(node))
			}
		}
	})
	data.Meta.Title = internalutil.FirstNonEmpty(data.Meta.Title, firstHeading)
	data.Text = compactAllWhitespace(strings.Join(textParts, " "))
	data.BodyText = compactAllWhitespace(strings.Join(bodyParts, " "))
	return data
}

func compactAllWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func walkHTML(node *html.Node, visit func(*html.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func htmlDocMeta(node *html.Node) moduleMeta {
	meta := moduleMeta{
		Status:     htmlAttr(node, "status"),
		Version:    htmlAttr(node, "version"),
		Compliance: htmlAttr(node, "compliance"),
		Priority:   htmlAttr(node, "priority"),
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		text := compactAllWhitespace(htmlNodeText(child))
		switch strings.ToLower(child.Data) {
		case "doc-title":
			meta.Title = text
		case "doc-description":
			meta.Description = text
		}
	}
	return meta
}

func htmlDocMetadataEntries(metaNode *html.Node, meta moduleMeta) []metadataEntry {
	entries := []metadataEntry{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			entries = append(entries, metadataEntry{Key: key, Value: strings.TrimSpace(value)})
		}
	}
	add("Status", meta.Status)
	add("Version", meta.Version)
	add("Compliance", meta.Compliance)
	add("Priority", meta.Priority)
	add("Description", meta.Description)
	for child := metaNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if !strings.EqualFold(child.Data, "a") {
			continue
		}
		href := htmlAttr(child, "href")
		if href == "" {
			continue
		}
		relationType := internalutil.FirstNonEmpty(htmlAttr(child, "type"), defaultSpecRelation)
		add(relationType, href)
	}
	return entries
}

func htmlRelationFromNode(node *html.Node) htmlDocRelation {
	target := internalutil.FirstNonEmpty(htmlAttr(node, "target"), htmlAttr(node, "href"))
	relationType := internalutil.FirstNonEmpty(htmlAttr(node, "type"), defaultSpecRelation)
	if !allowedSpecRelations[relationType] {
		relationType = defaultSpecRelation
	}
	return htmlDocRelation{Target: target, Type: relationType, Raw: compactAllWhitespace(htmlNodeText(node))}
}

func htmlAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func htmlNodeText(node *html.Node) string {
	parts := []string{}
	walkHTML(node, func(next *html.Node) {
		if next.Type == html.TextNode && !insideHTMLTag(next, "script") && !insideHTMLTag(next, "style") {
			parts = append(parts, strings.TrimSpace(next.Data))
		}
	})
	return strings.Join(parts, " ")
}

func insideHTMLTag(node *html.Node, tag string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && strings.EqualFold(parent.Data, tag) {
			return true
		}
	}
	return false
}

func headingLevel(line string) int {
	level := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		level++
	}
	return level
}

func splitMetadataLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(strings.TrimLeft(line, "-* "))
	if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, ":") {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, ":", 2)
	key := stripMarkdown(strings.TrimSpace(parts[0]))
	return key, strings.TrimSpace(parts[1]), key != ""
}

func metadataEntries(block string) []metadataEntry {
	out := []metadataEntry{}
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := splitMetadataLine(line)
		if ok {
			out = append(out, metadataEntry{Key: key, Value: value})
		}
	}
	out = append(out, metadataTableEntries(block)...)
	return out
}

func metadataTableEntries(block string) []metadataEntry {
	out := []metadataEntry{}
	lines := strings.Split(block, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		header := markdownTableCells(trimmed)
		if len(header) == 0 || isMarkdownSeparatorRow(header) {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) || !isMarkdownSeparatorRow(markdownTableCells(strings.TrimSpace(lines[j]))) {
			continue
		}
		headers := normalizeHeaders(header)
		for j++; j < len(lines); j++ {
			rowLine := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(rowLine, "|") {
				break
			}
			cells := markdownTableCells(rowLine)
			if isMarkdownSeparatorRow(cells) {
				continue
			}
			if isKeyValueMetadataTable(headers) {
				if len(cells) >= 2 {
					out = append(out, metadataEntry{Key: stripMarkdown(cells[0]), Value: strings.TrimSpace(cells[1])})
				}
				continue
			}
			for index, header := range headers {
				if index < len(cells) && header != "" {
					out = append(out, metadataEntry{Key: header, Value: strings.TrimSpace(cells[index])})
				}
			}
		}
		i = j - 1
	}
	return out
}

func isKeyValueMetadataTable(headers []string) bool {
	if len(headers) < 2 {
		return false
	}
	keyHeader := headers[0]
	valueHeader := headers[1]
	return (keyHeader == "key" || keyHeader == "field" || keyHeader == "property" || keyHeader == "meta" || keyHeader == "metadata") &&
		(valueHeader == "value" || valueHeader == "values" || valueHeader == "description")
}

func isSpecLinkMetadataKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, "-", " ")
	for _, token := range []string{
		"link", "links", "related", "related specs", "spec links",
		"dependency", "dependencies", "depends", "depends on",
		"implements", "blocks", "blocked by", "follows", "supersedes", "verifies", "consumes", "provides",
	} {
		if key == token {
			return true
		}
	}
	return strings.Contains(key, "spec") && (strings.Contains(key, "link") || strings.Contains(key, "related"))
}

func edgesFromMarkdownLinks(sourcePath, from, text, relationType, origin string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	out := []graphEdge{}
	re := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		if len(match) != 2 {
			continue
		}
		if target, ok := resolveSpecReference(sourcePath, match[1], docByPath, diagramLabelSet); ok {
			if from != target {
				out = append(out, graphEdge{From: from, To: target, Label: relationType, Type: relationType, Origin: origin, Raw: match[0]})
			}
		}
	}
	return out
}

func edgesFromSemanticReferences(sourcePath, from, text, origin string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	out := []graphEdge{}
	for _, ref := range extractSemanticSpecRefs(text) {
		if !ref.ValidRelation {
			continue
		}
		if target, ok := resolveSpecReference(sourcePath, ref.Target, docByPath, diagramLabelSet); ok && from != target {
			out = append(out, graphEdge{From: from, To: target, Label: ref.Relation, Type: ref.Relation, Origin: origin, Raw: ref.Raw})
		}
	}
	return out
}

func edgesFromPlainDocPaths(sourcePath, from, text, relationType, origin string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	out := []graphEdge{}
	for _, token := range extractPlainSpecPathRefs(text) {
		if target, ok := resolveSpecReference(sourcePath, token, docByPath, diagramLabelSet); ok && from != target {
			out = append(out, graphEdge{From: from, To: target, Label: relationType, Type: relationType, Origin: origin, Raw: token})
		}
	}
	return out
}

func edgesFromHTMLLinks(sourcePath, from, raw, relationType, origin string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	out := []graphEdge{}
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return out
	}
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || !strings.EqualFold(node.Data, "a") {
			return
		}
		if insideHTMLTag(node, "doc-meta") {
			return
		}
		href := htmlAttr(node, "href")
		if href == "" {
			return
		}
		if target, ok := resolveSpecReference(sourcePath, href, docByPath, diagramLabelSet); ok && from != target {
			out = append(out, graphEdge{From: from, To: target, Label: relationType, Type: relationType, Origin: origin, Raw: href})
		}
	})
	return out
}

func extractSemanticSpecRefs(text string) []semanticSpecRef {
	out := []semanticSpecRef{}
	for _, match := range semanticSpecRefRE.FindAllStringSubmatch(text, -1) {
		if len(match) != 3 {
			continue
		}
		raw := strings.TrimRight(match[0], ".,;")
		body := strings.TrimRight(match[2], ".,;")
		relation := defaultSpecRelation
		explicitRelation := false
		if open := strings.LastIndex(body, "{"); open >= 0 && strings.HasSuffix(body, "}") {
			relation = body[open+1 : len(body)-1]
			body = body[:open]
			explicitRelation = true
		}
		body = trimDocFragment(body)
		if body == "" {
			continue
		}
		out = append(out, semanticSpecRef{
			Raw:              raw,
			Type:             match[1],
			Target:           body,
			Relation:         relation,
			ExplicitRelation: explicitRelation,
			ValidRelation:    allowedSpecRelations[relation],
		})
	}
	return out
}

func extractPlainSpecPathRefs(text string) []string {
	re := regexp.MustCompile(`(?:^|[\s(["'])((?:\.{1,2}/|specs/)?[A-Za-z0-9_./-]+\.md)(?:#[A-Za-z0-9_-]+)?`)
	out := []string{}
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		if len(match) == 2 {
			out = append(out, match[1])
		}
	}
	return internalutil.UniqueStrings(out)
}

func trimDocFragment(value string) string {
	if idx := strings.IndexAny(value, "?#"); idx >= 0 {
		value = value[:idx]
	}
	if match := regexp.MustCompile(`:\d+(?:-\d+)?$`).FindString(value); match != "" {
		value = strings.TrimSuffix(value, match)
	}
	return strings.TrimSpace(value)
}

func relationTypeFromText(value string) string {
	value = strings.ToLower(stripMarkdown(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	switch {
	case strings.Contains(value, "implement"):
		return "implements"
	case strings.Contains(value, "depend"):
		return "depends"
	case strings.Contains(value, "block"):
		return "blocked-by"
	case strings.Contains(value, "follow"):
		return "follows"
	case strings.Contains(value, "supersede"):
		return "supersedes"
	case strings.Contains(value, "verif") || strings.Contains(value, "test"):
		return "verifies"
	case strings.Contains(value, "provide"):
		return "provides"
	case strings.Contains(value, "consume"):
		return "consumes"
	default:
		return "related"
	}
}

func resolveSpecReference(sourcePath, target string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) (string, bool) {
	target = strings.TrimSpace(stripMarkdown(target))
	if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
		return "", false
	}
	if idx := strings.IndexAny(target, "?#"); idx >= 0 {
		target = target[:idx]
	}
	candidates := []string{target}
	if !strings.HasSuffix(target, ".md") && !strings.Contains(filepath.Base(target), ".") {
		candidates = append(candidates, target+".md")
	}
	if !strings.HasPrefix(target, "/") {
		candidates = append(candidates, filepath.ToSlash(filepath.Join(filepath.Dir(sourcePath), target)))
		if !strings.HasSuffix(target, ".md") && !strings.Contains(filepath.Base(target), ".") {
			candidates = append(candidates, filepath.ToSlash(filepath.Join(filepath.Dir(sourcePath), target+".md")))
		}
	}
	for _, candidate := range candidates {
		cleanCandidate := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(candidate, "/")))
		if doc, ok := docByPath[cleanCandidate]; ok {
			return documentGraphID(doc, diagramLabelSet), true
		}
		path := normalizeSpecPath(cleanCandidate)
		if doc, ok := docByPath[path]; ok {
			return documentGraphID(doc, diagramLabelSet), true
		}
	}
	return "", false
}

func splitNodeList(value string) []string {
	out := []string{}
	for _, part := range strings.Split(value, ",") {
		node := cleanNodeName(part)
		if node != "" {
			out = append(out, node)
		}
	}
	return out
}

func parseForbiddenDependencies(markdown string) []graphConstraint {
	lines := strings.Split(markdown, "\n")
	inDependencyGraph := false
	inForbidden := false
	out := []graphConstraint{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inDependencyGraph = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), "Dependency Graph")
			inForbidden = false
			continue
		}
		if !inDependencyGraph {
			continue
		}
		if strings.EqualFold(strings.TrimSuffix(trimmed, ":"), "FORBIDDEN") {
			inForbidden = true
			continue
		}
		if !inForbidden || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		from, to, desc := parseForbiddenLine(raw)
		out = append(out, graphConstraint{From: from, To: to, Description: desc, Raw: raw})
	}
	return out
}

func parseForbiddenLine(raw string) (string, string, string) {
	desc := ""
	if start := strings.Index(raw, "("); start >= 0 {
		if end := strings.LastIndex(raw, ")"); end > start {
			desc = strings.TrimSpace(raw[start+1 : end])
			raw = strings.TrimSpace(raw[:start])
		}
	}
	for _, arrow := range []string{"->", "→"} {
		if idx := strings.Index(raw, arrow); idx >= 0 {
			to, inlineDesc := splitConstraintTarget(raw[idx+len(arrow):])
			if inlineDesc != "" {
				desc = strings.TrimSpace(internalutil.FirstNonEmpty(desc, inlineDesc))
			}
			return cleanConstraintNode(raw[:idx]), to, desc
		}
	}
	return "", "", desc
}

func splitConstraintTarget(value string) (string, string) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return "", ""
	}
	return cleanConstraintNode(fields[0]), strings.Join(fields[1:], " ")
}

func cleanConstraintNode(value string) string {
	value = cleanGraphLine(value)
	value = strings.Trim(value, "`_ ")
	value = strings.TrimSuffix(value, ".md")
	return strings.TrimSpace(value)
}

func renderMarkdown(data []byte) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	if err := md.Convert(data, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func readOptional(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func titleFromMarkdown(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func categoryFor(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) <= 1 {
		return "root"
	}
	return parts[0]
}

func moduleIDFromPath(rel string) string {
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	if base == "_index" || base == "_sync" || base == "overview" {
		return ""
	}
	if base == "_overview" {
		return strings.Trim(strings.TrimSuffix(filepath.Dir(rel), "."), "/")
	}
	return strings.ReplaceAll(base, "_", "-")
}

func markdownTableCells(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func normalizeHeaders(cells []string) []string {
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = strings.ToLower(stripMarkdown(cell))
	}
	return out
}

func isMarkdownSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		for _, r := range cell {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return true
}

func extractMarkdownLinkTarget(value string) string {
	re := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	match := re.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func normalizeSpecPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "specs/")
	path = strings.TrimPrefix(path, "docs/")
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." {
		return ""
	}
	if strings.HasSuffix(path, "/") {
		path += "_overview.md"
	}
	if !strings.HasSuffix(path, ".md") && !strings.Contains(filepath.Base(path), ".") {
		path = strings.TrimSuffix(path, "/") + "/_overview.md"
	}
	return path
}

func stripMarkdown(value string) string {
	value = strings.ReplaceAll(value, "**", "")
	value = strings.ReplaceAll(value, "`", "")
	value = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`).ReplaceAllString(value, "$1")
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return strings.TrimSpace(value)
}

func valueAfterColon(value string) string {
	idx := strings.Index(value, ":")
	if idx < 0 {
		return ""
	}
	return stripMarkdown(strings.TrimSpace(value[idx+1:]))
}

func betweenAfter(value, marker string) string {
	idx := strings.Index(strings.ToLower(value), strings.ToLower(marker))
	if idx < 0 {
		return ""
	}
	return valueAfterColon(value[idx:])
}

func fencedBlockAfterHeading(markdown, heading string) string {
	lines := strings.Split(markdown, "\n")
	inHeading := false
	inFence := false
	var buf strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inHeading = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), heading)
			inFence = false
			continue
		}
		if !inHeading {
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				break
			}
			inFence = true
			continue
		}
		if inFence {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

func cleanGraphLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "├└─│ \t")
	return strings.TrimSpace(line)
}

func cleanNodeName(value string) string {
	value = cleanGraphLine(value)
	if idx := strings.Index(value, "("); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, ":"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`*_ ")
	value = strings.TrimSuffix(value, ".md")
	return value
}

func dedupeEdges(edges []graphEdge) []graphEdge {
	seen := map[string]bool{}
	out := []graphEdge{}
	for _, edge := range edges {
		key := edge.From + "\x00" + edge.To + "\x00" + edge.Label
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, edge)
	}
	return out
}

func dedupeRelationships(relationships []graphRelation) []graphRelation {
	seen := map[string]bool{}
	out := []graphRelation{}
	for _, rel := range relationships {
		key := rel.From + "\x00" + rel.To + "\x00" + rel.Description + "\x00" + rel.Section
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, rel)
	}
	return out
}

func relationshipsFromEdges(edges []graphEdge) []graphRelation {
	out := []graphRelation{}
	for _, edge := range edges {
		if edge.From == "" || edge.To == "" {
			continue
		}
		if edge.Type == "depends" && edge.Origin == "index" {
			continue
		}
		out = append(out, graphRelation{
			From:        edge.From,
			To:          edge.To,
			Description: internalutil.FirstNonEmpty(edge.Label, edge.Type, defaultSpecRelation),
			Section:     edge.Origin,
		})
	}
	return dedupeRelationships(out)
}
