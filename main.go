package main

import (
	"embed"
	"fmt"
	"io"
	"os"

	synccli "github.com/ngosangns/ns-workspace/internal/cli"
	"github.com/ngosangns/ns-workspace/internal/graphquery"
	"github.com/ngosangns/ns-workspace/internal/kbmcp"
	"github.com/ngosangns/ns-workspace/internal/portal"
	"github.com/ngosangns/ns-workspace/internal/preview"
	"github.com/ngosangns/ns-workspace/internal/setup"
)

//go:embed presets/agents presets/mcp presets/minimax presets/opencode presets/registry presets/settings presets/adapters presets/manifest.json presets/skills/* presets/subagents
var presetFS embed.FS

// osExit is overridable from tests so they can intercept the exit path.
var osExit = os.Exit

// stderrWriter is overridable from tests so they can capture the error message.
var stderrWriter io.Writer = os.Stderr

func main() {
	osExit(runMain())
}

// runMain is the testable entry point. It returns the process exit code
// (0 on success, 1 on error) and writes the error to stderrWriter.
func runMain() int {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintf(stderrWriter, "error: %v\n", err)
		return 1
	}
	return 0
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
	if synccli.IsHarnessCommand(cmd) {
		return synccli.RunHarness(args[1:])
	}
	switch cmd {
	case "preview":
		return preview.Run(args[1:])
	case "search":
		return preview.RunSearch(args[1:])
	case "export":
		return preview.RunExport(args[1:])
	case "graph":
		return preview.RunGraph(args[1:])
	case "portal":
		return portal.Run(args[1:], presetFS)
	case "mcp":
		return kbmcp.Run(args[1:])
	case "kb":
		return preview.RunKB(args[1:])
	case "setup":
		return setup.Run(args[1:])
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
  go run github.com/ngosangns/ns-workspace@latest portal [flags]
  go run github.com/ngosangns/ns-workspace@latest harness <list|run|eval|status|resume|stop> [flags]
  go run github.com/ngosangns/ns-workspace@latest preview [flags]
  go run github.com/ngosangns/ns-workspace@latest search [flags]
  go run github.com/ngosangns/ns-workspace@latest export [flags]
  go run github.com/ngosangns/ns-workspace@latest graph [flags]
  go run github.com/ngosangns/ns-workspace@latest mcp <command> [flags]
  go run github.com/ngosangns/ns-workspace@latest kb <validate|index> [flags]
  go run github.com/ngosangns/ns-workspace@latest setup [flags]
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

Portal flags:
  --addr HOST:PORT    local server address, default 127.0.0.1:0 (auto-pick port)
  --open              open browser after the server starts
  --agents-home PATH  shared agents home

Harness flags:
  --project PATH      project root to inspect, default current directory
  --task ID           task id for run/eval/status/resume/stop
  --dry-run           show planned actions without running

Preview flags:
  --project PATH      project root to inspect, default current directory
  --docs-dir PATH     docs directory, default docs
  --addr HOST:PORT    local server address (only the port is used; Quartz binds to 127.0.0.1)
  --open              open browser after the server starts
  --quartz-dir PATH   local Quartz checkout (with package.json); default clones to user cache

Search flags:
  --project PATH      project root to inspect, default current directory
  --docs-dir PATH     docs directory, default docs
  --addr HOST:PORT    local server address, default 127.0.0.1:0 (auto-pick port)
  --out PATH          generated launcher HTML path, default ./ns-workspace-search.html
  --no-open           write the launcher without opening the browser

Export flags:
  --project PATH      project root to export, default current directory
  --docs PATH         docs directory, default docs
  --out PATH          output HTML file path, default ./ns-workspace-kb.html
  --name NAME         display name in the viewer header, default project name
  --no-graph          export documents only, without the relationship edges
  --inline-assets     inline render libraries for fully offline output, default true
  --open              open the generated file after writing

MCP usage:
  mcp [global-flags] <command> [command-flags]

MCP commands:
  list-docs [--type T] [--tag G]         list docs as JSON
  lookup-doc --id ID                     get a doc by id as JSON
  search-docs --query Q [--limit N]      search docs as JSON

MCP global flags (before the command):
  --project PATH      project root, default current directory
  --docs PATH         docs directory, default docs

KB commands:
  validate            check OKF conformance of docs (frontmatter + non-empty type)
  index               regenerate per-directory index.md progressive-disclosure listings

KB flags:
  --project PATH      project root, default current directory
  --docs PATH         docs directory, default docs
  --json              (validate) print the conformance report as JSON
  --strict            (validate) treat recommended-key warnings as failures
  --dry-run           (index) print directories that would be written without writing

Setup flags:
  --target PATH       directory to write Taskfile.yml, default current directory
  --dry-run           print planned Taskfile.yml on stdout instead of writing
  --force             replace existing Taskfile.yml instead of merging

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
