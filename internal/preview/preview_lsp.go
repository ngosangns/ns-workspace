package preview

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	lspRequestTimeout      = 6 * time.Second
	lspRelationTimeout     = 3 * time.Second
	lspMaxIndexedFileBytes = maxSearchFileBytes
)

type previewLSPCodeGraphProvider struct {
	projectRoot string
	docsRoot    string
	manager     *previewLSPManager

	mu    sync.Mutex
	token string
	index lspCodeGraphIndex
}

type lspCodeGraphIndex struct {
	Nodes     map[string]lspCodeNode
	ByPath    map[string][]string
	Warnings  []string
	Supported bool
}

type lspCodeNode struct {
	ID             string
	Name           string
	FullName       string
	Owner          string
	Kind           int
	KindLabel      string
	LanguageID     string
	ServerID       string
	Path           string
	AbsPath        string
	Range          lspRange
	SelectionRange lspRange
}

type lspCodeGraphCandidate struct {
	ID        string
	Node      lspCodeNode
	Title     string
	Path      string
	Score     float64
	Exactness int
}

type lspCodeEdge struct {
	Source   string
	Target   string
	Relation string
	SourceID string
	TargetID string
}

type lspLanguage struct {
	ServerID   string
	LanguageID string
	Name       string
	Command    string
	Args       []string
}

type lspSourceFile struct {
	Rel        string
	Abs        string
	Language   lspLanguage
	Size       int64
	ModifiedNS int64
}

type previewLSPManager struct {
	root    string
	mu      sync.Mutex
	servers map[string]*previewLSPServer
}

type previewLSPServer struct {
	root    string
	lang    lspLanguage
	command string
	args    []string

	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	reader      *bufio.Reader
	nextID      atomic.Int64
	running     bool
	initialized bool
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type lspDocumentSymbol struct {
	Name           string              `json:"name"`
	Detail         string              `json:"detail,omitempty"`
	Kind           int                 `json:"kind"`
	Range          lspRange            `json:"range"`
	SelectionRange lspRange            `json:"selectionRange"`
	Children       []lspDocumentSymbol `json:"children,omitempty"`
	ContainerName  string              `json:"containerName,omitempty"`
}

type lspCallHierarchyItem struct {
	Name           string   `json:"name"`
	Kind           int      `json:"kind"`
	URI            string   `json:"uri"`
	Range          lspRange `json:"range"`
	SelectionRange lspRange `json:"selectionRange"`
}

type lspIncomingCall struct {
	From       lspCallHierarchyItem `json:"from"`
	FromRanges []lspRange           `json:"fromRanges"`
}

type lspOutgoingCall struct {
	To         lspCallHierarchyItem `json:"to"`
	FromRanges []lspRange           `json:"fromRanges"`
}

func newPreviewLSPCodeGraphProvider(projectRoot, docsRoot string) *previewLSPCodeGraphProvider {
	return &previewLSPCodeGraphProvider{
		projectRoot: projectRoot,
		docsRoot:    docsRoot,
		manager:     newPreviewLSPManager(projectRoot),
	}
}

func (p *previewLSPCodeGraphProvider) Close(ctx context.Context) error {
	if p == nil || p.manager == nil {
		return nil
	}
	return p.manager.Close(ctx)
}

func (p *previewLSPCodeGraphProvider) SearchCodeGraph(ctx context.Context, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) ([]previewSearchResult, []string) {
	if p == nil {
		return nil, nil
	}
	index, warnings := p.cachedIndex(ctx)
	if len(index.Nodes) == 0 {
		return nil, warnings
	}
	results, resultWarnings := searchLSPCodeGraph(ctx, p, index, query, tokens, exclusionQuery, exclusionTokens, limit)
	warnings = append(warnings, resultWarnings...)
	return results, warnings
}

func (p *previewLSPCodeGraphProvider) cachedIndex(ctx context.Context) (lspCodeGraphIndex, []string) {
	token, files, warnings := lspSourceFiles(p.projectRoot, p.docsRoot)
	p.mu.Lock()
	if token != "" && token == p.token {
		index := p.index
		p.mu.Unlock()
		return index, uniqueStrings(append(append([]string{}, index.Warnings...), warnings...))
	}
	p.mu.Unlock()

	buildCtx, cancel := context.WithTimeout(ctx, lspRequestTimeout)
	defer cancel()
	index := p.buildIndex(buildCtx, files)
	index.Warnings = append(index.Warnings, warnings...)

	if len(index.Nodes) > 0 {
		p.mu.Lock()
		p.token = token
		p.index = index
		p.mu.Unlock()
	}
	return index, index.Warnings
}

func (p *previewLSPCodeGraphProvider) buildIndex(ctx context.Context, files []lspSourceFile) lspCodeGraphIndex {
	index := lspCodeGraphIndex{
		Nodes:    map[string]lspCodeNode{},
		ByPath:   map[string][]string{},
		Warnings: []string{},
	}
	missing := map[string]string{}
	for _, file := range files {
		if ctx.Err() != nil {
			index.Warnings = append(index.Warnings, "Code Graph LSP indexing timed out; showing partial results.")
			break
		}
		srv, err := p.manager.ServerFor(file.Language)
		if err != nil {
			missing[file.Language.Name] = err.Error()
			continue
		}
		symbols, err := srv.DocumentSymbols(ctx, file.Abs, file.Language.LanguageID)
		if err != nil {
			index.Warnings = append(index.Warnings, fmt.Sprintf("Code Graph could not read symbols from %s: %v", file.Rel, err))
			continue
		}
		flattenLSPSymbols(&index, file, symbols, nil)
	}
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			index.Warnings = append(index.Warnings, fmt.Sprintf("Code Graph LSP server for %s is unavailable: %s", name, missing[name]))
		}
	}
	index.Supported = len(index.Nodes) > 0
	return index
}

func flattenLSPSymbols(index *lspCodeGraphIndex, file lspSourceFile, symbols []lspDocumentSymbol, parents []lspDocumentSymbol) {
	for _, sym := range symbols {
		owner := lspSymbolOwner(sym, parents)
		fullName := lspFullSymbolName(sym.Name, owner)
		if lspSymbolIsCallable(sym.Kind) {
			node := lspCodeNode{
				ID:             lspCodeNodeID(file.Language.ServerID, file.Rel, fullName, sym.SelectionRange.Start),
				Name:           sym.Name,
				FullName:       fullName,
				Owner:          owner,
				Kind:           sym.Kind,
				KindLabel:      lspSymbolKindLabel(sym.Kind),
				LanguageID:     file.Language.LanguageID,
				ServerID:       file.Language.ServerID,
				Path:           file.Rel,
				AbsPath:        file.Abs,
				Range:          sym.Range,
				SelectionRange: sym.SelectionRange,
			}
			index.Nodes[node.ID] = node
			index.ByPath[node.Path] = append(index.ByPath[node.Path], node.ID)
		}
		nextParents := append(append([]lspDocumentSymbol{}, parents...), sym)
		flattenLSPSymbols(index, file, sym.Children, nextParents)
	}
}

func lspSymbolOwner(sym lspDocumentSymbol, parents []lspDocumentSymbol) string {
	if sym.ContainerName != "" {
		return strings.TrimSpace(sym.ContainerName)
	}
	for i := len(parents) - 1; i >= 0; i-- {
		if lspSymbolIsContainer(parents[i].Kind) {
			return strings.TrimSpace(parents[i].Name)
		}
	}
	return ""
}

func lspFullSymbolName(name, owner string) string {
	name = strings.TrimSpace(name)
	owner = strings.TrimSpace(owner)
	if owner == "" || name == "" {
		return firstNonEmpty(name, owner)
	}
	for _, prefix := range []string{owner + ".", owner + "#", owner + "::", "(" + owner + ")"} {
		if strings.HasPrefix(name, prefix) {
			return name
		}
	}
	return owner + "." + name
}

func lspCodeNodeID(serverID, rel, fullName string, pos lspPosition) string {
	key := strings.Join([]string{serverID, rel, fullName, strconv.Itoa(pos.Line), strconv.Itoa(pos.Character)}, ":")
	sum := sha256.Sum256([]byte(key))
	return "lsp:" + hex.EncodeToString(sum[:8])
}

func searchLSPCodeGraph(ctx context.Context, provider *previewLSPCodeGraphProvider, index lspCodeGraphIndex, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) ([]previewSearchResult, []string) {
	candidates := make([]lspCodeGraphCandidate, 0, len(index.Nodes))
	for id, node := range index.Nodes {
		title := lspCodeNodeTitle(node)
		haystack := lspCodeNodeHaystack(node)
		if excludedByKeywordSearch(exclusionQuery, exclusionTokens, title, node.Path, haystack) {
			continue
		}
		evidence := searchFieldEvidence(query, tokens, title, node.Path, []string{node.Name, node.FullName, node.Owner, node.KindLabel}, haystack)
		if evidence.Score <= 0 {
			continue
		}
		candidates = append(candidates, lspCodeGraphCandidate{
			ID:        id,
			Node:      node,
			Title:     title,
			Path:      node.Path,
			Score:     evidence.Score,
			Exactness: evidence.Exactness,
		})
	}
	sortLSPCodeGraphCandidates(candidates)

	results := map[string]previewSearchResult{}
	edgesByAnchor := []lspCodeEdge{}
	warnings := []string{}
	for _, candidate := range limitLSPCodeGraphCandidates(candidates, limit) {
		anchorResults, edges, relationWarnings := provider.expandLSPCodeGraphCallFlow(ctx, index, candidate, limit)
		warnings = append(warnings, relationWarnings...)
		edgesByAnchor = append(edgesByAnchor, edges...)
		for _, result := range anchorResults {
			mergeGraphResult(results, result)
		}
	}
	assignLSPGraphNeighbors(results, index, edgesByAnchor)
	out := make([]previewSearchResult, 0, len(results))
	for _, result := range results {
		result.ID = "code-lsp:" + result.NodeID
		out = append(out, result)
	}
	sortSearchResults(out)
	return limitResults(dedupeSearchResults(out), graphExpansionLimit(limit)), uniqueStrings(warnings)
}

func (p *previewLSPCodeGraphProvider) expandLSPCodeGraphCallFlow(ctx context.Context, index lspCodeGraphIndex, candidate lspCodeGraphCandidate, limit int) (map[string]previewSearchResult, []lspCodeEdge, []string) {
	results := map[string]previewSearchResult{}
	anchor := candidate.Node
	anchorResult := lspCodeNodeSearchResult(anchor, candidate.Score, []string{"graph"}, candidate.ID, 0)
	anchorResult.Anchor = true
	mergeGraphResult(results, anchorResult)

	relationCtx, cancel := context.WithTimeout(ctx, lspRelationTimeout)
	defer cancel()
	edges, warnings := p.relationsForNode(relationCtx, index, anchor)
	for _, edge := range edges {
		source, sourceOK := index.Nodes[edge.Source]
		target, targetOK := index.Nodes[edge.Target]
		if !sourceOK || !targetOK {
			continue
		}
		if edge.Source == candidate.ID {
			result := lspCodeNodeSearchResult(target, codeGraphFlowScore(candidate.Score, 1, edge.Relation), []string{"graph-callee", "graph-flow"}, candidate.ID, 1)
			mergeGraphResult(results, result)
			continue
		}
		if edge.Target == candidate.ID {
			result := lspCodeNodeSearchResult(source, codeGraphFlowScore(candidate.Score, 1, edge.Relation), []string{"graph-caller", "graph-flow"}, candidate.ID, 1)
			mergeGraphResult(results, result)
		}
	}
	return results, edges, warnings
}

func (p *previewLSPCodeGraphProvider) relationsForNode(ctx context.Context, index lspCodeGraphIndex, node lspCodeNode) ([]lspCodeEdge, []string) {
	srv, err := p.manager.ServerByID(node.ServerID)
	if err != nil {
		return nil, []string{"Code Graph relation expansion is unavailable: " + err.Error()}
	}
	edges, err := p.callHierarchyEdges(ctx, srv, index, node)
	if err == nil && len(edges) > 0 {
		return edges, nil
	}
	refEdges, refErr := p.referenceEdges(ctx, srv, index, node)
	if refErr == nil && len(refEdges) > 0 {
		return refEdges, nil
	}
	if err != nil && refErr != nil {
		return nil, []string{"Code Graph relation expansion is unavailable for this language server."}
	}
	return nil, nil
}

func (p *previewLSPCodeGraphProvider) callHierarchyEdges(ctx context.Context, srv *previewLSPServer, index lspCodeGraphIndex, node lspCodeNode) ([]lspCodeEdge, error) {
	items, err := srv.PrepareCallHierarchy(ctx, node.AbsPath, node.LanguageID, node.SelectionRange.Start)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	edges := []lspCodeEdge{}
	for _, item := range items {
		incoming, _ := srv.IncomingCalls(ctx, item)
		for _, call := range incoming {
			if callerID := index.nodeIDForLocation(p.projectRoot, call.From.URI, call.From.SelectionRange.Start); callerID != "" && callerID != node.ID {
				edges = append(edges, lspCodeEdge{Source: callerID, Target: node.ID, Relation: "calls", SourceID: callerID, TargetID: node.ID})
			}
		}
		outgoing, _ := srv.OutgoingCalls(ctx, item)
		for _, call := range outgoing {
			if calleeID := index.nodeIDForLocation(p.projectRoot, call.To.URI, call.To.SelectionRange.Start); calleeID != "" && calleeID != node.ID {
				edges = append(edges, lspCodeEdge{Source: node.ID, Target: calleeID, Relation: "calls", SourceID: node.ID, TargetID: calleeID})
			}
		}
	}
	return dedupeLSPCodeEdges(edges), nil
}

func (p *previewLSPCodeGraphProvider) referenceEdges(ctx context.Context, srv *previewLSPServer, index lspCodeGraphIndex, node lspCodeNode) ([]lspCodeEdge, error) {
	refs, err := srv.References(ctx, node.AbsPath, node.LanguageID, node.SelectionRange.Start)
	if err != nil {
		return nil, err
	}
	edges := []lspCodeEdge{}
	for _, ref := range refs {
		callerID := index.containingNodeIDForLocation(p.projectRoot, ref.URI, ref.Range.Start)
		if callerID == "" || callerID == node.ID {
			continue
		}
		edges = append(edges, lspCodeEdge{Source: callerID, Target: node.ID, Relation: "references", SourceID: callerID, TargetID: node.ID})
	}
	return dedupeLSPCodeEdges(edges), nil
}

func assignLSPGraphNeighbors(results map[string]previewSearchResult, index lspCodeGraphIndex, edges []lspCodeEdge) {
	for key, result := range results {
		neighbors := []previewSearchNeighbor{}
		for _, edge := range edges {
			switch result.NodeID {
			case edge.Source:
				if node, ok := index.Nodes[edge.Target]; ok {
					neighbors = append(neighbors, previewSearchNeighbor{
						ID:        edge.Target,
						Label:     lspCodeNodeTitle(node),
						Relation:  edge.Relation,
						Direction: "outgoing",
						SourceID:  edge.Source,
						TargetID:  edge.Target,
						Path:      node.Path,
						Line:      nodeLine(node),
					})
				}
			case edge.Target:
				if node, ok := index.Nodes[edge.Source]; ok {
					neighbors = append(neighbors, previewSearchNeighbor{
						ID:        edge.Source,
						Label:     lspCodeNodeTitle(node),
						Relation:  edge.Relation,
						Direction: "incoming",
						SourceID:  edge.Source,
						TargetID:  edge.Target,
						Path:      node.Path,
						Line:      nodeLine(node),
					})
				}
			}
		}
		result.Neighbors = limitNeighbors(dedupeLSPNeighbors(neighbors), maxGraphNeighborUI)
		results[key] = result
	}
}

func lspCodeNodeSearchResult(node lspCodeNode, score float64, matchedBy []string, anchorID string, depth int) previewSearchResult {
	return previewSearchResult{
		Title:      lspCodeNodeTitle(node),
		Path:       node.Path,
		Kind:       firstNonEmpty(node.KindLabel, "symbol"),
		Source:     "lsp",
		Line:       nodeLine(node),
		Score:      roundScore(score),
		MatchedBy:  matchedBy,
		NodeID:     node.ID,
		AnchorID:   anchorID,
		Depth:      depth,
		FlowAnchor: anchorID,
		FlowDepth:  depth,
		Confidence: "lsp",
		FlowRole:   codeGraphFlowRole(matchedBy),
	}
}

func lspCodeNodeTitle(node lspCodeNode) string {
	title := firstNonEmpty(node.FullName, node.Name, node.ID)
	if lspSymbolIsCallable(node.Kind) && !strings.Contains(title, "(") {
		title += "()"
	}
	return title
}

func lspCodeNodeHaystack(node lspCodeNode) string {
	return strings.Join([]string{node.ID, node.Name, node.FullName, node.Owner, node.KindLabel, node.Path, node.LanguageID}, " ")
}

func nodeLine(node lspCodeNode) int {
	line := node.SelectionRange.Start.Line + 1
	if line <= 0 {
		line = node.Range.Start.Line + 1
	}
	return maxInt(1, line)
}

func sortLSPCodeGraphCandidates(candidates []lspCodeGraphCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].Exactness != candidates[j].Exactness {
			return candidates[i].Exactness > candidates[j].Exactness
		}
		if candidates[i].Title != candidates[j].Title {
			return candidates[i].Title < candidates[j].Title
		}
		if candidates[i].Path != candidates[j].Path {
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].ID < candidates[j].ID
	})
}

func limitLSPCodeGraphCandidates(candidates []lspCodeGraphCandidate, limit int) []lspCodeGraphCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	return candidates[:minInt(len(candidates), maxInt(limit, limit*2))]
}

func (index lspCodeGraphIndex) nodeIDForLocation(projectRoot, uri string, pos lspPosition) string {
	rel := cleanProjectRel(projectRoot, pathFromLSPURI(uri))
	best := ""
	bestSpan := int(^uint(0) >> 1)
	for _, id := range index.ByPath[rel] {
		node := index.Nodes[id]
		if positionInLSPRange(pos, node.SelectionRange) || node.SelectionRange.Start.Line == pos.Line {
			span := lspRangeSpan(node.SelectionRange)
			if span < bestSpan {
				best = id
				bestSpan = span
			}
		}
	}
	if best != "" {
		return best
	}
	return index.containingNodeIDForLocation(projectRoot, uri, pos)
}

func (index lspCodeGraphIndex) containingNodeIDForLocation(projectRoot, uri string, pos lspPosition) string {
	rel := cleanProjectRel(projectRoot, pathFromLSPURI(uri))
	best := ""
	bestSpan := int(^uint(0) >> 1)
	for _, id := range index.ByPath[rel] {
		node := index.Nodes[id]
		if !positionInLSPRange(pos, node.Range) {
			continue
		}
		span := lspRangeSpan(node.Range)
		if span < bestSpan {
			best = id
			bestSpan = span
		}
	}
	return best
}

func lspRangeSpan(rng lspRange) int {
	span := (rng.End.Line - rng.Start.Line) + 1
	if span <= 0 {
		return 1
	}
	return span
}

func positionInLSPRange(pos lspPosition, rng lspRange) bool {
	if pos.Line < rng.Start.Line || pos.Line > rng.End.Line {
		return false
	}
	if pos.Line == rng.Start.Line && pos.Character < rng.Start.Character {
		return false
	}
	if pos.Line == rng.End.Line && pos.Character > rng.End.Character {
		return false
	}
	return true
}

func dedupeLSPCodeEdges(edges []lspCodeEdge) []lspCodeEdge {
	seen := map[string]bool{}
	out := []lspCodeEdge{}
	for _, edge := range edges {
		if edge.Source == "" || edge.Target == "" || edge.Source == edge.Target {
			continue
		}
		key := edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, edge)
	}
	return out
}

func dedupeLSPNeighbors(neighbors []previewSearchNeighbor) []previewSearchNeighbor {
	seen := map[string]bool{}
	out := []previewSearchNeighbor{}
	for _, neighbor := range neighbors {
		key := neighbor.ID + "\x00" + neighbor.Direction + "\x00" + neighbor.Relation
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, neighbor)
	}
	return out
}

func lspSourceFiles(projectRoot, docsRoot string) (string, []lspSourceFile, []string) {
	files := []lspSourceFile{}
	warnings := []string{}
	addFile := func(rel string) {
		rel = cleanRelPath(rel)
		if rel == "" || shouldSkipGitSearchPath(rel) || pathIsUnderDocsRoot(projectRoot, docsRoot, rel) {
			return
		}
		abs := filepath.Join(projectRoot, filepath.FromSlash(rel))
		lang, ok := lspLanguageForPath(abs)
		if !ok || !isSearchableCodePath(abs) {
			return
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() || info.Size() > lspMaxIndexedFileBytes {
			return
		}
		files = append(files, lspSourceFile{Rel: rel, Abs: abs, Language: lang, Size: info.Size(), ModifiedNS: info.ModTime().UnixNano()})
	}
	if gitFiles, ok := gitTrackedFiles(projectRoot); ok {
		rels := make([]string, 0, len(gitFiles))
		for rel := range gitFiles {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			addFile(rel)
		}
	} else {
		err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if sameCleanPath(path, docsRoot) && !sameCleanPath(path, projectRoot) {
					return filepath.SkipDir
				}
				if shouldSkipSearchDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			addFile(relPath(projectRoot, path))
			return nil
		})
		if err != nil {
			warnings = append(warnings, "Code Graph source scan failed: "+err.Error())
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	token := lspSourceToken(files)
	return token, files, warnings
}

func lspSourceToken(files []lspSourceFile) string {
	var b strings.Builder
	for _, file := range files {
		b.WriteString(file.Rel)
		b.WriteByte(':')
		b.WriteString(strconv.FormatInt(file.Size, 10))
		b.WriteByte(':')
		b.WriteString(strconv.FormatInt(file.ModifiedNS, 10))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func lspLanguageForPath(path string) (lspLanguage, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return lspLanguage{ServerID: "go", LanguageID: "go", Name: "Go", Command: "gopls", Args: []string{"serve"}}, true
	case ".ts":
		return lspLanguage{ServerID: "typescript", LanguageID: "typescript", Name: "TypeScript", Command: "typescript-language-server", Args: []string{"--stdio"}}, true
	case ".tsx":
		return lspLanguage{ServerID: "typescript", LanguageID: "typescriptreact", Name: "TypeScript", Command: "typescript-language-server", Args: []string{"--stdio"}}, true
	case ".js", ".cjs", ".mjs":
		return lspLanguage{ServerID: "typescript", LanguageID: "javascript", Name: "JavaScript", Command: "typescript-language-server", Args: []string{"--stdio"}}, true
	case ".jsx":
		return lspLanguage{ServerID: "typescript", LanguageID: "javascriptreact", Name: "JavaScript", Command: "typescript-language-server", Args: []string{"--stdio"}}, true
	default:
		return lspLanguage{}, false
	}
}

func newPreviewLSPManager(root string) *previewLSPManager {
	return &previewLSPManager{root: root, servers: map[string]*previewLSPServer{}}
}

func (m *previewLSPManager) ServerFor(lang lspLanguage) (*previewLSPServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if srv := m.servers[lang.ServerID]; srv != nil {
		return srv, nil
	}
	path, err := m.resolveCommand(lang)
	if err != nil {
		return nil, err
	}
	srv := &previewLSPServer{root: m.root, lang: lang, command: path, args: append([]string{}, lang.Args...)}
	m.servers[lang.ServerID] = srv
	return srv, nil
}

func (m *previewLSPManager) resolveCommand(lang lspLanguage) (string, error) {
	if path, err := exec.LookPath(lang.Command); err == nil {
		return path, nil
	}
	for _, candidate := range m.commandCandidates(lang.Command) {
		if executableFile(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH or known local tool locations", lang.Command)
}

func (m *previewLSPManager) commandCandidates(command string) []string {
	dirs := []string{}
	addDir := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		dirs = appendUniqueString(dirs, dir)
	}

	addDir(filepath.Join(m.root, "node_modules", ".bin"))
	if wd, err := os.Getwd(); err == nil {
		addDir(filepath.Join(wd, "node_modules", ".bin"))
		if moduleRoot, ok := previewModuleRoot(wd); ok {
			addDir(filepath.Join(moduleRoot, "node_modules", ".bin"))
		}
	}

	if command == "gopls" {
		addDir(os.Getenv("GOBIN"))
		if gopath := firstNonEmpty(os.Getenv("GOPATH"), goEnvValue("GOPATH")); gopath != "" {
			addDir(filepath.Join(gopath, "bin"))
		}
		if gobin := goEnvValue("GOBIN"); gobin != "" {
			addDir(gobin)
		}
		if home, err := os.UserHomeDir(); err == nil {
			// GUI-launched processes often miss GOPATH/bin in PATH even when gopls is installed there.
			addDir(filepath.Join(home, "go", "bin"))
		}
	}

	candidates := []string{}
	for _, dir := range dirs {
		for _, name := range executableNames(command) {
			candidates = appendUniqueString(candidates, filepath.Join(dir, name))
		}
	}
	return candidates
}

func executableNames(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{command, command + ".exe", command + ".cmd", command + ".bat"}
	}
	return []string{command}
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func goEnvValue(key string) string {
	if _, err := exec.LookPath("go"); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *previewLSPManager) ServerByID(serverID string) (*previewLSPServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	srv := m.servers[serverID]
	if srv == nil {
		return nil, fmt.Errorf("LSP server %s is not running", serverID)
	}
	return srv, nil
}

func (m *previewLSPManager) Close(ctx context.Context) error {
	m.mu.Lock()
	servers := make([]*previewLSPServer, 0, len(m.servers))
	for _, srv := range m.servers {
		servers = append(servers, srv)
	}
	m.mu.Unlock()
	for _, srv := range servers {
		_ = srv.Stop(ctx)
	}
	return nil
}

func (s *previewLSPServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	select {
	case <-ctx.Done():
		s.mu.Unlock()
		return ctx.Err()
	default:
	}
	// LSP servers are preview-server resources; per-request contexts should bound I/O, not own the process lifetime.
	cmd := exec.Command(s.command, s.args...)
	cmd.Dir = s.root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.cmd = cmd
	s.stdin = stdin
	s.reader = bufio.NewReader(stdout)
	s.running = true
	s.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.running = false
			s.initialized = false
			s.cmd = nil
			s.stdin = nil
			s.reader = nil
		}
		s.mu.Unlock()
	}()
	if err := s.initialize(ctx); err != nil {
		_ = s.Stop(context.Background())
		return err
	}
	return nil
}

func (s *previewLSPServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running || s.cmd == nil {
		s.mu.Unlock()
		return nil
	}
	_ = s.requestLocked(ctx, "shutdown", nil, nil)
	_ = s.notifyLocked(ctx, "exit", nil)
	cmd := s.cmd
	s.running = false
	s.initialized = false
	s.cmd = nil
	s.stdin = nil
	s.reader = nil
	s.mu.Unlock()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return nil
}

func (s *previewLSPServer) initialize(ctx context.Context) error {
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   fileURI(s.root),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true},
				"callHierarchy":  map[string]any{"dynamicRegistration": false},
			},
			"window": map[string]any{"workDoneProgress": true},
		},
	}
	if err := s.request(ctx, "initialize", params, nil); err != nil {
		return err
	}
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	return s.notify(ctx, "initialized", map[string]any{})
}

func (s *previewLSPServer) DocumentSymbols(ctx context.Context, path, languageID string) ([]lspDocumentSymbol, error) {
	var symbols []lspDocumentSymbol
	err := s.withOpenFile(ctx, path, languageID, func() error {
		params := map[string]any{"textDocument": map[string]any{"uri": fileURI(path)}}
		var raw json.RawMessage
		if err := s.request(ctx, "textDocument/documentSymbol", params, &raw); err != nil {
			return err
		}
		parsed, err := parseLSPDocumentSymbols(raw)
		if err != nil {
			return err
		}
		symbols = parsed
		return nil
	})
	return symbols, err
}

func (s *previewLSPServer) PrepareCallHierarchy(ctx context.Context, path, languageID string, pos lspPosition) ([]lspCallHierarchyItem, error) {
	var items []lspCallHierarchyItem
	err := s.withOpenFile(ctx, path, languageID, func() error {
		params := map[string]any{"textDocument": map[string]any{"uri": fileURI(path)}, "position": pos}
		var raw json.RawMessage
		if err := s.request(ctx, "textDocument/prepareCallHierarchy", params, &raw); err != nil {
			return err
		}
		if len(raw) == 0 || string(raw) == "null" {
			return nil
		}
		return json.Unmarshal(raw, &items)
	})
	return items, err
}

func (s *previewLSPServer) IncomingCalls(ctx context.Context, item lspCallHierarchyItem) ([]lspIncomingCall, error) {
	var calls []lspIncomingCall
	err := s.request(ctx, "callHierarchy/incomingCalls", map[string]any{"item": item}, &calls)
	return calls, err
}

func (s *previewLSPServer) OutgoingCalls(ctx context.Context, item lspCallHierarchyItem) ([]lspOutgoingCall, error) {
	var calls []lspOutgoingCall
	err := s.request(ctx, "callHierarchy/outgoingCalls", map[string]any{"item": item}, &calls)
	return calls, err
}

func (s *previewLSPServer) References(ctx context.Context, path, languageID string, pos lspPosition) ([]lspLocation, error) {
	var locs []lspLocation
	err := s.withOpenFile(ctx, path, languageID, func() error {
		params := map[string]any{
			"textDocument": map[string]any{"uri": fileURI(path)},
			"position":     pos,
			"context":      map[string]any{"includeDeclaration": false},
		}
		rawLocs, err := s.locations(ctx, "textDocument/references", params)
		if err != nil {
			return err
		}
		locs = rawLocs
		return nil
	})
	return locs, err
}

func (s *previewLSPServer) withOpenFile(ctx context.Context, path, languageID string, fn func() error) error {
	if err := s.Start(ctx); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("file is not valid UTF-8")
	}
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":        fileURI(path),
			"languageId": languageID,
			"version":    1,
			"text":       string(data),
		},
	}
	if err := s.notify(ctx, "textDocument/didOpen", params); err != nil {
		return err
	}
	defer func() {
		_ = s.notify(context.Background(), "textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": fileURI(path)}})
	}()
	return fn()
}

func (s *previewLSPServer) locations(ctx context.Context, method string, params map[string]any) ([]lspLocation, error) {
	var raw json.RawMessage
	if err := s.request(ctx, method, params, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '[' {
		var links []struct {
			TargetURI            string   `json:"targetUri"`
			TargetRange          lspRange `json:"targetRange"`
			TargetSelectionRange lspRange `json:"targetSelectionRange"`
		}
		if err := json.Unmarshal(raw, &links); err == nil && len(links) > 0 && links[0].TargetURI != "" {
			locs := make([]lspLocation, 0, len(links))
			for _, link := range links {
				rng := link.TargetSelectionRange
				if rng == (lspRange{}) {
					rng = link.TargetRange
				}
				locs = append(locs, lspLocation{URI: link.TargetURI, Range: rng})
			}
			return locs, nil
		}
		var locs []lspLocation
		if err := json.Unmarshal(raw, &locs); err != nil {
			return nil, err
		}
		return locs, nil
	}
	var loc lspLocation
	if err := json.Unmarshal(raw, &loc); err != nil {
		return nil, err
	}
	return []lspLocation{loc}, nil
}

func (s *previewLSPServer) request(ctx context.Context, method string, params any, result any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if method != "initialize" && !s.initialized {
		return fmt.Errorf("LSP server %s is not initialized", s.lang.ServerID)
	}
	return s.requestLocked(ctx, method, params, result)
}

func (s *previewLSPServer) notify(ctx context.Context, method string, params any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notifyLocked(ctx, method, params)
}

func (s *previewLSPServer) requestLocked(ctx context.Context, method string, params any, result any) error {
	id := s.nextID.Add(1)
	if err := s.writeLocked(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return err
	}
	return s.readResponseLocked(ctx, id, result)
}

func (s *previewLSPServer) notifyLocked(ctx context.Context, method string, params any) error {
	return s.writeLocked(ctx, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (s *previewLSPServer) writeLocked(ctx context.Context, msg any) error {
	if !s.running || s.stdin == nil {
		return fmt.Errorf("LSP server %s is not running", s.lang.ServerID)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err = fmt.Fprintf(s.stdin, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

func (s *previewLSPServer) readResponseLocked(ctx context.Context, id int64, result any) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msg, err := s.readMessageLocked()
		if err != nil {
			s.running = false
			s.initialized = false
			return err
		}
		var envelope struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(msg, &envelope); err != nil {
			return err
		}
		if envelope.Method != "" && envelope.ID != nil && *envelope.ID != id {
			_ = s.writeLocked(ctx, map[string]any{"jsonrpc": "2.0", "id": *envelope.ID, "result": nil})
		}
		if envelope.ID == nil || *envelope.ID != id {
			continue
		}
		if envelope.Error != nil {
			return fmt.Errorf("lsp %s: %s", s.lang.ServerID, envelope.Error.Message)
		}
		if result != nil && len(envelope.Result) > 0 {
			return json.Unmarshal(envelope.Result, result)
		}
		return nil
	}
}

func (s *previewLSPServer) readMessageLocked() ([]byte, error) {
	var length int
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			_, _ = fmt.Sscanf(line, "Content-Length: %d", &length)
		}
	}
	if length <= 0 {
		return nil, errors.New("missing LSP content length")
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(s.reader, buf)
	return buf, err
}

func parseLSPDocumentSymbols(raw json.RawMessage) ([]lspDocumentSymbol, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var flat []struct {
		Name          string      `json:"name"`
		Kind          int         `json:"kind"`
		Location      lspLocation `json:"location"`
		ContainerName string      `json:"containerName,omitempty"`
	}
	if err := json.Unmarshal(raw, &flat); err == nil && len(flat) > 0 && flat[0].Location.URI != "" {
		symbols := make([]lspDocumentSymbol, 0, len(flat))
		for _, item := range flat {
			symbols = append(symbols, lspDocumentSymbol{
				Name:           item.Name,
				Kind:           item.Kind,
				Range:          item.Location.Range,
				SelectionRange: item.Location.Range,
				ContainerName:  item.ContainerName,
			})
		}
		return symbols, nil
	}
	var symbols []lspDocumentSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}

func pathFromLSPURI(uri string) string {
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "file://") {
		parsed, err := url.Parse(uri)
		if err == nil {
			path, unescapeErr := url.PathUnescape(parsed.Path)
			if unescapeErr == nil {
				return path
			}
			return parsed.Path
		}
	}
	return uri
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func lspSymbolIsCallable(kind int) bool {
	switch kind {
	case 6, 9, 12:
		return true
	default:
		return false
	}
}

func lspSymbolIsContainer(kind int) bool {
	switch kind {
	case 2, 3, 5, 11, 18, 23:
		return true
	default:
		return false
	}
}

func lspSymbolKindLabel(kind int) string {
	switch kind {
	case 2:
		return "module"
	case 3:
		return "namespace"
	case 5:
		return "class"
	case 6:
		return "method"
	case 9:
		return "constructor"
	case 11:
		return "interface"
	case 12:
		return "function"
	case 18:
		return "object"
	case 23:
		return "struct"
	default:
		return "symbol"
	}
}
