package graphquery

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
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

const CacheEnv = "NS_WORKSPACE_LSP_CACHE"

type SymbolMode string

const (
	SymbolModeCallable SymbolMode = "callable"
	SymbolModeDocument SymbolMode = "document"
	SymbolModeSelector SymbolMode = "selector"
)

type Prerequisite struct {
	Name        string
	Command     string
	Args        []string
	InstallHint string
	MinMajor    int
}

type InstallSpec struct {
	ID            string
	Name          string
	Aliases       []string
	ServerID      string
	Extensions    []string
	Command       string
	Args          []string
	CheckArgs     []string
	Prerequisites []Prerequisite
}

type LanguageSpec struct {
	ID         string
	Name       string
	Aliases    []string
	Extensions []string
	ServerID   string
	LanguageID string
	SymbolMode SymbolMode
}

type StatusRow struct {
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

type InstallResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

type EnsureOptions struct {
	Force    bool
	DryRun   bool
	Progress io.Writer
}

type SourceDetector interface {
	DetectedLanguages(projectRoot, docsDir string) map[string]bool
	DetectedInstallIDs(projectRoot, docsDir string) map[string]bool
}

type lspImplementation interface {
	installSpec() InstallSpec
	languageSpecs() []LanguageSpec
	cacheCommandDirs() []string
	installCommand() string
	install(context.Context) (string, error)
}

type emptySourceDetector struct{}

func (emptySourceDetector) DetectedLanguages(string, string) map[string]bool {
	return map[string]bool{}
}

func (emptySourceDetector) DetectedInstallIDs(string, string) map[string]bool {
	return map[string]bool{}
}

var installLocks sync.Map

func RunLSP(args []string, detector SourceDetector) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		PrintLSPUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return RunLSPList(args[1:], os.Stdout, detector)
	case "install":
		return RunLSPInstall(args[1:], os.Stdout, detector)
	default:
		PrintLSPUsage(os.Stdout)
		return fmt.Errorf("unknown lsp command %q", args[0])
	}
}

func RunLSPList(args []string, out io.Writer, detector SourceDetector) error {
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
	projectRoot = normalizeProjectRoot(projectRoot)
	rows := ListStatus(projectRoot, docsDir, detector)
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

func RunLSPInstall(args []string, out io.Writer, detector SourceDetector) error {
	language, flags := splitInstallArgs(args)
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
	projectRoot = normalizeProjectRoot(projectRoot)
	opts := EnsureOptions{Force: force, DryRun: dryRun, Progress: os.Stderr}

	var results []InstallResult
	if language == "auto" {
		results = InstallProjectLSPs(context.Background(), projectRoot, docsDir, opts, detector)
	} else {
		impl, ok := implementationByID(language)
		if !ok {
			return fmt.Errorf("unsupported LSP language %q; supported: %s", language, strings.Join(SupportedIDs(), ", "))
		}
		results = []InstallResult{InstallLSP(context.Background(), impl, projectRoot, opts)}
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

func PrintLSPUsage(out io.Writer) {
	fmt.Fprintln(out, `LSP commands:
  lsp list [--project PATH] [--docs-dir docs] [--json]
  lsp install <language|auto> [--project PATH] [--docs-dir docs] [--force] [--dry-run] [--json]

Supported languages:
  html, css, scss, javascript, typescript, go/golang, kotlin`)
}

func ListStatus(projectRoot, docsDir string, detector SourceDetector) []StatusRow {
	detected := detectorOrEmpty(detector).DetectedLanguages(projectRoot, docsDir)
	rows := []StatusRow{}
	seenRows := map[string]bool{}
	for _, lang := range LanguageSpecs() {
		if seenRows[lang.ID] {
			continue
		}
		seenRows[lang.ID] = true
		spec, ok := InstallSpecByServerID(lang.ServerID)
		if !ok {
			continue
		}
		path, source, err := ResolveCommandWithSource(spec.Command, projectRoot)
		row := StatusRow{
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
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func InstallProjectLSPs(ctx context.Context, projectRoot, docsDir string, opts EnsureOptions, detector SourceDetector) []InstallResult {
	detected := detectorOrEmpty(detector).DetectedInstallIDs(projectRoot, docsDir)
	if len(detected) == 0 {
		return []InstallResult{{ID: "auto", Name: "Auto", Status: "skipped", Message: "no supported LSP languages detected"}}
	}
	ids := make([]string, 0, len(detected))
	for id := range detected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	results := []InstallResult{}
	for _, id := range ids {
		impl, ok := implementationByID(id)
		if !ok {
			continue
		}
		results = append(results, InstallLSP(ctx, impl, projectRoot, opts))
	}
	return results
}

func EnsureProjectLSP(ctx context.Context, projectRoot, docsDir string, opts EnsureOptions, detector SourceDetector) []string {
	results := InstallProjectLSPs(ctx, projectRoot, docsDir, opts, detector)
	return InstallWarnings(results, opts)
}

func InstallWarnings(results []InstallResult, opts EnsureOptions) []string {
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

func InstallLSP(ctx context.Context, impl lspImplementation, projectRoot string, opts EnsureOptions) InstallResult {
	spec := impl.installSpec()
	if !opts.Force {
		if path, source, err := ResolveCommandWithSource(spec.Command, projectRoot); err == nil {
			return InstallResult{ID: spec.ID, Name: spec.Name, Status: "already-installed", Path: path, Message: "found in " + source}
		}
	}
	command := impl.installCommand()
	if opts.DryRun {
		return InstallResult{ID: spec.ID, Name: spec.Name, Status: "dry-run", Message: command}
	}
	lock := installMutex(spec.ID)
	lock.Lock()
	defer lock.Unlock()
	if !opts.Force {
		// Re-check after taking the process-local lock in case another ensure call just populated the cache.
		if path, source, err := ResolveCommandWithSource(spec.Command, projectRoot); err == nil {
			return InstallResult{ID: spec.ID, Name: spec.Name, Status: "already-installed", Path: path, Message: "found in " + source}
		}
	}
	if err := checkPrerequisites(ctx, spec); err != nil {
		return InstallResult{ID: spec.ID, Name: spec.Name, Status: "failed", Message: err.Error()}
	}
	path, err := impl.install(ctx)
	if err != nil {
		return InstallResult{ID: spec.ID, Name: spec.Name, Status: "failed", Message: err.Error()}
	}
	if err := checkBinary(ctx, path, spec.CheckArgs); err != nil {
		return InstallResult{ID: spec.ID, Name: spec.Name, Status: "failed", Message: err.Error()}
	}
	return InstallResult{ID: spec.ID, Name: spec.Name, Status: "installed", Path: path}
}

func InstallSpecs() []InstallSpec {
	specs := []InstallSpec{}
	for _, impl := range implementations() {
		specs = append(specs, impl.installSpec())
	}
	return specs
}

func LanguageSpecs() []LanguageSpec {
	specs := []LanguageSpec{}
	for _, impl := range implementations() {
		specs = append(specs, impl.languageSpecs()...)
	}
	return specs
}

func SupportedIDs() []string {
	ids := []string{}
	for _, spec := range InstallSpecs() {
		ids = append(ids, spec.ID)
		ids = append(ids, spec.Aliases...)
	}
	for _, lang := range LanguageSpecs() {
		ids = append(ids, lang.ID)
		ids = append(ids, lang.Aliases...)
	}
	sort.Strings(ids)
	return internalutil.UniqueSortedStrings(ids)
}

func InstallSpecByID(id string) (InstallSpec, bool) {
	if impl, ok := implementationByID(id); ok {
		return impl.installSpec(), true
	}
	return InstallSpec{}, false
}

func InstallSpecByServerID(serverID string) (InstallSpec, bool) {
	if impl, ok := implementationByServerID(serverID); ok {
		return impl.installSpec(), true
	}
	return InstallSpec{}, false
}

func CacheRoot() string {
	if value := strings.TrimSpace(os.Getenv(CacheEnv)); value != "" {
		return internalutil.ExpandPath(value)
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "ns-workspace", "lsp")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "ns-workspace", "lsp")
	}
	return filepath.Join(os.TempDir(), "ns-workspace", "lsp")
}

func CacheCommandDirs(command string) []string {
	dirs := []string{}
	for _, impl := range implementations() {
		spec := impl.installSpec()
		if spec.Command != command {
			continue
		}
		dirs = append(dirs, impl.cacheCommandDirs()...)
	}
	return dirs
}

func ResolveCommandWithSource(command, projectRoot string) (string, string, error) {
	if path, err := exec.LookPath(command); err == nil {
		return path, CommandSource(path, projectRoot), nil
	}
	for _, candidate := range commandCandidates(command, projectRoot) {
		if internalutil.ExecutableFile(candidate) {
			return candidate, CommandSource(candidate, projectRoot), nil
		}
	}
	return "", "", fmt.Errorf("command not found: %s", command)
}

func CommandSource(path, projectRoot string) string {
	clean := filepath.Clean(path)
	cache := filepath.Clean(CacheRoot())
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

func UnavailableWarning(serverID, fallbackName, detail string) string {
	name := fallbackName
	if spec, ok := InstallSpecByServerID(serverID); ok {
		name = spec.Name
	}
	command := "go run . lsp install " + serverID
	if spec, ok := InstallSpecByServerID(serverID); ok {
		command = "go run . lsp install " + spec.ID
	}
	return fmt.Sprintf("Code Graph LSP server for %s is unavailable: %s. Run: %s. CLI graph queries auto-ensure LSP by default; use --no-ensure-lsp only when install side effects must be skipped.", name, detail, command)
}

func splitInstallArgs(args []string) (string, []string) {
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

func implementations() []lspImplementation {
	return []lspImplementation{
		htmlImplementation{},
		cssImplementation{},
		typeScriptImplementation{},
		goImplementation{},
		kotlinImplementation{},
	}
}

func implementationByID(id string) (lspImplementation, bool) {
	id = normalizeID(id)
	for _, impl := range implementations() {
		spec := impl.installSpec()
		if normalizeID(spec.ID) == id || slices.Contains(normalizedIDs(spec.Aliases), id) {
			return impl, true
		}
	}
	for _, lang := range LanguageSpecs() {
		if normalizeID(lang.ID) == id || slices.Contains(normalizedIDs(lang.Aliases), id) {
			return implementationByServerID(lang.ServerID)
		}
	}
	return nil, false
}

func implementationByServerID(serverID string) (lspImplementation, bool) {
	serverID = normalizeID(serverID)
	for _, impl := range implementations() {
		spec := impl.installSpec()
		if normalizeID(spec.ServerID) == serverID || normalizeID(spec.ID) == serverID {
			return impl, true
		}
	}
	return nil, false
}

func detectorOrEmpty(detector SourceDetector) SourceDetector {
	if detector != nil {
		return detector
	}
	return emptySourceDetector{}
}

func installMutex(id string) *sync.Mutex {
	value, _ := installLocks.LoadOrStore(id, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func checkPrerequisites(ctx context.Context, spec InstallSpec) error {
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

func checkBinary(ctx context.Context, path string, args []string) error {
	if !internalutil.ExecutableFile(path) {
		return fmt.Errorf("installed LSP binary is not executable: %s", path)
	}
	if len(args) == 0 {
		return nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(checkCtx, path, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("installed LSP binary check failed: %w: %s", err, trimCommandOutput(out))
	}
	return nil
}

func normalizeProjectRoot(root string) string {
	root = internalutil.ExpandPath(strings.TrimSpace(root))
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(root)
}

func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func normalizedIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = normalizeID(id); id != "" {
			out = append(out, id)
		}
	}
	return out
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

func commandCandidates(command, projectRoot string) []string {
	dirs := []string{}
	addDir := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		dirs = internalutil.AppendUniqueString(dirs, internalutil.ExpandPath(dir))
	}
	if projectRoot != "" {
		addDir(filepath.Join(projectRoot, "node_modules", ".bin"))
	}
	if cwd, err := os.Getwd(); err == nil {
		addDir(filepath.Join(cwd, "node_modules", ".bin"))
	}
	if command == "gopls" {
		addDir(os.Getenv("GOBIN"))
		if gopath := internalutil.FirstNonEmpty(os.Getenv("GOPATH"), internalutil.GoEnvValue("GOPATH")); gopath != "" {
			addDir(filepath.Join(gopath, "bin"))
		}
		if gobin := internalutil.GoEnvValue("GOBIN"); gobin != "" {
			addDir(gobin)
		}
		if home, err := os.UserHomeDir(); err == nil {
			addDir(filepath.Join(home, "go", "bin"))
		}
	}
	for _, dir := range CacheCommandDirs(command) {
		addDir(dir)
	}

	candidates := []string{}
	for _, dir := range dirs {
		for _, name := range internalutil.ExecutableNames(command) {
			candidates = internalutil.AppendUniqueString(candidates, filepath.Join(dir, name))
		}
	}
	return candidates
}



func trimCommandOutput(out []byte) string {
	text := strings.TrimSpace(string(out))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}
