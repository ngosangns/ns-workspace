package kbmcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
)

// --- Run / NewServer -----------------------------------------------------

func TestNewServerFields(t *testing.T) {
	s := NewServer("/p", "d")
	if s.projectRoot != "/p" || s.docsDir != "d" {
		t.Fatalf("NewServer fields: %+v", s)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	if err := Run([]string{"-h"}); err != nil {
		t.Fatalf("Run(-h) error = %v, want nil", err)
	}
}

func TestRun_LongHelp(t *testing.T) {
	if err := Run([]string{"--help"}); err != nil {
		t.Fatalf("Run(--help) error = %v, want nil", err)
	}
}

func TestRun_BadFlag(t *testing.T) {
	if err := Run([]string{"-undefined-flag"}); err == nil {
		t.Fatal("Run with unknown flag expected error, got nil")
	}
}

func TestRun_NoSubcommand(t *testing.T) {
	if err := Run([]string{}); err != nil {
		t.Fatalf("Run with no subcommand error = %v, want nil", err)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	if err := Run([]string{"no-such-command"}); err == nil {
		t.Fatal("Run with unknown subcommand expected error, got nil")
	}
}

func TestRun_GetwdError(t *testing.T) {
	origGetwd := getwdFn
	defer func() { getwdFn = origGetwd }()
	getwdFn = func() (string, error) { return "", errors.New("synthetic getwd failure") }

	if err := Run([]string{"list-docs"}); err == nil || !strings.Contains(err.Error(), "synthetic getwd failure") {
		t.Fatalf("Run(getwd-err) = %v, want synthetic getwd failure", err)
	}
}

// --- list-docs command ---------------------------------------------------

func TestRun_ListDocs(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = Run([]string{"--project", projectRoot, "--docs", docsDir, "list-docs"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run list-docs error: %v", err)
	}

	var out bytes.Buffer
	if _, readErr := out.ReadFrom(r); readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}

	var result listDocsResult
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("decode list-docs output: %v\noutput: %s", jsonErr, out.String())
	}
	if result.Count != 4 {
		t.Fatalf("list-docs count = %d, want 4", result.Count)
	}
}

func TestRun_ListDocs_WithFilters(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Run([]string{"--project", projectRoot, "--docs", docsDir, "list-docs", "--type", "module"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run list-docs --type error: %v", err)
	}

	var out bytes.Buffer
	out.ReadFrom(r)

	var result listDocsResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Count != 1 || result.Docs[0].ID != "alpha.md" {
		t.Fatalf("filtered result = %+v, want only alpha.md", result)
	}
}

// --- lookup-doc command --------------------------------------------------

func TestRun_LookupDoc(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Run([]string{"--project", projectRoot, "--docs", docsDir, "lookup-doc", "--id", "alpha.md"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run lookup-doc error: %v", err)
	}

	var out bytes.Buffer
	out.ReadFrom(r)

	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode lookup-doc output: %v", err)
	}
	if doc["id"] != "alpha.md" {
		t.Fatalf("lookup-doc id = %v, want alpha.md", doc["id"])
	}
}

func TestRun_LookupDoc_MissingID(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	if err := Run([]string{"--project", projectRoot, "--docs", docsDir, "lookup-doc", "--id", "missing.md"}); err == nil {
		t.Fatal("lookup-doc missing id expected error")
	}
}

// --- search-docs command -------------------------------------------------

func TestRun_SearchDocs(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Run([]string{"--project", projectRoot, "--docs", docsDir, "search-docs", "--query", "preview"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run search-docs error: %v", err)
	}

	var out bytes.Buffer
	out.ReadFrom(r)

	var result any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode search-docs output: %v", err)
	}
}

// --- Flag parsing sanity -------------------------------------------------

func TestRun_FlagParsingViaFlagSet(t *testing.T) {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	projectRoot := "/some/root"
	docsDir := "docs"
	fs.StringVar(&projectRoot, "project", projectRoot, "")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "")
	fs.StringVar(&docsDir, "docs", docsDir, "")

	if err := fs.Parse([]string{"--project", "/p", "--docs", "d2"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if projectRoot != "/p" || docsDir != "d2" {
		t.Fatalf("parsed = (%q,%q)", projectRoot, docsDir)
	}
}

