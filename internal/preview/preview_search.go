package preview

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	defaultSearchLimit = 8
	maxSearchLimit     = 24
	maxSearchFileBytes = 256 * 1024
	maxGraphNeighborUI = 10
)

type previewSearchResponse struct {
	Query    string              `json:"query"`
	Mode     string              `json:"mode"`
	Panels   previewSearchPanels `json:"panels"`
	Stats    map[string]int      `json:"stats"`
	Warnings []string            `json:"warnings,omitempty"`
}

type previewSearchPanels struct {
	DocsSemantic []previewSearchResult `json:"docsSemantic"`
	DocsGraph    []previewSearchResult `json:"docsGraph"`
	CodeSemantic []previewSearchResult `json:"codeSemantic"`
	CodeGraph    []previewSearchResult `json:"codeGraph"`
}

type previewSearchResult struct {
	ID         string                  `json:"id"`
	Title      string                  `json:"title"`
	Path       string                  `json:"path,omitempty"`
	Kind       string                  `json:"kind,omitempty"`
	Source     string                  `json:"source,omitempty"`
	Line       int                     `json:"line,omitempty"`
	Score      float64                 `json:"score"`
	MatchedBy  []string                `json:"matchedBy,omitempty"`
	Excerpt    string                  `json:"excerpt,omitempty"`
	SpecID     string                  `json:"specId,omitempty"`
	NodeID     string                  `json:"nodeId,omitempty"`
	Community  string                  `json:"community,omitempty"`
	Relation   string                  `json:"relation,omitempty"`
	Confidence string                  `json:"confidence,omitempty"`
	Anchor     bool                    `json:"anchor,omitempty"`
	AnchorID   string                  `json:"anchorId,omitempty"`
	Depth      int                     `json:"depth,omitempty"`
	Neighbors  []previewSearchNeighbor `json:"neighbors,omitempty"`
}

type previewSearchNeighbor struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Relation   string `json:"relation,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Path       string `json:"path,omitempty"`
	Line       int    `json:"line,omitempty"`
}

type graphifyGraph struct {
	Nodes     map[string]graphifyNode
	Neighbors map[string][]previewSearchNeighbor
	Warnings  []string
}

type graphifyNode struct {
	ID             string
	Label          string
	FileType       string
	SourceFile     string
	SourceLocation string
	Community      string
	NormLabel      string
}

type graphifyLink struct {
	Source     string
	Target     string
	Relation   string
	Confidence string
}

type graphSearchAnchor struct {
	ID     string
	Title  string
	Path   string
	SpecID string
	Line   int
	Score  float64
}

type docsGraphIndex struct {
	Nodes map[string]graphNode
	Keys  map[string][]string
	Edges map[string][]graphEdge
}

type graphifyIndex struct {
	Nodes map[string]graphifyNode
	Keys  map[string][]string
	Edges map[string][]graphifyEdge
	Root  string
}

type graphifyEdge struct {
	To         string
	Relation   string
	Confidence string
}

type codeSearchDoc struct {
	ID      string
	Title   string
	Path    string
	Content string
}

type docsSearchDoc struct {
	ID      string
	Title   string
	Path    string
	Content string
	SpecID  string
	Kind    string
}

type previewEmbeddingConfig struct {
	APIBase     string `json:"apiBase"`
	APIKey      string `json:"-"`
	Model       string `json:"model"`
	Dimensions  int    `json:"dimensions"`
	BatchSize   int    `json:"batchSize"`
	Timeout     int    `json:"timeout"`
	QueryPrefix string `json:"queryPrefix,omitempty"`
	DocPrefix   string `json:"docPrefix,omitempty"`
	Source      string `json:"source,omitempty"`
}

type previewEmbeddingChunk struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Path      string    `json:"path"`
	SpecID    string    `json:"specId,omitempty"`
	Line      int       `json:"line,omitempty"`
	Content   string    `json:"content"`
	Hash      string    `json:"hash"`
	Embedding []float32 `json:"embedding"`
}

type previewEmbeddingIndex struct {
	Model      string                  `json:"model"`
	APIBase    string                  `json:"apiBase"`
	Dimensions int                     `json:"dimensions"`
	Chunks     []previewEmbeddingChunk `json:"chunks"`
	IndexedAt  time.Time               `json:"indexedAt"`
}

type previewEmbeddingSearch struct {
	Config previewEmbeddingConfig
	Index  previewEmbeddingIndex
}

type knownsEmbeddingSettings struct {
	Providers map[string]struct {
		APIBase   string `json:"apiBase"`
		APIKey    string `json:"apiKey"`
		Timeout   int    `json:"timeout"`
		BatchSize int    `json:"batchSize"`
	} `json:"embeddingProviders"`
	Models map[string]struct {
		Provider   string `json:"provider"`
		Model      string `json:"model"`
		Dimensions int    `json:"dimensions"`
	} `json:"embeddingModels"`
	DefaultModel string `json:"defaultEmbeddingModel"`
}

type ollamaModel struct {
	Name string `json:"name"`
}

func (ps *previewServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := ps.load()
	warnings := []string{}
	if err != nil {
		project = emptySearchProject(ps.opt.projectRoot, ps.opt.docsDir)
		warnings = append(warnings, "Docs directory is unavailable; searching code and graphify data only: "+err.Error())
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	mode := "hybrid"
	limit := parseSearchLimit(r.URL.Query().Get("limit"))
	graphify := loadGraphifyGraph(ps.opt.projectRoot)
	response := buildPreviewSearchResponse(project, graphify, ps.opt.projectRoot, query, mode, limit)
	response.Warnings = append(warnings, response.Warnings...)
	writeJSON(w, response)
}

func emptySearchProject(projectRoot, docsDir string) specProject {
	root := docsRoot(projectRoot, docsDir)
	return specProject{
		Summary: projectSummary{
			Name:         filepath.Base(projectRoot),
			ProjectRoot:  projectRoot,
			DocsRoot:     root,
			Categories:   map[string]int{},
			StatusCounts: map[string]int{},
			Compliance:   map[string]int{},
			Sync:         map[string]string{},
		},
		Documents: []specDocument{},
		Graph:     specGraph{},
	}
}

func buildPreviewSearchResponse(project specProject, graphify graphifyGraph, projectRoot, query, mode string, limit int) previewSearchResponse {
	response := previewSearchResponse{
		Query:    query,
		Mode:     mode,
		Stats:    map[string]int{},
		Warnings: append([]string{}, graphify.Warnings...),
	}
	if query == "" {
		response.Warnings = append(response.Warnings, "Enter a query to search docs and code.")
		return response
	}
	tokens := searchTokens(query)
	if len(tokens) == 0 {
		response.Warnings = append(response.Warnings, "Query has no searchable tokens.")
		return response
	}

	if mode != "graph" {
		docsDocs, docsWarnings := scanDocsSearchDocs(projectRoot, project.Summary.DocsRoot, project.Documents)
		response.Warnings = append(response.Warnings, docsWarnings...)
		codeDocs, warnings := scanCodeSearchDocs(projectRoot, project.Summary.DocsRoot)
		response.Warnings = append(response.Warnings, warnings...)
		var embedSearch *previewEmbeddingSearch
		if mode == "semantic" || mode == "hybrid" {
			embedSearch, _ = loadPreviewEmbeddingSearch(projectRoot, docsDocs, codeDocs)
		}
		if embedSearch != nil {
			docKeyword := searchDocsSemantic(docsDocs, query, tokens, "keyword", limit*2)
			codeKeyword := searchCodeSemantic(codeDocs, query, tokens, "keyword", limit*2)
			docSemantic, codeSemantic, err := embedSearch.search(query, limit*2)
			if err == nil {
				response.Panels.DocsSemantic = combineEmbeddingResults(docKeyword, docSemantic, mode, limit)
				response.Panels.CodeSemantic = combineEmbeddingResults(codeKeyword, codeSemantic, mode, limit)
			} else {
				response.Warnings = append(response.Warnings, "Embedding search failed; using lexical fallback: "+err.Error())
				response.Panels.DocsSemantic = searchDocsSemantic(docsDocs, query, tokens, mode, limit)
				response.Panels.CodeSemantic = searchCodeSemantic(codeDocs, query, tokens, mode, limit)
			}
		} else {
			response.Panels.DocsSemantic = searchDocsSemantic(docsDocs, query, tokens, mode, limit)
			response.Panels.CodeSemantic = searchCodeSemantic(codeDocs, query, tokens, mode, limit)
		}
	}
	if mode != "keyword" && mode != "semantic" {
		response.Panels.DocsGraph = searchDocsGraph(project.Graph, graphify, projectRoot, query, tokens, response.Panels.DocsSemantic, limit)
		response.Panels.CodeGraph = searchCodeGraph(graphify, projectRoot, query, tokens, response.Panels.CodeSemantic, limit)
		boostSemanticWithGraph(response.Panels.DocsSemantic, response.Panels.DocsGraph)
		boostSemanticWithGraph(response.Panels.CodeSemantic, response.Panels.CodeGraph)
	}

	response.Stats["docsSemantic"] = len(response.Panels.DocsSemantic)
	response.Stats["docsGraph"] = len(response.Panels.DocsGraph)
	response.Stats["codeSemantic"] = len(response.Panels.CodeSemantic)
	response.Stats["codeGraph"] = len(response.Panels.CodeGraph)
	return response
}

func parseSearchLimit(raw string) int {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

func searchDocsSemantic(docs []docsSearchDoc, query string, tokens []string, mode string, limit int) []previewSearchResult {
	results := []previewSearchResult{}
	for _, doc := range docs {
		if doc.SpecID != "" && isSpecControlFile(doc.SpecID) {
			continue
		}
		keyword := keywordScore(query, tokens, doc.Title, doc.Path, doc.Content)
		semantic := semanticScore(tokens, doc.Title, doc.Path, headingsFromMarkdown(doc.Content), doc.Content)
		score, matchedBy := combineSearchScores(keyword, semantic, mode)
		if score <= 0 {
			continue
		}
		results = append(results, previewSearchResult{
			ID:        "doc:" + doc.ID,
			Title:     doc.Title,
			Path:      doc.Path,
			Kind:      doc.Kind,
			Score:     score,
			MatchedBy: matchedBy,
			Excerpt:   excerptForQuery(doc.Content, tokens),
			SpecID:    doc.SpecID,
			Source:    "docs",
		})
	}
	sortSearchResults(results)
	return limitResults(results, limit)
}

func searchCodeSemantic(codeDocs []codeSearchDoc, query string, tokens []string, mode string, limit int) []previewSearchResult {
	results := []previewSearchResult{}
	for _, doc := range codeDocs {
		keyword := keywordScore(query, tokens, doc.Title, doc.Path, doc.Content)
		symbols := codeSymbols(doc.Content)
		semantic := semanticScore(tokens, doc.Title, doc.Path, symbols, doc.Content)
		score, matchedBy := combineSearchScores(keyword, semantic, mode)
		if score <= 0 {
			continue
		}
		line, excerpt := codeExcerptForQuery(doc.Content, tokens)
		results = append(results, previewSearchResult{
			ID:        "code:" + doc.Path,
			Title:     doc.Title,
			Path:      doc.Path,
			Kind:      "file",
			Score:     score,
			MatchedBy: matchedBy,
			Excerpt:   excerpt,
			Line:      line,
			Source:    "code",
		})
	}
	sortSearchResults(results)
	return limitResults(results, limit)
}

func loadPreviewEmbeddingSearch(projectRoot string, docs []docsSearchDoc, codeDocs []codeSearchDoc) (*previewEmbeddingSearch, []string) {
	cfg, warning := previewEmbeddingConfigForProject(projectRoot)
	if warning != "" {
		return nil, []string{warning}
	}
	chunks := buildPreviewEmbeddingChunks(docs, codeDocs)
	index, warnings := loadOrBuildPreviewEmbeddingIndex(projectRoot, cfg, chunks)
	if len(index.Chunks) == 0 {
		warnings = append(warnings, "Embedding index is empty; using lexical fallback.")
		return nil, warnings
	}
	return &previewEmbeddingSearch{Config: cfg, Index: index}, warnings
}

func buildPreviewEmbeddingChunks(docs []docsSearchDoc, codeDocs []codeSearchDoc) []previewEmbeddingChunk {
	chunks := []previewEmbeddingChunk{}
	for _, doc := range docs {
		if doc.SpecID != "" && isSpecControlFile(doc.SpecID) {
			continue
		}
		content := strings.TrimSpace(strings.Join([]string{doc.Title, doc.Path, strings.Join(headingsFromMarkdown(doc.Content), "\n"), doc.Content}, "\n\n"))
		if content == "" {
			continue
		}
		chunks = append(chunks, previewEmbeddingChunk{
			ID:      "doc:" + doc.ID,
			Type:    "doc",
			Title:   doc.Title,
			Path:    doc.Path,
			SpecID:  doc.SpecID,
			Content: content,
			Hash:    contentHash(content),
		})
	}
	for _, doc := range codeDocs {
		content := strings.TrimSpace(strings.Join([]string{doc.Title, doc.Path, strings.Join(codeSymbols(doc.Content), "\n"), doc.Content}, "\n\n"))
		if content == "" {
			continue
		}
		chunks = append(chunks, previewEmbeddingChunk{
			ID:      "code:" + doc.Path,
			Type:    "code",
			Title:   doc.Title,
			Path:    doc.Path,
			Content: content,
			Hash:    contentHash(content),
		})
	}
	return chunks
}

func loadOrBuildPreviewEmbeddingIndex(projectRoot string, cfg previewEmbeddingConfig, chunks []previewEmbeddingChunk) (previewEmbeddingIndex, []string) {
	warnings := []string{}
	index := readPreviewEmbeddingIndex(projectRoot)
	if index.Model != cfg.Model || index.APIBase != cfg.APIBase || index.Dimensions != cfg.Dimensions {
		index = previewEmbeddingIndex{Model: cfg.Model, APIBase: cfg.APIBase, Dimensions: cfg.Dimensions}
	}
	byID := map[string]previewEmbeddingChunk{}
	for _, chunk := range index.Chunks {
		byID[chunk.ID] = chunk
	}
	next := make([]previewEmbeddingChunk, 0, len(chunks))
	toEmbed := []int{}
	for _, chunk := range chunks {
		cached, ok := byID[chunk.ID]
		if ok && cached.Hash == chunk.Hash && len(cached.Embedding) == cfg.Dimensions {
			chunk.Embedding = cached.Embedding
		} else {
			toEmbed = append(toEmbed, len(next))
		}
		next = append(next, chunk)
	}
	if len(toEmbed) > 0 {
		texts := make([]string, len(toEmbed))
		for i, idx := range toEmbed {
			texts[i] = cfg.DocPrefix + next[idx].Content
		}
		vectors, err := cfg.embedBatch(texts)
		if err != nil {
			warnings = append(warnings, "Embedding index update failed; using cached vectors only: "+err.Error())
		} else {
			for i, idx := range toEmbed {
				next[idx].Embedding = vectors[i]
			}
		}
	}
	kept := next[:0]
	for _, chunk := range next {
		if len(chunk.Embedding) == cfg.Dimensions {
			kept = append(kept, chunk)
		}
	}
	index.Chunks = kept
	index.IndexedAt = time.Now()
	if err := writePreviewEmbeddingIndex(projectRoot, index); err != nil {
		warnings = append(warnings, "Embedding index cache could not be saved: "+err.Error())
	}
	return index, warnings
}

func (s *previewEmbeddingSearch) search(query string, limit int) ([]previewSearchResult, []previewSearchResult, error) {
	queryParts := searchQueryParts(query)
	inputs := make([]string, 0, len(queryParts))
	for _, part := range queryParts {
		inputs = append(inputs, s.Config.QueryPrefix+part)
	}
	queryVecs, err := s.Config.embedBatch(inputs)
	if err != nil {
		return nil, nil, err
	}
	docs := []previewSearchResult{}
	code := []previewSearchResult{}
	for _, chunk := range s.Index.Chunks {
		score := 0.0
		for _, queryVec := range queryVecs {
			score = math.Max(score, cosineSimilarity(queryVec, chunk.Embedding))
		}
		if score < 0.3 {
			continue
		}
		result := previewSearchResult{
			ID:        chunk.ID,
			Title:     chunk.Title,
			Path:      chunk.Path,
			Kind:      chunk.Type,
			Source:    "embedding",
			Score:     roundScore(score),
			MatchedBy: []string{"semantic"},
			Excerpt:   compactWhitespace(chunk.Content, 260),
			SpecID:    chunk.SpecID,
			Line:      chunk.Line,
		}
		if chunk.Type == "doc" {
			docs = append(docs, result)
		} else {
			code = append(code, result)
		}
	}
	sortSearchResults(docs)
	sortSearchResults(code)
	return limitResults(docs, limit), limitResults(code, limit), nil
}

func previewEmbeddingConfigForProject(projectRoot string) (previewEmbeddingConfig, string) {
	resolvers := []func(string) (previewEmbeddingConfig, error){
		previewEmbeddingConfigFromKnownsProject,
		func(string) (previewEmbeddingConfig, error) {
			return previewEmbeddingConfigFromKnownsDefault()
		},
		func(string) (previewEmbeddingConfig, error) {
			return previewEmbeddingConfigFromOllama()
		},
	}
	errs := []string{}
	for _, resolve := range resolvers {
		cfg, err := resolve(projectRoot)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if cfg.Dimensions <= 0 {
			dim, err := cfg.probeDimensions()
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			cfg.Dimensions = dim
		}
		return cfg, ""
	}
	warning := "Embedding search is unavailable; using lexical fallback."
	if len(errs) > 0 {
		warning += " " + errs[len(errs)-1]
	}
	return previewEmbeddingConfig{}, warning
}

func previewEmbeddingConfigFromKnownsProject(projectRoot string) (previewEmbeddingConfig, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".knowns", "config.json"))
	if err != nil {
		return previewEmbeddingConfig{}, fmt.Errorf("Knowns project semantic search is not configured.")
	}
	var project struct {
		Settings struct {
			SemanticSearch struct {
				Enabled    bool   `json:"enabled"`
				Model      string `json:"model"`
				Provider   string `json:"provider"`
				Dimensions int    `json:"dimensions"`
				MaxTokens  int    `json:"maxTokens"`
			} `json:"semanticSearch"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(data, &project); err != nil || !project.Settings.SemanticSearch.Enabled || project.Settings.SemanticSearch.Model == "" {
		return previewEmbeddingConfig{}, fmt.Errorf("Knowns project semantic search is not enabled.")
	}
	if project.Settings.SemanticSearch.Provider != "api" {
		return previewEmbeddingConfig{}, fmt.Errorf("Knowns project semantic search uses a non-API provider.")
	}
	settings, err := loadKnownsEmbeddingSettings()
	if err != nil {
		return previewEmbeddingConfig{}, err
	}
	return previewEmbeddingConfigFromKnownsSettings(settings, project.Settings.SemanticSearch.Model, project.Settings.SemanticSearch.Dimensions, "knowns-project")
}

func previewEmbeddingConfigFromKnownsDefault() (previewEmbeddingConfig, error) {
	settings, err := loadKnownsEmbeddingSettings()
	if err != nil {
		return previewEmbeddingConfig{}, err
	}
	modelID := settings.DefaultModel
	if modelID == "" && len(settings.Models) == 1 {
		for id := range settings.Models {
			modelID = id
		}
	}
	if modelID == "" {
		return previewEmbeddingConfig{}, fmt.Errorf("Knowns default embedding model is not configured.")
	}
	return previewEmbeddingConfigFromKnownsSettings(settings, modelID, 0, "knowns-default")
}

func loadKnownsEmbeddingSettings() (knownsEmbeddingSettings, error) {
	home, _ := os.UserHomeDir()
	settingsData, err := os.ReadFile(filepath.Join(home, ".knowns", "settings.json"))
	if err != nil {
		return knownsEmbeddingSettings{}, fmt.Errorf("Knowns embedding settings not found.")
	}
	var settings knownsEmbeddingSettings
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		return knownsEmbeddingSettings{}, fmt.Errorf("Knowns embedding settings could not be parsed.")
	}
	return settings, nil
}

func previewEmbeddingConfigFromKnownsSettings(settings knownsEmbeddingSettings, modelID string, fallbackDimensions int, source string) (previewEmbeddingConfig, error) {
	model, ok := settings.Models[modelID]
	if !ok {
		return previewEmbeddingConfig{}, fmt.Errorf("Knowns embedding model %q is not registered.", modelID)
	}
	modelName := firstNonEmpty(model.Model, modelID)
	provider, ok := settings.Providers[model.Provider]
	if !ok || provider.APIBase == "" {
		return previewEmbeddingConfig{}, fmt.Errorf("Knowns embedding provider %q is not registered.", model.Provider)
	}
	dimensions := model.Dimensions
	if dimensions <= 0 {
		dimensions = fallbackDimensions
	}
	timeout := provider.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	batchSize := provider.BatchSize
	if batchSize <= 0 {
		batchSize = 64
	}
	return previewEmbeddingConfig{
		APIBase:     strings.TrimRight(provider.APIBase, "/"),
		APIKey:      provider.APIKey,
		Model:       modelName,
		Dimensions:  dimensions,
		BatchSize:   batchSize,
		Timeout:     timeout,
		QueryPrefix: embeddingQueryPrefix(modelID + " " + modelName),
		DocPrefix:   embeddingDocPrefix(modelID + " " + modelName),
		Source:      source,
	}, nil
}

func previewEmbeddingConfigFromOllama() (previewEmbeddingConfig, error) {
	const ollamaBase = "http://localhost:11434"
	client := http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(ollamaBase + "/api/tags")
	if err != nil {
		return previewEmbeddingConfig{}, fmt.Errorf("Ollama embedding models were not detected.")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return previewEmbeddingConfig{}, fmt.Errorf("Ollama model discovery returned %s.", res.Status)
	}
	var decoded struct {
		Models []ollamaModel `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		return previewEmbeddingConfig{}, fmt.Errorf("Ollama model discovery could not be parsed.")
	}
	modelName := selectOllamaEmbeddingModel(decoded.Models)
	if modelName == "" {
		return previewEmbeddingConfig{}, fmt.Errorf("No local Ollama embedding model was found.")
	}
	return previewEmbeddingConfig{
		APIBase:     ollamaBase + "/v1",
		Model:       modelName,
		BatchSize:   32,
		Timeout:     30,
		QueryPrefix: embeddingQueryPrefix(modelName),
		DocPrefix:   embeddingDocPrefix(modelName),
		Source:      "ollama",
	}, nil
}

func selectOllamaEmbeddingModel(models []ollamaModel) string {
	priorities := []string{
		"nomic-embed-text",
		"mxbai-embed-large",
		"all-minilm",
		"bge-m3",
		"bge-large",
		"bge-base",
		"bge-small",
	}
	for _, prefix := range priorities {
		for _, model := range models {
			name := strings.ToLower(model.Name)
			if name == prefix || strings.HasPrefix(name, prefix+":") || strings.HasPrefix(name, prefix+"-") {
				return model.Name
			}
		}
	}
	return ""
}

func combineSearchScores(keyword, semantic float64, mode string) (float64, []string) {
	switch mode {
	case "keyword":
		if keyword <= 0 {
			return 0, nil
		}
		return roundScore(keyword), []string{"keyword"}
	case "semantic":
		if semantic <= 0 {
			return 0, nil
		}
		return roundScore(semantic), []string{"semantic"}
	default:
		if keyword <= 0 && semantic <= 0 {
			return 0, nil
		}
		score := (keyword * 0.46) + (semantic * 0.54)
		matchedBy := []string{}
		if keyword > 0 {
			matchedBy = append(matchedBy, "keyword")
		}
		if semantic > 0 {
			matchedBy = append(matchedBy, "semantic")
		}
		return roundScore(score), matchedBy
	}
}

func searchDocsGraph(graph specGraph, graphify graphifyGraph, projectRoot, query string, tokens []string, semantic []previewSearchResult, limit int) []previewSearchResult {
	results := searchDocsGraphFromSemantic(graph, semantic, graphExpansionLimit(limit))
	for _, result := range searchGraphifyFromSemantic(graphify, projectRoot, semantic, graphExpansionLimit(limit), true) {
		result.ID = "docs-graphify:" + result.NodeID
		results = append(results, result)
	}
	results = dedupeSearchResults(results)
	if len(results) == 0 {
		return searchDocsGraphByQuery(graph, graphify, projectRoot, query, tokens, limit)
	}
	sortSearchResults(results)
	return limitResults(results, graphExpansionLimit(limit))
}

func searchDocsGraphByQuery(graph specGraph, graphify graphifyGraph, projectRoot, query string, tokens []string, limit int) []previewSearchResult {
	results := []previewSearchResult{}
	for _, node := range graph.Nodes {
		haystack := strings.Join([]string{node.ID, node.Label, node.Path, node.Category, node.Status}, " ")
		score := graphScore(query, tokens, haystack)
		if score <= 0 {
			continue
		}
		results = append(results, previewSearchResult{
			ID:        "docs-graph:" + node.ID,
			Title:     firstNonEmpty(node.Label, node.ID),
			Path:      node.Path,
			Kind:      firstNonEmpty(node.Type, "doc-node"),
			Score:     score,
			MatchedBy: []string{"graph"},
			SpecID:    node.SpecID,
			NodeID:    node.ID,
			Source:    "docs graph",
			Neighbors: limitNeighbors(docGraphNeighbors(graph, node.ID), 8),
		})
	}
	for _, result := range searchGraphifyNodes(graphify, projectRoot, query, tokens, limit*2, true) {
		result.ID = "docs-graphify:" + result.NodeID
		results = append(results, result)
	}
	sortSearchResults(results)
	return limitResults(dedupeSearchResults(results), limit)
}

func searchCodeGraph(graphify graphifyGraph, projectRoot, query string, tokens []string, semantic []previewSearchResult, limit int) []previewSearchResult {
	results := searchGraphifyFromSemantic(graphify, projectRoot, semantic, graphExpansionLimit(limit), false)
	for i := range results {
		results[i].ID = "code-graphify:" + results[i].NodeID
	}
	results = dedupeSearchResults(results)
	if len(results) == 0 {
		return searchCodeGraphByQuery(graphify, projectRoot, query, tokens, limit)
	}
	sortSearchResults(results)
	return limitResults(results, graphExpansionLimit(limit))
}

func searchCodeGraphByQuery(graphify graphifyGraph, projectRoot, query string, tokens []string, limit int) []previewSearchResult {
	results := searchGraphifyNodes(graphify, projectRoot, query, tokens, limit, false)
	for i := range results {
		results[i].ID = "code-graphify:" + results[i].NodeID
	}
	sortSearchResults(results)
	return limitResults(dedupeSearchResults(results), limit)
}

func searchGraphifyNodes(graph graphifyGraph, projectRoot, query string, tokens []string, limit int, docs bool) []previewSearchResult {
	results := []previewSearchResult{}
	for _, node := range graph.Nodes {
		if classifyGraphifyNode(node) == "doc" != docs {
			continue
		}
		haystack := strings.Join([]string{node.ID, node.Label, node.NormLabel, node.FileType, node.SourceFile, node.SourceLocation, node.Community}, " ")
		score := graphScore(query, tokens, haystack)
		if score <= 0 {
			continue
		}
		line := lineFromLocation(node.SourceLocation)
		results = append(results, previewSearchResult{
			Title:      firstNonEmpty(node.Label, node.ID),
			Path:       relPath(projectRoot, node.SourceFile),
			Kind:       firstNonEmpty(node.FileType, "graph-node"),
			Source:     "graphify",
			Line:       line,
			Score:      score,
			MatchedBy:  []string{"graph"},
			NodeID:     node.ID,
			Community:  node.Community,
			Neighbors:  limitNeighbors(graph.Neighbors[node.ID], maxGraphNeighborUI),
			Confidence: "graphify",
		})
	}
	sortSearchResults(results)
	return limitResults(results, limit)
}

func searchDocsGraphFromSemantic(graph specGraph, semantic []previewSearchResult, limit int) []previewSearchResult {
	if len(semantic) == 0 || len(graph.Nodes) == 0 {
		return nil
	}
	index := newDocsGraphIndex(graph)
	anchors := graphAnchorsFromSemantic(semantic)
	results := map[string]previewSearchResult{}
	for _, anchor := range anchors {
		for _, nodeID := range index.match(anchor) {
			expandDocsGraphAnchor(graph, index, nodeID, anchor, limit, results)
			if len(results) >= limit {
				break
			}
		}
		if len(results) >= limit {
			break
		}
	}
	out := make([]previewSearchResult, 0, len(results))
	for _, result := range results {
		out = append(out, result)
	}
	sortSearchResults(out)
	return limitResults(out, limit)
}

func searchGraphifyFromSemantic(graph graphifyGraph, projectRoot string, semantic []previewSearchResult, limit int, docs bool) []previewSearchResult {
	if len(semantic) == 0 || len(graph.Nodes) == 0 {
		return nil
	}
	index := newGraphifyIndex(graph, projectRoot, docs)
	anchors := graphAnchorsFromSemantic(semantic)
	results := map[string]previewSearchResult{}
	for _, anchor := range anchors {
		for _, nodeID := range index.match(anchor) {
			expandGraphifyAnchor(graph, index, projectRoot, nodeID, anchor, limit, results)
			if len(results) >= limit {
				break
			}
		}
		if len(results) >= limit {
			break
		}
	}
	out := make([]previewSearchResult, 0, len(results))
	for _, result := range results {
		out = append(out, result)
	}
	sortSearchResults(out)
	return limitResults(out, limit)
}

func expandDocsGraphAnchor(graph specGraph, index docsGraphIndex, startID string, anchor graphSearchAnchor, limit int, results map[string]previewSearchResult) {
	type queued struct {
		ID    string
		Depth int
	}
	seen := map[string]bool{startID: true}
	queue := []queued{{ID: startID}}
	for len(queue) > 0 && len(results) < limit {
		item := queue[0]
		queue = queue[1:]
		node, ok := index.Nodes[item.ID]
		if !ok {
			continue
		}
		result := previewSearchResult{
			ID:        "docs-graph:" + node.ID,
			Title:     firstNonEmpty(node.Label, node.ID),
			Path:      node.Path,
			Kind:      firstNonEmpty(node.Type, "doc-node"),
			Score:     graphExpansionScore(anchor.Score, item.Depth),
			MatchedBy: graphExpansionMatchedBy(item.Depth),
			SpecID:    node.SpecID,
			NodeID:    node.ID,
			Source:    "docs graph",
			Anchor:    item.Depth == 0,
			AnchorID:  anchor.ID,
			Depth:     item.Depth,
			Neighbors: limitNeighbors(docGraphNeighbors(graph, node.ID), maxGraphNeighborUI),
		}
		mergeGraphResult(results, result)
		if item.Depth >= graphExpansionDepth(limit) {
			continue
		}
		for _, edge := range index.Edges[item.ID] {
			nextID := edge.To
			if seen[nextID] {
				continue
			}
			seen[nextID] = true
			queue = append(queue, queued{ID: nextID, Depth: item.Depth + 1})
		}
	}
}

func expandGraphifyAnchor(graph graphifyGraph, index graphifyIndex, projectRoot, startID string, anchor graphSearchAnchor, limit int, results map[string]previewSearchResult) {
	type queued struct {
		ID    string
		Depth int
	}
	seen := map[string]bool{startID: true}
	queue := []queued{{ID: startID}}
	for len(queue) > 0 && len(results) < limit {
		item := queue[0]
		queue = queue[1:]
		node, ok := index.Nodes[item.ID]
		if !ok {
			continue
		}
		result := graphifyNodeSearchResult(graph, projectRoot, node, graphExpansionScore(anchor.Score, item.Depth), graphExpansionMatchedBy(item.Depth))
		result.Anchor = item.Depth == 0
		result.AnchorID = anchor.ID
		result.Depth = item.Depth
		result.Neighbors = limitNeighbors(graph.Neighbors[node.ID], maxGraphNeighborUI)
		mergeGraphResult(results, result)
		if item.Depth >= graphExpansionDepth(limit) {
			continue
		}
		for _, edge := range index.Edges[item.ID] {
			nextID := edge.To
			if seen[nextID] {
				continue
			}
			seen[nextID] = true
			queue = append(queue, queued{ID: nextID, Depth: item.Depth + 1})
		}
	}
}

func graphifyNodeSearchResult(graph graphifyGraph, projectRoot string, node graphifyNode, score float64, matchedBy []string) previewSearchResult {
	line := lineFromLocation(node.SourceLocation)
	return previewSearchResult{
		Title:      firstNonEmpty(node.Label, node.ID),
		Path:       relPath(projectRoot, node.SourceFile),
		Kind:       firstNonEmpty(node.FileType, "graph-node"),
		Source:     "graphify",
		Line:       line,
		Score:      score,
		MatchedBy:  matchedBy,
		NodeID:     node.ID,
		Community:  node.Community,
		Neighbors:  limitNeighbors(graph.Neighbors[node.ID], maxGraphNeighborUI),
		Confidence: "graphify",
	}
}

func docGraphNeighbors(graph specGraph, nodeID string) []previewSearchNeighbor {
	neighbors := []previewSearchNeighbor{}
	for _, edge := range graph.Edges {
		switch {
		case edge.From == nodeID:
			neighbors = append(neighbors, previewSearchNeighbor{ID: edge.To, Label: edge.To, Relation: edge.Type})
		case edge.To == nodeID:
			neighbors = append(neighbors, previewSearchNeighbor{ID: edge.From, Label: edge.From, Relation: edge.Type})
		}
	}
	return neighbors
}

func newDocsGraphIndex(graph specGraph) docsGraphIndex {
	index := docsGraphIndex{
		Nodes: map[string]graphNode{},
		Keys:  map[string][]string{},
		Edges: map[string][]graphEdge{},
	}
	for _, node := range graph.Nodes {
		index.Nodes[node.ID] = node
		for _, key := range graphNodeKeys(node) {
			index.Keys[key] = appendUniqueString(index.Keys[key], node.ID)
		}
	}
	for _, edge := range graph.Edges {
		if edge.From == "" || edge.To == "" {
			continue
		}
		index.Edges[edge.From] = append(index.Edges[edge.From], graphEdge{From: edge.From, To: edge.To, Type: firstNonEmpty(edge.Type, edge.Label, "references"), Label: edge.Label, Origin: edge.Origin, Raw: edge.Raw})
		index.Edges[edge.To] = append(index.Edges[edge.To], graphEdge{From: edge.To, To: edge.From, Type: firstNonEmpty(edge.Type, edge.Label, "references"), Label: edge.Label, Origin: edge.Origin, Raw: edge.Raw})
	}
	return index
}

func (index docsGraphIndex) match(anchor graphSearchAnchor) []string {
	matches := []string{}
	for _, key := range anchorKeys(anchor) {
		for _, nodeID := range index.Keys[key] {
			matches = appendUniqueString(matches, nodeID)
		}
	}
	return matches
}

func newGraphifyIndex(graph graphifyGraph, projectRoot string, docs bool) graphifyIndex {
	index := graphifyIndex{
		Nodes: map[string]graphifyNode{},
		Keys:  map[string][]string{},
		Edges: map[string][]graphifyEdge{},
		Root:  projectRoot,
	}
	for id, node := range graph.Nodes {
		if classifyGraphifyNode(node) == "doc" != docs {
			continue
		}
		index.Nodes[id] = node
		for _, key := range graphifyNodeKeys(node, projectRoot) {
			index.Keys[key] = appendUniqueString(index.Keys[key], id)
		}
	}
	for id, neighbors := range graph.Neighbors {
		if _, ok := index.Nodes[id]; !ok {
			continue
		}
		for _, neighbor := range neighbors {
			if _, ok := index.Nodes[neighbor.ID]; !ok {
				continue
			}
			index.Edges[id] = append(index.Edges[id], graphifyEdge{To: neighbor.ID, Relation: neighbor.Relation, Confidence: neighbor.Confidence})
		}
	}
	return index
}

func (index graphifyIndex) match(anchor graphSearchAnchor) []string {
	matches := []string{}
	for _, key := range anchorKeys(anchor) {
		for _, nodeID := range index.Keys[key] {
			matches = appendUniqueString(matches, nodeID)
		}
	}
	if len(matches) > 0 {
		return matches
	}
	for id, node := range index.Nodes {
		if anchor.Path != "" && graphPathMatches(relPath(index.Root, node.SourceFile), anchor.Path) {
			matches = appendUniqueString(matches, id)
			continue
		}
		if anchor.Title != "" && fuzzyGraphKeyMatch(anchor.Title, strings.Join([]string{node.ID, node.Label, node.NormLabel}, " ")) {
			matches = appendUniqueString(matches, id)
		}
	}
	return matches
}

func graphAnchorsFromSemantic(semantic []previewSearchResult) []graphSearchAnchor {
	anchors := make([]graphSearchAnchor, 0, len(semantic))
	for _, result := range semantic {
		anchor := graphSearchAnchor{
			ID:     firstNonEmpty(result.ID, result.SpecID, result.Path, result.Title),
			Title:  result.Title,
			Path:   result.Path,
			SpecID: result.SpecID,
			Line:   result.Line,
			Score:  result.Score,
		}
		if anchor.Score <= 0 {
			anchor.Score = 0.55
		}
		if anchor.ID == "" && anchor.Path == "" && anchor.Title == "" {
			continue
		}
		anchors = append(anchors, anchor)
	}
	return anchors
}

func graphNodeKeys(node graphNode) []string {
	values := []string{node.ID, node.Label, node.Path, node.SpecID}
	if node.Path != "" {
		values = append(values, "docs/"+strings.TrimPrefix(node.Path, "docs/"))
	}
	return normalizedGraphKeys(values...)
}

func graphifyNodeKeys(node graphifyNode, projectRoot string) []string {
	rel := relPath(projectRoot, node.SourceFile)
	return normalizedGraphKeys(node.ID, node.Label, node.NormLabel, rel, strings.TrimPrefix(rel, "docs/"), filepath.Base(rel))
}

func anchorKeys(anchor graphSearchAnchor) []string {
	values := []string{anchor.ID, anchor.Title, anchor.Path, anchor.SpecID}
	if anchor.Path != "" {
		values = append(values, "docs/"+strings.TrimPrefix(anchor.Path, "docs/"), strings.TrimPrefix(anchor.Path, "docs/"), filepath.Base(anchor.Path))
	}
	return normalizedGraphKeys(values...)
}

func normalizedGraphKeys(values ...string) []string {
	keys := []string{}
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(filepath.ToSlash(value)))
		if value == "" {
			continue
		}
		keys = appendUniqueString(keys, value)
		tokenKey := strings.Join(searchTokens(value), "")
		if tokenKey != "" {
			keys = appendUniqueString(keys, tokenKey)
		}
	}
	return keys
}

func graphPathMatches(sourcePath, anchorPath string) bool {
	sourcePath = strings.TrimPrefix(strings.ToLower(filepath.ToSlash(sourcePath)), "./")
	anchorPath = strings.TrimPrefix(strings.ToLower(filepath.ToSlash(anchorPath)), "./")
	return sourcePath == anchorPath || strings.TrimPrefix(sourcePath, "docs/") == strings.TrimPrefix(anchorPath, "docs/") || strings.HasSuffix(sourcePath, "/"+anchorPath)
}

func fuzzyGraphKeyMatch(needle, haystack string) bool {
	needleKey := strings.Join(searchTokens(needle), "")
	haystackKey := strings.Join(searchTokens(haystack), "")
	return needleKey != "" && haystackKey != "" && (strings.Contains(haystackKey, needleKey) || strings.Contains(needleKey, haystackKey))
}

func graphExpansionLimit(limit int) int {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	return minInt(maxSearchLimit, maxInt(limit, limit*3))
}

func graphExpansionDepth(limit int) int {
	if limit >= 8 {
		return 3
	}
	return 2
}

func graphExpansionScore(anchorScore float64, depth int) float64 {
	if anchorScore <= 0 {
		anchorScore = 0.55
	}
	return roundScore(math.Max(0.05, anchorScore-(float64(depth)*0.12)))
}

func graphExpansionMatchedBy(depth int) []string {
	if depth == 0 {
		return []string{"semantic-anchor", "graph"}
	}
	return []string{"graph-expansion", "graph"}
}

func mergeGraphResult(results map[string]previewSearchResult, next previewSearchResult) {
	key := graphResultMergeKey(next)
	current, ok := results[key]
	if !ok || next.Score > current.Score || (next.Anchor && !current.Anchor) || next.Depth < current.Depth {
		results[key] = next
	}
}

func graphResultMergeKey(result previewSearchResult) string {
	return firstNonEmpty(result.NodeID, result.ID, result.SpecID, result.Path, result.Title)
}

func appendUniqueString(values []string, value string) []string {
	if value == "" || containsString(values, value) {
		return values
	}
	return append(values, value)
}

func limitNeighbors(neighbors []previewSearchNeighbor, limit int) []previewSearchNeighbor {
	if limit <= 0 || len(neighbors) <= limit {
		return neighbors
	}
	return neighbors[:limit]
}

func boostSemanticWithGraph(semantic []previewSearchResult, graph []previewSearchResult) {
	if len(semantic) == 0 || len(graph) == 0 {
		return
	}
	graphPaths := map[string]bool{}
	for _, item := range graph {
		if item.Path != "" {
			graphPaths[item.Path] = true
		}
		if item.SpecID != "" {
			graphPaths[item.SpecID] = true
		}
	}
	for i := range semantic {
		if graphPaths[semantic[i].Path] || graphPaths[semantic[i].SpecID] {
			semantic[i].Score = roundScore(math.Min(1, semantic[i].Score+0.08))
			if !containsString(semantic[i].MatchedBy, "graph") {
				semantic[i].MatchedBy = append(semantic[i].MatchedBy, "graph")
			}
		}
	}
	sortSearchResults(semantic)
}

func combineEmbeddingResults(keyword, semantic []previewSearchResult, mode string, limit int) []previewSearchResult {
	switch mode {
	case "semantic":
		sortSearchResults(semantic)
		return limitResults(semantic, limit)
	case "keyword":
		sortSearchResults(keyword)
		return limitResults(keyword, limit)
	default:
		return limitResults(mergeResultsRRF(keyword, semantic), limit)
	}
}

func mergeResultsRRF(keyword, semantic []previewSearchResult) []previewSearchResult {
	const k = 60.0
	type merged struct {
		result    previewSearchResult
		rrf       float64
		matchedBy []string
	}
	items := map[string]*merged{}
	sortSearchResults(keyword)
	sortSearchResults(semantic)
	for rank, result := range keyword {
		key := searchResultMergeKey(result)
		items[key] = &merged{result: result, rrf: 1 / (k + float64(rank+1)), matchedBy: []string{"keyword"}}
	}
	for rank, result := range semantic {
		key := searchResultMergeKey(result)
		score := 1 / (k + float64(rank+1))
		if item, ok := items[key]; ok {
			item.rrf += score
			item.matchedBy = mergeMatchMethods(item.matchedBy, "semantic")
			continue
		}
		items[key] = &merged{result: result, rrf: score, matchedBy: []string{"semantic"}}
	}
	maxScore := 0.0
	for _, item := range items {
		if item.rrf > maxScore {
			maxScore = item.rrf
		}
	}
	results := make([]previewSearchResult, 0, len(items))
	for _, item := range items {
		if maxScore > 0 {
			item.result.Score = roundScore(item.rrf / maxScore)
		}
		item.result.MatchedBy = item.matchedBy
		results = append(results, item.result)
	}
	sortSearchResults(results)
	return results
}

func searchResultMergeKey(result previewSearchResult) string {
	return firstNonEmpty(result.SpecID, result.Path, result.ID, result.Title)
}

func mergeMatchMethods(methods []string, method string) []string {
	if containsString(methods, method) {
		return methods
	}
	return append([]string{method}, methods...)
}

func keywordScore(query string, tokens []string, title string, path string, content string) float64 {
	lowerTitle := strings.ToLower(title)
	lowerPath := strings.ToLower(path)
	lowerContent := strings.ToLower(content)
	score := 0.0
	for _, lowerQuery := range searchQueryParts(query) {
		if strings.Contains(lowerTitle, lowerQuery) {
			score += 0.52
		}
		if strings.Contains(lowerPath, lowerQuery) {
			score += 0.42
		}
		if strings.Contains(lowerContent, lowerQuery) {
			score += 0.24
		}
	}
	for _, token := range tokens {
		switch {
		case strings.Contains(lowerTitle, token):
			score += 0.16
		case strings.Contains(lowerPath, token):
			score += 0.13
		case strings.Contains(lowerContent, token):
			score += 0.05
		}
	}
	return clamp01(score)
}

func semanticScore(tokens []string, title string, path string, headers []string, content string) float64 {
	fields := searchTokens(strings.Join(append([]string{title, path}, headers...), " "))
	contentTokens := searchTokens(content)
	if len(tokens) == 0 {
		return 0
	}
	score := 0.0
	for _, token := range tokens {
		if tokenIn(token, fields) {
			score += 0.22
			continue
		}
		if fuzzyTokenIn(token, fields) {
			score += 0.14
			continue
		}
		if tokenIn(token, contentTokens) {
			score += 0.09
			continue
		}
		if fuzzyTokenIn(token, contentTokens) {
			score += 0.045
		}
	}
	return clamp01(score / math.Max(1, float64(len(tokens))) * 3.2)
}

func graphScore(query string, tokens []string, haystack string) float64 {
	lower := strings.ToLower(haystack)
	score := 0.0
	for _, lowerQuery := range searchQueryParts(query) {
		if strings.Contains(lower, lowerQuery) {
			score += 0.65
		}
	}
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			score += 0.18
		}
	}
	return clamp01(score)
}

func searchQueryParts(query string) []string {
	parts := []string{}
	for _, part := range strings.Split(query, ",") {
		part = strings.ToLower(strings.Join(strings.Fields(part), " "))
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		query = strings.ToLower(strings.Join(strings.Fields(query), " "))
		if query != "" {
			parts = append(parts, query)
		}
	}
	return uniqueStrings(parts)
}

func searchTokens(value string) []string {
	value = strings.ToLower(value)
	tokens := []string{}
	var b strings.Builder
	flush := func() {
		if b.Len() < 2 {
			b.Reset()
			return
		}
		tokens = append(tokens, b.String())
		b.Reset()
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return uniqueStrings(tokens)
}

func headingsFromMarkdown(raw string) []string {
	headings := []string{}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			headings = append(headings, strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
		}
	}
	return headings
}

func codeSymbols(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*(?:func\s+(?:\([^)]*\)\s*)?|type\s+|const\s+|var\s+|class\s+|interface\s+|function\s+)([A-Za-z_][A-Za-z0-9_]*)`)
	out := []string{}
	for _, match := range re.FindAllStringSubmatch(content, -1) {
		if len(match) == 2 {
			out = append(out, match[1])
		}
	}
	return out
}

func tokenIn(token string, values []string) bool {
	for _, value := range values {
		if value == token {
			return true
		}
	}
	return false
}

func fuzzyTokenIn(token string, values []string) bool {
	if len(token) < 3 {
		return false
	}
	for _, value := range values {
		if len(value) >= 3 && (strings.Contains(value, token) || strings.Contains(token, value)) {
			return true
		}
	}
	return false
}

func excerptForQuery(content string, tokens []string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if lineMatchesTokens(line, tokens) {
			return compactWhitespace(line, 220)
		}
	}
	return compactWhitespace(content, 220)
}

func codeExcerptForQuery(content string, tokens []string) (int, string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lineMatchesTokens(line, tokens) {
			start := maxInt(0, i-1)
			end := minInt(len(lines), i+2)
			return i + 1, compactWhitespace(strings.Join(lines[start:end], "\n"), 260)
		}
	}
	return 1, compactWhitespace(content, 260)
}

func lineMatchesTokens(line string, tokens []string) bool {
	lower := strings.ToLower(line)
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func compactWhitespace(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit-1]) + "…"
}

func scanDocsSearchDocs(projectRoot, docsRoot string, specs []specDocument) ([]docsSearchDoc, []string) {
	docs := make([]docsSearchDoc, 0, len(specs))
	seen := map[string]bool{}
	for _, doc := range specs {
		docs = append(docs, docsSearchDoc{
			ID:      doc.ID,
			Title:   doc.Title,
			Path:    doc.Path,
			Content: doc.Raw,
			SpecID:  doc.ID,
			Kind:    "doc",
		})
		seen[doc.ID] = true
	}
	warnings := []string{}
	err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		docRel, err := filepath.Rel(docsRoot, path)
		if err != nil {
			return nil
		}
		docRel = filepath.ToSlash(docRel)
		if seen[docRel] {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxSearchFileBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !utf8.Valid(data) {
			return nil
		}
		projectRel := relPath(projectRoot, path)
		docs = append(docs, docsSearchDoc{
			ID:      projectRel,
			Title:   filepath.Base(path),
			Path:    projectRel,
			Content: string(data),
			Kind:    "file",
		})
		seen[docRel] = true
		return nil
	})
	if err != nil {
		warnings = append(warnings, "Docs search scan failed: "+err.Error())
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})
	return docs, warnings
}

func scanCodeSearchDocs(projectRoot, docsRoot string) ([]codeSearchDoc, []string) {
	docs := []codeSearchDoc{}
	warnings := []string{}
	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if sameCleanPath(path, docsRoot) && !sameCleanPath(path, projectRoot) {
				return filepath.SkipDir
			}
			if shouldSkipSearchDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isSearchableCodePath(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxSearchFileBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !utf8.Valid(data) {
			return nil
		}
		rel := relPath(projectRoot, path)
		docs = append(docs, codeSearchDoc{
			ID:      rel,
			Title:   filepath.Base(path),
			Path:    rel,
			Content: string(data),
		})
		return nil
	})
	if err != nil {
		warnings = append(warnings, "Code search scan failed: "+err.Error())
	}
	return docs, warnings
}

func sameCleanPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil {
		a = absA
	}
	if errB == nil {
		b = absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func shouldSkipSearchDir(name string) bool {
	switch name {
	case ".git", "node_modules", "graphify-out", ".cache", "dist", "build", "vendor":
		return true
	default:
		return false
	}
}

func isSearchableCodePath(path string) bool {
	return isPreviewableFilePath(path) && filepath.Ext(path) != ".md"
}

func isPreviewableFilePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".css", ".scss", ".sass", ".html", ".json", ".yaml", ".yml", ".toml", ".md", ".cjs", ".mjs", ".vue", ".svelte", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".swift", ".c", ".h", ".cpp", ".hpp", ".cs", ".php", ".sh", ".bash", ".zsh", ".fish", ".sql", ".xml", ".graphql", ".gql", ".dockerfile":
		return true
	default:
		return strings.EqualFold(filepath.Base(path), "Dockerfile")
	}
}

func languageForPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	if base == "dockerfile" {
		return "dockerfile"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js", ".cjs", ".mjs":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".sass":
		return "sass"
	case ".html":
		return "xml"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".sh", ".bash", ".zsh", ".fish":
		return "bash"
	case ".sql":
		return "sql"
	case ".xml":
		return "xml"
	case ".graphql", ".gql":
		return "graphql"
	default:
		return "plaintext"
	}
}

func loadGraphifyGraph(projectRoot string) graphifyGraph {
	graph := graphifyGraph{Nodes: map[string]graphifyNode{}, Neighbors: map[string][]previewSearchNeighbor{}}
	path := filepath.Join(projectRoot, "graphify-out", "graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		graph.Warnings = append(graph.Warnings, "Graphify graph not found; code graph panel will use available sources only.")
		return graph
	}
	var raw struct {
		Nodes []map[string]any `json:"nodes"`
		Links []map[string]any `json:"links"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		graph.Warnings = append(graph.Warnings, "Graphify graph could not be parsed: "+err.Error())
		return graph
	}
	for _, item := range raw.Nodes {
		node := graphifyNode{
			ID:             stringField(item, "id"),
			Label:          stringField(item, "label"),
			FileType:       stringField(item, "file_type"),
			SourceFile:     stringField(item, "source_file"),
			SourceLocation: stringField(item, "source_location"),
			Community:      stringField(item, "community"),
			NormLabel:      stringField(item, "norm_label"),
		}
		if node.ID == "" {
			continue
		}
		graph.Nodes[node.ID] = node
	}
	for _, link := range append(raw.Links, raw.Edges...) {
		edge := graphifyLink{
			Source:     firstNonEmpty(stringField(link, "source"), stringField(link, "_src"), stringField(link, "from")),
			Target:     firstNonEmpty(stringField(link, "target"), stringField(link, "_tgt"), stringField(link, "to")),
			Relation:   firstNonEmpty(stringField(link, "relation"), stringField(link, "type"), stringField(link, "label")),
			Confidence: stringField(link, "confidence"),
		}
		if edge.Source == "" || edge.Target == "" {
			continue
		}
		target := graph.Nodes[edge.Target]
		source := graph.Nodes[edge.Source]
		graph.Neighbors[edge.Source] = append(graph.Neighbors[edge.Source], previewSearchNeighbor{
			ID:         edge.Target,
			Label:      firstNonEmpty(target.Label, edge.Target),
			Relation:   edge.Relation,
			Confidence: edge.Confidence,
			Path:       relPath(projectRoot, target.SourceFile),
			Line:       lineFromLocation(target.SourceLocation),
		})
		graph.Neighbors[edge.Target] = append(graph.Neighbors[edge.Target], previewSearchNeighbor{
			ID:         edge.Source,
			Label:      firstNonEmpty(source.Label, edge.Source),
			Relation:   edge.Relation,
			Confidence: edge.Confidence,
			Path:       relPath(projectRoot, source.SourceFile),
			Line:       lineFromLocation(source.SourceLocation),
		})
	}
	return graph
}

func classifyGraphifyNode(node graphifyNode) string {
	ext := strings.ToLower(filepath.Ext(node.SourceFile))
	if node.FileType == "doc" || ext == ".md" {
		return "doc"
	}
	return "code"
}

func stringField(item map[string]any, key string) string {
	value, ok := item[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == math.Trunc(typed) {
			return strconv.Itoa(int(typed))
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(toJSONish(typed)), "\n", " "), "\t", " "), `"`))
	}
}

func toJSONish(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func lineFromLocation(value string) int {
	for _, pattern := range []string{`(?i)\bL(\d+)\b`, `(?i)\bline\s+(\d+)\b`, `:(\d+)(?::\d+)?(?:\D*$|$)`} {
		match := regexp.MustCompile(pattern).FindStringSubmatch(value)
		if len(match) == 2 {
			line, _ := strconv.Atoi(match[1])
			return line
		}
	}
	return 0
}

func (cfg previewEmbeddingConfig) embedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 64
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		vectors, err := cfg.embedBatchRaw(texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vectors...)
	}
	return out, nil
}

func (cfg previewEmbeddingConfig) probeDimensions() (int, error) {
	vectors, err := cfg.embedBatch([]string{cfg.QueryPrefix + "preview search"})
	if err != nil {
		return 0, fmt.Errorf("%s embedding dimensions could not be detected: %w", cfg.Source, err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return 0, fmt.Errorf("%s embedding dimensions could not be detected.", cfg.Source)
	}
	return len(vectors[0]), nil
}

func (cfg previewEmbeddingConfig) embedBatchRaw(texts []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"model": cfg.Model, "input": texts})
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(cfg.APIBase, "/")+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	client := http.Client{Timeout: time.Duration(timeout) * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("embedding API returned %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	var decoded struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("embedding API returned %d vectors for %d texts", len(decoded.Data), len(texts))
	}
	vectors := make([][]float32, len(texts))
	for i, item := range decoded.Data {
		idx := item.Index
		if idx < 0 || idx >= len(texts) {
			idx = i
		}
		if cfg.Dimensions > 0 && len(item.Embedding) != cfg.Dimensions {
			return nil, fmt.Errorf("embedding vector has %d dimensions, want %d", len(item.Embedding), cfg.Dimensions)
		}
		vectors[idx] = item.Embedding
	}
	return vectors, nil
}

func readPreviewEmbeddingIndex(projectRoot string) previewEmbeddingIndex {
	data, err := os.ReadFile(previewEmbeddingIndexPath(projectRoot))
	if err != nil {
		return previewEmbeddingIndex{}
	}
	var index previewEmbeddingIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return previewEmbeddingIndex{}
	}
	return index
}

func writePreviewEmbeddingIndex(projectRoot string, index previewEmbeddingIndex) error {
	path := previewEmbeddingIndexPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func previewEmbeddingIndexPath(projectRoot string) string {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	sum := sha256.Sum256([]byte(projectRoot))
	return filepath.Join(cache, "ns-workspace", "preview-search", hex.EncodeToString(sum[:8]), "embedding-index.json")
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func embeddingQueryPrefix(model string) string {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "nomic-embed-text"):
		return "search_query: "
	case strings.Contains(model, "multilingual-e5"):
		return "query: "
	case strings.Contains(model, "bge-"):
		return "Represent this sentence: "
	default:
		return ""
	}
}

func embeddingDocPrefix(model string) string {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "nomic-embed-text"):
		return "search_document: "
	case strings.Contains(model, "multilingual-e5"):
		return "passage: "
	case strings.Contains(model, "bge-"):
		return "Represent this sentence: "
	default:
		return ""
	}
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		magA += av * av
		magB += bv * bv
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

func sortSearchResults(results []previewSearchResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Title != results[j].Title {
			return results[i].Title < results[j].Title
		}
		return results[i].Path < results[j].Path
	})
}

func limitResults(results []previewSearchResult, limit int) []previewSearchResult {
	if len(results) > limit {
		return results[:limit]
	}
	return results
}

func dedupeSearchResults(results []previewSearchResult) []previewSearchResult {
	seen := map[string]bool{}
	out := []previewSearchResult{}
	for _, result := range results {
		key := firstNonEmpty(result.ID, result.NodeID, result.Path, result.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, result)
	}
	return out
}

func relPath(root, path string) string {
	if path == "" {
		return ""
	}
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func roundScore(score float64) float64 {
	return math.Round(clamp01(score)*1000) / 1000
}

func clamp01(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
