package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type specProject struct {
	Summary   projectSummary `json:"summary"`
	Documents []specDocument `json:"documents"`
	Graph     specGraph      `json:"graph"`
}

type projectSummary struct {
	Name           string            `json:"name"`
	ProjectRoot    string            `json:"projectRoot"`
	SpecsRoot      string            `json:"specsRoot"`
	AgentsPath     string            `json:"agentsPath"`
	AgentsFound    bool              `json:"agentsFound"`
	IndexFound     bool              `json:"indexFound"`
	SyncFound      bool              `json:"syncFound"`
	OverviewFound  bool              `json:"overviewFound"`
	TotalSpecs     int               `json:"totalSpecs"`
	Categories     map[string]int    `json:"categories"`
	StatusCounts   map[string]int    `json:"statusCounts"`
	Compliance     map[string]int    `json:"compliance"`
	Sync           map[string]string `json:"sync"`
	Warnings       []string          `json:"warnings"`
	GeneratedTitle string            `json:"generatedTitle"`
}

type specDocument struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Path       string `json:"path"`
	Category   string `json:"category"`
	Status     string `json:"status,omitempty"`
	Version    string `json:"version,omitempty"`
	Compliance string `json:"compliance,omitempty"`
	Priority   string `json:"priority,omitempty"`
	Raw        string `json:"raw,omitempty"`
	HTML       string `json:"html,omitempty"`
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
	Title      string
	Path       string
	Status     string
	Version    string
	Compliance string
	Priority   string
}

type semanticSpecRef struct {
	Raw              string
	Type             string
	Target           string
	Relation         string
	ExplicitRelation bool
	ValidRelation    bool
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

func scanSpecProject(projectRoot, specsDir string) (specProject, error) {
	specsRoot := specsRoot(projectRoot, specsDir)
	info, err := os.Stat(specsRoot)
	if err != nil {
		return specProject{}, fmt.Errorf("specs directory not found: %s", specsRoot)
	}
	if !info.IsDir() {
		return specProject{}, fmt.Errorf("specs path is not a directory: %s", specsRoot)
	}

	indexPath := filepath.Join(specsRoot, "_index.md")
	syncPath := filepath.Join(specsRoot, "_sync.md")
	overviewPath := filepath.Join(specsRoot, "overview.md")
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")

	indexRaw := readOptional(indexPath)
	moduleMetaByPath := parseModuleTable(indexRaw)
	sync := parseSyncState(readOptional(syncPath))
	documents, err := scanSpecDocuments(specsRoot, moduleMetaByPath)
	if err != nil {
		return specProject{}, err
	}
	graph := parseSpecGraph(indexRaw, documents)
	summary := buildSummary(projectRoot, specsRoot, agentsPath, indexPath, syncPath, overviewPath, documents, sync)
	return specProject{Summary: summary, Documents: documents, Graph: graph}, nil
}

func specsRoot(projectRoot, specsDir string) string {
	specsDir = expandPath(specsDir)
	if filepath.IsAbs(specsDir) {
		return filepath.Clean(specsDir)
	}
	return filepath.Join(projectRoot, specsDir)
}

func scanSpecDocuments(specsRoot string, table map[string]moduleMeta) ([]specDocument, error) {
	docs := []specDocument{}
	err := filepath.WalkDir(specsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(specsRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		rawBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		raw := string(rawBytes)
		meta := table[rel]
		if meta.Path == "" {
			meta = parseDocumentMeta(rel, raw)
		}
		docs = append(docs, specDocument{
			ID:         rel,
			Title:      firstNonEmpty(meta.Title, titleFromMarkdown(raw), rel),
			Path:       rel,
			Category:   categoryFor(rel),
			Status:     meta.Status,
			Version:    meta.Version,
			Compliance: meta.Compliance,
			Priority:   meta.Priority,
			Raw:        raw,
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

func buildSummary(projectRoot, specsRoot, agentsPath, indexPath, syncPath, overviewPath string, docs []specDocument, sync map[string]string) projectSummary {
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
		warnings = append(warnings, "Missing specs/_index.md; using filesystem scan fallback.")
	}
	if !exists(syncPath) {
		warnings = append(warnings, "Missing specs/_sync.md; sync state is unavailable.")
	}
	if !exists(agentsPath) {
		warnings = append(warnings, "Missing project AGENTS.md.")
	}
	return projectSummary{
		Name:           filepath.Base(projectRoot),
		ProjectRoot:    projectRoot,
		SpecsRoot:      specsRoot,
		AgentsPath:     agentsPath,
		AgentsFound:    exists(agentsPath),
		IndexFound:     exists(indexPath),
		SyncFound:      exists(syncPath),
		OverviewFound:  exists(overviewPath),
		TotalSpecs:     len(docs),
		Categories:     categories,
		StatusCounts:   status,
		Compliance:     compliance,
		Sync:           sync,
		Warnings:       warnings,
		GeneratedTitle: "Spec Preview",
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
			Title:      stripMarkdown(row["module"]),
			Path:       path,
			Status:     stripMarkdown(row["status"]),
			Version:    stripMarkdown(row["version"]),
			Compliance: stripMarkdown(row["compliance"]),
			Priority:   stripMarkdown(row["priority"]),
		}
	}
	return out
}

func parseDocumentMeta(rel, raw string) moduleMeta {
	meta := moduleMeta{Title: titleFromMarkdown(raw), Path: rel}
	for _, line := range strings.Split(raw, "\n") {
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
		if strings.Contains(trimmed, "**Meta**") {
			meta.Status = firstNonEmpty(meta.Status, betweenAfter(trimmed, "Status"))
			meta.Version = firstNonEmpty(meta.Version, betweenAfter(trimmed, "Version"))
			meta.Compliance = firstNonEmpty(meta.Compliance, betweenAfter(trimmed, "Compliance"))
		}
	}
	return meta
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
			node.Type = firstNonEmpty(node.Type, "doc")
			node.Category = doc.Category
			node.Status = doc.Status
		} else if doc, ok := docByNodeID[node.ID]; ok {
			node.SpecID = doc.ID
			node.Path = doc.Path
			node.Type = firstNonEmpty(node.Type, "doc")
			node.Category = doc.Category
			node.Status = doc.Status
			node.Label = firstNonEmpty(node.Label, doc.Title)
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
	return strings.TrimSuffix(doc.Path, ".md")
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
	pathAlias := strings.TrimSuffix(doc.Path, ".md")
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
					relationType := relationTypeFromText(firstNonEmpty(section, desc, "related"))
					out = append(out, graphEdge{From: from, To: to, Label: firstNonEmpty(desc, relationType), Type: relationType, Origin: "relationship-map", Raw: item})
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
	meta := metadataBlock(doc.Raw)
	if meta == "" {
		return nil
	}
	edges := edgesFromMarkdownLinks(doc.Path, from, meta, "related", "metadata", docByPath, diagramLabelSet)
	edges = append(edges, edgesFromSemanticReferences(doc.Path, from, meta, "metadata", docByPath, diagramLabelSet)...)
	for _, line := range strings.Split(meta, "\n") {
		key, value, ok := splitMetadataLine(line)
		if !ok || !isSpecLinkMetadataKey(key) {
			continue
		}
		relationType := relationTypeFromText(key)
		edges = append(edges, edgesFromMarkdownLinks(doc.Path, from, value, relationType, "metadata", docByPath, diagramLabelSet)...)
		edges = append(edges, edgesFromPlainDocPaths(doc.Path, from, value, relationType, "metadata", docByPath, diagramLabelSet)...)
	}
	return dedupeEdges(edges)
}

func parseDocumentContentEdges(doc specDocument, from string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	content := contentWithoutMetadata(doc.Raw)
	edges := edgesFromSemanticReferences(doc.Path, from, content, "inline", docByPath, diagramLabelSet)
	edges = append(edges, edgesFromMarkdownLinks(doc.Path, from, content, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	edges = append(edges, edgesFromPlainDocPaths(doc.Path, from, content, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
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

func isSpecLinkMetadataKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, "-", " ")
	for _, token := range []string{"link", "links", "related", "related specs", "spec links", "dependencies", "depends on", "consumes", "provides"} {
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
	return uniqueStrings(out)
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
				desc = strings.TrimSpace(firstNonEmpty(desc, inlineDesc))
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
	if strings.HasSuffix(rel, "_overview.md") {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
			Description: firstNonEmpty(edge.Label, edge.Type, defaultSpecRelation),
			Section:     edge.Origin,
		})
	}
	return dedupeRelationships(out)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
