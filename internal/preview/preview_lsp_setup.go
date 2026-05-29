package preview

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const lspCacheEnv = "NS_WORKSPACE_LSP_CACHE"

type lspInstallKind string

const (
	lspInstallGo     lspInstallKind = "go"
	lspInstallNPM    lspInstallKind = "npm"
	lspInstallManual lspInstallKind = "manual"
)

type lspSymbolMode string

const (
	lspSymbolModeCallable lspSymbolMode = "callable"
	lspSymbolModeDocument lspSymbolMode = "document"
	lspSymbolModeSelector lspSymbolMode = "selector"
)

type lspPrerequisite struct {
	Name        string
	Command     string
	Args        []string
	InstallHint string
	MinMajor    int
}

type lspInstallSpec struct {
	ID            string
	Name          string
	Aliases       []string
	ServerID      string
	Extensions    []string
	Command       string
	Args          []string
	CheckArgs     []string
	Prerequisites []lspPrerequisite
	InstallKind   lspInstallKind
	GoPackage     string
	NPMPackages   []string
	ManualInstall string
}

type lspLanguageSpec struct {
	ID         string
	Name       string
	Aliases    []string
	Extensions []string
	ServerID   string
	LanguageID string
	SymbolMode lspSymbolMode
}

type lspStatusRow struct {
	ID             string `json:"id"`
	ServerID       string `json:"serverId,omitempty"`
	Name           string `json:"name"`
	Detected       bool   `json:"detected"`
	Status         string `json:"status"`
	Binary         string `json:"binary"`
	Source         string `json:"source,omitempty"`
	Path           string `json:"path,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
}

type lspInstallResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

type lspEnsureOptions struct {
	Force    bool
	DryRun   bool
	Progress io.Writer
}

var lspInstallLocks sync.Map

func RunLSP(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printLSPUsage()
		return nil
	}
	switch args[0] {
	case "list":
		return runLSPList(args[1:], os.Stdout)
	case "install":
		return runLSPInstall(args[1:], os.Stdout)
	default:
		printLSPUsage()
		return fmt.Errorf("unknown lsp command %q", args[0])
	}
}

func runLSPList(args []string, out io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot := cwd
	docsDir := "docs"
	jsonOutput := false
	fs := flag.NewFlagSet("lsp list", flag.ContinueOnError)
	fs.StringVar(&projectRoot, "project", projectRoot, "project root to inspect")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "docs directory relative to project root, or absolute path")
	fs.BoolVar(&jsonOutput, "json", false, "print LSP status as JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	projectRoot = normalizePreviewProjectRoot(projectRoot)
	rows := lspListStatus(projectRoot, docsDir)
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(rows)
	}
	if _, err := fmt.Fprintln(out, "Language\tDetected\tStatus\tBinary\tInstall"); err != nil {
		return err
	}
	for _, row := range rows {
		detected := "no"
		if row.Detected {
			detected = "yes"
		}
		binary := "-"
		if row.Path != "" {
			binary = row.Path
			if row.Source != "" {
				binary += " (" + row.Source + ")"
			}
		}
		install := row.InstallCommand
		if install == "" {
			install = "-"
		}
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", row.ID, detected, row.Status, binary, install); err != nil {
			return err
		}
	}
	return nil
}

func runLSPInstall(args []string, out io.Writer) error {
	language, flags := splitLSPInstallArgs(args)
	if language == "" {
		return fmt.Errorf("lsp install requires <language|auto>")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectRoot := cwd
	docsDir := "docs"
	force := false
	dryRun := false
	jsonOutput := false
	fs := flag.NewFlagSet("lsp install", flag.ContinueOnError)
	fs.StringVar(&projectRoot, "project", projectRoot, "project root to inspect")
	fs.StringVar(&docsDir, "docs-dir", docsDir, "docs directory relative to project root, or absolute path")
	fs.BoolVar(&force, "force", false, "reinstall even when a binary is already available")
	fs.BoolVar(&dryRun, "dry-run", false, "show planned installs without running them")
	fs.BoolVar(&jsonOutput, "json", false, "print install results as JSON")
	if err := fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	projectRoot = normalizePreviewProjectRoot(projectRoot)
	opts := lspEnsureOptions{Force: force, DryRun: dryRun, Progress: os.Stderr}

	var results []lspInstallResult
	if language == "auto" {
		results = installProjectLSPs(context.Background(), projectRoot, docsDir, opts)
	} else {
		spec, ok := lspInstallSpecByID(language)
		if !ok {
			return fmt.Errorf("unsupported LSP language %q; supported: %s", language, strings.Join(lspSupportedIDs(), ", "))
		}
		results = []lspInstallResult{installLSP(context.Background(), spec, projectRoot, opts)}
	}
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(results); err != nil {
			return err
		}
	} else {
		for _, result := range results {
			line := fmt.Sprintf("%s: %s", result.ID, result.Status)
			if result.Path != "" {
				line += " " + result.Path
			}
			if result.Message != "" {
				line += " - " + result.Message
			}
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
	}
	if language != "auto" {
		for _, result := range results {
			if result.Status == "failed" || result.Status == "manual" {
				return fmt.Errorf("install %s failed: %s", result.ID, result.Message)
			}
		}
	}
	return nil
}

func splitLSPInstallArgs(args []string) (string, []string) {
	language := ""
	flags := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--project" || arg == "--docs-dir":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(arg, "--project=") || strings.HasPrefix(arg, "--docs-dir="):
			flags = append(flags, arg)
		case arg == "--force" || arg == "--dry-run" || arg == "--json":
			flags = append(flags, arg)
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		case language == "":
			language = arg
		default:
			flags = append(flags, arg)
		}
	}
	return language, flags
}

func printLSPUsage() {
	fmt.Println(`LSP commands:
  lsp list [--project PATH] [--docs-dir docs] [--json]
  lsp install <language|auto> [--project PATH] [--docs-dir docs] [--force] [--dry-run] [--json]

Supported languages:
  html, css, scss, javascript, typescript, go/golang, kotlin`)
}

func lspInstallSpecs() []lspInstallSpec {
	return []lspInstallSpec{
		{
			ID:         "go",
			Name:       "Go/Golang",
			Aliases:    []string{"golang"},
			ServerID:   "go",
			Extensions: []string{".go"},
			Command:    "gopls",
			Args:       []string{"serve"},
			CheckArgs:  []string{"version"},
			Prerequisites: []lspPrerequisite{{
				Name:        "Go",
				Command:     "go",
				Args:        []string{"version"},
				InstallHint: "Install Go from https://go.dev/dl/",
			}},
			InstallKind: lspInstallGo,
			GoPackage:   "golang.org/x/tools/gopls@latest",
		},
		{
			ID:         "typescript",
			Name:       "TypeScript/JavaScript",
			Aliases:    []string{"ts", "javascript", "js"},
			ServerID:   "typescript",
			Extensions: []string{".ts", ".tsx", ".js", ".jsx", ".cjs", ".mjs"},
			Command:    "typescript-language-server",
			Args:       []string{"--stdio"},
			CheckArgs:  []string{"--version"},
			Prerequisites: []lspPrerequisite{
				{Name: "Node.js 18+", Command: "node", Args: []string{"--version"}, InstallHint: "Install Node.js 18+ from https://nodejs.org/", MinMajor: 18},
				{Name: "npm", Command: "npm", Args: []string{"--version"}, InstallHint: "Install npm with Node.js."},
			},
			InstallKind: lspInstallNPM,
			NPMPackages: []string{"typescript-language-server", "typescript"},
		},
		{
			ID:         "html",
			Name:       "HTML",
			ServerID:   "html",
			Extensions: []string{".html", ".htm"},
			Command:    "vscode-html-language-server",
			Args:       []string{"--stdio"},
			CheckArgs:  []string{"--version"},
			Prerequisites: []lspPrerequisite{
				{Name: "Node.js 18+", Command: "node", Args: []string{"--version"}, InstallHint: "Install Node.js 18+ from https://nodejs.org/", MinMajor: 18},
				{Name: "npm", Command: "npm", Args: []string{"--version"}, InstallHint: "Install npm with Node.js."},
			},
			InstallKind: lspInstallNPM,
			NPMPackages: []string{"vscode-langservers-extracted"},
		},
		{
			ID:         "css",
			Name:       "CSS/SCSS/Sass",
			Aliases:    []string{"scss", "sass"},
			ServerID:   "css",
			Extensions: []string{".css", ".scss", ".sass"},
			Command:    "vscode-css-language-server",
			Args:       []string{"--stdio"},
			CheckArgs:  []string{"--version"},
			Prerequisites: []lspPrerequisite{
				{Name: "Node.js 18+", Command: "node", Args: []string{"--version"}, InstallHint: "Install Node.js 18+ from https://nodejs.org/", MinMajor: 18},
				{Name: "npm", Command: "npm", Args: []string{"--version"}, InstallHint: "Install npm with Node.js."},
			},
			InstallKind: lspInstallNPM,
			NPMPackages: []string{"vscode-langservers-extracted"},
		},
		{
			ID:            "kotlin",
			Name:          "Kotlin",
			Aliases:       []string{"kt"},
			ServerID:      "kotlin",
			Extensions:    []string{".kt", ".kts"},
			Command:       "kotlin-lsp",
			Args:          []string{"--stdio"},
			CheckArgs:     []string{"--help"},
			InstallKind:   lspInstallManual,
			ManualInstall: "Install Kotlin LSP CLI with Homebrew: brew install JetBrains/utils/kotlin-lsp; or download the standalone zip from https://github.com/Kotlin/kotlin-lsp/releases and symlink kotlin-lsp.sh as kotlin-lsp.",
		},
	}
}

func lspLanguageSpecs() []lspLanguageSpec {
	return []lspLanguageSpec{
		{ID: "html", Name: "HTML", Extensions: []string{".html", ".htm"}, ServerID: "html", LanguageID: "html", SymbolMode: lspSymbolModeDocument},
		{ID: "css", Name: "CSS", Extensions: []string{".css"}, ServerID: "css", LanguageID: "css", SymbolMode: lspSymbolModeSelector},
		{ID: "scss", Name: "SCSS", Aliases: []string{"sass"}, Extensions: []string{".scss", ".sass"}, ServerID: "css", LanguageID: "scss", SymbolMode: lspSymbolModeSelector},
		{ID: "javascript", Name: "JavaScript", Aliases: []string{"js"}, Extensions: []string{".js", ".cjs", ".mjs"}, ServerID: "typescript", LanguageID: "javascript", SymbolMode: lspSymbolModeCallable},
		{ID: "javascript", Name: "JavaScript", Aliases: []string{"jsx"}, Extensions: []string{".jsx"}, ServerID: "typescript", LanguageID: "javascriptreact", SymbolMode: lspSymbolModeCallable},
		{ID: "typescript", Name: "TypeScript", Aliases: []string{"ts"}, Extensions: []string{".ts"}, ServerID: "typescript", LanguageID: "typescript", SymbolMode: lspSymbolModeCallable},
		{ID: "typescript", Name: "TypeScript", Aliases: []string{"tsx"}, Extensions: []string{".tsx"}, ServerID: "typescript", LanguageID: "typescriptreact", SymbolMode: lspSymbolModeCallable},
		{ID: "go", Name: "Go/Golang", Aliases: []string{"golang"}, Extensions: []string{".go"}, ServerID: "go", LanguageID: "go", SymbolMode: lspSymbolModeCallable},
		{ID: "kotlin", Name: "Kotlin", Aliases: []string{"kt"}, Extensions: []string{".kt", ".kts"}, ServerID: "kotlin", LanguageID: "kotlin", SymbolMode: lspSymbolModeCallable},
	}
}

func lspSupportedIDs() []string {
	ids := []string{}
	for _, spec := range lspInstallSpecs() {
		ids = append(ids, spec.ID)
		ids = append(ids, spec.Aliases...)
	}
	for _, lang := range lspLanguageSpecs() {
		ids = append(ids, lang.ID)
		ids = append(ids, lang.Aliases...)
	}
	sort.Strings(ids)
	return uniqueSortedStrings(ids)
}

func normalizeLSPID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func normalizedLSPIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = normalizeLSPID(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func lspInstallSpecByID(id string) (lspInstallSpec, bool) {
	id = normalizeLSPID(id)
	for _, spec := range lspInstallSpecs() {
		if normalizeLSPID(spec.ID) == id || stringInSlice(id, normalizedLSPIDs(spec.Aliases)) {
			return spec, true
		}
	}
	for _, lang := range lspLanguageSpecs() {
		if normalizeLSPID(lang.ID) == id || stringInSlice(id, normalizedLSPIDs(lang.Aliases)) {
			return lspInstallSpecByServerID(lang.ServerID)
		}
	}
	return lspInstallSpec{}, false
}

func lspInstallSpecByServerID(serverID string) (lspInstallSpec, bool) {
	serverID = normalizeLSPID(serverID)
	for _, spec := range lspInstallSpecs() {
		if normalizeLSPID(spec.ServerID) == serverID || normalizeLSPID(spec.ID) == serverID {
			return spec, true
		}
	}
	return lspInstallSpec{}, false
}

func lspListStatus(projectRoot, docsDir string) []lspStatusRow {
	detected := detectedLSPLanguages(projectRoot, docsDir)
	rows := []lspStatusRow{}
	seenRows := map[string]bool{}
	manager := newPreviewLSPManager(projectRoot)
	for _, lang := range lspLanguageSpecs() {
		if seenRows[lang.ID] {
			continue
		}
		seenRows[lang.ID] = true
		spec, ok := lspInstallSpecByServerID(lang.ServerID)
		if !ok {
			continue
		}
		path, source, err := manager.resolveCommandWithSource(spec.Command)
		row := lspStatusRow{
			ID:       lang.ID,
			ServerID: spec.ID,
			Name:     lang.Name,
			Detected: detected[lang.ID],
			Status:   "missing",
			Binary:   spec.Command,
		}
		if err == nil {
			row.Status = "installed"
			row.Source = source
			row.Path = path
		} else {
			row.InstallCommand = "go run . lsp install " + lang.ID
			if spec.InstallKind == lspInstallManual {
				row.InstallCommand = spec.ManualInstall
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func detectedLSPLanguages(projectRoot, docsDir string) map[string]bool {
	_, files, _ := lspSourceFiles(projectRoot, docsRoot(projectRoot, docsDir))
	detected := map[string]bool{}
	for _, file := range files {
		detected[file.Language.ID] = true
	}
	return detected
}

func detectedLSPInstallSpecs(projectRoot, docsDir string) map[string]bool {
	_, files, _ := lspSourceFiles(projectRoot, docsRoot(projectRoot, docsDir))
	detected := map[string]bool{}
	for _, file := range files {
		if spec, ok := lspInstallSpecByServerID(file.Language.ServerID); ok {
			detected[spec.ID] = true
		}
	}
	return detected
}

func installProjectLSPs(ctx context.Context, projectRoot, docsDir string, opts lspEnsureOptions) []lspInstallResult {
	detected := detectedLSPInstallSpecs(projectRoot, docsDir)
	if len(detected) == 0 {
		return []lspInstallResult{{ID: "auto", Name: "Auto", Status: "skipped", Message: "no supported LSP languages detected"}}
	}
	ids := make([]string, 0, len(detected))
	for id := range detected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	results := []lspInstallResult{}
	for _, id := range ids {
		spec, ok := lspInstallSpecByID(id)
		if !ok {
			continue
		}
		results = append(results, installLSP(ctx, spec, projectRoot, opts))
	}
	return results
}

func ensureProjectLSP(ctx context.Context, projectRoot, docsDir string, opts lspEnsureOptions) []string {
	results := installProjectLSPs(ctx, projectRoot, docsDir, opts)
	warnings := []string{}
	for _, result := range results {
		switch result.Status {
		case "installed":
			if opts.Progress != nil {
				fmt.Fprintf(opts.Progress, "installed %s LSP: %s\n", result.ID, result.Path)
			}
		case "failed":
			warnings = append(warnings, fmt.Sprintf("%s LSP install failed: %s", result.Name, result.Message))
		case "manual":
			warnings = append(warnings, fmt.Sprintf("%s LSP requires manual installation: %s", result.Name, result.Message))
		}
	}
	return warnings
}

func installLSP(ctx context.Context, spec lspInstallSpec, projectRoot string, opts lspEnsureOptions) lspInstallResult {
	manager := newPreviewLSPManager(projectRoot)
	if !opts.Force {
		if path, source, err := manager.resolveCommandWithSource(spec.Command); err == nil {
			return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "already-installed", Path: path, Message: "found in " + source}
		}
	}
	command := lspInstallCommand(spec)
	if opts.DryRun {
		return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "dry-run", Message: command}
	}
	if spec.InstallKind == lspInstallManual {
		return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "manual", Message: command}
	}
	lock := lspInstallMutex(spec.ID)
	lock.Lock()
	defer lock.Unlock()
	if !opts.Force {
		// Re-check after taking the process-local lock in case another ensure call just populated the cache.
		if path, source, err := manager.resolveCommandWithSource(spec.Command); err == nil {
			return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "already-installed", Path: path, Message: "found in " + source}
		}
	}
	if err := checkLSPPrerequisites(ctx, spec); err != nil {
		return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "failed", Message: err.Error()}
	}
	var path string
	var err error
	switch spec.InstallKind {
	case lspInstallGo:
		path, err = installGoLSP(ctx, spec)
	case lspInstallNPM:
		path, err = installNPMLSP(ctx, spec)
	default:
		err = fmt.Errorf("unsupported install strategy %q", spec.InstallKind)
	}
	if err != nil {
		return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "failed", Message: err.Error()}
	}
	if err := checkLSPBinary(ctx, path, spec.CheckArgs); err != nil {
		return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "failed", Message: err.Error()}
	}
	return lspInstallResult{ID: spec.ID, Name: spec.Name, Status: "installed", Path: path}
}

func lspInstallMutex(id string) *sync.Mutex {
	value, _ := lspInstallLocks.LoadOrStore(id, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func checkLSPPrerequisites(ctx context.Context, spec lspInstallSpec) error {
	for _, prereq := range spec.Prerequisites {
		out, err := exec.CommandContext(ctx, prereq.Command, prereq.Args...).CombinedOutput()
		if err != nil {
			message := fmt.Sprintf("%s prerequisite is missing: %v", prereq.Name, err)
			if prereq.InstallHint != "" {
				message += ". " + prereq.InstallHint
			}
			return errors.New(message)
		}
		if prereq.MinMajor > 0 {
			major, ok := parseMajorVersion(string(out))
			if !ok || major < prereq.MinMajor {
				message := fmt.Sprintf("%s prerequisite is too old: got %q, need major version %d+", prereq.Name, strings.TrimSpace(string(out)), prereq.MinMajor)
				if prereq.InstallHint != "" {
					message += ". " + prereq.InstallHint
				}
				return errors.New(message)
			}
		}
	}
	return nil
}

func parseMajorVersion(text string) (int, bool) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "v")
	for i, r := range text {
		if r >= '0' && r <= '9' {
			start := i
			end := i
			for end < len(text) && text[end] >= '0' && text[end] <= '9' {
				end++
			}
			n, err := strconv.Atoi(text[start:end])
			return n, err == nil
		}
	}
	return 0, false
}

func installGoLSP(ctx context.Context, spec lspInstallSpec) (string, error) {
	binDir := filepath.Join(lspCacheRoot(), spec.ID, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "install", spec.GoPackage)
	cmd.Env = append(os.Environ(), "GOBIN="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go install failed: %w: %s", err, trimCommandOutput(out))
	}
	path := filepath.Join(binDir, executableNames(spec.Command)[0])
	if !executableFile(path) {
		return "", fmt.Errorf("%s installed but not found at %s", spec.Command, path)
	}
	return path, nil
}

func installNPMLSP(ctx context.Context, spec lspInstallSpec) (string, error) {
	prefix := filepath.Join(lspCacheRoot(), spec.ID)
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return "", err
	}
	args := append([]string{"install", "--prefix", prefix}, spec.NPMPackages...)
	cmd := exec.CommandContext(ctx, "npm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("npm install failed: %w: %s", err, trimCommandOutput(out))
	}
	for _, name := range executableNames(spec.Command) {
		path := filepath.Join(prefix, "node_modules", ".bin", name)
		if executableFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s installed but not found in %s", spec.Command, filepath.Join(prefix, "node_modules", ".bin"))
}

func checkLSPBinary(ctx context.Context, path string, args []string) error {
	if !executableFile(path) {
		return fmt.Errorf("installed LSP binary is not executable: %s", path)
	}
	if len(args) == 0 {
		return nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(checkCtx, path, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("installed LSP binary check failed: %w: %s", err, trimCommandOutput(out))
	}
	return nil
}

func lspInstallCommand(spec lspInstallSpec) string {
	switch spec.InstallKind {
	case lspInstallGo:
		return "GOBIN=" + shellQuote(filepath.Join(lspCacheRoot(), spec.ID, "bin")) + " go install " + spec.GoPackage
	case lspInstallNPM:
		return "npm install --prefix " + shellQuote(filepath.Join(lspCacheRoot(), spec.ID)) + " " + strings.Join(spec.NPMPackages, " ")
	case lspInstallManual:
		return spec.ManualInstall
	default:
		return ""
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!*?[]{}()&;<>|") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func lspCacheRoot() string {
	if value := strings.TrimSpace(os.Getenv(lspCacheEnv)); value != "" {
		return expandPath(value)
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "ns-workspace", "lsp")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "ns-workspace", "lsp")
	}
	return filepath.Join(os.TempDir(), "ns-workspace", "lsp")
}

func lspCacheCommandDirs(command string) []string {
	dirs := []string{}
	for _, spec := range lspInstallSpecs() {
		if spec.Command != command {
			continue
		}
		switch spec.InstallKind {
		case lspInstallGo:
			dirs = append(dirs, filepath.Join(lspCacheRoot(), spec.ID, "bin"))
		case lspInstallNPM:
			dirs = append(dirs, filepath.Join(lspCacheRoot(), spec.ID, "node_modules", ".bin"))
		case lspInstallManual:
			dirs = append(dirs, filepath.Join(lspCacheRoot(), spec.ID, "bin"))
		}
	}
	return dirs
}

func lspCommandSource(path, projectRoot string) string {
	clean := filepath.Clean(path)
	cache := filepath.Clean(lspCacheRoot())
	if strings.HasPrefix(clean, cache+string(os.PathSeparator)) || clean == cache {
		return "cache"
	}
	if projectRoot != "" {
		project := filepath.Clean(projectRoot)
		if strings.HasPrefix(clean, project+string(os.PathSeparator)) || clean == project {
			return "project"
		}
	}
	return "path"
}

func lspUnavailableWarning(lang lspLanguage, detail string) string {
	name := lang.Name
	if spec, ok := lspInstallSpecByServerID(lang.ServerID); ok {
		name = spec.Name
	}
	command := "go run . lsp install " + lang.ServerID
	if spec, ok := lspInstallSpecByServerID(lang.ServerID); ok {
		command = "go run . lsp install " + spec.ID
	}
	return fmt.Sprintf("Code Graph LSP server for %s is unavailable: %s. Run: %s. CLI graph queries auto-ensure LSP by default; use --no-ensure-lsp only when install side effects must be skipped.", name, detail, command)
}

func trimCommandOutput(out []byte) string {
	text := strings.TrimSpace(string(out))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}
