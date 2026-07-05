// Package kbmcp implements command-line access to a project's docs knowledge
// base. It is intentionally not a long-running MCP server: each invocation runs
// one command, prints JSON to stdout, and exits. This keeps the blast radius
// small and avoids a persistent process holding files open.
//
// Commands:
//   - list-docs [--type T] [--tag G]
//   - lookup-doc --id ID
//   - search-docs --query Q [--limit N]
//
// The concrete tool handlers live in tools.go and are reused by the CLI
// dispatcher.
package kbmcp

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Server holds the project root and docs directory so every command operates on
// the same knowledge base. It no longer represents a persistent stdio server;
// it is just the execution context for one-shot commands.
type Server struct {
	projectRoot string
	docsDir     string
}

// NewServer builds a Server bound to the given project root and docs directory.
func NewServer(projectRoot, docsDir string) *Server {
	return &Server{
		projectRoot: projectRoot,
		docsDir:     docsDir,
	}
}

// getwdFn is a test seam for os.Getwd.
var getwdFn = os.Getwd

// Run parses global flags (--project, --docs) and dispatches to a subcommand.
// Each subcommand runs once, prints JSON to stdout, and exits. No subcommand
// starts a persistent server.
func Run(args []string) error {
	cwd, err := getwdFn()
	if err != nil {
		return err
	}
	projectRoot := cwd
	docsDir := "docs"

	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.StringVar(&projectRoot, "project", projectRoot, "project root to inspect")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&docsDir, "docs", docsDir, "alias for --docs-dir")
	fs.Usage = func() { printUsage(fs.Output()) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		printUsage(os.Stdout)
		return nil
	}

	server := NewServer(projectRoot, docsDir)

	switch remaining[0] {
	case "list-docs":
		return runListDocs(server, remaining[1:])
	case "lookup-doc":
		return runLookupDoc(server, remaining[1:])
	case "search-docs":
		return runSearchDocs(server, remaining[1:])
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown mcp subcommand %q", remaining[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  mcp [global-flags] <command> [command-flags]

Commands:
  list-docs [--type T] [--tag G]          list docs as JSON
  lookup-doc --id ID                       get a doc by id as JSON
  search-docs --query Q [--limit N]        search docs as JSON

Global flags (must come before the command):
  --project PATH   project root, default current directory
  --docs PATH      docs directory, default docs`)
}

func runListDocs(s *Server, args []string) error {
	fs := flag.NewFlagSet("list-docs", flag.ContinueOnError)
	var in listDocsArgs
	fs.StringVar(&in.Type, "type", "", "filter by doc type")
	fs.StringVar(&in.Tag, "tag", "", "filter by tag")
	if err := fs.Parse(args); err != nil {
		return err
	}

	raw, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal list-docs args: %w", err)
	}

	result, err := s.handleListDocs(raw)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func runLookupDoc(s *Server, args []string) error {
	fs := flag.NewFlagSet("lookup-doc", flag.ContinueOnError)
	var in lookupArgs
	fs.StringVar(&in.ID, "id", "", "doc id relative to docs root")
	if err := fs.Parse(args); err != nil {
		return err
	}

	raw, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal lookup-doc args: %w", err)
	}

	result, err := s.handleLookupDoc(raw)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func runSearchDocs(s *Server, args []string) error {
	fs := flag.NewFlagSet("search-docs", flag.ContinueOnError)
	var in searchArgs
	fs.StringVar(&in.Query, "query", "", "search query")
	fs.IntVar(&in.Limit, "limit", 0, "maximum results")
	if err := fs.Parse(args); err != nil {
		return err
	}

	raw, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal search-docs args: %w", err)
	}

	result, err := s.handleSearchDocs(context.Background(), raw)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
