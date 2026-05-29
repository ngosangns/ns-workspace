package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
	"github.com/ngosangns/ns-workspace/internal/preview"
)

//go:embed presets/agents presets/mcp presets/opencode presets/registry presets/settings presets/skills/* presets/subagents
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
	switch cmd {
	case "init", "update", "status", "doctor", "registry", "agents", "catalog", "preview", "search", "graph", "lsp":
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", cmd)
	}
	if cmd == "preview" {
		return preview.Run(args[1:])
	}
	if cmd == "search" {
		return preview.RunSearch(args[1:])
	}
	if cmd == "graph" {
		return preview.RunGraph(args[1:])
	}
	if cmd == "lsp" {
		return preview.RunLSP(args[1:])
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	homeDefault, err := agentsync.DefaultAgentsDir()
	if err != nil {
		return err
	}
	opt := agentsync.Options{Command: cmd, AgentsDir: homeDefault}
	tools := fs.String("tools", "all", "comma-separated tools: all,stable,manual,experimental,claude,opencode,grok,kimi,kiro,kiro-cli,qwen,gemini,codex,cline,windsurf,aider,cursor,github-copilot,jetbrains,antigravity,trae,roo")
	fs.StringVar(&opt.AgentsDir, "agents-home", homeDefault, "shared agents home")
	fs.BoolVar(&opt.DryRun, "dry-run", false, "show planned writes without changing files")
	fs.BoolVar(&opt.Yes, "yes", false, "skip interactive confirmations")
	fs.BoolVar(&opt.Force, "force", false, "replace existing files during init")
	fs.BoolVar(&opt.CopyMode, "copy", false, "copy files instead of creating symlinks")
	fs.BoolVar(&opt.NoMCP, "no-mcp", false, "skip MCP configuration")
	fs.BoolVar(&opt.NoRegistry, "no-registry", false, "skip skills registry installation")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	opt.ToolFilter = agentsync.ParseTools(*tools)

	manager := agentsync.Manager{Presets: presetFS}
	switch cmd {
	case "init":
		return manager.Apply(opt, false)
	case "update":
		return manager.Apply(opt, true)
	case "status":
		return manager.Status(opt)
	case "doctor":
		return manager.Doctor(opt)
	case "registry":
		return manager.InstallRegistrySkills(opt)
	case "agents", "catalog":
		return manager.Catalog(opt)
	}
	return nil
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
  --ensure-lsp        install missing LSP servers for detected project languages before querying
  --json              print query results as JSON

LSP commands:
  list                show supported language servers and install status
  install LANG        install a supported language server or auto-detected missing servers`)
}
