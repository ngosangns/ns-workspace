package preview

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RunKB dispatches the `kb` knowledge-base subcommands. These operate on the
// docs bundle using the same scan/path helpers as preview/export, applying Open
// Knowledge Format (OKF) conventions: `validate` checks conformance (SPEC §9)
// and `index` regenerates per-directory `index.md` progressive-disclosure files
// (SPEC §6).
func RunKB(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kb requires a subcommand: validate | index")
	}
	sub := args[0]
	switch sub {
	case "validate":
		return runKBValidate(args[1:])
	case "index":
		return runKBIndex(args[1:])
	case "-h", "--help", "help":
		fmt.Println("kb subcommands:\n  validate   check OKF conformance of docs\n  index      regenerate per-directory index.md listings")
		return nil
	default:
		return fmt.Errorf("unknown kb subcommand %q (want: validate | index)", sub)
	}
}

// okfReservedFiles are the OKF reserved filenames that are exempt from the
// frontmatter/type requirement (they have defined structural meaning instead).
var okfReservedFiles = map[string]bool{
	"index.md": true,
	"log.md":   true,
}

// okfRecommendedKeys are the OKF recommended frontmatter keys reported as
// warnings (not hard conformance failures) when absent.
var okfRecommendedKeys = []string{"title", "description", "timestamp"}

// readFrontmatterMap extracts and decodes a leading YAML frontmatter block into
// a generic map. It mirrors OKF's OKFDocument.parse: returns hasBlock=false when
// the file does not begin with a `---` delimiter; returns an error when the
// block is unterminated, the YAML is invalid, or it is not a mapping.
func readFrontmatterMap(raw string) (fm map[string]any, hasBlock bool, err error) {
	text := strings.TrimLeft(raw, "\ufeff")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, false, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, true, fmt.Errorf("unterminated YAML frontmatter block")
	}
	block := strings.Join(lines[1:end], "\n")
	var decoded map[string]any
	if e := yaml.Unmarshal([]byte(block), &decoded); e != nil {
		return nil, true, fmt.Errorf("invalid YAML in frontmatter: %w", e)
	}
	if decoded == nil {
		decoded = map[string]any{}
	}
	return decoded, true, nil
}

// fmString reads a frontmatter value as a trimmed string ("" when absent or
// nil). Non-string scalars are formatted: YAML auto-types an unquoted ISO 8601
// datetime (e.g. `timestamp: 2026-06-23T00:00:00Z`) into a time.Time, so it is
// rendered back to RFC3339; other scalars use their default formatting.
func fmString(fm map[string]any, key string) string {
	v, ok := fm[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

// ---------------------------------------------------------------------------
// kb validate — OKF conformance (SPEC §9)
// ---------------------------------------------------------------------------

// kbValidateIssue is a single conformance finding for one document.
type kbValidateIssue struct {
	Path     string   `json:"path"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// kbValidateReport is the full conformance result over a docs bundle.
type kbValidateReport struct {
	DocsRoot   string            `json:"docsRoot"`
	Checked    int               `json:"checked"`
	Conformant bool              `json:"conformant"`
	Issues     []kbValidateIssue `json:"issues,omitempty"`
}

func runKBValidate(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot := cwd
	docsDir := "docs"
	var asJSON, strict bool

	fs := flag.NewFlagSet("kb validate", flag.ContinueOnError)
	fs.StringVar(&projectRoot, "project", projectRoot, "project root to validate")
	fs.StringVar(&docsDir, "docs", docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "alias for --docs")
	fs.BoolVar(&asJSON, "json", false, "print the report as JSON")
	fs.BoolVar(&strict, "strict", false, "treat recommended-key warnings as failures")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	root := docsRoot(normalizePreviewProjectRoot(projectRoot), docsDir)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("kb validate: docs directory not found: %s", root)
	}

	report, err := validateOKFBundle(root, strict)
	if err != nil {
		return err
	}

	if asJSON {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
	} else {
		printKBValidateText(report)
	}
	if !report.Conformant {
		return fmt.Errorf("kb validate: %d document(s) not OKF-conformant", countErrorDocs(report))
	}
	return nil
}

// validateOKFBundle walks every markdown file under root and applies the OKF
// conformance rules: each non-reserved `.md` file must begin with a parseable
// YAML frontmatter mapping that carries a non-empty `type` (SPEC §9). Missing
// recommended keys (title/description/timestamp) are warnings; with strict they
// are promoted to errors.
func validateOKFBundle(root string, strict bool) (kbValidateReport, error) {
	report := kbValidateReport{DocsRoot: root, Conformant: true}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		report.Checked++

		if okfReservedFiles[strings.ToLower(d.Name())] {
			return nil // reserved files are exempt from the frontmatter/type rule
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			report.add(rel, []string{fmt.Sprintf("read failed: %v", readErr)}, nil)
			return nil
		}

		fm, hasBlock, fmErr := readFrontmatterMap(string(raw))
		switch {
		case !hasBlock:
			report.add(rel, []string{"missing YAML frontmatter block (OKF requires `type`)"}, nil)
			return nil
		case fmErr != nil:
			report.add(rel, []string{fmErr.Error()}, nil)
			return nil
		}

		var errs, warns []string
		if fmString(fm, "type") == "" {
			errs = append(errs, "missing required frontmatter key `type`")
		}
		for _, key := range okfRecommendedKeys {
			if fmString(fm, key) == "" {
				warns = append(warns, fmt.Sprintf("missing recommended key `%s`", key))
			}
		}
		if strict {
			errs = append(errs, warns...)
			warns = nil
		}
		report.add(rel, errs, warns)
		return nil
	})
	if walkErr != nil {
		return report, fmt.Errorf("kb validate: %w", walkErr)
	}
	sort.Slice(report.Issues, func(i, j int) bool { return report.Issues[i].Path < report.Issues[j].Path })
	return report, nil
}

// add records an issue and flips conformance off when any hard error is present.
func (r *kbValidateReport) add(path string, errs, warns []string) {
	if len(errs) == 0 && len(warns) == 0 {
		return
	}
	r.Issues = append(r.Issues, kbValidateIssue{Path: path, Errors: errs, Warnings: warns})
	if len(errs) > 0 {
		r.Conformant = false
	}
}

func countErrorDocs(report kbValidateReport) int {
	n := 0
	for _, issue := range report.Issues {
		if len(issue.Errors) > 0 {
			n++
		}
	}
	return n
}

func printKBValidateText(report kbValidateReport) {
	fmt.Printf("kb validate: %s\n", report.DocsRoot)
	fmt.Printf("checked: %d document(s)\n", report.Checked)
	for _, issue := range report.Issues {
		for _, e := range issue.Errors {
			fmt.Printf("  ERROR  %s: %s\n", issue.Path, e)
		}
		for _, w := range issue.Warnings {
			fmt.Printf("  warn   %s: %s\n", issue.Path, w)
		}
	}
	if report.Conformant {
		fmt.Println("result: OKF-conformant ✓")
	} else {
		fmt.Printf("result: NOT conformant (%d document(s) with errors)\n", countErrorDocs(report))
	}
}

// ---------------------------------------------------------------------------
// kb index — regenerate per-directory index.md (SPEC §6, port of index.py)
// ---------------------------------------------------------------------------

// indexEntry is one row in a generated index.md listing.
type indexEntry struct {
	group string // grouping heading (frontmatter type, or "Subdirectories")
	title string
	link  string // relative link from the index.md's directory
	desc  string
}

func runKBIndex(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot := cwd
	docsDir := "docs"
	var dryRun bool

	fs := flag.NewFlagSet("kb index", flag.ContinueOnError)
	fs.StringVar(&projectRoot, "project", projectRoot, "project root")
	fs.StringVar(&docsDir, "docs", docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "alias for --docs")
	fs.BoolVar(&dryRun, "dry-run", false, "print the directories that would be written without writing")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	root := docsRoot(normalizePreviewProjectRoot(projectRoot), docsDir)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("kb index: docs directory not found: %s", root)
	}

	written, err := regenerateIndexes(root, dryRun)
	if err != nil {
		return err
	}
	verb := "wrote"
	if dryRun {
		verb = "would write"
	}
	for _, p := range written {
		rel, _ := filepath.Rel(root, p)
		fmt.Printf("kb index: %s %s\n", verb, filepath.ToSlash(rel))
	}
	fmt.Printf("kb index: %d index file(s) %s\n", len(written), verb)
	return nil
}

// regenerateIndexes walks the bundle and writes an `index.md` for every
// directory that contains documents, grouping entries by frontmatter `type`.
// It ports the reference index.py: deepest directories are processed first so a
// parent's listing can reuse a single-child subdirectory's description. The
// reference's LLM-based description synthesis is replaced with a dependency-free
// local heuristic (deriveDirDescription). Returns the list of index paths.
func regenerateIndexes(root string, dryRun bool) ([]string, error) {
	dirs, err := directoriesToIndex(root)
	if err != nil {
		return nil, err
	}
	// Deepest first (more path parts ranks earlier), then by path for stability.
	sort.Slice(dirs, func(i, j int) bool {
		di := strings.Count(strings.TrimPrefix(dirs[i], root), string(os.PathSeparator))
		dj := strings.Count(strings.TrimPrefix(dirs[j], root), string(os.PathSeparator))
		if di != dj {
			return di > dj
		}
		return dirs[i] < dirs[j]
	})

	dirDesc := map[string]string{}
	indexed := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		indexed[filepath.Clean(dir)] = true
	}
	var written []string
	for _, dir := range dirs {
		entries, err := collectIndexEntries(dir, indexed, dirDesc)
		if err != nil {
			return written, err
		}
		if len(entries) == 0 {
			continue
		}
		content := buildIndexText(dir, root, entries)
		indexPath := filepath.Join(dir, "index.md")
		if !dryRun {
			if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
				return written, fmt.Errorf("kb index: write %s: %w", indexPath, err)
			}
		}
		written = append(written, indexPath)
		if dir != root {
			dirDesc[filepath.Clean(dir)] = deriveDirDescription(entries)
		}
	}
	sort.Strings(written)
	return written, nil
}

// directoriesToIndex returns every directory from each markdown file up to the
// bundle root (inclusive), mirroring OKF index.py: ancestor directories are
// indexed too so a parent (even one with no direct docs) still lists its
// subdirectories for progressive disclosure.
func directoriesToIndex(root string) ([]string, error) {
	root = filepath.Clean(root)
	set := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		if strings.EqualFold(d.Name(), "index.md") {
			return nil
		}
		cur := filepath.Dir(path)
		for {
			set[cur] = true
			if cur == root {
				break
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(set))
	for dir := range set {
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

// collectIndexEntries builds the listing entries for one directory: every
// non-index markdown file (by frontmatter type/title/description) plus every
// immediate subdirectory that will itself get an index.md (per the planned
// `indexed` set), linking to it with a previously derived description when
// available.
func collectIndexEntries(dir string, indexed map[string]bool, dirDesc map[string]string) ([]indexEntry, error) {
	children, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var entries []indexEntry
	for _, child := range children {
		name := child.Name()
		if child.IsDir() {
			sub := filepath.Clean(filepath.Join(dir, name))
			if indexed[sub] {
				entries = append(entries, indexEntry{
					group: "Subdirectories",
					title: name,
					link:  name + "/index.md",
					desc:  dirDesc[sub],
				})
			}
			continue
		}
		if strings.ToLower(filepath.Ext(name)) != ".md" || strings.EqualFold(name, "index.md") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			continue
		}
		fm, _, _ := readFrontmatterMap(string(raw))
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		title := fmString(fm, "title")
		if title == "" {
			title = stem
		}
		entries = append(entries, indexEntry{
			group: fmString(fm, "type"),
			title: title,
			link:  name,
			desc:  fmString(fm, "description"),
		})
	}
	return entries, nil
}

// buildIndexText renders the index.md content for a directory. To stay clean
// under this repo's markdownlint (single H1 per file), the directory name is the
// single H1 and each type group is an H2 — semantically equivalent to OKF's flat
// multi-section listing for progressive disclosure.
func buildIndexText(dir, root string, entries []indexEntry) string {
	groups := map[string][]indexEntry{}
	var order []string
	for _, e := range entries {
		g := e.group
		if g == "" {
			g = "Other"
		}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], e)
	}
	sort.Strings(order)

	title := indexDirTitle(dir, root)
	var b strings.Builder
	b.WriteString("# " + title + "\n")
	for _, g := range order {
		list := groups[g]
		sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].title) < strings.ToLower(list[j].title) })
		b.WriteString("\n## " + g + "\n\n")
		for _, e := range list {
			suffix := ""
			if e.desc != "" {
				suffix = " - " + e.desc
			}
			b.WriteString(fmt.Sprintf("* [%s](%s)%s\n", e.title, e.link, suffix))
		}
	}
	return b.String()
}

// indexDirTitle derives a human title for a directory's index ("Docs" for the
// bundle root, otherwise the directory's base name).
func indexDirTitle(dir, root string) string {
	if filepath.Clean(dir) == filepath.Clean(root) {
		return "Docs Index"
	}
	return strings.Title(filepath.Base(dir)) + " Index" //nolint:staticcheck // ASCII dir names
}

// deriveDirDescription is the dependency-free replacement for OKF's LLM-based
// synthesize_description: a single-doc directory borrows that doc's description;
// otherwise it reports the document count.
func deriveDirDescription(entries []indexEntry) string {
	docs := 0
	var only string
	for _, e := range entries {
		if e.group == "Subdirectories" {
			continue
		}
		docs++
		only = e.desc
	}
	if docs == 1 && only != "" {
		return only
	}
	if docs > 0 {
		return fmt.Sprintf("%d documents", docs)
	}
	return ""
}
