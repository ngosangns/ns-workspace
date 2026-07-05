package preview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

const (
	defaultSearchLimit = 8
	maxSearchLimit     = 24
	maxSearchFileBytes = 256 * 1024
	maxGraphNeighborUI = 10
)

type previewSearchResponse struct {
	Query           string              `json:"query"`
	Mode            string              `json:"mode"`
	KeywordOperator string              `json:"keywordOperator"`
	Panels          previewSearchPanels `json:"panels"`
	Stats           map[string]int      `json:"stats"`
	Warnings        []string            `json:"warnings,omitempty"`
}

type previewSearchPanels struct {
	DocsSemantic []previewSearchResult `json:"docsSemantic"`
	DocsGraph    []previewSearchResult `json:"docsGraph"`
	CodeSemantic []previewSearchResult `json:"codeSemantic"`
	CodeGraph    []previewSearchResult `json:"codeGraph"`
}

type previewSearchResult struct {
	ID          string                  `json:"id"`
	Title       string                  `json:"title"`
	Path        string                  `json:"path,omitempty"`
	Kind        string                  `json:"kind,omitempty"`
	Source      string                  `json:"source,omitempty"`
	Line        int                     `json:"line,omitempty"`
	Score       float64                 `json:"score"`
	MatchedBy   []string                `json:"matchedBy,omitempty"`
	Description string                  `json:"description,omitempty"`
	Excerpt     string                  `json:"excerpt,omitempty"`
	SpecID      string                  `json:"specId,omitempty"`
	NodeID      string                  `json:"nodeId,omitempty"`
	Community   string                  `json:"community,omitempty"`
	Relation    string                  `json:"relation,omitempty"`
	Confidence  string                  `json:"confidence,omitempty"`
	Anchor      bool                    `json:"anchor,omitempty"`
	AnchorID    string                  `json:"anchorId,omitempty"`
	Depth       int                     `json:"depth,omitempty"`
	FlowRole    string                  `json:"flowRole,omitempty"`
	FlowAnchor  string                  `json:"flowAnchorId,omitempty"`
	FlowDepth   int                     `json:"flowDepth,omitempty"`
	Neighbors   []previewSearchNeighbor `json:"neighbors,omitempty"`
}

type previewSearchNeighbor struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Relation   string `json:"relation,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Direction  string `json:"direction,omitempty"`
	SourceID   string `json:"sourceId,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	Path       string `json:"path,omitempty"`
	Line       int    `json:"line,omitempty"`
}

type PreviewCodeGraphProvider interface {
	SearchCodeGraph(ctx context.Context, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) ([]previewSearchResult, []string)
	Close(ctx context.Context) error
}

// previewCodeGraphProvider is the old unexported alias, kept for compatibility
// with existing code in this package until a full rename pass is done.
type previewCodeGraphProvider = PreviewCodeGraphProvider

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

type codeSearchDoc struct {
	ID      string
	Title   string
	Path    string
	Content string
	Symbols []string
}

type searchEvidence struct {
	Score     float64
	Exactness int
}

type docsSearchDoc struct {
	ID          string
	Title       string
	Path        string
	Content     string
	Headings    []string
	Description string
	SpecID      string
	Kind        string
}

type previewSearchSnapshot struct {
	Docs     []docsSearchDoc
	Code     []codeSearchDoc
	Warnings []string
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
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Path        string    `json:"path"`
	SpecID      string    `json:"specId,omitempty"`
	Line        int       `json:"line,omitempty"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"`
	Hash        string    `json:"hash"`
	Embedding   []float32 `json:"embedding"`
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

type knownsEmbeddingProvider struct {
	APIBase   string `json:"apiBase"`
	APIKey    string `json:"apiKey"`
	Timeout   int    `json:"timeout"`
	BatchSize int    `json:"batchSize"`
}

type knownsEmbeddingModel struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

type knownsEmbeddingSettings struct {
	Providers    map[string]knownsEmbeddingProvider `json:"embeddingProviders"`
	Models       map[string]knownsEmbeddingModel    `json:"embeddingModels"`
	DefaultModel string                             `json:"defaultEmbeddingModel"`
}

type ollamaModel struct {
	Name string `json:"name"`
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

func buildPreviewSearchResponse(ctx context.Context, project specProject, codeGraph previewCodeGraphProvider, projectRoot, query, mode, keywordOperator string, limit int) previewSearchResponse {
	docsDocs, docsWarnings := scanDocsSearchDocs(projectRoot, project.Summary.DocsRoot, project.Documents)
	codeDocs, codeWarnings := scanCodeSearchDocs(projectRoot, project.Summary.DocsRoot)
	return buildPreviewSearchResponseFromCorpus(ctx, project, codeGraph, query, mode, keywordOperator, limit, docsDocs, codeDocs, append(docsWarnings, codeWarnings...))
}

func buildPreviewSearchResponseFromCorpus(ctx context.Context, project specProject, codeGraph previewCodeGraphProvider, query, mode, keywordOperator string, limit int, docsDocs []docsSearchDoc, codeDocs []codeSearchDoc, corpusWarnings []string) previewSearchResponse {
	response := previewSearchResponse{
		Query:           query,
		Mode:            mode,
		KeywordOperator: keywordOperator,
		Stats:           map[string]int{},
		Warnings:        append([]string{}, corpusWarnings...),
	}
	if query == "" {
		response.Warnings = append(response.Warnings, "Enter a query to search docs and code.")
		return response
	}
	searchQuery, exclusionQuery := searchQueriesForKeywordOperator(query, keywordOperator)
	tokens := searchTokens(searchQuery)
	if len(tokens) == 0 {
		response.Warnings = append(response.Warnings, "Query has no searchable tokens.")
		return response
	}
	exclusionTokens := searchTokens(exclusionQuery)

	if mode != "graph" {
		docsDocs = filterDocsSearchDocs(docsDocs, exclusionQuery, exclusionTokens)
		codeDocs = filterCodeSearchDocs(codeDocs, exclusionQuery, exclusionTokens)
		var embedSearch *previewEmbeddingSearch
		if mode == "semantic" || mode == "hybrid" {
			embedSearch, _ = loadPreviewEmbeddingSearch(project.Summary.ProjectRoot, docsDocs, codeDocs)
		}
		if embedSearch != nil {
			docKeyword := searchDocsSemantic(docsDocs, searchQuery, tokens, "keyword", limit*2)
			codeKeyword := searchCodeSemantic(codeDocs, searchQuery, tokens, "keyword", limit*2)
			docSemantic, codeSemantic, err := embedSearch.search(searchQuery, limit*2)
			if err == nil {
				codeSemantic = filterCodeEmbeddingResultsByKeywordEvidence(codeSemantic, codeDocs, searchQuery, tokens)
				response.Panels.DocsSemantic = combineEmbeddingResults(docKeyword, docSemantic, mode, limit)
				response.Panels.CodeSemantic = combineEmbeddingResults(codeKeyword, codeSemantic, mode, limit)
			} else {
				response.Warnings = append(response.Warnings, "Embedding search failed; using lexical fallback: "+err.Error())
				response.Panels.DocsSemantic = searchDocsSemantic(docsDocs, searchQuery, tokens, mode, limit)
				response.Panels.CodeSemantic = searchCodeSemantic(codeDocs, searchQuery, tokens, mode, limit)
			}
		} else {
			response.Panels.DocsSemantic = searchDocsSemantic(docsDocs, searchQuery, tokens, mode, limit)
			response.Panels.CodeSemantic = searchCodeSemantic(codeDocs, searchQuery, tokens, mode, limit)
		}
	}
	if mode != "keyword" && mode != "semantic" {
		response.Panels.DocsGraph = searchDocsGraph(project.Graph, searchQuery, tokens, exclusionQuery, exclusionTokens, limit)
		if codeGraph != nil {
			var codeWarnings []string
			response.Panels.CodeGraph, codeWarnings = codeGraph.SearchCodeGraph(ctx, searchQuery, tokens, exclusionQuery, exclusionTokens, limit)
			response.Warnings = append(response.Warnings, codeWarnings...)
		}
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

func parseSearchKeywordOperator(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "difference":
		return "difference"
	default:
		return "sum"
	}
}

func searchQueriesForKeywordOperator(query, operator string) (string, string) {
	if operator != "difference" {
		return query, ""
	}
	parts := searchQueryParts(query)
	if len(parts) <= 1 {
		return query, ""
	}
	return parts[0], strings.Join(parts[1:], ",")
}

func filterDocsSearchDocs(docs []docsSearchDoc, exclusionQuery string, exclusionTokens []string) []docsSearchDoc {
	if exclusionQuery == "" || len(exclusionTokens) == 0 {
		return docs
	}
	filtered := docs[:0]
	for _, doc := range docs {
		if excludedByKeywordSearch(exclusionQuery, exclusionTokens, doc.Title, doc.Path, doc.Content) {
			continue
		}
		filtered = append(filtered, doc)
	}
	return filtered
}

func filterCodeSearchDocs(docs []codeSearchDoc, exclusionQuery string, exclusionTokens []string) []codeSearchDoc {
	if exclusionQuery == "" || len(exclusionTokens) == 0 {
		return docs
	}
	filtered := docs[:0]
	for _, doc := range docs {
		if excludedByKeywordSearch(exclusionQuery, exclusionTokens, doc.Title, doc.Path, doc.Content) {
			continue
		}
		filtered = append(filtered, doc)
	}
	return filtered
}

func excludedByKeywordSearch(query string, tokens []string, title string, path string, content string) bool {
	return query != "" && len(tokens) > 0 && keywordScore(query, tokens, title, path, content) > 0
}

func searchDocsSemantic(docs []docsSearchDoc, query string, tokens []string, mode string, limit int) []previewSearchResult {
	results := []previewSearchResult{}
	for _, doc := range docs {
		if doc.SpecID != "" && isSpecControlFile(doc.SpecID) {
			continue
		}
		keyword := keywordScore(query, tokens, doc.Title, doc.Path, doc.Content)
		headings := doc.Headings
		if len(headings) == 0 {
			headings = headingsFromMarkdown(doc.Content)
		}
		semantic := semanticScore(tokens, doc.Title, doc.Path, headings, doc.Content)
		score, matchedBy := combineSearchScores(keyword, semantic, mode)
		if score <= 0 {
			continue
		}
		results = append(results, previewSearchResult{
			ID:          "doc:" + doc.ID,
			Title:       doc.Title,
			Path:        doc.Path,
			Kind:        doc.Kind,
			Score:       score,
			MatchedBy:   matchedBy,
			Description: doc.Description,
			Excerpt:     excerptForQuery(doc.Content, tokens),
			SpecID:      doc.SpecID,
			Source:      "docs",
		})
	}
	sortSearchResults(results)
	return limitResults(results, limit)
}

func searchCodeSemantic(codeDocs []codeSearchDoc, query string, tokens []string, mode string, limit int) []previewSearchResult {
	results := []previewSearchResult{}
	for _, doc := range codeDocs {
		symbols := doc.Symbols
		if len(symbols) == 0 {
			symbols = codeSymbols(doc.Content)
		}
		keyword := codeKeywordEvidence(query, tokens, doc.Title, doc.Path, symbols, doc.Content).Score
		if keyword <= 0 {
			continue
		}
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
		headings := doc.Headings
		if len(headings) == 0 {
			headings = headingsFromMarkdown(doc.Content)
		}
		content := strings.TrimSpace(strings.Join([]string{doc.Title, doc.Path, strings.Join(headings, "\n"), doc.Content}, "\n\n"))
		if content == "" {
			continue
		}
		chunks = append(chunks, previewEmbeddingChunk{
			ID:          "doc:" + doc.ID,
			Type:        "doc",
			Title:       doc.Title,
			Path:        doc.Path,
			SpecID:      doc.SpecID,
			Description: doc.Description,
			Content:     content,
			Hash:        contentHash(content),
		})
	}
	for _, doc := range codeDocs {
		symbols := doc.Symbols
		if len(symbols) == 0 {
			symbols = codeSymbols(doc.Content)
		}
		content := strings.TrimSpace(strings.Join([]string{doc.Title, doc.Path, strings.Join(symbols, "\n"), doc.Content}, "\n\n"))
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
			ID:          chunk.ID,
			Title:       chunk.Title,
			Path:        chunk.Path,
			Kind:        chunk.Type,
			Source:      "embedding",
			Score:       roundScore(score),
			MatchedBy:   []string{"semantic"},
			Description: chunk.Description,
			Excerpt:     compactWhitespace(chunk.Content, 260),
			SpecID:      chunk.SpecID,
			Line:        chunk.Line,
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
	settings, err := loadKnownsEmbeddingSettingsForTest()
	if err != nil {
		return previewEmbeddingConfig{}, err
	}
	return previewEmbeddingConfigFromKnownsSettings(settings, project.Settings.SemanticSearch.Model, project.Settings.SemanticSearch.Dimensions, "knowns-project")
}

func previewEmbeddingConfigFromKnownsDefault() (previewEmbeddingConfig, error) {
	settings, err := loadKnownsEmbeddingSettingsForTest()
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

var loadKnownsEmbeddingSettingsForTest = loadKnownsEmbeddingSettings

// ollamaGetForTest lets tests stub the HTTP call to the local Ollama server.
var ollamaGetForTest = ollamaHTTPGet

func ollamaHTTPGet(url string) (*http.Response, error) {
	client := http.Client{Timeout: 2 * time.Second}
	return client.Get(url)
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
	modelName := internalutil.FirstNonEmpty(model.Model, modelID)
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
	res, err := ollamaGetForTest(ollamaBase + "/api/tags")
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

func searchDocsGraph(graph specGraph, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) []previewSearchResult {
	return searchDocsGraphByQuery(graph, query, tokens, exclusionQuery, exclusionTokens, limit)
}

func searchDocsGraphByQuery(graph specGraph, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) []previewSearchResult {
	results := []previewSearchResult{}
	for _, node := range graph.Nodes {
		haystack := strings.Join([]string{node.ID, node.Label, node.Path, node.Category, node.Status}, " ")
		if excludedByKeywordSearch(exclusionQuery, exclusionTokens, node.Label, node.Path, haystack) {
			continue
		}
		score := graphScore(query, tokens, haystack)
		if score <= 0 {
			continue
		}
		results = append(results, previewSearchResult{
			ID:        "docs-graph:" + node.ID,
			Title:     internalutil.FirstNonEmpty(node.Label, node.ID),
			Path:      node.Path,
			Kind:      internalutil.FirstNonEmpty(node.Type, "doc-node"),
			Score:     score,
			MatchedBy: []string{"graph"},
			SpecID:    node.SpecID,
			NodeID:    node.ID,
			Source:    "docs graph",
			Neighbors: limitNeighbors(docGraphNeighbors(graph, node.ID), 8),
		})
	}
	sortSearchResults(results)
	return limitResults(dedupeSearchResults(results), limit)
}

func codeGraphFlowScore(anchorScore float64, depth int, relation string) float64 {
	score := graphExpansionScore(anchorScore, depth)
	relation = strings.ToLower(strings.TrimSpace(relation))
	if depth >= 1 && isCallRelation(relation) {
		return roundScore(math.Max(score, anchorScore-0.03))
	}
	return score
}

func isCallRelation(relation string) bool {
	relation = strings.ToLower(strings.TrimSpace(relation))
	return relation == "calls" || relation == "call"
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
			Title:     internalutil.FirstNonEmpty(node.Label, node.ID),
			Path:      node.Path,
			Kind:      internalutil.FirstNonEmpty(node.Type, "doc-node"),
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

func codeGraphFlowRole(matchedBy []string) string {
	switch {
	case slices.Contains(matchedBy, "graph-root-caller"):
		return "root-caller"
	case slices.Contains(matchedBy, "graph-caller"):
		return "caller"
	case slices.Contains(matchedBy, "graph-callee"):
		return "callee"
	case slices.Contains(matchedBy, "graph-flow"):
		return "context"
	case slices.Contains(matchedBy, "graph"):
		return "match"
	default:
		return ""
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
			index.Keys[key] = internalutil.AppendUniqueString(index.Keys[key], node.ID)
		}
	}
	for _, edge := range graph.Edges {
		if edge.From == "" || edge.To == "" {
			continue
		}
		index.Edges[edge.From] = append(index.Edges[edge.From], graphEdge{From: edge.From, To: edge.To, Type: internalutil.FirstNonEmpty(edge.Type, edge.Label, "references"), Label: edge.Label, Origin: edge.Origin, Raw: edge.Raw})
		index.Edges[edge.To] = append(index.Edges[edge.To], graphEdge{From: edge.To, To: edge.From, Type: internalutil.FirstNonEmpty(edge.Type, edge.Label, "references"), Label: edge.Label, Origin: edge.Origin, Raw: edge.Raw})
	}
	return index
}

func (index docsGraphIndex) match(anchor graphSearchAnchor) []string {
	matches := []string{}
	for _, key := range anchorKeys(anchor) {
		for _, nodeID := range index.Keys[key] {
			matches = internalutil.AppendUniqueString(matches, nodeID)
		}
	}
	return matches
}

func graphAnchorsFromSemantic(semantic []previewSearchResult) []graphSearchAnchor {
	anchors := make([]graphSearchAnchor, 0, len(semantic))
	for _, result := range semantic {
		anchor := graphSearchAnchor{
			ID:     internalutil.FirstNonEmpty(result.ID, result.SpecID, result.Path, result.Title),
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
		keys = internalutil.AppendUniqueString(keys, value)
		tokenKey := strings.Join(searchTokens(value), "")
		if tokenKey != "" {
			keys = internalutil.AppendUniqueString(keys, tokenKey)
		}
	}
	return keys
}

func graphExpansionLimit(limit int) int {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	return min(maxSearchLimit, max(limit, limit*3))
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
	return internalutil.FirstNonEmpty(result.NodeID, result.ID, result.SpecID, result.Path, result.Title)
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
			if !slices.Contains(semantic[i].MatchedBy, "graph") {
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

func filterCodeEmbeddingResultsByKeywordEvidence(results []previewSearchResult, codeDocs []codeSearchDoc, query string, tokens []string) []previewSearchResult {
	if len(results) == 0 {
		return results
	}
	docsByPath := map[string]codeSearchDoc{}
	for _, doc := range codeDocs {
		docsByPath[doc.Path] = doc
	}
	filtered := results[:0]
	for _, result := range results {
		doc, ok := docsByPath[result.Path]
		if !ok {
			continue
		}
		// Embedding providers can return broad code neighbors; code search keeps only files with visible query evidence.
		if codeKeywordEvidence(query, tokens, doc.Title, doc.Path, codeSymbols(doc.Content), doc.Content).Score <= 0 {
			continue
		}
		filtered = append(filtered, result)
	}
	return filtered
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
	return internalutil.FirstNonEmpty(result.SpecID, result.Path, result.ID, result.Title)
}

func mergeMatchMethods(methods []string, method string) []string {
	if slices.Contains(methods, method) {
		return methods
	}
	return append([]string{method}, methods...)
}

func codeKeywordEvidence(query string, tokens []string, title string, path string, symbols []string, content string) searchEvidence {
	return searchFieldEvidence(query, tokens, title, path, symbols, content)
}

func searchFieldEvidence(query string, tokens []string, title string, path string, symbols []string, content string) searchEvidence {
	lowerTitle := strings.ToLower(title)
	lowerPath := strings.ToLower(path)
	lowerSymbols := strings.ToLower(strings.Join(symbols, " "))
	lowerContent := strings.ToLower(content)
	score := 0.0
	exactness := 0
	boost := func(value float64, rank int) {
		score += value
		if rank > exactness {
			exactness = rank
		}
	}
	for _, lowerQuery := range searchQueryParts(query) {
		switch {
		case strings.Contains(lowerSymbols, lowerQuery):
			boost(0.62, 6)
		case strings.Contains(lowerTitle, lowerQuery):
			boost(0.54, 5)
		case pathContainsSearchPart(lowerPath, lowerQuery):
			boost(0.46, 4)
		case strings.Contains(lowerContent, lowerQuery):
			boost(0.18, 2)
		}
	}
	symbolTokens := searchTokens(lowerSymbols)
	titleTokens := searchTokens(lowerTitle)
	pathTokens := searchTokens(lowerPath)
	for _, token := range tokens {
		switch {
		case tokenIn(token, symbolTokens):
			boost(0.2, 5)
		case tokenIn(token, titleTokens):
			boost(0.16, 4)
		case tokenIn(token, pathTokens):
			boost(0.13, 3)
		case strings.Contains(lowerContent, token):
			boost(0.04, 1)
		}
	}
	return searchEvidence{Score: clamp01(score), Exactness: exactness}
}

func pathContainsSearchPart(path string, queryPart string) bool {
	if strings.Contains(path, queryPart) {
		return true
	}
	queryKey := strings.Join(searchTokens(queryPart), "")
	if queryKey == "" {
		return false
	}
	for _, segment := range strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\' || r == '.' || r == '-' || r == '_'
	}) {
		if strings.Join(searchTokens(segment), "") == queryKey {
			return true
		}
	}
	return false
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
	return internalutil.UniqueStrings(parts)
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
	return internalutil.UniqueStrings(tokens)
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

var codeSymbolPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:type|class|interface|enum|struct)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
	regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+|internal\s+|static\s+|final\s+|open\s+|override\s+|suspend\s+|async\s+)*fun\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+|internal\s+|static\s+|final\s+|open\s+|override\s+|async\s+)*func\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+|internal\s+|static\s+|final\s+|override\s+|async\s+)*(?:[A-Za-z_][A-Za-z0-9_<>,\[\]?]*\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
	regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+|static\s+|async\s+|readonly\s+)*([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)\s*(?::\s*[^={]+)?\s*\{`),
}

func codeSymbols(content string) []string {
	out := []string{}
	for _, re := range codeSymbolPatterns {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			if len(match) == 2 && !isControlFlowSymbol(match[1]) {
				out = append(out, match[1])
			}
		}
	}
	return internalutil.UniqueStrings(out)
}

func isControlFlowSymbol(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "if", "for", "while", "switch", "catch", "return":
		return true
	default:
		return false
	}
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
			start := max(0, i-1)
			end := min(len(lines), i+2)
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
	gitFiles, gitFilesKnown := gitTrackedFiles(projectRoot)
	docs := make([]docsSearchDoc, 0, len(specs))
	seen := map[string]bool{}
	for _, doc := range specs {
		docPath := filepath.Join(docsRoot, filepath.FromSlash(doc.Path))
		if !isSearchableDocsPath(docPath) {
			continue
		}
		docProjectRel := relPath(projectRoot, docPath)
		if gitFilesKnown && !gitFiles[docProjectRel] {
			continue
		}
		docs = append(docs, docsSearchDoc{
			ID:          doc.ID,
			Title:       doc.Title,
			Path:        doc.Path,
			Content:     internalutil.FirstNonEmpty(doc.SearchText, doc.Raw),
			Headings:    headingsFromMarkdown(internalutil.FirstNonEmpty(doc.SearchText, doc.Raw)),
			Description: doc.Description,
			SpecID:      doc.ID,
			Kind:        "doc",
		})
		seen[docProjectRel] = true
	}
	warnings := []string{}
	if gitFilesKnown {
		rels := make([]string, 0, len(gitFiles))
		for rel := range gitFiles {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			if shouldSkipGitSearchPath(rel) || seen[rel] {
				continue
			}
			path := filepath.Join(projectRoot, filepath.FromSlash(rel))
			if !isSearchableDocsPath(path) {
				continue
			}
			doc, ok := readDocsSearchFile(projectRoot, path, rel)
			if !ok {
				continue
			}
			docs = append(docs, doc)
			seen[rel] = true
		}
	} else {
		err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if isGeneratedPreviewUIPath(relPath(projectRoot, path)) {
					return filepath.SkipDir
				}
				if shouldSkipSearchDir(name) {
					return filepath.SkipDir
				}
				return nil
			}
			projectRel := relPath(projectRoot, path)
			if seen[projectRel] || !isSearchableDocsPath(path) {
				return nil
			}
			doc, ok := readDocsSearchFile(projectRoot, path, projectRel)
			if !ok {
				return nil
			}
			docs = append(docs, doc)
			seen[projectRel] = true
			return nil
		})
		if err != nil {
			warnings = append(warnings, "Docs search scan failed: "+err.Error())
		}
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})
	return docs, warnings
}

func readDocsSearchFile(projectRoot, path, projectRel string) (docsSearchDoc, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > maxSearchFileBytes {
		return docsSearchDoc{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) {
		return docsSearchDoc{}, false
	}
	return docsSearchDoc{
		ID:       projectRel,
		Title:    filepath.Base(path),
		Path:     projectRel,
		Content:  string(data),
		Headings: headingsFromMarkdown(string(data)),
		Kind:     "file",
	}, true
}

func scanCodeSearchDocs(projectRoot, docsRoot string) ([]codeSearchDoc, []string) {
	docs := []codeSearchDoc{}
	warnings := []string{}
	if gitFiles, ok := gitTrackedFiles(projectRoot); ok {
		rels := make([]string, 0, len(gitFiles))
		for rel := range gitFiles {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			if shouldSkipGitSearchPath(rel) || pathIsUnderDocsRoot(projectRoot, docsRoot, rel) {
				continue
			}
			path := filepath.Join(projectRoot, filepath.FromSlash(rel))
			if !isSearchableCodePath(path) {
				continue
			}
			info, err := os.Stat(path)
			if err != nil || info.IsDir() || info.Size() > maxSearchFileBytes {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil || !utf8.Valid(data) {
				continue
			}
			docs = append(docs, codeSearchDoc{
				ID:      rel,
				Title:   filepath.Base(path),
				Path:    rel,
				Content: string(data),
				Symbols: codeSymbols(string(data)),
			})
		}
		return docs, warnings
	}
	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if isGeneratedPreviewUIPath(relPath(projectRoot, path)) {
				return filepath.SkipDir
			}
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
			Symbols: codeSymbols(string(data)),
		})
		return nil
	})
	if err != nil {
		warnings = append(warnings, "Code search scan failed: "+err.Error())
	}
	return docs, warnings
}

func gitTrackedFiles(projectRoot string) (map[string]bool, bool) {
	return gitTrackedFilesForTest(projectRoot)
}

// gitTrackedFilesForTest lets tests stub the git ls-files call so the
// gitFilesKnown branch in scanDocsSearchDocs can be exercised.
var gitTrackedFilesForTest = func(projectRoot string) (map[string]bool, bool) {
	// Preview search uses Git's tracked corpus when available so generated and scratch files stay out of results.
	cmd := exec.Command("git", "-C", projectRoot, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	files := map[string]bool{}
	for _, raw := range strings.Split(string(out), "\x00") {
		rel := strings.TrimSpace(filepath.ToSlash(raw))
		if rel == "" {
			continue
		}
		files[rel] = true
	}
	return files, len(files) > 0
}

func shouldSkipGitSearchPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	if isGeneratedPreviewUIPath(rel) {
		return true
	}
	for _, part := range strings.Split(rel, "/") {
		if shouldSkipSearchDir(part) {
			return true
		}
	}
	return false
}

func isGeneratedPreviewUIPath(rel string) bool {
	rel = strings.Trim(strings.TrimPrefix(filepath.ToSlash(rel), "./"), "/")
	return rel == "internal/preview/preview_ui" || strings.HasPrefix(rel, "internal/preview/preview_ui/")
}

func searchSnapshotToken(projectRoot, docsRoot string) string {
	gitFiles, gitFilesKnown := gitTrackedFiles(projectRoot)
	h := sha256.New()
	_, _ = io.WriteString(h, relPath(projectRoot, docsRoot))
	_, _ = io.WriteString(h, "\ndocs:"+newestModToken(docsRoot))
	if gitFilesKnown {
		rels := make([]string, 0, len(gitFiles))
		for rel := range gitFiles {
			if shouldSkipGitSearchPath(rel) || pathIsUnderDocsRoot(projectRoot, docsRoot, rel) {
				continue
			}
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			path := filepath.Join(projectRoot, filepath.FromSlash(rel))
			info, err := os.Stat(path)
			if err != nil || info.IsDir() || info.Size() > maxSearchFileBytes {
				continue
			}
			_, _ = io.WriteString(h, fmt.Sprintf("\n%s:%d:%d", rel, info.Size(), info.ModTime().UnixNano()))
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if isGeneratedPreviewUIPath(relPath(projectRoot, path)) {
				return filepath.SkipDir
			}
			if sameCleanPath(path, docsRoot) && !sameCleanPath(path, projectRoot) {
				return filepath.SkipDir
			}
			if shouldSkipSearchDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isSearchableCodePath(path) && !isSearchableDocsPath(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxSearchFileBytes {
			return nil
		}
		_, _ = io.WriteString(h, fmt.Sprintf("\n%s:%d:%d", relPath(projectRoot, path), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

func pathIsUnderDocsRoot(projectRoot, docsRoot, rel string) bool {
	docsRel := strings.Trim(strings.TrimPrefix(relPath(projectRoot, docsRoot), "./"), "/")
	rel = strings.Trim(strings.TrimPrefix(filepath.ToSlash(rel), "./"), "/")
	if docsRel == "" || docsRel == "." {
		return false
	}
	return rel == docsRel || strings.HasPrefix(rel, docsRel+"/")
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
	return isPreviewableFilePath(path) && !isSearchableDocsPath(path)
}

func isSearchableDocsPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".html", ".htm":
		return true
	default:
		return false
	}
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
	case ".html", ".htm":
		return "html"
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

func cleanProjectRel(projectRoot, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return cleanRelPath(relPath(projectRoot, path))
	}
	return cleanRelPath(path)
}

func cleanRelPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	return path
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
		if req.URL.Scheme == "http" && !isLoopbackHost(req.URL.Host) {
			return nil, fmt.Errorf("refusing to send API key over non-TLS connection to %s; use https:// or a localhost address", req.URL.Host)
		}
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
	return writePreviewEmbeddingIndexForTest(projectRoot, index)
}

// writePreviewEmbeddingIndexForTest lets tests stub the JSON marshal so the
// normally-unreachable error path can be exercised.
var writePreviewEmbeddingIndexForTest = func(projectRoot string, index previewEmbeddingIndex) error {
	path := previewEmbeddingIndexPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := previewEmbeddingIndexMarshalForTest(index)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// previewEmbeddingIndexMarshalForTest lets tests stub the JSON marshal of
// previewEmbeddingIndex so the unreachable marshal error branch can be exercised.
var previewEmbeddingIndexMarshalForTest = func(v previewEmbeddingIndex) ([]byte, error) {
	return json.Marshal(v)
}

func previewEmbeddingIndexPath(projectRoot string) string {
	cache, err := previewUserCacheDirForTest()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	sum := sha256.Sum256([]byte(projectRoot))
	return filepath.Join(cache, "ns-workspace", "preview-search", hex.EncodeToString(sum[:8]), "embedding-index.json")
}

// previewUserCacheDirForTest lets tests stub the user cache directory lookup so
// the error branches of previewEmbeddingIndexPath can be exercised.
var previewUserCacheDirForTest = func() (string, error) {
	return os.UserCacheDir()
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
		key := internalutil.FirstNonEmpty(result.ID, result.NodeID, result.Path, result.Title)
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

func isLoopbackHost(host string) bool {
	hostname := host
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		hostname = host[:idx]
	}
	hostname = strings.Trim(hostname, "[]")
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == ""
}


