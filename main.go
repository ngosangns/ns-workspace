package main

import (
	"embed"
	"fmt"
	"os"

	synccli "github.com/ngosangns/ns-workspace/internal/cli"
	"github.com/ngosangns/ns-workspace/internal/graphquery"
	"github.com/ngosangns/ns-workspace/internal/preview"
)

//go:embed presets/agents presets/mcp presets/minimax presets/opencode presets/registry presets/settings presets/adapters presets/manifest.json presets/skills/* presets/subagents
var presetFS embed.FS

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmd := args[0]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printUsage()
		return nil
	}
	if synccli.IsAgentSyncCommand(cmd) {
		return synccli.RunAgentSync(cmd, args[1:], presetFS)
	}
	switch cmd {
	case "preview":
		return preview.Run(args[1:])
	case "search":
		return preview.RunSearch(args[1:])
	case "graph":
		return preview.RunGraph(args[1:])
	case "lsp":
		return graphquery.RunLSP(args[1:], preview.GraphQueryLSPDetector{})
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func printUsage() {
	fmt.Println(`agent-bootstrap sets up shared personal agent config.

Usage:
  go run github.com/ngosangns/ns-workspace@latest init [flags]
  go run github.com/ngosangns/ns-workspace@latest update [flags]
  go run github.com/ngosangns/ns-workspace@latest status [flags]
  go run github.com/ngosangns/ns-workspace@latest doctor [flags]
  go run github.com/ngosangns/ns-workspace@latest registry [flags]
  go run github.com/ngosangns/ns-workspace@latest agents [flags]
  go run github.com/ngosangns/ns-workspace@latest preview [flags]
  go run github.com/ngosangns/ns-workspace@latest search [flags]
  go run github.com/ngosangns/ns-workspace@latest graph [flags]
  go run github.com/ngosangns/ns-workspace@latest lsp <list|install> [flags]

Local checkout usage:
  cd /path/to/ns-workspace
  go run . preview --project /path/to/project --open

Flags:
  --agents-home PATH   shared home, default ~/.agents
  --config PATH       user config JSON overriding embedded presets;
                       empty disables the overlay
  --tools LIST         all,stable,manual,experimental or comma-separated agent names
  --dry-run           print actions without writing
  --force             replace existing files during init
  --copy              copy instead of symlink
  --no-mcp            skip MCP config
  --no-registry       skip skills registry installation

Preview flags:
  --project PATH      project root to inspect, default current directory
  --docs-dir PATH     docs directory, default docs
  --addr HOST:PORT    local server address, default 127.0.0.1:0 (auto-pick port)
  --open              open browser after the server starts

Search flags:
  --project PATH      project root to inspect, default current directory
  --docs-dir PATH     docs directory, default docs
  --addr HOST:PORT    local server address, default 127.0.0.1:0 (auto-pick port)
  --out PATH          generated launcher HTML path, default ./ns-workspace-search.html
  --no-open           write the launcher without opening the browser

Graph flags:
  --project PATH      project root to inspect, default current directory
  --docs-dir PATH     docs directory, default docs
  --query TEXT        run a Search/Code Graph query
  --limit N           maximum results per search panel, default 8
  --keyword-op OP     keyword operator for comma-separated query terms: sum or difference
  --ensure-lsp        ensure missing LSP servers before querying, default true
  --no-ensure-lsp     skip automatic LSP install before querying
  --json              print query results as JSON

LSP commands:
  list                show supported language servers and install status
  install LANG        install a supported language server or auto-detected missing servers`)
}
