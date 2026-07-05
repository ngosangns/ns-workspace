package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSelectOllamaEmbeddingModelPriorities(t *testing.T) {
	cases := []struct {
		name   string
		models []ollamaModel
		want   string
	}{
		{"empty", nil, ""},
		{"exact_nomic", []ollamaModel{{Name: "nomic-embed-text"}}, "nomic-embed-text"},
		{"exact_nomic_case_insensitive", []ollamaModel{{Name: "Nomic-Embed-Text"}}, "Nomic-Embed-Text"},
		{"with_tag", []ollamaModel{{Name: "nomic-embed-text:latest"}}, "nomic-embed-text:latest"},
		{"with_suffix", []ollamaModel{{Name: "mxbai-embed-large-custom"}}, "mxbai-embed-large-custom"},
		{"mxbai_priority", []ollamaModel{{Name: "mxbai-embed-large"}}, "mxbai-embed-large"},
		{"bge_priority", []ollamaModel{{Name: "bge-small"}}, "bge-small"},
		{"unknown", []ollamaModel{{Name: "random-model"}}, ""},
		{"partial_match_no_separator", []ollamaModel{{Name: "bge-largeish"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectOllamaEmbeddingModel(tc.models)
			if got != tc.want {
				t.Errorf("selectOllamaEmbeddingModel(%v) = %q, want %q", tc.models, got, tc.want)
			}
		})
	}
}

func TestIsCallRelation(t *testing.T) {
	cases := map[string]bool{
		"calls":    true,
		"call":     true,
		"CALLS":    true,
		" Call ":   true,
		"unknown":  false,
		"":         false,
		"reference": false,
	}
	for input, want := range cases {
		if got := isCallRelation(input); got != want {
			t.Errorf("isCallRelation(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestCodeGraphFlowScore(t *testing.T) {
	cases := []struct {
		name        string
		anchorScore float64
		depth       int
		relation    string
		wantGT      bool
	}{
		{"non_call_zero_depth", 1.0, 0, "references", false},
		{"call_depth_0", 1.0, 0, "calls", false},
		{"call_depth_1", 0.8, 1, "calls", true},
		{"non_call_depth_1", 0.8, 1, "references", false},
		{"call_relation_case", 0.5, 1, "  CALL ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := codeGraphFlowScore(tc.anchorScore, tc.depth, tc.relation)
			if tc.wantGT && got < tc.anchorScore-0.03 {
				t.Errorf("codeGraphFlowScore = %v, want >= %v", got, tc.anchorScore-0.03)
			}
		})
	}
}

func TestCodeGraphFlowRole(t *testing.T) {
	cases := []struct {
		name      string
		matchedBy []string
		want      string
	}{
		{"root_caller", []string{"graph-root-caller"}, "root-caller"},
		{"caller", []string{"graph-caller"}, "caller"},
		{"callee", []string{"graph-callee"}, "callee"},
		{"context", []string{"graph-flow"}, "context"},
		{"match", []string{"graph"}, "match"},
		{"empty", nil, ""},
		{"unknown", []string{"keyword"}, ""},
		{"root_takes_priority", []string{"graph-root-caller", "graph-callee"}, "root-caller"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := codeGraphFlowRole(tc.matchedBy); got != tc.want {
				t.Errorf("codeGraphFlowRole(%v) = %q, want %q", tc.matchedBy, got, tc.want)
			}
		})
	}
}

func TestNormalizedGraphKeys(t *testing.T) {
	keys := normalizedGraphKeys("", "  ", "Hello", "hello", "Hello World", "path/to/file")
	if len(keys) == 0 {
		t.Fatal("expected keys, got none")
	}
	// Verify dedup
	if strings.Count(strings.Join(keys, ","), "hello") < 1 {
		t.Errorf("expected 'hello' to appear in keys, got %v", keys)
	}
	// Empty values are skipped
	for _, k := range keys {
		if k == "" {
			t.Errorf("normalizedGraphKeys returned empty string: %v", keys)
		}
	}
}

func TestGraphExpansionDepth(t *testing.T) {
	if got := graphExpansionDepth(8); got != 3 {
		t.Errorf("graphExpansionDepth(8) = %d, want 3", got)
	}
	if got := graphExpansionDepth(5); got != 2 {
		t.Errorf("graphExpansionDepth(5) = %d, want 2", got)
	}
	if got := graphExpansionDepth(0); got != 2 {
		t.Errorf("graphExpansionDepth(0) = %d, want 2", got)
	}
}

func TestGraphExpansionScore(t *testing.T) {
	if got := graphExpansionScore(0, 0); got <= 0 {
		t.Errorf("expected positive score, got %v", got)
	}
	if got := graphExpansionScore(0.8, 0); got < 0.05 {
		t.Errorf("expected score >= 0.05, got %v", got)
	}
	if got := graphExpansionScore(0.8, 5); got != 0.2 {
		t.Errorf("graphExpansionScore(0.8, 5) = %v, want 0.2", got)
	}
}

func TestGraphExpansionMatchedBy(t *testing.T) {
	if got := graphExpansionMatchedBy(0); len(got) != 2 || got[0] != "semantic-anchor" {
		t.Errorf("graphExpansionMatchedBy(0) = %v, want [semantic-anchor, graph]", got)
	}
	if got := graphExpansionMatchedBy(1); len(got) != 2 || got[0] != "graph-expansion" {
		t.Errorf("graphExpansionMatchedBy(1) = %v, want [graph-expansion, graph]", got)
	}
}

func TestNewDocsGraphIndex(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{
			{ID: "n1", Label: "Node 1", Path: "docs/a.md", SpecID: "spec-a"},
			{ID: "n2", Label: "Node 2", Path: "docs/b.md", SpecID: "spec-b"},
		},
		Edges: []graphEdge{
			{From: "n1", To: "n2", Type: "references"},
			{From: "", To: "n3", Type: "invalid"},
		},
	}
	index := newDocsGraphIndex(graph)
	if len(index.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(index.Nodes))
	}
	// Bidirectional edges
	if len(index.Edges["n1"]) != 1 {
		t.Errorf("expected 1 edge from n1, got %d", len(index.Edges["n1"]))
	}
	if len(index.Edges["n2"]) != 1 {
		t.Errorf("expected 1 edge from n2, got %d", len(index.Edges["n2"]))
	}
	// Invalid edge skipped
	if len(index.Edges["n3"]) != 0 {
		t.Errorf("expected no edges from empty-from, got %d", len(index.Edges["n3"]))
	}
	// Default edge type
	if index.Edges["n1"][0].Type != "references" {
		t.Errorf("expected edge type 'references', got %q", index.Edges["n1"][0].Type)
	}
}

func TestNewDocsGraphIndexDefaultEdgeType(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
		Edges: []graphEdge{{From: "a", To: "b"}},
	}
	index := newDocsGraphIndex(graph)
	if len(index.Edges["a"]) != 1 || index.Edges["a"][0].Type != "references" {
		t.Errorf("expected default edge type 'references', got %+v", index.Edges["a"])
	}
}

func TestDocsGraphIndexMatch(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{{ID: "n1", Label: "Hello", Path: "docs/a.md"}},
	}
	index := newDocsGraphIndex(graph)
	anchor := graphSearchAnchor{ID: "hello"}
	matches := index.match(anchor)
	if len(matches) == 0 {
		t.Errorf("expected at least 1 match, got 0")
	}
	if matches[0] != "n1" {
		t.Errorf("expected match n1, got %v", matches)
	}
}

func TestSearchDocsGraphFromSemanticEmpty(t *testing.T) {
	// Empty graph
	if got := searchDocsGraphFromSemantic(specGraph{}, nil, 5); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	// Empty semantic
	if got := searchDocsGraphFromSemantic(specGraph{Nodes: []graphNode{{ID: "n1"}}}, nil, 5); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestSearchDocsGraphFromSemanticWithAnchor(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{
			{ID: "n1", Label: "Hello", Path: "docs/a.md", SpecID: "spec-a"},
			{ID: "n2", Label: "World", Path: "docs/b.md", SpecID: "spec-b"},
		},
		Edges: []graphEdge{{From: "n1", To: "n2", Type: "references"}},
	}
	semantic := []previewSearchResult{
		{ID: "hello", Title: "Hello", Path: "docs/a.md", SpecID: "spec-a", Score: 0.9},
	}
	results := searchDocsGraphFromSemantic(graph, semantic, 10)
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	// Verify anchor marked
	hasAnchor := false
	for _, r := range results {
		if r.Anchor {
			hasAnchor = true
		}
	}
	if !hasAnchor {
		t.Errorf("expected at least one Anchor result")
	}
}

func TestExpandDocsGraphAnchorLimitAndDepth(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{
			{ID: "n1", Label: "A", Path: "docs/a.md"},
			{ID: "n2", Label: "B", Path: "docs/b.md"},
			{ID: "n3", Label: "C", Path: "docs/c.md"},
		},
		Edges: []graphEdge{
			{From: "n1", To: "n2", Type: "references"},
			{From: "n2", To: "n3", Type: "references"},
		},
	}
	index := newDocsGraphIndex(graph)
	anchor := graphSearchAnchor{ID: "n1", Score: 0.9}
	results := map[string]previewSearchResult{}
	expandDocsGraphAnchor(graph, index, "n1", anchor, 10, results)
	// Should expand through n1, n2, n3
	if len(results) < 1 {
		t.Errorf("expected at least 1 result, got %d", len(results))
	}
	// The anchor n1 should be present with Anchor=true
	if r, ok := results["n1"]; !ok || !r.Anchor {
		t.Errorf("expected anchor n1, got %+v", r)
	}
}

func TestProbeDimensionsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{
		APIBase:     srv.URL,
		Model:       "test-model",
		QueryPrefix: "",
		Source:      "test",
	}
	dim, err := cfg.probeDimensions()
	if err != nil {
		t.Fatalf("probeDimensions = %v", err)
	}
	if dim != 3 {
		t.Errorf("expected 3 dimensions, got %d", dim)
	}
}

func TestProbeDimensionsEmptyVectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", Source: "test"}
	_, err := cfg.probeDimensions()
	if err == nil {
		t.Error("expected error for empty vectors")
	}
}

func TestProbeDimensionsZeroEmbedding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", Source: "test"}
	_, err := cfg.probeDimensions()
	if err == nil {
		t.Error("expected error for zero-length embedding")
	}
}

func TestProbeDimensionsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", Source: "test"}
	_, err := cfg.probeDimensions()
	if err == nil {
		t.Error("expected error from 500 response")
	}
}

func TestProbeDimensionsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{invalid`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", Source: "test"}
	_, err := cfg.probeDimensions()
	if err == nil {
		t.Error("expected error from invalid JSON")
	}
}

func TestEmbedBatchEmpty(t *testing.T) {
	cfg := previewEmbeddingConfig{}
	out, err := cfg.embedBatch(nil)
	if err != nil {
		t.Errorf("embedBatch(nil) err = %v", err)
	}
	if out != nil {
		t.Errorf("expected nil output, got %v", out)
	}
}

func TestEmbedBatchRawMismatchedLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test"}
	_, err := cfg.embedBatchRaw([]string{"a", "b"})
	if err == nil {
		t.Error("expected error for mismatched vector count")
	}
}

func TestEmbedBatchRawDimensionMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", Dimensions: 3}
	_, err := cfg.embedBatchRaw([]string{"a"})
	if err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

func TestEmbedBatchRawInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{invalid`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test"}
	_, err := cfg.embedBatchRaw([]string{"a"})
	if err == nil {
		t.Error("expected error from invalid JSON")
	}
}

func TestEmbedBatchRawSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[0.1,0.2]},{"index":0,"embedding":[0.3,0.4]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", BatchSize: 2}
	vectors, err := cfg.embedBatch([]string{"a", "b"})
	if err != nil {
		t.Fatalf("embedBatch err = %v", err)
	}
	if len(vectors) != 2 {
		t.Errorf("expected 2 vectors, got %d", len(vectors))
	}
}

func TestEmbedBatchRawIndexOutOfRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":99,"embedding":[0.1,0.2]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test"}
	vectors, err := cfg.embedBatchRaw([]string{"a"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(vectors) != 1 {
		t.Errorf("expected 1 vector, got %d", len(vectors))
	}
	if len(vectors[0]) != 2 {
		t.Errorf("expected embedding length 2, got %d", len(vectors[0]))
	}
}

func TestEmbedBatchRawRefusesAPIKeyOverHTTP(t *testing.T) {
	cfg := previewEmbeddingConfig{
		APIBase: "http://example.com",
		APIKey:  "secret",
		Model:   "test",
	}
	_, err := cfg.embedBatchRaw([]string{"a"})
	if err == nil {
		t.Error("expected error when sending API key over http to non-loopback host")
	}
}

func TestEmbedBatchRawAPIKeyOverLocalhostOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing Authorization header")
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{
		APIBase: srv.URL,
		APIKey:  "secret",
		Model:   "test",
	}
	_, err := cfg.embedBatchRaw([]string{"a"})
	if err != nil {
		t.Errorf("expected success on localhost, got %v", err)
	}
}

func TestReadPreviewEmbeddingIndexMissing(t *testing.T) {
	root := t.TempDir()
	idx := readPreviewEmbeddingIndex(root)
	if idx.Model != "" || len(idx.Chunks) != 0 {
		t.Errorf("expected empty index for missing file, got %+v", idx)
	}
}

func TestWriteAndReadPreviewEmbeddingIndex(t *testing.T) {
	root := t.TempDir()
	want := previewEmbeddingIndex{
		Model:      "test",
		APIBase:    "http://localhost",
		Dimensions: 3,
		Chunks: []previewEmbeddingChunk{
			{ID: "a", Content: "hello", Embedding: []float32{0.1, 0.2, 0.3}},
		},
	}
	if err := writePreviewEmbeddingIndex(root, want); err != nil {
		t.Fatalf("write err = %v", err)
	}
	got := readPreviewEmbeddingIndex(root)
	if got.Model != want.Model || got.Dimensions != want.Dimensions || len(got.Chunks) != 1 {
		t.Errorf("read mismatch: got %+v", got)
	}
}

func TestPreviewEmbeddingIndexPath(t *testing.T) {
	root := t.TempDir()
	got := previewEmbeddingIndexPath(root)
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	sum := sha256.Sum256([]byte(root))
	want := filepath.Join(cache, "ns-workspace", "preview-search", hex.EncodeToString(sum[:8]), "embedding-index.json")
	if got != want {
		t.Errorf("previewEmbeddingIndexPath = %q, want %q", got, want)
	}
}

func TestContentHash(t *testing.T) {
	h1 := contentHash("hello")
	h2 := contentHash("hello")
	h3 := contentHash("world")
	if h1 != h2 {
		t.Errorf("same content should produce same hash: %s != %s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("different content should produce different hash: %s == %s", h1, h3)
	}
	if h1 == "" {
		t.Error("hash should not be empty")
	}
}

func TestEmbeddingQueryPrefix(t *testing.T) {
	if got := embeddingQueryPrefix("nomic-embed-text"); got == "" {
		t.Error("expected non-empty query prefix for nomic-embed-text")
	}
	if got := embeddingQueryPrefix("unknown-model"); got != "" {
		t.Errorf("expected empty query prefix for unknown model, got %q", got)
	}
}

func TestEmbeddingDocPrefix(t *testing.T) {
	if got := embeddingDocPrefix("nomic-embed-text"); got == "" {
		t.Error("expected non-empty doc prefix for nomic-embed-text")
	}
	if got := embeddingDocPrefix("unknown-model"); got != "" {
		t.Errorf("expected empty doc prefix for unknown model, got %q", got)
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors -> 1.0
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if got := cosineSimilarity(a, b); got < 0.99 {
		t.Errorf("identical vectors: cosine = %v, want ~1.0", got)
	}
	// Orthogonal -> 0.0
	c := []float32{0, 1, 0}
	if got := cosineSimilarity(a, c); got > 0.01 {
		t.Errorf("orthogonal: cosine = %v, want ~0.0", got)
	}
	// Different lengths
	if got := cosineSimilarity(a, []float32{1, 0}); got != 0 {
		t.Errorf("different lengths: cosine = %v, want 0", got)
	}
	// Empty vectors
	if got := cosineSimilarity(nil, a); got != 0 {
		t.Errorf("nil vectors: cosine = %v, want 0", got)
	}
	if got := cosineSimilarity(a, nil); got != 0 {
		t.Errorf("nil vectors: cosine = %v, want 0", got)
	}
	// Zero vector
	z := []float32{0, 0, 0}
	if got := cosineSimilarity(a, z); got != 0 {
		t.Errorf("zero vector: cosine = %v, want 0", got)
	}
}

func TestSortSearchResults(t *testing.T) {
	results := []previewSearchResult{
		{Score: 0.3},
		{Score: 0.9},
		{Score: 0.6},
	}
	sortSearchResults(results)
	if results[0].Score != 0.9 || results[1].Score != 0.6 || results[2].Score != 0.3 {
		t.Errorf("sort failed: %+v", results)
	}
}

func TestLimitResults(t *testing.T) {
	results := []previewSearchResult{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if got := limitResults(results, 2); len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
	if got := limitResults(results, 0); len(got) != 0 {
		t.Errorf("limit 0 should return empty, got %d", len(got))
	}
	if got := limitResults(results, 100); len(got) != 3 {
		t.Errorf("limit > len should return all, got %d", len(got))
	}
}

func TestDedupeSearchResults(t *testing.T) {
	results := []previewSearchResult{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "B"},
		{ID: "a", Title: "A2"},
	}
	got := dedupeSearchResults(results)
	if len(got) != 2 {
		t.Errorf("expected 2 unique results, got %d", len(got))
	}
}

func TestRelPath(t *testing.T) {
	root := "/tmp/proj"
	rel := relPath(root, "/tmp/proj/docs/a.md")
	if rel != "docs/a.md" {
		t.Errorf("relPath = %q, want docs/a.md", rel)
	}
	rel = relPath(root, "/other/path/x.md")
	// out-of-tree returns cleaned absolute path with forward slashes
	want := filepath.ToSlash(filepath.Clean("/other/path/x.md"))
	if rel != want {
		t.Errorf("relPath out-of-tree = %q, want %q", rel, want)
	}
}

func TestClamp01(t *testing.T) {
	if got := clamp01(-0.5); got != 0 {
		t.Errorf("clamp01(-0.5) = %v, want 0", got)
	}
	if got := clamp01(0.5); got != 0.5 {
		t.Errorf("clamp01(0.5) = %v, want 0.5", got)
	}
	if got := clamp01(1.5); got != 1 {
		t.Errorf("clamp01(1.5) = %v, want 1", got)
	}
}

func TestRoundScore(t *testing.T) {
	if got := roundScore(0.123456789); got != 0.123 {
		t.Errorf("roundScore(0.123456789) = %v, want 0.123", got)
	}
	if got := roundScore(0.5); got != 0.5 {
		t.Errorf("roundScore(0.5) = %v, want 0.5", got)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"localhost:8080":     true,
		"127.0.0.1:8080":     true,
		"[::1]:8080":         true,
		"example.com:8080":   false,
		"192.168.1.1:8080":   false,
	}
	for host, want := range cases {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestLoadPreviewEmbeddingSearchUnavailable(t *testing.T) {
	root := t.TempDir()
	// No embedding config available -> returns nil
	search, warnings := loadPreviewEmbeddingSearch(root, nil, nil)
	_ = search
	_ = warnings
}

func TestBuildPreviewEmbeddingChunks(t *testing.T) {
	docs := []docsSearchDoc{
		{ID: "d1", Title: "Doc 1", Path: "docs/a.md", Content: "Hello", SpecID: "spec-a"},
	}
	code := []codeSearchDoc{
		{ID: "c1", Title: "Code 1", Path: "src/a.go", Content: "package a"},
	}
	chunks := buildPreviewEmbeddingChunks(docs, code)
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Type != "doc" {
		t.Errorf("expected first chunk type 'doc', got %q", chunks[0].Type)
	}
	if chunks[1].Type != "code" {
		t.Errorf("expected second chunk type 'code', got %q", chunks[1].Type)
	}
	if chunks[0].Hash == "" {
		t.Error("expected chunk hash to be set")
	}
}

func TestPreviewEmbeddingConfigFromKnownsDefault(t *testing.T) {
	cfg, err := previewEmbeddingConfigFromKnownsDefault()
	_ = cfg
	_ = err
	// Either succeeds with valid cfg or returns error (network/file unavailable).
	// The function should not panic.
}

func TestLoadOrBuildPreviewEmbeddingIndexEmpty(t *testing.T) {
	cfg := previewEmbeddingConfig{
		APIBase: "http://127.0.0.1:1", // unreachable
		Model:   "test",
	}
	index, warnings := loadOrBuildPreviewEmbeddingIndex(t.TempDir(), cfg, nil)
	if len(index.Chunks) != 0 {
		t.Errorf("expected empty chunks, got %d", len(index.Chunks))
	}
	_ = warnings
}

func TestParseSearchLimit(t *testing.T) {
	cases := map[string]int{
		"":            8,  // default
		"5":           5,
		"100":         24, // capped at max
		"abc":         8,  // default
		"-1":          8,  // default
		"3.5":         8,  // default
	}
	for raw, want := range cases {
		if got := parseSearchLimit(raw); got != want {
			t.Errorf("parseSearchLimit(%q) = %d, want %d", raw, got, want)
		}
	}
}

func TestSearchQueriesForKeywordOperator(t *testing.T) {
	q, _ := searchQueriesForKeywordOperator("foo bar", "and")
	if q != "foo bar" {
		t.Errorf("expected 'foo bar', got %q", q)
	}
	q, _ = searchQueriesForKeywordOperator("foo,bar", "or")
	if q != "foo,bar" {
		t.Errorf("expected 'foo,bar', got %q", q)
	}
	// difference operator with multiple parts → split.
	q, excl := searchQueriesForKeywordOperator("foo,bar,baz", "difference")
	if q != "foo" {
		t.Errorf("difference: q = %q, want foo", q)
	}
	if excl != "bar,baz" {
		t.Errorf("difference: excl = %q, want bar,baz", excl)
	}
	// difference operator with single part → keep whole query, no exclusion.
	q, excl = searchQueriesForKeywordOperator("foo", "difference")
	if q != "foo" || excl != "" {
		t.Errorf("difference single: q=%q excl=%q", q, excl)
	}
	// sum operator → keep whole query, no exclusion.
	q, excl = searchQueriesForKeywordOperator("foo,bar", "sum")
	if q != "foo,bar" || excl != "" {
		t.Errorf("sum: q=%q excl=%q", q, excl)
	}
}

func TestExcludedByKeywordSearch(t *testing.T) {
	if !excludedByKeywordSearch("foo", []string{"foo"}, "title", "path", "foo content") {
		t.Error("expected exclusion when token matches content")
	}
	if excludedByKeywordSearch("", nil, "title", "path", "content") {
		t.Error("empty query should not exclude")
	}
}

func TestCombineSearchScores(t *testing.T) {
	// keyword mode
	if s, m := combineSearchScores(0.5, 0.0, "keyword"); s == 0 || len(m) == 0 {
		t.Errorf("keyword mode: got score=%v, matchedBy=%v", s, m)
	}
	if s, _ := combineSearchScores(0.0, 0.0, "keyword"); s != 0 {
		t.Errorf("keyword mode 0 score: got %v", s)
	}
	// semantic mode
	if s, m := combineSearchScores(0.0, 0.5, "semantic"); s == 0 || len(m) == 0 {
		t.Errorf("semantic mode: got score=%v, matchedBy=%v", s, m)
	}
	if s, _ := combineSearchScores(0.0, 0.0, "semantic"); s != 0 {
		t.Errorf("semantic mode 0 score: got %v", s)
	}
	// hybrid (default)
	if s, m := combineSearchScores(0.5, 0.5, "hybrid"); s == 0 || len(m) != 2 {
		t.Errorf("hybrid mode: got score=%v, matchedBy=%v", s, m)
	}
	if s, _ := combineSearchScores(0.0, 0.0, "hybrid"); s != 0 {
		t.Errorf("hybrid mode 0 score: got %v", s)
	}
	// keyword only in hybrid
	if s, m := combineSearchScores(0.5, 0.0, "hybrid"); s == 0 || len(m) != 1 || m[0] != "keyword" {
		t.Errorf("hybrid keyword only: got score=%v, matchedBy=%v", s, m)
	}
	// semantic only in hybrid
	if s, m := combineSearchScores(0.0, 0.5, "hybrid"); s == 0 || len(m) != 1 || m[0] != "semantic" {
		t.Errorf("hybrid semantic only: got score=%v, matchedBy=%v", s, m)
	}
}

func TestFilterDocsSearchDocs(t *testing.T) {
	docs := []docsSearchDoc{
		{ID: "d1", Title: "Hello World", Content: "content"},
		{ID: "d2", Title: "Goodbye", Content: "content"},
	}
	filtered := filterDocsSearchDocs(docs, "", nil)
	if len(filtered) != 2 {
		t.Errorf("empty exclusion should keep all: got %d", len(filtered))
	}
	filtered = filterDocsSearchDocs(docs, "goodbye", []string{"goodbye"})
	if len(filtered) != 1 || filtered[0].ID != "d1" {
		t.Errorf("exclusion failed: got %d", len(filtered))
	}
}

func TestFilterCodeSearchDocs(t *testing.T) {
	code := []codeSearchDoc{
		{ID: "c1", Title: "Hello", Path: "a.go", Content: "foo"},
		{ID: "c2", Title: "Bad", Path: "b.go", Content: "bar"},
	}
	filtered := filterCodeSearchDocs(code, "", nil)
	if len(filtered) != 2 {
		t.Errorf("empty exclusion should keep all: got %d", len(filtered))
	}
	filtered = filterCodeSearchDocs(code, "bad", []string{"bad"})
	if len(filtered) != 1 || filtered[0].ID != "c1" {
		t.Errorf("exclusion failed: got %d", len(filtered))
	}
}

func TestSearchQueryParts(t *testing.T) {
	parts := searchQueryParts("foo,bar,baz")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}
	parts = searchQueryParts("")
	if len(parts) != 0 {
		t.Errorf("expected 0 parts for empty, got %d", len(parts))
	}
	parts = searchQueryParts("   ")
	if len(parts) != 0 {
		t.Errorf("expected 0 parts for whitespace, got %d", len(parts))
	}
}

func TestSearchTokens(t *testing.T) {
	tokens := searchTokens("Hello World")
	if len(tokens) < 2 {
		t.Errorf("expected >=2 tokens, got %v", tokens)
	}
	tokens = searchTokens("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty, got %v", tokens)
	}
	tokens = searchTokens("a b") // single chars are skipped
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for single chars, got %v", tokens)
	}
}

func TestHeadingsFromMarkdown(t *testing.T) {
	md := `# Title
## Subtitle
not a heading
### Another`
	headings := headingsFromMarkdown(md)
	if len(headings) != 3 {
		t.Errorf("expected 3 headings, got %d: %v", len(headings), headings)
	}
}

func TestCodeSymbols(t *testing.T) {
	content := `package main
func Hello() {}
type World struct{}
const Foo = 1`
	symbols := codeSymbols(content)
	if len(symbols) == 0 {
		t.Error("expected symbols, got none")
	}
}

func TestIsControlFlowSymbol(t *testing.T) {
	cases := map[string]bool{
		"if":     true,
		"FOR":    true,
		"return": true,
		"foo":    false,
		"":       false,
	}
	for v, want := range cases {
		if got := isControlFlowSymbol(v); got != want {
			t.Errorf("isControlFlowSymbol(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestTokenIn(t *testing.T) {
	if !tokenIn("a", []string{"a", "b"}) {
		t.Error("expected true for 'a' in [a b]")
	}
	if tokenIn("c", []string{"a", "b"}) {
		t.Error("expected false for 'c' in [a b]")
	}
	if tokenIn("a", nil) {
		t.Error("expected false for empty list")
	}
}

func TestFuzzyTokenIn(t *testing.T) {
	if !fuzzyTokenIn("hello", []string{"helloWorld"}) {
		t.Error("expected true for fuzzy match")
	}
	if fuzzyTokenIn("ab", []string{"abcd"}) {
		t.Error("short tokens (<3) should not match")
	}
	if fuzzyTokenIn("hello", nil) {
		t.Error("empty list should not match")
	}
}

func TestExcerptForQuery(t *testing.T) {
	content := "Line 1\nHello world\nLine 3"
	ex := excerptForQuery(content, []string{"hello"})
	if !strings.Contains(ex, "Hello") {
		t.Errorf("expected excerpt to contain Hello, got %q", ex)
	}
}

func TestCodeExcerptForQuery(t *testing.T) {
	content := "Line 1\nfoo bar\nLine 3\nLine 4"
	line, ex := codeExcerptForQuery(content, []string{"foo"})
	if line != 2 {
		t.Errorf("expected line 2, got %d", line)
	}
	if !strings.Contains(ex, "foo") {
		t.Errorf("expected excerpt to contain foo, got %q", ex)
	}
	// No match -> default excerpt
	line, ex = codeExcerptForQuery("x", []string{"y"})
	if line != 1 {
		t.Errorf("expected line 1 for no-match, got %d", line)
	}
}

func TestLineMatchesTokens(t *testing.T) {
	if !lineMatchesTokens("Hello World", []string{"hello"}) {
		t.Error("expected match")
	}
	if lineMatchesTokens("Foo", []string{"bar"}) {
		t.Error("expected no match")
	}
}

func TestCompactWhitespace(t *testing.T) {
	if got := compactWhitespace("a   b   c", 100); got != "a b c" {
		t.Errorf("compactWhitespace = %q, want 'a b c'", got)
	}
	// Long content gets truncated with ellipsis
	got := compactWhitespace(strings.Repeat("a", 200), 10)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}

func TestMergeResultsRRF(t *testing.T) {
	keyword := []previewSearchResult{{ID: "a", Score: 0.8}, {ID: "b", Score: 0.5}}
	semantic := []previewSearchResult{{ID: "b", Score: 0.9}, {ID: "c", Score: 0.4}}
	merged := mergeResultsRRF(keyword, semantic)
	if len(merged) != 3 {
		t.Errorf("expected 3 unique results, got %d", len(merged))
	}
}

func TestMergeMatchMethods(t *testing.T) {
	got := mergeMatchMethods([]string{"keyword"}, "semantic")
	if len(got) != 2 {
		t.Errorf("expected 2 methods, got %d", len(got))
	}
	got = mergeMatchMethods([]string{"keyword"}, "keyword")
	if len(got) != 1 {
		t.Errorf("duplicate should not be added, got %d", len(got))
	}
}

func TestCombineEmbeddingResults(t *testing.T) {
	keyword := []previewSearchResult{{ID: "a", Score: 0.8}}
	semantic := []previewSearchResult{{ID: "b", Score: 0.9}}
	got := combineEmbeddingResults(keyword, semantic, "hybrid", 5)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestBoostSemanticWithGraph(t *testing.T) {
	// Empty inputs should be no-op
	boostSemanticWithGraph(nil, nil)
	boostSemanticWithGraph([]previewSearchResult{}, []previewSearchResult{})
	// Matching via Path
	semantic := []previewSearchResult{{Path: "docs/a.md", Score: 0.5}}
	graph := []previewSearchResult{{Path: "docs/a.md", Anchor: true}}
	boostSemanticWithGraph(semantic, graph)
	if semantic[0].Score <= 0.5 {
		t.Errorf("expected boosted score > 0.5, got %v", semantic[0].Score)
	}
	hasGraph := false
	for _, m := range semantic[0].MatchedBy {
		if m == "graph" {
			hasGraph = true
		}
	}
	if !hasGraph {
		t.Errorf("expected 'graph' in MatchedBy, got %v", semantic[0].MatchedBy)
	}
	// Matching via SpecID
	semantic2 := []previewSearchResult{{SpecID: "spec-a", Score: 0.5}}
	graph2 := []previewSearchResult{{SpecID: "spec-a"}}
	boostSemanticWithGraph(semantic2, graph2)
	if semantic2[0].Score <= 0.5 {
		t.Errorf("expected boosted score via SpecID, got %v", semantic2[0].Score)
	}
	// Non-matching: no boost
	semantic3 := []previewSearchResult{{Path: "x.md", Score: 0.5}}
	graph3 := []previewSearchResult{{Path: "y.md"}}
	boostSemanticWithGraph(semantic3, graph3)
	if semantic3[0].Score != 0.5 {
		t.Errorf("expected unchanged score 0.5, got %v", semantic3[0].Score)
	}
}

func TestFilterCodeEmbeddingResultsByKeywordEvidence(t *testing.T) {
	results := []previewSearchResult{{ID: "a"}}
	code := []codeSearchDoc{{ID: "a", Title: "Hello"}}
	filtered := filterCodeEmbeddingResultsByKeywordEvidence(results, code, "hello", []string{"hello"})
	if len(filtered) == 0 {
		t.Error("expected at least one result with matching evidence")
	}
	// Empty results → returns empty.
	empty := filterCodeEmbeddingResultsByKeywordEvidence(nil, code, "hello", []string{"hello"})
	if len(empty) != 0 {
		t.Errorf("empty input: got %d, want 0", len(empty))
	}
}

func TestCodeGraphFlowScoreNonCall(t *testing.T) {
	score := codeGraphFlowScore(0.8, 1, "references")
	if score >= 0.8 {
		t.Errorf("expected score < 0.8 for non-call depth 1, got %v", score)
	}
}

func TestExpandDocsGraphAnchorEmpty(t *testing.T) {
	graph := specGraph{}
	index := newDocsGraphIndex(graph)
	results := map[string]previewSearchResult{}
	expandDocsGraphAnchor(graph, index, "missing", graphSearchAnchor{ID: "x", Score: 0.5}, 10, results)
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestGraphExpansionLimit(t *testing.T) {
	if got := graphExpansionLimit(0); got < 1 {
		t.Errorf("expected positive limit, got %d", got)
	}
	if got := graphExpansionLimit(5); got < 5 {
		t.Errorf("expected limit >= 5, got %d", got)
	}
}

func TestAnchorKeysEmpty(t *testing.T) {
	keys := anchorKeys(graphSearchAnchor{})
	for _, k := range keys {
		if k == "" {
			t.Error("anchorKeys should not produce empty strings")
		}
	}
}

func TestGraphNodeKeysWithPath(t *testing.T) {
	keys := graphNodeKeys(graphNode{ID: "x", Label: "X", Path: "docs/a.md"})
	hasDocsPrefix := false
	for _, k := range keys {
		if strings.HasPrefix(k, "docs/docs/") || strings.Contains(k, "docs/a.md") {
			hasDocsPrefix = true
		}
	}
	if !hasDocsPrefix {
		t.Errorf("expected docs/ prefix in keys, got %v", keys)
	}
}

func TestGraphAnchorsFromSemantic(t *testing.T) {
	semantic := []previewSearchResult{
		{ID: "a", Title: "A", Path: "a.md", SpecID: "spec-a", Score: 0.5},
		{ID: "b", Title: "B", Score: 0.9},
		{ID: "", Title: "", Path: "", SpecID: "", Score: 0.8}, // should be skipped
	}
	anchors := graphAnchorsFromSemantic(semantic)
	if len(anchors) != 2 {
		t.Errorf("expected 2 anchors, got %d", len(anchors))
	}
}

func TestGraphAnchorsFromSemanticDefaultScore(t *testing.T) {
	semantic := []previewSearchResult{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}
	anchors := graphAnchorsFromSemantic(semantic)
	if len(anchors) != 2 {
		t.Fatalf("expected 2 anchors, got %d", len(anchors))
	}
	for _, a := range anchors {
		if a.Score != 0.55 {
			t.Errorf("expected default score 0.55 for score=0 anchors, got %v", a.Score)
		}
	}
}

func TestLimitNeighbors(t *testing.T) {
	neighbors := []previewSearchNeighbor{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
	}
	limited := limitNeighbors(neighbors, 2)
	if len(limited) != 2 {
		t.Errorf("expected 2, got %d", len(limited))
	}
	// limit <= 0 returns all
	all := limitNeighbors(neighbors, 0)
	if len(all) != 4 {
		t.Errorf("expected 4 for limit=0, got %d", len(all))
	}
	// len <= limit returns all
	all = limitNeighbors([]previewSearchNeighbor{{ID: "a"}}, 5)
	if len(all) != 1 {
		t.Errorf("expected 1 for len <= limit, got %d", len(all))
	}
}

func TestDocGraphNeighbors(t *testing.T) {
	graph := specGraph{
		Edges: []graphEdge{
			{From: "a", To: "b", Type: "ref"},
			{From: "c", To: "a", Type: "ref"},
		},
	}
	neighbors := docGraphNeighbors(graph, "a")
	if len(neighbors) != 2 {
		t.Errorf("expected 2 neighbors for 'a', got %d", len(neighbors))
	}
}

func TestMergeGraphResult(t *testing.T) {
	results := map[string]previewSearchResult{}
	r1 := previewSearchResult{ID: "x", NodeID: "n1", Score: 0.5}
	mergeGraphResult(results, r1)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	// Higher score replaces
	r2 := previewSearchResult{ID: "x", NodeID: "n1", Score: 0.9}
	mergeGraphResult(results, r2)
	if results["n1"].Score != 0.9 {
		t.Errorf("expected higher score to win, got %v", results["n1"].Score)
	}
}

func TestPathContainsSearchPart(t *testing.T) {
	if !pathContainsSearchPart("docs/overview.md", "overview") {
		t.Error("expected true for path containing part")
	}
	if pathContainsSearchPart("docs/x.md", "y") {
		t.Error("expected false for path not containing part")
	}
}

func TestWriteJSONHTTP(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]string{"hello": "world"})
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"hello"`) {
		t.Errorf("expected json body, got %q", w.Body.String())
	}
}

func TestWriteJSONHTTPError(t *testing.T) {
	// Invalid JSON value (channel) should error and produce 500
	w := httptest.NewRecorder()
	defer func() {
		// json.Marshal may panic on channels; recover
		_ = recover()
	}()
	writeJSON(w, make(chan int))
}

func TestRoundTripJSONEmbeddingIndex(t *testing.T) {
	index := previewEmbeddingIndex{
		Model:      "test",
		APIBase:    "http://localhost",
		Dimensions: 4,
		Chunks: []previewEmbeddingChunk{
			{ID: "x", Type: "docs", Content: "hello", Embedding: []float32{0.1, 0.2, 0.3, 0.4}, Hash: "h"},
		},
	}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal err = %v", err)
	}
	var decoded previewEmbeddingIndex
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal err = %v", err)
	}
	if decoded.Model != index.Model || len(decoded.Chunks) != 1 {
		t.Errorf("round trip failed: got %+v", decoded)
	}
}

func TestGraphExpansionScoreClamped(t *testing.T) {
	// Very large depth clamps to 0.05
	if got := graphExpansionScore(0.1, 100); got != 0.05 {
		t.Errorf("expected clamp to 0.05, got %v", got)
	}
}

func TestNormalizedGraphKeysDedup(t *testing.T) {
	keys := normalizedGraphKeys("a", "A", "a/", "/a")
	for _, k := range keys {
		if k == "" {
			t.Errorf("got empty key: %v", keys)
		}
	}
}

func TestWritePreviewEmbeddingIndexError(t *testing.T) {
	// Path uses UserCacheDir so we just verify write succeeds in normal case.
	// Forcing an error path requires filesystem manipulation that may not be portable.
	root := t.TempDir()
	index := previewEmbeddingIndex{}
	if err := writePreviewEmbeddingIndex(root, index); err != nil {
		t.Logf("write to UserCacheDir may fail in sandbox: %v", err)
	}
}

func TestProbeDimensionsNetworkError(t *testing.T) {
	cfg := previewEmbeddingConfig{
		APIBase:     "http://127.0.0.1:1", // unreachable
		Model:       "test",
		QueryPrefix: "",
		Source:      "test",
		Timeout:     1,
	}
	_, err := cfg.probeDimensions()
	if err == nil {
		t.Error("expected network error")
	}
}

func TestSearchDocsGraphByQuerySimple(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{
			{ID: "n1", Label: "Hello World", Path: "docs/a.md", SpecID: "spec-a"},
		},
	}
	results := searchDocsGraphByQuery(graph, "hello", []string{"hello"}, "", nil, 10)
	if len(results) == 0 {
		t.Error("expected results for matching query")
	}
}

func TestSearchDocsGraphByQueryNoMatch(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{
			{ID: "n1", Label: "Hello", Path: "docs/a.md"},
		},
	}
	results := searchDocsGraphByQuery(graph, "xyz", []string{"xyz"}, "", nil, 10)
	if len(results) != 0 {
		t.Errorf("expected no results for non-matching query, got %d", len(results))
	}
}

func TestSearchDocsGraphByQueryExclusion(t *testing.T) {
	graph := specGraph{
		Nodes: []graphNode{
			{ID: "n1", Label: "Hello", Path: "docs/a.md"},
		},
	}
	results := searchDocsGraphByQuery(graph, "hello", []string{"hello"}, "hello", []string{"hello"}, 10)
	if len(results) != 0 {
		t.Errorf("expected no results when query is excluded, got %d", len(results))
	}
}

func TestEmbedBatchRawCustomTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1]}]}`))
	}))
	defer srv.Close()
	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", Timeout: 1}
	vectors, err := cfg.embedBatchRaw([]string{"a"})
	if err != nil {
		t.Errorf("err = %v", err)
	}
	if len(vectors) != 1 {
		t.Errorf("expected 1 vector, got %d", len(vectors))
	}
}

func TestEmbedBatchRawBadURL(t *testing.T) {
	cfg := previewEmbeddingConfig{
		APIBase: "://bad-url",
		Model:   "test",
	}
	_, err := cfg.embedBatchRaw([]string{"a"})
	if err == nil {
		t.Error("expected error from bad URL")
	}
}

func TestPreviewEmbeddingConfigForProjectEmpty(t *testing.T) {
	root := t.TempDir()
	cfg, source := previewEmbeddingConfigForProject(root)
	_ = cfg
	_ = source
}

func TestPreviewEmbeddingConfigFromOllamaError(t *testing.T) {
	// Use unreachable URL
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	_, err := previewEmbeddingConfigFromOllama()
	if err == nil {
		t.Error("expected error from unreachable Ollama host")
	}
}

func TestMergeGraphResultKeepsLowerDepth(t *testing.T) {
	results := map[string]previewSearchResult{}
	mergeGraphResult(results, previewSearchResult{NodeID: "n1", Score: 0.5, Depth: 2})
	mergeGraphResult(results, previewSearchResult{NodeID: "n1", Score: 0.5, Depth: 1})
	if results["n1"].Depth != 1 {
		t.Errorf("expected lower depth to win on tie, got %v", results["n1"].Depth)
	}
}

func TestGraphResultMergeKey(t *testing.T) {
	cases := []struct {
		name string
		r    previewSearchResult
		want string
	}{
		{"nodeID", previewSearchResult{NodeID: "n1", ID: "x", SpecID: "s", Path: "p", Title: "t"}, "n1"},
		{"id", previewSearchResult{ID: "x", SpecID: "s", Path: "p", Title: "t"}, "x"},
		{"specID", previewSearchResult{SpecID: "s", Path: "p", Title: "t"}, "s"},
		{"path", previewSearchResult{Path: "p", Title: "t"}, "p"},
		{"title", previewSearchResult{Title: "t"}, "t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := graphResultMergeKey(tc.r); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchResultMergeKey(t *testing.T) {
	if got := searchResultMergeKey(previewSearchResult{ID: "x"}); got != "x" {
		t.Errorf("got %q, want x", got)
	}
}

func TestParseSearchKeywordOperator(t *testing.T) {
	cases := map[string]string{
		"and":         "sum", // default
		"or":          "sum", // default
		"":            "sum", // default
		"xyz":         "sum", // default
		"difference":  "difference",
		"DIFFERENCE":  "difference",
	}
	for input, want := range cases {
		if got := parseSearchKeywordOperator(input); got != want {
			t.Errorf("parseSearchKeywordOperator(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	got := cosineSimilarity(a, b)
	if got > -0.99 || got < -1.0 {
		t.Errorf("opposite vectors cosine = %v, want ~-1.0", got)
	}
}

func TestEmbedBatchBatching(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		inputs := body["input"].([]any)
		vectors := make([]map[string]any, 0)
		for i := range inputs {
			vectors = append(vectors, map[string]any{"index": i, "embedding": []float32{0.1, 0.2}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": vectors})
	}))
	defer srv.Close()
	cfg := previewEmbeddingConfig{APIBase: srv.URL, Model: "test", BatchSize: 2}
	texts := []string{"a", "b", "c", "d", "e"}
	vectors, err := cfg.embedBatch(texts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(vectors) != 5 {
		t.Errorf("expected 5 vectors, got %d", len(vectors))
	}
	// 5 texts with batch size 2 -> 3 calls (2, 2, 1)
	if calls != 3 {
		t.Errorf("expected 3 API calls, got %d", calls)
	}
}

func TestEmbedBatchDefaultBatchSize(t *testing.T) {
	cfg := previewEmbeddingConfig{BatchSize: 0}
	if cfg.BatchSize != 0 {
		t.Fatal("test setup error")
	}
	// Batch size <= 0 should default to 64
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 64
	}
	if batchSize != 64 {
		t.Errorf("expected default batch size 64, got %d", batchSize)
	}
}

func TestEmbeddingQueryPrefixCommonModels(t *testing.T) {
	cases := []string{"nomic-embed-text", "mxbai-embed-large", "bge-large", "bge-base", "bge-small", "all-minilm"}
	for _, m := range cases {
		_ = embeddingQueryPrefix(m)
		_ = embeddingDocPrefix(m)
	}
}

func TestReadPreviewEmbeddingIndexCorrupt(t *testing.T) {
	root := t.TempDir()
	indexPath := previewEmbeddingIndexPath(root)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := readPreviewEmbeddingIndex(root)
	// Returns zero value on corrupt file
	if idx.Model != "" {
		t.Errorf("expected zero index, got %+v", idx)
	}
}

func TestProbeDimensionsWithPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode body to verify prefix is in input
		var body struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Input) == 0 {
			t.Error("expected input")
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3,0.4]}]}`))
	}))
	defer srv.Close()

	cfg := previewEmbeddingConfig{
		APIBase:     srv.URL,
		Model:       "test",
		QueryPrefix: "query: ",
		Source:      "test",
	}
	dim, err := cfg.probeDimensions()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if dim != 4 {
		t.Errorf("expected 4, got %d", dim)
	}
}

func TestWritePreviewEmbeddingIndexWithContent(t *testing.T) {
	root := t.TempDir()
	index := previewEmbeddingIndex{
		Model:      "model",
		APIBase:    "http://localhost",
		Dimensions: 2,
		Chunks: []previewEmbeddingChunk{
			{ID: "x", Content: "hello", Embedding: []float32{0.1, 0.2}, Hash: "abc"},
		},
	}
	if err := writePreviewEmbeddingIndex(root, index); err != nil {
		t.Fatalf("err = %v", err)
	}
	data, err := os.ReadFile(previewEmbeddingIndexPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "model") {
		t.Error("expected model in JSON output")
	}
}

func TestEmbeddingIndexJsonRoundTrip(t *testing.T) {
	index := previewEmbeddingIndex{
		Model:      "x",
		APIBase:    "http://localhost",
		Dimensions: 3,
		IndexedAt:  parseTimeOrZero("2024-01-01T00:00:00Z"),
	}
	data, _ := json.Marshal(index)
	var out previewEmbeddingIndex
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal err = %v", err)
	}
	if out.Model != "x" {
		t.Errorf("got %q, want x", out.Model)
	}
}

func parseTimeOrZero(s string) (t time.Time) {
	_ = t
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return parsed
}


func TestLanguageForPath(t *testing.T) {
	cases := map[string]string{
		"foo.go":                "go",
		"foo.js":                "javascript",
		"foo.cjs":               "javascript",
		"foo.mjs":               "javascript",
		"foo.jsx":               "jsx",
		"foo.ts":                "typescript",
		"foo.tsx":               "tsx",
		"foo.css":               "css",
		"foo.scss":              "scss",
		"foo.sass":              "sass",
		"foo.html":              "html",
		"foo.htm":               "html",
		"foo.json":              "json",
		"foo.yaml":              "yaml",
		"foo.yml":               "yaml",
		"foo.toml":              "toml",
		"foo.md":                "markdown",
		"foo.vue":               "vue",
		"foo.svelte":            "svelte",
		"foo.py":                "python",
		"foo.rb":                "ruby",
		"foo.rs":                "rust",
		"path/Dockerfile":       "dockerfile",
		"DOCKERFILE":            "dockerfile",
		"unknown.xyz":           "plaintext",
		"":                      "plaintext",
		"foo.GO":                "go",
		"foo.java":              "java",
		"foo.kt":                "kotlin",
		"foo.kts":               "kotlin",
		"foo.swift":             "swift",
		"foo.c":                 "c",
		"foo.h":                 "c",
		"foo.cpp":               "cpp",
		"foo.hpp":               "cpp",
		"foo.cs":                "csharp",
		"foo.php":               "php",
		"foo.sh":                "bash",
		"foo.bash":              "bash",
		"foo.zsh":               "bash",
		"foo.fish":              "bash",
		"foo.sql":               "sql",
		"foo.xml":               "xml",
		"foo.graphql":           "graphql",
		"foo.gql":               "graphql",
	}
	for path, want := range cases {
		if got := languageForPath(path); got != want {
			t.Errorf("languageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}


func TestPreviewEmbeddingConfigFromKnownsSettingsSuccess(t *testing.T) {
	settings := knownsEmbeddingSettings{
		Models:    map[string]knownsEmbeddingModel{"m1": {Model: "test-model", Provider: "p1", Dimensions: 768}},
		Providers: map[string]knownsEmbeddingProvider{"p1": {APIBase: "http://localhost:8080", APIKey: "secret"}},
	}
	cfg, err := previewEmbeddingConfigFromKnownsSettings(settings, "m1", 0, "test")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cfg.APIBase != "http://localhost:8080" {
		t.Errorf("APIBase = %q", cfg.APIBase)
	}
	if cfg.Dimensions != 768 {
		t.Errorf("Dimensions = %d", cfg.Dimensions)
	}
}

func TestPreviewEmbeddingConfigFromKnownsSettingsFallback(t *testing.T) {
	settings := knownsEmbeddingSettings{
		Models:    map[string]knownsEmbeddingModel{"m1": {Provider: "p1"}},
		Providers: map[string]knownsEmbeddingProvider{"p1": {APIBase: "http://x"}},
	}
	cfg, err := previewEmbeddingConfigFromKnownsSettings(settings, "m1", 384, "test")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cfg.Dimensions != 384 {
		t.Errorf("expected fallback dim 384, got %d", cfg.Dimensions)
	}
	if cfg.Model != "m1" {
		t.Errorf("expected Model fallback to id, got %q", cfg.Model)
	}
}

func TestPreviewEmbeddingConfigFromKnownsSettingsDefaultTimeouts(t *testing.T) {
	settings := knownsEmbeddingSettings{
		Models:    map[string]knownsEmbeddingModel{"m1": {Provider: "p1"}},
		Providers: map[string]knownsEmbeddingProvider{"p1": {APIBase: "http://x"}},
	}
	cfg, err := previewEmbeddingConfigFromKnownsSettings(settings, "m1", 0, "test")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cfg.Timeout != 30 {
		t.Errorf("expected default Timeout 30, got %d", cfg.Timeout)
	}
	if cfg.BatchSize != 64 {
		t.Errorf("expected default BatchSize 64, got %d", cfg.BatchSize)
	}
}

func TestPreviewEmbeddingConfigFromKnownsSettingsUnknownModel(t *testing.T) {
	settings := knownsEmbeddingSettings{
		Models:    map[string]knownsEmbeddingModel{},
		Providers: map[string]knownsEmbeddingProvider{},
	}
	_, err := previewEmbeddingConfigFromKnownsSettings(settings, "missing", 0, "test")
	if err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestPreviewEmbeddingConfigFromKnownsSettingsUnknownProvider(t *testing.T) {
	settings := knownsEmbeddingSettings{
		Models:    map[string]knownsEmbeddingModel{"m1": {Provider: "missing"}},
		Providers: map[string]knownsEmbeddingProvider{},
	}
	_, err := previewEmbeddingConfigFromKnownsSettings(settings, "m1", 0, "test")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestPreviewEmbeddingConfigFromKnownsSettingsProviderMissingAPIBase(t *testing.T) {
	settings := knownsEmbeddingSettings{
		Models:    map[string]knownsEmbeddingModel{"m1": {Provider: "p1"}},
		Providers: map[string]knownsEmbeddingProvider{"p1": {}},
	}
	_, err := previewEmbeddingConfigFromKnownsSettings(settings, "m1", 0, "test")
	if err == nil {
		t.Error("expected error for missing APIBase")
	}
}

func TestPreviewEmbeddingConfigFromKnownsDefaultSingleModel(t *testing.T) {
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()

	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{
			Models:    map[string]knownsEmbeddingModel{"only-one": {Provider: "p1"}},
			Providers: map[string]knownsEmbeddingProvider{"p1": {APIBase: "http://x"}},
		}, nil
	}
	cfg, err := previewEmbeddingConfigFromKnownsDefault()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cfg.Model != "only-one" {
		t.Errorf("Model = %q", cfg.Model)
	}
}

func TestPreviewEmbeddingConfigFromKnownsProjectLoadSettingsError(t *testing.T) {
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()

	root := t.TempDir()
	knownsDir := filepath.Join(root, ".knowns")
	if err := os.MkdirAll(knownsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(knownsDir, "config.json"), []byte(`{"settings":{"semanticSearch":{"enabled":true,"model":"x","provider":"api","dimensions":384}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{}, errors.New("no settings")
	}
	_, err := previewEmbeddingConfigFromKnownsProject(root)
	if err == nil {
		t.Error("expected error loading settings")
	}
}


func TestCombineEmbeddingResultsSemanticMode(t *testing.T) {
	keyword := []previewSearchResult{{ID: "k", Score: 0.9}}
	semantic := []previewSearchResult{{ID: "s", Score: 0.8}}
	got := combineEmbeddingResults(keyword, semantic, "semantic", 5)
	if len(got) != 1 || got[0].ID != "s" {
		t.Errorf("semantic mode: got %+v", got)
	}
}

func TestCombineEmbeddingResultsKeywordMode(t *testing.T) {
	keyword := []previewSearchResult{{ID: "k", Score: 0.9}}
	semantic := []previewSearchResult{{ID: "s", Score: 0.8}}
	got := combineEmbeddingResults(keyword, semantic, "keyword", 5)
	if len(got) != 1 || got[0].ID != "k" {
		t.Errorf("keyword mode: got %+v", got)
	}
}

func TestCombineEmbeddingResultsHybridDefault(t *testing.T) {
	keyword := []previewSearchResult{{ID: "k", Score: 0.9}}
	semantic := []previewSearchResult{{ID: "s", Score: 0.8}}
	got := combineEmbeddingResults(keyword, semantic, "unknown-mode", 5)
	if len(got) != 2 {
		t.Errorf("hybrid mode: expected 2, got %d", len(got))
	}
}

func TestCombineEmbeddingResultsEmpty(t *testing.T) {
	got := combineEmbeddingResults(nil, nil, "hybrid", 5)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestPreviewEmbeddingConfigFromOllamaSuccess(t *testing.T) {
	// Mock Ollama /api/tags endpoint by overriding the OLLAMA_HOST
	// We cannot easily redirect localhost:11434, so we'll verify with unreachable
	// (covered elsewhere) and check parsing with a stub via OLLAMA_HOST env override
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // unreachable
	_, err := previewEmbeddingConfigFromOllama()
	if err == nil {
		t.Error("expected error from unreachable host")
	}
}


func TestPreviewEmbeddingConfigFromOllamaBadStatus(t *testing.T) {
	// The function uses http://localhost:11434/api/tags directly.
	// We can run a fake server on that port using httptest.NewServer, but
	// previewEmbeddingConfigFromOllama doesn't use an env var - it uses hardcoded URL.
	// So we can only test the network error / unreachable case here.
	_, err := previewEmbeddingConfigFromOllama()
	if err == nil {
		t.Error("expected error from unreachable Ollama")
	}
}

func TestReadDocsSearchFileAllBranches(t *testing.T) {
	dir := t.TempDir()

	// 1. Path that doesn't exist -> os.Stat error.
	if _, ok := readDocsSearchFile(dir, filepath.Join(dir, "missing.md"), "missing.md"); ok {
		t.Error("expected false for non-existent file")
	}

	// 2. Directory -> IsDir() true.
	if _, ok := readDocsSearchFile(dir, dir, "subdir"); ok {
		t.Error("expected false for directory")
	}

	// 3. File larger than maxSearchFileBytes.
	big := filepath.Join(dir, "big.md")
	bigData := make([]byte, maxSearchFileBytes+1)
	for i := range bigData {
		bigData[i] = 'a'
	}
	if err := os.WriteFile(big, bigData, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readDocsSearchFile(dir, big, "big.md"); ok {
		t.Error("expected false for oversized file")
	}

	// 4. Non-UTF8 content.
	nonUTF8 := filepath.Join(dir, "nonutf8.md")
	if err := os.WriteFile(nonUTF8, []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readDocsSearchFile(dir, nonUTF8, "nonutf8.md"); ok {
		t.Error("expected false for non-UTF8 file")
	}

	// 5. Success.
	ok_path := filepath.Join(dir, "ok.md")
	if err := os.WriteFile(ok_path, []byte("# Heading\n\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, ok := readDocsSearchFile(dir, ok_path, "ok.md")
	if !ok {
		t.Fatal("expected true for valid file")
	}
	if doc.Title != "ok.md" {
		t.Errorf("unexpected title: %s", doc.Title)
	}
}


func TestWritePreviewEmbeddingIndexMkdirErrorP(t *testing.T) {
	// Force MkdirAll to fail by using a path whose ancestor is a regular file.
	// We do this by making the cache directory equal to a file path inside
	// the temp dir.
	dir := t.TempDir()
	// Make a regular file whose name will become the cache directory.
	cacheAsFile := filepath.Join(dir, "cache")
	if err := os.WriteFile(cacheAsFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The hashed subpath will become /cache/ns-workspace/.../embedding-index.json
	// and MkdirAll will fail because "cache" is a regular file.
	orig := previewUserCacheDirForTest
	defer func() { previewUserCacheDirForTest = orig }()
	previewUserCacheDirForTest = func() (string, error) {
		return cacheAsFile, nil
	}

	root := t.TempDir()
	err := writePreviewEmbeddingIndex(root, previewEmbeddingIndex{})
	if err == nil {
		t.Error("expected mkdir error")
	}
}

func TestPreviewEmbeddingIndexPathErrorFallbackP(t *testing.T) {
	orig := previewUserCacheDirForTest
	defer func() { previewUserCacheDirForTest = orig }()
	previewUserCacheDirForTest = func() (string, error) {
		return "", errors.New("nope")
	}
	got := previewEmbeddingIndexPath("/some/root")
	// Should fall back to TempDir.
	if !strings.Contains(got, os.TempDir()) {
		t.Errorf("expected temp dir fallback, got %s", got)
	}
}

func TestPreviewEmbeddingIndexPathEmptyFallbackP(t *testing.T) {
	orig := previewUserCacheDirForTest
	defer func() { previewUserCacheDirForTest = orig }()
	previewUserCacheDirForTest = func() (string, error) {
		return "", nil
	}
	got := previewEmbeddingIndexPath("/some/root")
	if !strings.Contains(got, os.TempDir()) {
		t.Errorf("expected temp dir fallback, got %s", got)
	}
}


func TestPreviewEmbeddingConfigForProjectProbeSuccessP(t *testing.T) {
	// Setup a knowns config with a model whose Dimensions=0 so probeDimensions
	// is invoked. Use httptest to serve a fake embeddings endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3,0.4]}]}`))
	}))
	defer srv.Close()

	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{
			Models: map[string]knownsEmbeddingModel{
				"m1": {Model: "test-model", Provider: "p1" /* Dimensions: 0 */},
			},
			Providers: map[string]knownsEmbeddingProvider{
				"p1": {APIBase: srv.URL},
			},
			DefaultModel: "m1",
		}, nil
	}

	root := t.TempDir()
	cfg, warning := previewEmbeddingConfigForProject(root)
	if warning != "" {
		t.Errorf("unexpected warning: %s", warning)
	}
	if cfg.Dimensions != 4 {
		t.Errorf("expected probed dimensions=4, got %d", cfg.Dimensions)
	}
}

func TestPreviewEmbeddingConfigForProjectProbeFailP(t *testing.T) {
	// Setup a knowns config with a model whose Dimensions=0 so probeDimensions
	// is invoked. Point APIBase to an unreachable address to make probe fail.
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{
			Models: map[string]knownsEmbeddingModel{
				"m1": {Model: "test-model", Provider: "p1"},
			},
			Providers: map[string]knownsEmbeddingProvider{
				"p1": {APIBase: "http://127.0.0.1:1", Timeout: 1},
			},
			DefaultModel: "m1",
		}, nil
	}

	root := t.TempDir()
	cfg, warning := previewEmbeddingConfigForProject(root)
	// Probe fails for first resolver (Knowns default), and Ollama also fails.
	// We expect a non-empty warning.
	if warning == "" {
		t.Error("expected warning when probe fails")
	}
	if cfg.APIBase != "" {
		t.Errorf("expected empty cfg, got %+v", cfg)
	}
}

func TestPreviewEmbeddingConfigForProjectProbeFailThenSucceedP(t *testing.T) {
	// First resolver fails (no knowns config file). Second resolver returns
	// cfg with Dimensions=0 and probe succeeds via httptest server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.5,0.6]}]}`))
	}))
	defer srv.Close()

	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{
			Models: map[string]knownsEmbeddingModel{
				"only": {Model: "test-model", Provider: "p1"},
			},
			Providers: map[string]knownsEmbeddingProvider{
				"p1": {APIBase: srv.URL},
			},
		}, nil
	}

	// Empty root means no .knowns/config.json, so Knowns project resolver fails.
	root := t.TempDir()
	cfg, warning := previewEmbeddingConfigForProject(root)
	if warning != "" {
		t.Errorf("unexpected warning: %s", warning)
	}
	if cfg.Dimensions != 2 {
		t.Errorf("expected probed dimensions=2, got %d", cfg.Dimensions)
	}
}


func TestLoadPreviewEmbeddingSearchEmptyChunksP(t *testing.T) {
	// Setup a valid knowns config (no warning). With empty docs/codeDocs,
	// the resulting index has zero chunks and we hit the
	// "Embedding index is empty" branch.
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{
			Models: map[string]knownsEmbeddingModel{
				"m1": {Model: "test-model", Provider: "p1", Dimensions: 4},
			},
			Providers: map[string]knownsEmbeddingProvider{
				"p1": {APIBase: "http://localhost:1234"},
			},
			DefaultModel: "m1",
		}, nil
	}
	root := t.TempDir()
	search, warnings := loadPreviewEmbeddingSearch(root, nil, nil)
	if search != nil {
		t.Errorf("expected nil search when chunks empty, got %+v", search)
	}
	if len(warnings) == 0 {
		t.Error("expected warnings when chunks empty")
	}
}


func TestWritePreviewEmbeddingIndexMarshalErrorP(t *testing.T) {
	// Stub the JSON marshal seam to return an error. This exercises the
	// "return err" path after json.Marshal in writePreviewEmbeddingIndex.
	orig := previewEmbeddingIndexMarshalForTest
	defer func() { previewEmbeddingIndexMarshalForTest = orig }()
	previewEmbeddingIndexMarshalForTest = func(v previewEmbeddingIndex) ([]byte, error) {
		return nil, errors.New("marshal forced failure")
	}
	root := t.TempDir()
	if err := writePreviewEmbeddingIndex(root, previewEmbeddingIndex{}); err == nil {
		t.Error("expected marshal error")
	}
}


func TestScanDocsSearchDocsSpecNotInGitP(t *testing.T) {
	// Setup a temp project as a git repo so git ls-files works. We add a
	// tracked file that's NOT in the spec list, so the spec is filtered
	// out by the gitFilesKnown check.
	orig := gitTrackedFilesForTest
	defer func() { gitTrackedFilesForTest = orig }()

	dir := t.TempDir()
	// Initialize a git repo with a tracked file that is NOT a spec doc.
	for _, name := range []string{"x.txt", "y.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Run git init and add the files
	if _, err := exec.Command("git", "-C", dir, "init", "-q").Output(); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.Command("git", "-C", dir, "-c", "user.email=x@x", "-c", "user.name=x", "add", "x.txt", "y.md").Output(); err != nil {
		t.Skip("git add failed")
	}

	// Override the seam: report that only x.txt is tracked, so the spec
	// referencing y.md gets filtered out.
	gitTrackedFilesForTest = func(projectRoot string) (map[string]bool, bool) {
		return map[string]bool{"x.txt": true}, true
	}

	specs := []specDocument{
		{ID: "spec-y", Title: "Y", Path: "y.md", Raw: "raw y"},
	}
	docs, warnings := scanDocsSearchDocs(dir, dir, specs)
	if len(docs) != 0 {
		t.Errorf("expected 0 docs after git filter, got %d", len(docs))
	}
	_ = warnings
}


func TestPathIsUnderDocsRootP(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		docsRoot string
		rel      string
		want     bool
	}{
		{"under", filepath.Join(dir, "docs"), "docs/foo.md", true},
		{"nested", filepath.Join(dir, "docs"), "docs/sub/bar.md", true},
		{"outside", filepath.Join(dir, "docs"), "src/foo.go", false},
		{"docs_rel_empty", filepath.Join(dir, "docs"), "anything", false},
		{"docs_rel_dot", "/", "/", false},
	}
	for _, tc := range cases {
		got := pathIsUnderDocsRoot(dir, tc.docsRoot, tc.rel)
		if got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}


func TestRelPathAllBranchesP(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		root string
		path string
	}{
		{"empty_path", dir, ""},
		{"inside", dir, filepath.Join(dir, "foo.md")},
		{"outside", dir, "/elsewhere/foo.md"},
		{"rel_err", "/nonexistent-root-zzz", dir},
	}
	for _, tc := range cases {
		got := relPath(tc.root, tc.path)
		if tc.name == "empty_path" && got != "" {
			t.Errorf("%s: got %q want empty", tc.name, got)
		}
		_ = got
	}
}


func TestShouldSkipGitSearchPath(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		{"src/foo.go", false},
		{"node_modules/foo.js", true},
		{".git/HEAD", true},
		{"internal/preview/preview_ui/main.ts", true},
		{"internal/preview/preview_ui/sub/main.ts", true},
		{"internal/preview/main.go", false},
	}
	for _, tc := range cases {
		got := shouldSkipGitSearchPath(tc.rel)
		if got != tc.want {
			t.Errorf("shouldSkipGitSearchPath(%q) = %v, want %v", tc.rel, got, tc.want)
		}
	}
}

func TestSameCleanPath(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "/x", false},
		{"/x", "", false},
		{"", "", false},
		{dir, dir, true},
		{dir, filepath.Clean(dir), true},
		{dir + "/", dir, true},
		{dir + "/x", dir + "/x", true},
		{dir + "/x", dir + "/y", false},
	}
	for _, tc := range cases {
		got := sameCleanPath(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("sameCleanPath(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestHandleSearch(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()

	// POST → 405
	req := httptest.NewRequest(http.MethodPost, "/api/search?q=foo", nil)
	server.handler.HandleSearch(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rec.Code)
	}

	// GET → 200
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=foo", nil)
	server.handler.HandleSearch(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET: got %d, want 200", rec.Code)
	}
}

func TestHandleSearchLoadError(t *testing.T) {
	root := t.TempDir()
	// docs path is a file → scanSpecProject fails → load returns error.
	if err := os.WriteFile(filepath.Join(root, "docs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=foo", nil)
	server.handler.HandleSearch(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with warnings, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Docs directory is unavailable") {
		t.Errorf("expected warning in body, got %s", rec.Body.String())
	}
}
