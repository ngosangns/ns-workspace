package agentsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Options struct {
	Command    string
	AgentsDir  string
	DryRun     bool
	Yes        bool
	Force      bool
	CopyMode   bool
	NoMCP      bool
	NoRegistry bool
	ToolFilter map[string]bool
}

type Manager struct {
	Presets fs.FS
}

type MCPManifest struct {
	MCPServers map[string]any `json:"mcpServers"`
}

type SettingsManifest struct {
	Hooks map[string]any `json:"hooks"`
}

type RegistryManifest struct {
	Skills []RegistrySkill `json:"skills"`
}

type RegistrySkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Skill  string `json:"skill"`
}

type ArtifactKind string

const (
	ArtifactInstructions ArtifactKind = "instructions"
	ArtifactSkills       ArtifactKind = "skills"
	ArtifactSubagents    ArtifactKind = "subagents"
	ArtifactSettings     ArtifactKind = "settings"
	ArtifactHooks        ArtifactKind = "hooks"
	ArtifactMCP          ArtifactKind = "mcp"
	ArtifactRules        ArtifactKind = "rules"
	ArtifactCommands     ArtifactKind = "commands"
)

type SupportTier string

const (
	TierStable       SupportTier = "stable"
	TierManual       SupportTier = "manual"
	TierExperimental SupportTier = "experimental"
	TierCatalog      SupportTier = "catalog"
)

type AgentCapabilities struct {
	Tier      SupportTier
	DocsURL   []string
	Artifacts []ArtifactKind
	Notes     string
}

type StatusReporter interface {
	Line(format string, args ...any)
}

type stdoutReporter struct{}

func (stdoutReporter) Line(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

type Context struct {
	Options
	Home          string
	XDGConfigHome string
	Presets       fs.FS
	Report        StatusReporter
}

type AgentAdapter interface {
	Name() string
	Capabilities() AgentCapabilities
	Plan(ctx Context, update bool) ([]Operation, error)
	StatusPaths(ctx Context) []string
	DoctorExecutables() []string
}

type Operation interface {
	Apply(ctx Context) error
	Describe(ctx Context)
	Path() string
}

type WriteFile struct {
	Dst     string
	Data    []byte
	Replace bool
}

func (op WriteFile) Apply(ctx Context) error {
	return writeFileManaged(ctx, op.Dst, op.Data, op.Replace)
}

func (op WriteFile) Describe(ctx Context) { ctx.Report.Line("write: %s", op.Dst) }
func (op WriteFile) Path() string         { return op.Dst }

type InstallPresetFile struct {
	Src     string
	Dst     string
	Replace bool
}

func (op InstallPresetFile) Apply(ctx Context) error {
	data, err := fs.ReadFile(ctx.Presets, op.Src)
	if err != nil {
		return err
	}
	return writeFileManaged(ctx, op.Dst, data, op.Replace)
}

func (op InstallPresetFile) Describe(ctx Context) { ctx.Report.Line("write: %s", op.Dst) }
func (op InstallPresetFile) Path() string         { return op.Dst }

type InstallPresetTree struct {
	SrcRoot string
	DstRoot string
	Replace bool
}

func (op InstallPresetTree) Apply(ctx Context) error {
	return fs.WalkDir(ctx.Presets, op.SrcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(op.SrcRoot, path)
		if err != nil || rel == "." {
			return err
		}
		dst := filepath.Join(op.DstRoot, rel)
		if d.IsDir() {
			return ensureDir(ctx, dst)
		}
		data, err := fs.ReadFile(ctx.Presets, path)
		if err != nil {
			return err
		}
		return writeFileManaged(ctx, dst, data, op.Replace)
	})
}

func (op InstallPresetTree) Describe(ctx Context) {
	ctx.Report.Line("tree: %s -> %s", op.SrcRoot, op.DstRoot)
}
func (op InstallPresetTree) Path() string { return op.DstRoot }

type LinkOrCopy struct {
	Src     string
	Dst     string
	Replace bool
}

func (op LinkOrCopy) Apply(ctx Context) error {
	if err := ensureDir(ctx, filepath.Dir(op.Dst)); err != nil {
		return err
	}
	return linkOrCopy(ctx, op.Src, op.Dst, op.Replace)
}

func (op LinkOrCopy) Describe(ctx Context) { ctx.Report.Line("link/copy: %s -> %s", op.Src, op.Dst) }
func (op LinkOrCopy) Path() string         { return op.Dst }

type LinkSkillDirs struct {
	SrcRoot string
	DstRoot string
	Replace bool
}

func (op LinkSkillDirs) Apply(ctx Context) error {
	if err := ensureDir(ctx, op.DstRoot); err != nil {
		return err
	}
	entries, err := os.ReadDir(op.SrcRoot)
	if err != nil {
		if !ctx.DryRun {
			return err
		}
		names, err := embeddedEntryNames(ctx.Presets, embeddedRootFor(op.SrcRoot))
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := linkOrCopy(ctx, filepath.Join(op.SrcRoot, name), filepath.Join(op.DstRoot, name), op.Replace); err != nil {
				return err
			}
		}
		return nil
	}
	for _, entry := range entries {
		if err := linkOrCopy(ctx, filepath.Join(op.SrcRoot, entry.Name()), filepath.Join(op.DstRoot, entry.Name()), op.Replace); err != nil {
			return err
		}
	}
	return nil
}

func (op LinkSkillDirs) Describe(ctx Context) {
	ctx.Report.Line("skills: %s -> %s", op.SrcRoot, op.DstRoot)
}
func (op LinkSkillDirs) Path() string { return op.DstRoot }

type MergeJSON struct {
	Dst     string
	KeyPath []string
	Values  map[string]any
}

func (op MergeJSON) Apply(ctx Context) error {
	if len(op.Values) == 0 {
		return nil
	}
	obj := map[string]any{}
	if data, err := os.ReadFile(op.Dst); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", op.Dst, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mergeJSONAt(obj, op.KeyPath, op.Values)
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileManaged(ctx, op.Dst, data, true)
}

func (op MergeJSON) Describe(ctx Context) { ctx.Report.Line("merge json: %s", op.Dst) }
func (op MergeJSON) Path() string         { return op.Dst }

type AppendManagedBlock struct {
	Dst     string
	Label   string
	Content string
	Replace bool
}

func (op AppendManagedBlock) Apply(ctx Context) error {
	begin := fmt.Sprintf("# >>> ns-workspace %s >>>", op.Label)
	end := fmt.Sprintf("# <<< ns-workspace %s <<<", op.Label)
	block := begin + "\n" + strings.TrimSpace(op.Content) + "\n" + end + "\n"
	current := ""
	if data, err := os.ReadFile(op.Dst); err == nil {
		current = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next := replaceManagedBlock(current, begin, end, block)
	return writeFileManaged(ctx, op.Dst, []byte(next), op.Replace)
}

func (op AppendManagedBlock) Describe(ctx Context) { ctx.Report.Line("managed block: %s", op.Dst) }
func (op AppendManagedBlock) Path() string         { return op.Dst }

type ManualStep struct {
	Agent string
	Dst   string
	Text  string
}

func (op ManualStep) Apply(ctx Context) error {
	return writeFileManaged(ctx, op.Dst, []byte(strings.TrimSpace(op.Text)+"\n"), true)
}

func (op ManualStep) Describe(ctx Context) { ctx.Report.Line("manual: %s", op.Dst) }
func (op ManualStep) Path() string         { return op.Dst }

type fileAdapter struct {
	name         string
	aliases      []string
	tier         SupportTier
	docs         []string
	notes        string
	exes         []string
	instruction  string
	skills       string
	subagents    string
	settings     string
	hooksPath    string
	hooksKeyPath []string
	mcpPath      string
	mcpKeyPath   []string
	manual       bool
}

func (a fileAdapter) Name() string { return a.name }

func (a fileAdapter) Aliases() []string { return a.aliases }

func (a fileAdapter) Capabilities() AgentCapabilities {
	artifacts := []ArtifactKind{}
	if a.instruction != "" {
		artifacts = append(artifacts, ArtifactInstructions)
	}
	if a.skills != "" {
		artifacts = append(artifacts, ArtifactSkills)
	}
	if a.subagents != "" {
		artifacts = append(artifacts, ArtifactSubagents)
	}
	if a.settings != "" {
		artifacts = append(artifacts, ArtifactSettings)
	}
	if a.settings != "" || a.hooksPath != "" {
		artifacts = append(artifacts, ArtifactHooks)
	}
	if a.mcpPath != "" {
		artifacts = append(artifacts, ArtifactMCP)
	}
	if a.manual {
		artifacts = append(artifacts, ArtifactRules, ArtifactCommands)
	}
	return AgentCapabilities{Tier: a.tier, DocsURL: a.docs, Artifacts: artifacts, Notes: a.notes}
}

func (a fileAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	replace := update || ctx.Force
	if a.manual {
		return []Operation{ManualStep{
			Agent: a.name,
			Dst:   filepath.Join(ctx.Options.AgentsDir, "generated", a.name, "README.md"),
			Text:  manualReadme(a),
		}}, nil
	}

	sourceAgents := filepath.Join(ctx.Options.AgentsDir, "AGENTS.md")
	sourceSkills := filepath.Join(ctx.Options.AgentsDir, "skills")
	sourceSubagents := filepath.Join(ctx.Options.AgentsDir, "agents")
	ops := []Operation{}
	if a.instruction != "" {
		ops = append(ops, LinkOrCopy{Src: sourceAgents, Dst: a.instruction, Replace: replace})
	}
	if a.skills != "" {
		ops = append(ops, LinkSkillDirs{SrcRoot: sourceSkills, DstRoot: a.skills, Replace: replace})
	}
	if a.subagents != "" {
		ops = append(ops, LinkSkillDirs{SrcRoot: sourceSubagents, DstRoot: a.subagents, Replace: replace})
	}
	if a.settings != "" {
		ops = append(ops, LinkOrCopy{Src: filepath.Join(ctx.Options.AgentsDir, "settings.json"), Dst: a.settings, Replace: replace})
	}
	if a.hooksPath != "" && len(a.hooksKeyPath) > 0 {
		manifest, err := readSettingsManifest(ctx)
		if err != nil {
			return nil, err
		}
		ops = append(ops, MergeJSON{Dst: a.hooksPath, KeyPath: a.hooksKeyPath, Values: manifest.Hooks})
	}
	if !ctx.NoMCP && a.mcpPath != "" && len(a.mcpKeyPath) > 0 {
		manifest, err := readMCPManifest(ctx)
		if err != nil {
			return nil, err
		}
		ops = append(ops, MergeJSON{Dst: a.mcpPath, KeyPath: a.mcpKeyPath, Values: manifest.MCPServers})
	}
	return ops, nil
}

func (a fileAdapter) StatusPaths(ctx Context) []string {
	paths := []string{a.instruction, a.skills, a.subagents, a.settings, a.hooksPath, a.mcpPath}
	if a.manual {
		paths = append(paths, filepath.Join(ctx.Options.AgentsDir, "generated", a.name, "README.md"))
	}
	return compact(paths)
}

func (a fileAdapter) DoctorExecutables() []string { return a.exes }

type claudeAdapter struct{ fileAdapter }

func (a claudeAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	ops, err := a.fileAdapter.Plan(ctx, update)
	if err != nil {
		return nil, err
	}
	if ctx.NoMCP {
		return ops, nil
	}
	script, err := mcpCommandScript(ctx, "claude", func(name string, server string) string {
		return fmt.Sprintf("claude mcp add-json %s '%s' --scope user\n", shellWord(name), shellSingleQuotePayload(server))
	})
	if err != nil {
		return nil, err
	}
	ops = append(ops, WriteFile{
		Dst:     filepath.Join(ctx.Options.AgentsDir, "generated", "claude", "mcp.commands.sh"),
		Data:    []byte(script),
		Replace: update || ctx.Force,
	})
	return ops, nil
}

type codexAdapter struct{ fileAdapter }

func (a codexAdapter) Capabilities() AgentCapabilities {
	caps := a.fileAdapter.Capabilities()
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

func (a codexAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	ops, err := a.fileAdapter.Plan(ctx, update)
	if err != nil {
		return nil, err
	}
	if ctx.NoMCP {
		return ops, nil
	}
	manifest, err := readMCPManifest(ctx)
	if err != nil {
		return nil, err
	}
	if len(manifest.MCPServers) == 0 {
		return ops, nil
	}
	ops = append(ops, AppendManagedBlock{
		Dst:     filepath.Join(ctx.Home, ".codex", "config.toml"),
		Label:   "mcp",
		Content: codexMCPBlock(manifest),
		Replace: true,
	})
	return ops, nil
}

func (a codexAdapter) StatusPaths(ctx Context) []string {
	paths := a.fileAdapter.StatusPaths(ctx)
	paths = append(paths, filepath.Join(ctx.Home, ".codex", "config.toml"))
	return compact(paths)
}

type aiderAdapter struct{ fileAdapter }

func (a aiderAdapter) Capabilities() AgentCapabilities {
	caps := a.fileAdapter.Capabilities()
	caps.Artifacts = append(caps.Artifacts, ArtifactSettings, ArtifactRules)
	return caps
}

func (a aiderAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	return []Operation{AppendManagedBlock{
		Dst: filepath.Join(ctx.Home, ".aider.conf.yml"), Label: "conventions",
		Content: "read: " + filepath.ToSlash(filepath.Join(ctx.Options.AgentsDir, "AGENTS.md")),
		Replace: true,
	}}, nil
}

func (a aiderAdapter) StatusPaths(ctx Context) []string {
	return []string{filepath.Join(ctx.Home, ".aider.conf.yml")}
}

func DefaultAgentsDir() (string, error) {
	if env := os.Getenv("AGENTS_HOME"); env != "" {
		return ExpandPath(env), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents"), nil
}

func ExpandPath(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func ParseTools(value string) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func (m Manager) Apply(opt Options, update bool) error {
	ctx, err := m.context(opt)
	if err != nil {
		return err
	}
	mode := "init"
	if update {
		mode = "update"
	}
	ctx.Report.Line("%s shared agent config at %s", mode, ctx.Options.AgentsDir)
	if err := m.applyCore(ctx, update); err != nil {
		return err
	}
	for _, adapter := range m.adapters(ctx) {
		if !selected(ctx.Options, adapter) {
			continue
		}
		ops, err := adapter.Plan(ctx, update)
		if err != nil {
			return fmt.Errorf("%s adapter: %w", adapter.Name(), err)
		}
		for _, op := range ops {
			if err := op.Apply(ctx); err != nil {
				return fmt.Errorf("%s %s: %w", adapter.Name(), op.Path(), err)
			}
		}
	}
	ctx.Report.Line("done")
	return nil
}

func (m Manager) Status(opt Options) error {
	ctx, err := m.context(opt)
	if err != nil {
		return err
	}
	paths := []string{
		filepath.Join(ctx.Options.AgentsDir, "AGENTS.md"),
		filepath.Join(ctx.Options.AgentsDir, "agents"),
		filepath.Join(ctx.Options.AgentsDir, "registry", "skills.json"),
		filepath.Join(ctx.Options.AgentsDir, "skills"),
		filepath.Join(ctx.Options.AgentsDir, "settings.json"),
		filepath.Join(ctx.Options.AgentsDir, "mcp", "servers.json"),
	}
	for _, adapter := range m.adapters(ctx) {
		if selected(ctx.Options, adapter) {
			paths = append(paths, adapter.StatusPaths(ctx)...)
		}
	}
	for _, path := range compact(paths) {
		printPathStatus(ctx, path)
	}
	return nil
}

func (m Manager) Doctor(opt Options) error {
	ctx, err := m.context(opt)
	if err != nil {
		return err
	}
	ctx.Report.Line("os: %s/%s", runtime.GOOS, runtime.GOARCH)
	ctx.Report.Line("agents home: %s", ctx.Options.AgentsDir)
	printPathStatus(ctx, ctx.Options.AgentsDir)
	checkJSON(ctx, filepath.Join(ctx.Options.AgentsDir, "mcp", "servers.json"))
	checkJSON(ctx, filepath.Join(ctx.Options.AgentsDir, "settings.json"))
	checkJSON(ctx, filepath.Join(ctx.Options.AgentsDir, "registry", "skills.json"))
	seen := map[string]bool{}
	for _, adapter := range m.adapters(ctx) {
		if !selected(ctx.Options, adapter) {
			continue
		}
		caps := adapter.Capabilities()
		ctx.Report.Line("agent %-14s tier=%s artifacts=%s", adapter.Name(), caps.Tier, artifactList(caps.Artifacts))
		for _, exe := range adapter.DoctorExecutables() {
			if seen[exe] {
				continue
			}
			seen[exe] = true
			if path, err := exec.LookPath(exe); err == nil {
				ctx.Report.Line("found %-14s %s", exe, path)
			} else {
				ctx.Report.Line("missing %-12s not on PATH", exe)
			}
		}
		for _, path := range adapter.StatusPaths(ctx) {
			printPathStatus(ctx, path)
			if strings.HasSuffix(path, ".json") {
				checkJSON(ctx, path)
			}
		}
	}
	return nil
}

func (m Manager) Catalog(opt Options) error {
	ctx, err := m.context(opt)
	if err != nil {
		return err
	}
	for _, adapter := range m.adapters(ctx) {
		if !selected(ctx.Options, adapter) {
			continue
		}
		caps := adapter.Capabilities()
		ctx.Report.Line("%-16s %-12s %s", adapter.Name(), caps.Tier, artifactList(caps.Artifacts))
		if caps.Notes != "" {
			ctx.Report.Line("  %s", caps.Notes)
		}
	}
	return nil
}

func (m Manager) InstallRegistrySkills(opt Options) error {
	ctx, err := m.context(opt)
	if err != nil {
		return err
	}
	return installRegistrySkills(ctx)
}

func (m Manager) context(opt Options) (Context, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Context{}, err
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	opt.AgentsDir = ExpandPath(opt.AgentsDir)
	if opt.ToolFilter == nil {
		opt.ToolFilter = map[string]bool{"all": true}
	}
	return Context{Options: opt, Home: home, XDGConfigHome: xdg, Presets: m.Presets, Report: stdoutReporter{}}, nil
}

func (m Manager) applyCore(ctx Context, update bool) error {
	replace := update || ctx.Force
	coreDirs := []string{
		ctx.Options.AgentsDir,
		filepath.Join(ctx.Options.AgentsDir, "skills"),
		filepath.Join(ctx.Options.AgentsDir, "agents"),
		filepath.Join(ctx.Options.AgentsDir, "mcp"),
		filepath.Join(ctx.Options.AgentsDir, "generated"),
	}
	for _, dir := range coreDirs {
		if err := ensureDir(ctx, dir); err != nil {
			return err
		}
	}
	ops := []Operation{
		InstallPresetFile{Src: "presets/agents/AGENTS.md", Dst: filepath.Join(ctx.Options.AgentsDir, "AGENTS.md"), Replace: replace},
		InstallPresetTree{SrcRoot: "presets/skills", DstRoot: filepath.Join(ctx.Options.AgentsDir, "skills"), Replace: replace},
		InstallPresetTree{SrcRoot: "presets/subagents", DstRoot: filepath.Join(ctx.Options.AgentsDir, "agents"), Replace: replace},
		InstallPresetFile{Src: "presets/settings/settings.json", Dst: filepath.Join(ctx.Options.AgentsDir, "settings.json"), Replace: replace},
	}
	for _, op := range ops {
		if err := op.Apply(ctx); err != nil {
			return err
		}
	}
	if err := writeRegistryHelpers(ctx, replace); err != nil {
		return err
	}
	if !ctx.NoRegistry {
		if err := installRegistrySkills(ctx); err != nil {
			return err
		}
	}
	if !ctx.NoMCP {
		if err := (InstallPresetFile{Src: "presets/mcp/servers.json", Dst: filepath.Join(ctx.Options.AgentsDir, "mcp", "servers.json"), Replace: replace}).Apply(ctx); err != nil {
			return err
		}
		if err := writeMCPReadme(ctx, replace); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) adapters(ctx Context) []AgentAdapter {
	home := ctx.Home
	xdg := ctx.XDGConfigHome
	return []AgentAdapter{
		claudeAdapter{fileAdapter{name: "claude", tier: TierStable, exes: []string{"claude"}, instruction: filepath.Join(home, ".claude", "CLAUDE.md"), skills: filepath.Join(home, ".claude", "skills"), subagents: filepath.Join(home, ".claude", "agents"), settings: filepath.Join(home, ".claude", "settings.json"), docs: []string{"https://docs.claude.com/en/docs/claude-code/settings", "https://docs.claude.com/en/docs/claude-code/mcp"}}},
		fileAdapter{name: "opencode", tier: TierStable, exes: []string{"opencode"}, instruction: filepath.Join(xdg, "opencode", "AGENTS.md"), skills: filepath.Join(xdg, "opencode", "skill"), subagents: filepath.Join(xdg, "opencode", "agent"), hooksPath: filepath.Join(xdg, "opencode", "opencode.json"), hooksKeyPath: []string{"hooks"}, mcpPath: filepath.Join(xdg, "opencode", "opencode.json"), mcpKeyPath: []string{"mcp"}, docs: []string{"https://opencode.ai/docs/config/", "https://opencode.ai/docs/agents/", "https://opencode.ai/docs/mcp-servers/"}},
		fileAdapter{name: "kimi", tier: TierStable, exes: []string{"kimi"}, instruction: filepath.Join(home, ".kimi", "AGENTS.md"), skills: filepath.Join(home, ".kimi", "skills"), mcpPath: filepath.Join(home, ".kimi", "mcp.json"), mcpKeyPath: []string{"mcpServers"}, docs: []string{"https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html"}},
		fileAdapter{name: "kiro", aliases: []string{"kiro-cli"}, tier: TierStable, exes: []string{"kiro", "kiro-cli"}, instruction: filepath.Join(home, ".kiro", "steering", "AGENTS.md"), mcpPath: filepath.Join(home, ".kiro", "settings", "mcp.json"), mcpKeyPath: []string{"mcpServers"}, docs: []string{"https://kiro.dev/docs/cli/chat/configuration/", "https://kiro.dev/docs/cli/mcp/", "https://kiro.dev/docs/cli/reference/settings/"}, notes: "Kiro CLI alias: kiro-cli. Shared instructions sync to global steering; MCP presets sync to the shared Kiro settings path."},
		fileAdapter{name: "qwen", tier: TierStable, exes: []string{"qwen"}, instruction: filepath.Join(home, ".qwen", "QWEN.md"), skills: filepath.Join(home, ".qwen", "skills"), hooksPath: filepath.Join(home, ".qwen", "settings.json"), hooksKeyPath: []string{"hooks"}, mcpPath: filepath.Join(home, ".qwen", "settings.json"), mcpKeyPath: []string{"mcpServers"}, docs: []string{"https://qwenlm.github.io/qwen-code-docs/en/cli/configuration/", "https://qwenlm.github.io/qwen-code-docs/en/users/features/mcp/"}},
		fileAdapter{name: "gemini", tier: TierStable, exes: []string{"gemini"}, instruction: filepath.Join(home, ".gemini", "GEMINI.md"), skills: filepath.Join(home, ".gemini", "skills"), hooksPath: filepath.Join(home, ".gemini", "settings.json"), hooksKeyPath: []string{"hooks"}, mcpPath: filepath.Join(home, ".gemini", "settings.json"), mcpKeyPath: []string{"mcpServers"}, docs: []string{"https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md"}},
		codexAdapter{fileAdapter{name: "codex", tier: TierStable, exes: []string{"codex"}, instruction: filepath.Join(home, ".codex", "AGENTS.md"), skills: filepath.Join(home, ".codex", "skills"), docs: []string{"https://github.com/openai/codex/blob/main/docs/config.md", "https://github.com/openai/codex/blob/main/docs/agents_md.md"}}},
		fileAdapter{name: "cline", tier: TierStable, exes: []string{"cline"}, skills: filepath.Join(home, ".cline", "data", "skills"), subagents: filepath.Join(home, ".cline", "data", "agents"), mcpPath: filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"), mcpKeyPath: []string{"mcpServers"}, docs: []string{"https://docs.cline.bot/cline-cli/configuration"}},
		fileAdapter{name: "windsurf", tier: TierStable, instruction: filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md"), docs: []string{"https://docs.windsurf.com/windsurf/cascade/memories"}},
		aiderAdapter{fileAdapter{name: "aider", tier: TierStable, exes: []string{"aider"}, docs: []string{"https://aider.chat/docs/config/aider_conf.html", "https://aider.chat/docs/usage/conventions.html"}}},
		fileAdapter{name: "cursor", tier: TierManual, exes: []string{"cursor-agent"}, manual: true, docs: []string{"https://docs.cursor.com/en/context", "https://docs.cursor.com/cli/mcp"}, notes: "Cursor user rules are stored through Cursor settings; generated helper only."},
		fileAdapter{name: "github-copilot", tier: TierManual, manual: true, docs: []string{"https://code.visualstudio.com/docs/copilot/customization/custom-instructions"}, notes: "Copilot instructions are repo/editor scoped; generated helper only."},
		fileAdapter{name: "jetbrains", tier: TierManual, manual: true, docs: []string{"https://www.jetbrains.com/help/ai-assistant/mcp.html"}, notes: "JetBrains AI MCP setup is product/version specific."},
		fileAdapter{name: "antigravity", tier: TierExperimental, manual: true, notes: "No stable official user-level filesystem path confirmed yet."},
		fileAdapter{name: "trae", tier: TierExperimental, exes: []string{"trae"}, manual: true, notes: "No stable official user-level filesystem path confirmed yet."},
		fileAdapter{name: "roo", tier: TierExperimental, manual: true, notes: "Roo Code support is guarded because the project status is unstable."},
	}
}

func selected(opt Options, adapter AgentAdapter) bool {
	name := strings.ToLower(adapter.Name())
	if opt.ToolFilter["all"] || opt.ToolFilter[name] {
		return true
	}
	if aliased, ok := adapter.(interface{ Aliases() []string }); ok {
		for _, alias := range aliased.Aliases() {
			if opt.ToolFilter[strings.ToLower(alias)] {
				return true
			}
		}
	}
	tier := string(adapter.Capabilities().Tier)
	return opt.ToolFilter[tier]
}

func readMCPManifest(ctx Context) (MCPManifest, error) {
	var manifest MCPManifest
	path := filepath.Join(ctx.Options.AgentsDir, "mcp", "servers.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !ctx.DryRun {
			return manifest, err
		}
		data, err = fs.ReadFile(ctx.Presets, "presets/mcp/servers.json")
		if err != nil {
			return manifest, err
		}
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func readSettingsManifest(ctx Context) (SettingsManifest, error) {
	var manifest SettingsManifest
	path := filepath.Join(ctx.Options.AgentsDir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !ctx.DryRun {
			return manifest, err
		}
		data, err = fs.ReadFile(ctx.Presets, "presets/settings/settings.json")
		if err != nil {
			return manifest, err
		}
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Hooks == nil {
		manifest.Hooks = map[string]any{}
	}
	return manifest, nil
}

func writeRegistryHelpers(ctx Context, replace bool) error {
	manifest, err := readRegistryManifest(ctx.Presets)
	if err != nil {
		return err
	}
	registryDir := filepath.Join(ctx.Options.AgentsDir, "registry")
	if err := ensureDir(ctx, registryDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := writeFileManaged(ctx, filepath.Join(registryDir, "skills.json"), data, replace); err != nil {
		return err
	}
	var script strings.Builder
	script.WriteString("#!/usr/bin/env sh\nset -eu\n\n")
	script.WriteString("# Install registry-managed skills. Custom skills live in ~/.agents/skills.\n")
	for _, skill := range manifest.Skills {
		script.WriteString(registryCommand(skill, true, ctx.CopyMode))
		script.WriteString("\n")
	}
	if err := writeFileManaged(ctx, filepath.Join(registryDir, "install.sh"), []byte(script.String()), replace); err != nil {
		return err
	}
	readme := "# Registry Skills\n\nThese skills are installed from the public Skills registry so updates can come from upstream.\n\nRun:\n\n```bash\nsh ~/.agents/registry/install.sh\n```\n"
	return writeFileManaged(ctx, filepath.Join(registryDir, "README.md"), []byte(readme), replace)
}

func writeMCPReadme(ctx Context, replace bool) error {
	readme := "# Shared MCP Presets\n\n`servers.json` is the source of truth for personal MCP servers.\n\nStable file-based adapters merge these presets into their native user-level config. CLI-backed or UI-backed tools get generated helper files under `~/.agents/generated/<agent>/`.\n"
	return writeFileManaged(ctx, filepath.Join(ctx.Options.AgentsDir, "mcp", "README.md"), []byte(readme), replace)
}

func readRegistryManifest(presets fs.FS) (RegistryManifest, error) {
	var manifest RegistryManifest
	data, err := fs.ReadFile(presets, "presets/registry/skills.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func installRegistrySkills(ctx Context) error {
	manifest, err := readRegistryManifest(ctx.Presets)
	if err != nil {
		return err
	}
	if len(manifest.Skills) == 0 {
		return nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return fmt.Errorf("npx is required to install registry skills; rerun with --no-registry or run ~/.agents/registry/install.sh later")
	}
	for _, skill := range manifest.Skills {
		args := []string{"--yes", "skills", "add", skill.Source, "--skill", skill.Skill, "--global", "--agent", "*", "--yes"}
		if ctx.CopyMode {
			args = append(args, "--copy")
		}
		ctx.Report.Line("registry: %s from %s@%s", skill.Name, skill.Source, skill.Skill)
		if ctx.DryRun {
			ctx.Report.Line("run: npx %s", strings.Join(args, " "))
			continue
		}
		cmd := exec.Command("npx", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			ctx.Report.Line("warning: registry skill %s failed: %v", skill.Name, err)
			continue
		}
	}
	return nil
}

func registryCommand(skill RegistrySkill, global bool, copyMode bool) string {
	parts := []string{"npx --yes skills add", shellWord(skill.Source), "--skill", shellWord(skill.Skill)}
	if global {
		parts = append(parts, "--global")
	}
	parts = append(parts, "--agent '*'", "--yes")
	if copyMode {
		parts = append(parts, "--copy")
	}
	return strings.Join(parts, " ")
}

func mcpCommandScript(ctx Context, agent string, line func(name string, server string) string) (string, error) {
	manifest, err := readMCPManifest(ctx)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("#!/usr/bin/env sh\nset -eu\n\n")
	b.WriteString("# Apply shared MCP presets to " + agent + " user scope.\n")
	for _, name := range names {
		server, err := json.Marshal(manifest.MCPServers[name])
		if err != nil {
			return "", err
		}
		b.WriteString(line(name, string(server)))
	}
	return b.String(), nil
}

func codexMCPBlock(manifest MCPManifest) string {
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		server, _ := manifest.MCPServers[name].(map[string]any)
		b.WriteString(fmt.Sprintf("[mcp_servers.%q]\n", name))
		if typ, _ := server["type"].(string); typ != "" {
			b.WriteString(fmt.Sprintf("type = %q\n", typ))
		}
		if url, _ := server["url"].(string); url != "" {
			b.WriteString(fmt.Sprintf("url = %q\n", url))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func manualReadme(a fileAdapter) string {
	var b strings.Builder
	b.WriteString("# " + a.name + " manual setup\n\n")
	if a.notes != "" {
		b.WriteString(a.notes + "\n\n")
	}
	b.WriteString("Shared source files live in `~/.agents`:\n\n")
	b.WriteString("- `~/.agents/AGENTS.md`\n")
	b.WriteString("- `~/.agents/skills/`\n")
	b.WriteString("- `~/.agents/agents/`\n")
	b.WriteString("- `~/.agents/mcp/servers.json`\n\n")
	if len(a.docs) > 0 {
		b.WriteString("Docs:\n\n")
		for _, url := range a.docs {
			b.WriteString("- " + url + "\n")
		}
	}
	return b.String()
}

func writeFileManaged(ctx Context, path string, data []byte, replace bool) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == string(data) {
			ctx.Report.Line("ok: %s", path)
			return nil
		}
		if !replace {
			ctx.Report.Line("skip existing: %s", path)
			return nil
		}
		if err := backupPath(ctx, path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureDir(ctx, filepath.Dir(path)); err != nil {
		return err
	}
	ctx.Report.Line("write: %s", path)
	if ctx.DryRun {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

func linkOrCopy(ctx Context, src, dst string, replace bool) error {
	if _, err := os.Lstat(dst); err == nil {
		if sameLink(dst, src) {
			ctx.Report.Line("ok: %s -> %s", dst, src)
			return nil
		}
		if !replace {
			ctx.Report.Line("skip existing: %s", dst)
			return nil
		}
		if err := backupAndRemove(ctx, dst); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ctx.DryRun {
		if ctx.CopyMode || runtime.GOOS == "windows" {
			ctx.Report.Line("copy: %s -> %s", src, dst)
			return nil
		}
		ctx.Report.Line("link: %s -> %s", dst, src)
		return nil
	}
	if ctx.CopyMode || runtime.GOOS == "windows" {
		return copyAny(ctx, src, dst)
	}
	ctx.Report.Line("link: %s -> %s", dst, src)
	return os.Symlink(src, dst)
}

func copyAny(ctx Context, src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(ctx, src, dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFileManaged(ctx, dst, data, true)
}

func copyDir(ctx Context, src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return ensureDir(ctx, target)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFileManaged(ctx, target, data, true)
	})
}

func backupAndRemove(ctx Context, path string) error {
	if err := backupPath(ctx, path); err != nil {
		return err
	}
	ctx.Report.Line("remove: %s", path)
	if ctx.DryRun {
		return nil
	}
	return os.RemoveAll(path)
}

func backupPath(ctx Context, path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	backup := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
	if !ctx.DryRun {
		backup = uniqueBackupPath(backup)
	}
	ctx.Report.Line("backup: %s -> %s", path, backup)
	if ctx.DryRun {
		return nil
	}
	return os.Rename(path, backup)
}

func uniqueBackupPath(path string) string {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func ensureDir(ctx Context, path string) error {
	if path == "" || path == "." {
		return nil
	}
	ctx.Report.Line("mkdir: %s", path)
	if ctx.DryRun {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func printPathStatus(ctx Context, path string) {
	if path == "" {
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		ctx.Report.Line("missing: %s", path)
		return
	}
	kind := "file"
	if info.IsDir() {
		kind = "dir"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(path)
		ctx.Report.Line("link: %s -> %s", path, target)
		return
	}
	ctx.Report.Line("ok %-4s %s", kind, path)
}

func checkJSON(ctx Context, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		ctx.Report.Line("invalid json: %s: %v", path, err)
		return
	}
	ctx.Report.Line("valid json: %s", path)
}

func embeddedRootFor(sourceRoot string) string {
	switch filepath.Base(sourceRoot) {
	case "agents":
		return "presets/subagents"
	default:
		return "presets/skills"
	}
}

func embeddedEntryNames(presets fs.FS, root string) ([]string, error) {
	entries, err := fs.ReadDir(presets, root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func mergeJSONAt(obj map[string]any, keyPath []string, values map[string]any) {
	cursor := obj
	for _, key := range keyPath {
		next, _ := cursor[key].(map[string]any)
		if next == nil {
			next = map[string]any{}
			cursor[key] = next
		}
		cursor = next
	}
	for name, value := range values {
		cursor[name] = value
	}
}

func replaceManagedBlock(current, begin, end, block string) string {
	start := strings.Index(current, begin)
	if start >= 0 {
		stop := strings.Index(current[start:], end)
		if stop >= 0 {
			stop = start + stop + len(end)
			next := strings.TrimRight(current[:start], "\n") + "\n" + block + strings.TrimLeft(current[stop:], "\n")
			return strings.TrimLeft(next, "\n")
		}
	}
	if strings.TrimSpace(current) == "" {
		return block
	}
	return strings.TrimRight(current, "\n") + "\n\n" + block
}

func shellWord(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
	}) == -1 {
		return value
	}
	return "'" + shellSingleQuotePayload(value) + "'"
}

func shellSingleQuotePayload(value string) string {
	return strings.ReplaceAll(value, "'", "'\"'\"'")
}

func sameLink(path, want string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	return target == want
}

func compact(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func artifactList(values []ArtifactKind) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, string(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
