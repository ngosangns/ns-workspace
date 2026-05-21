package preview

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const defaultGraphLauncherName = "ns-workspace-graph.html"

type graphOptions struct {
	projectRoot string
	docsDir     string
	addr        string
	outPath     string
	openBrowser bool
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
	}
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to inspect")
	fs.StringVar(&opt.docsDir, "docs-dir", opt.docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address")
	fs.StringVar(&opt.outPath, "out", opt.outPath, "generated launcher HTML path")
	fs.BoolVar(&opt.openBrowser, "open", true, "open browser after the launcher is written")
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
