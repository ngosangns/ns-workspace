package preview

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultGraphLauncherName = "ns-workspace-graph.html"

type graphOptions struct {
	projectRoot string
	docsDir     string
	addr        string
	outPath     string
	openBrowser bool
	query       string
	limit       int
	keywordOp   string
	jsonOutput  bool
}

func RunGraph(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opt := graphOptions{
		projectRoot: cwd,
		docsDir:     "docs",
		addr:        defaultPreviewAddr,
		outPath:     filepath.Join(cwd, defaultGraphLauncherName),
		openBrowser: true,
		limit:       defaultSearchLimit,
		keywordOp:   "sum",
	}
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to inspect")
	fs.StringVar(&opt.docsDir, "docs-dir", opt.docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address")
	fs.StringVar(&opt.outPath, "out", opt.outPath, "generated launcher HTML path")
	fs.BoolVar(&opt.openBrowser, "open", true, "open browser after the launcher is written")
	fs.StringVar(&opt.query, "query", "", "run a non-interactive Search/Code Graph query and exit")
	fs.IntVar(&opt.limit, "limit", defaultSearchLimit, "maximum results per search panel in query mode")
	fs.StringVar(&opt.keywordOp, "keyword-op", "sum", "keyword operator for comma-separated query terms: sum or difference")
	fs.BoolVar(&opt.jsonOutput, "json", false, "print query results as JSON")
	noOpen := fs.Bool("no-open", false, "write the launcher without opening the browser")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *noOpen {
		opt.openBrowser = false
	}
	opt.projectRoot = normalizePreviewProjectRoot(opt.projectRoot)
	opt.outPath = normalizeGraphOutputPath(cwd, opt.outPath)
	opt.keywordOp = parseSearchKeywordOperator(opt.keywordOp)
	opt.limit = normalizeGraphQueryLimit(opt.limit)
	if strings.TrimSpace(opt.query) != "" {
		opt.query = strings.TrimSpace(opt.query)
		return runGraphQuery(opt, os.Stdout)
	}

	server := newPreviewServer(previewOptions{projectRoot: opt.projectRoot, docsDir: opt.docsDir, addr: opt.addr})
	listener, err := net.Listen("tcp", opt.addr)
	if err != nil {
		return err
	}
	addr := listener.Addr().String()
	displayURL := "http://" + addr
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "[::1]:") {
		displayURL = "http://localhost:" + portOf(addr)
	}
	appURL := displayURL + "/search.html"
	if err := writeGraphLauncher(opt.outPath, appURL, opt.projectRoot, docsRoot(opt.projectRoot, opt.docsDir)); err != nil {
		_ = listener.Close()
		return err
	}

	fmt.Printf("graph search: %s\n", appURL)
	fmt.Printf("launcher: %s\n", opt.outPath)
	fmt.Printf("project: %s\n", opt.projectRoot)
	fmt.Printf("docs: %s\n", docsRoot(opt.projectRoot, opt.docsDir))
	if opt.openBrowser {
		if err := openURL(opt.outPath); err != nil {
			fmt.Printf("open browser failed: %v\n", err)
		}
	}
	if err := server.srv.Serve(listener); err != nil {
		return err
	}
	return nil
}

func runGraphQuery(opt graphOptions, out io.Writer) error {
	server := newPreviewServer(previewOptions{projectRoot: opt.projectRoot, docsDir: opt.docsDir, addr: opt.addr})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = server.shutdown(shutdownCtx)
	}()
	return runGraphQueryWithProvider(ctx, opt, server.codeGraph, out)
}

func runGraphQueryWithProvider(ctx context.Context, opt graphOptions, codeGraph previewCodeGraphProvider, out io.Writer) error {
	response := buildGraphQueryResponse(ctx, opt, codeGraph)
	if opt.jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(response)
	}
	return writeGraphQueryText(out, response)
}

func buildGraphQueryResponse(ctx context.Context, opt graphOptions, codeGraph previewCodeGraphProvider) previewSearchResponse {
	project, err := scanSpecProject(opt.projectRoot, opt.docsDir)
	warnings := []string{}
	if err != nil {
		project = emptySearchProject(opt.projectRoot, opt.docsDir)
		warnings = append(warnings, "Docs directory is unavailable; searching code and LSP code graph only: "+err.Error())
	}
	response := buildPreviewSearchResponse(ctx, project, codeGraph, opt.projectRoot, opt.query, "hybrid", opt.keywordOp, opt.limit)
	response.Warnings = append(warnings, response.Warnings...)
	return response
}

func writeGraphQueryText(out io.Writer, response previewSearchResponse) error {
	if _, err := fmt.Fprintf(out, "Query: %s\n", response.Query); err != nil {
		return err
	}
	if len(response.Warnings) > 0 {
		if _, err := fmt.Fprintln(out, "\nWarnings:"); err != nil {
			return err
		}
		for _, warning := range response.Warnings {
			if _, err := fmt.Fprintf(out, "- %s\n", warning); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(out, "\nStats: docsSemantic=%d docsGraph=%d codeSemantic=%d codeGraph=%d\n",
		response.Stats["docsSemantic"],
		response.Stats["docsGraph"],
		response.Stats["codeSemantic"],
		response.Stats["codeGraph"],
	); err != nil {
		return err
	}
	panels := []struct {
		title   string
		results []previewSearchResult
	}{
		{"Code Graph", response.Panels.CodeGraph},
		{"Docs Graph", response.Panels.DocsGraph},
		{"Code Semantic", response.Panels.CodeSemantic},
		{"Docs Semantic", response.Panels.DocsSemantic},
	}
	for _, panel := range panels {
		if _, err := fmt.Fprintf(out, "\n%s:\n", panel.title); err != nil {
			return err
		}
		if len(panel.results) == 0 {
			if _, err := fmt.Fprintln(out, "- no results"); err != nil {
				return err
			}
			continue
		}
		for _, result := range panel.results {
			if err := writeGraphQueryResult(out, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeGraphQueryResult(out io.Writer, result previewSearchResult) error {
	location := result.Path
	if result.Line > 0 {
		location = fmt.Sprintf("%s:%d", result.Path, result.Line)
	}
	if location == "" {
		location = result.NodeID
	}
	if _, err := fmt.Fprintf(out, "- %s", result.Title); err != nil {
		return err
	}
	if location != "" {
		if _, err := fmt.Fprintf(out, " (%s)", location); err != nil {
			return err
		}
	}
	if result.Source != "" || result.Confidence != "" || result.FlowRole != "" {
		if _, err := fmt.Fprintf(out, " [%s]", strings.Join(nonEmptyStrings(result.Source, result.Confidence, result.FlowRole), ", ")); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	for i, neighbor := range result.Neighbors {
		if i >= 3 {
			if _, err := fmt.Fprintf(out, "  - +%d more neighbors\n", len(result.Neighbors)-i); err != nil {
				return err
			}
			break
		}
		neighborLocation := neighbor.Path
		if neighbor.Line > 0 {
			neighborLocation = fmt.Sprintf("%s:%d", neighbor.Path, neighbor.Line)
		}
		if neighborLocation == "" {
			neighborLocation = neighbor.ID
		}
		if _, err := fmt.Fprintf(out, "  - %s %s", neighbor.Direction, neighbor.Label); err != nil {
			return err
		}
		if neighbor.Relation != "" {
			if _, err := fmt.Fprintf(out, " via %s", neighbor.Relation); err != nil {
				return err
			}
		}
		if neighborLocation != "" {
			if _, err := fmt.Fprintf(out, " (%s)", neighborLocation); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	return nil
}

func nonEmptyStrings(values ...string) []string {
	nonEmpty := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" && !seen[value] {
			nonEmpty = append(nonEmpty, value)
			seen[value] = true
		}
	}
	return nonEmpty
}

func normalizeGraphQueryLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

func normalizeGraphOutputPath(cwd, path string) string {
	path = expandPath(strings.TrimSpace(path))
	if path == "" {
		path = defaultGraphLauncherName
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func writeGraphLauncher(path, appURL, projectRoot, docsRoot string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// The launcher is a small file-system entrypoint; the dynamic search data
	// stays behind the local server so new queries reuse the Go search pipeline.
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return graphLauncherTemplate.Execute(file, struct {
		AppURL      string
		AppURLJS    template.JSStr
		ProjectRoot string
		DocsRoot    string
	}{
		AppURL:      appURL,
		AppURLJS:    template.JSStr(appURL),
		ProjectRoot: projectRoot,
		DocsRoot:    docsRoot,
	})
}

var graphLauncherTemplate = template.Must(template.New("graph-launcher").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta http-equiv="refresh" content="0; url={{ .AppURL }}" />
    <title>ns-workspace graph search</title>
    <style>
      :root {
        color-scheme: light dark;
        font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }
      body {
        align-items: center;
        display: grid;
        min-height: 100vh;
        margin: 0;
        padding: 2rem;
      }
      main {
        max-width: 42rem;
      }
      code {
        word-break: break-word;
      }
    </style>
    <script>
      window.location.replace("{{ .AppURLJS }}");
    </script>
  </head>
  <body>
    <main>
      <h1>Opening graph search...</h1>
      <p>Project: <code>{{ .ProjectRoot }}</code></p>
      <p>Docs: <code>{{ .DocsRoot }}</code></p>
      <p><a href="{{ .AppURL }}">Open graph search</a></p>
    </main>
  </body>
</html>
`))
