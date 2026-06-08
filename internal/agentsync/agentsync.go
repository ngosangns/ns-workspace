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
	ConfigPath string
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

type OpenCodeConfigManifest struct {
	Permission any `json:"permission,omitempty"`
}

type RegistrySkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Skill  string `json:"skill"`
}

const registryAgentTarget = "universal"

type ArtifactKind string

const (
	ArtifactDirectory    ArtifactKind = "directory"
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
	UserConfig    UserConfig
	Report        StatusReporter
	Update        bool
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
	data, err := readPresetFile(ctx, op.Src)
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
	if op.Replace {
		if err := backupAndRemove(ctx, op.DstRoot); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	walkErr := fs.WalkDir(ctx.Presets, op.SrcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(op.SrcRoot, path)
		if err != nil || rel == "." {
			return err
		}
		dst := filepath.Join(op.DstRoot, rel)
		seen[filepath.ToSlash(rel)] = true
		if d.IsDir() {
			return ensureDir(ctx, dst)
		}
		data, err := readPresetFile(ctx, path)
		if err != nil {
			return err
		}
		return writeFileManaged(ctx, dst, data, op.Replace)
	})
	if walkErr != nil {
		return walkErr
	}
	// Materialize user additions under this tree root. The user config can
	// point at brand-new files (e.g. an extra skill) that do not exist in
	// the embedded presets, so the tree walk above never visits them.
	for _, rel := range ctx.UserConfig.EntriesUnder(op.SrcRoot) {
		if seen[rel] {
			continue
		}
		fullKey := filepath.ToSlash(filepath.Join(op.SrcRoot, rel))
		data, err := readPresetFileFromUser(ctx, fullKey)
		if err != nil {
			return err
		}
		dst := filepath.Join(op.DstRoot, filepath.FromSlash(rel))
		if err := ensureDir(ctx, filepath.Dir(dst)); err != nil {
			return err
		}
		if err := writeFileManaged(ctx, dst, data, op.Replace); err != nil {
			return err
		}
	}
	return nil
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
	if op.Replace {
		if err := backupAndRemove(ctx, op.DstRoot); err != nil {
			return err
		}
	}
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
	Replace bool
}

func (op MergeJSON) Apply(ctx Context) error {
	if len(op.Values) == 0 && !op.Replace {
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
	if op.Replace {
		replaceJSONAt(obj, op.KeyPath, op.Values)
	} else {
		mergeJSONAt(obj, op.KeyPath, op.Values)
	}
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

type AdapterTargets struct {
	Instruction  string
	Skills       string
	Subagents    string
	Settings     string
	HooksPath    string
	HooksKeyPath []string
	MCPPath      string
	MCPKeyPath   []string
}

type AdapterSpec struct {
	ID          string
	Aliases     []string
	Tier        SupportTier
	Docs        []string
	Notes       string
	Executables []string
	Targets     AdapterTargets
	Manual      bool
}

type AdapterPlugin any

type specAdapter struct {
	spec   AdapterSpec
	plugin AdapterPlugin
}

func (a specAdapter) Name() string { return a.spec.ID }

func (a specAdapter) Aliases() []string { return a.spec.Aliases }

func (a specAdapter) Capabilities() AgentCapabilities {
	artifacts := []ArtifactKind{}
	targets := a.spec.Targets
	if targets.Instruction != "" {
		artifacts = append(artifacts, ArtifactInstructions)
	}
	if targets.Skills != "" {
		artifacts = append(artifacts, ArtifactSkills)
	}
	if targets.Subagents != "" {
		artifacts = append(artifacts, ArtifactSubagents)
	}
	if targets.Settings != "" {
		artifacts = append(artifacts, ArtifactSettings)
	}
	if targets.Settings != "" || targets.HooksPath != "" {
		artifacts = append(artifacts, ArtifactHooks)
	}
	if targets.MCPPath != "" {
		artifacts = append(artifacts, ArtifactMCP)
	}
	if a.spec.Manual {
		artifacts = append(artifacts, ArtifactRules, ArtifactCommands)
	}
	caps := AgentCapabilities{Tier: a.spec.Tier, DocsURL: a.spec.Docs, Artifacts: artifacts, Notes: a.spec.Notes}
	if plugin, ok := a.plugin.(interface {
		ExtendCapabilities(AdapterSpec, AgentCapabilities) AgentCapabilities
	}); ok {
		caps = plugin.ExtendCapabilities(a.spec, caps)
	}
	return caps
}

// adapterSettingsProfile trả về profile path cho adapter nếu có trong manifest.
func (a specAdapter) adapterSettingsProfile(ctx Context) (string, error) {
	manifest, err := loadAdapterSettingsManifest(ctx)
	if err != nil {
		return "", err
	}
	return manifest[a.spec.ID], nil
}

// adapterSettingsHomeDir trả về user home directory để áp dụng settings
// profile. Target path trong profile là relative to home (vd
// ), nên khi apply cần resolve từ user home.
func adapterSettingsHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (a specAdapter) Plan(ctx Context, update bool) ([]Operation, error) {
	replace := update || ctx.Force
	ops := []Operation{}
	if a.spec.Manual {
		ops = append(ops, ManualStep{
			Agent: a.spec.ID,
			Dst:   filepath.Join(ctx.Options.AgentsDir, "generated", a.spec.ID, "README.md"),
			Text:  manualReadme(a.spec),
		})
		return ops, nil
	}

	sourceAgents := filepath.Join(ctx.Options.AgentsDir, "AGENTS.md")
	sourceSkills := filepath.Join(ctx.Options.AgentsDir, "skills")
	sourceSubagents := filepath.Join(ctx.Options.AgentsDir, "agents")
	targets := a.spec.Targets
	if targets.Instruction != "" {
		ops = append(ops, LinkOrCopy{Src: sourceAgents, Dst: targets.Instruction, Replace: replace})
	}
	if targets.Skills != "" {
		ops = append(ops, LinkSkillDirs{SrcRoot: sourceSkills, DstRoot: targets.Skills, Replace: replace})
	}
	if targets.Subagents != "" {
		ops = append(ops, LinkSkillDirs{SrcRoot: sourceSubagents, DstRoot: targets.Subagents, Replace: replace})
	}
	profilePath, err := a.adapterSettingsProfile(ctx)
	if err != nil {
		return nil, err
	}
	if profilePath != "" {
		homeDir, err := adapterSettingsHomeDir()
		if err != nil {
			return nil, err
		}
		targetPath, err := resolveAdapterSettingsTarget(ctx, profilePath, homeDir)
		if err != nil {
			return nil, err
		}
		ops = append(ops, ApplyAdapterSettings{ProfilePath: profilePath, TargetPath: targetPath, HomeDir: homeDir, Replace: replace})
	} else {
		if targets.Settings != "" {
			ops = append(ops, LinkOrCopy{Src: filepath.Join(ctx.Options.AgentsDir, "settings.json"), Dst: targets.Settings, Replace: replace})
		}
		if targets.HooksPath != "" && len(targets.HooksKeyPath) > 0 {
			manifest, err := readSettingsManifest(ctx)
			if err != nil {
				return nil, err
			}
			ops = append(ops, MergeJSON{Dst: targets.HooksPath, KeyPath: targets.HooksKeyPath, Values: manifest.Hooks, Replace: replace})
		}
	}
	if !ctx.NoMCP && targets.MCPPath != "" && len(targets.MCPKeyPath) > 0 {
		manifest, err := readMCPManifest(ctx)
		if err != nil {
			return nil, err
		}
		ops = append(ops, MergeJSON{Dst: targets.MCPPath, KeyPath: targets.MCPKeyPath, Values: manifest.MCPServers, Replace: replace})
	}
	if plugin, ok := a.plugin.(interface {
		ExtraOperations(Context, AdapterSpec, bool) ([]Operation, error)
	}); ok {
		extraOps, err := plugin.ExtraOperations(ctx, a.spec, update)
		if err != nil {
			return nil, err
		}
		ops = append(ops, extraOps...)
	}
	return ops, nil
}

func (a specAdapter) StatusPaths(ctx Context) []string {
	targets := a.spec.Targets
	paths := []string{targets.Instruction, targets.Skills, targets.Subagents, targets.Settings, targets.HooksPath, targets.MCPPath}
	if a.spec.Manual {
		paths = append(paths, filepath.Join(ctx.Options.AgentsDir, "generated", a.spec.ID, "README.md"))
	}
	if profilePath, err := a.adapterSettingsProfile(ctx); err == nil && profilePath != "" {
		homeDir, _ := adapterSettingsHomeDir()
		if targetPath, err := resolveAdapterSettingsTarget(ctx, profilePath, homeDir); err == nil {
			paths = append(paths, ApplyAdapterSettings{ProfilePath: profilePath, TargetPath: targetPath, HomeDir: homeDir}.Path())
		}
	}
	if plugin, ok := a.plugin.(interface {
		ExtraStatusPaths(Context, AdapterSpec) []string
	}); ok {
		paths = append(paths, plugin.ExtraStatusPaths(ctx, a.spec)...)
	}
	return compact(paths)
}

func (a specAdapter) DoctorExecutables() []string { return a.spec.Executables }

type opencodePlugin struct {
	ConfigPath string
}

func (p opencodePlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

func (p opencodePlugin) ExtraOperations(ctx Context, _ AdapterSpec, update bool) ([]Operation, error) {
	replace := update || ctx.Force
	configValues := map[string]any{}
	if !ctx.NoMCP && p.ConfigPath != "" {
		manifest, err := readMCPManifest(ctx)
		if err != nil {
			return nil, err
		}
		manifest = opencodeMCPManifest(manifest)
		configValues["mcp"] = manifest.MCPServers
	}
	presetValues, err := readOpenCodeConfigValues(ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range presetValues {
		// `permission` has a typed struct field for legacy readers; copy
		// the value verbatim so user-defined key shape (string or object)
		// survives untouched.
		configValues[key] = value
	}
	if len(configValues) > 0 {
		// Update rewrites the managed OpenCode config object so removed preset keys
		// do not survive indefinitely in the native config file.
		return []Operation{MergeJSON{Dst: p.ConfigPath, KeyPath: []string{}, Values: configValues, Replace: replace && !ctx.NoMCP}}, nil
	}
	return nil, nil
}

func (p opencodePlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string {
	if p.ConfigPath == "" {
		return nil
	}
	return []string{p.ConfigPath}
}

type claudePlugin struct{}

func (p claudePlugin) ExtraOperations(ctx Context, _ AdapterSpec, update bool) ([]Operation, error) {
	if ctx.NoMCP {
		return nil, nil
	}
	script, err := mcpCommandScript(ctx, "claude", func(name string, server string) string {
		return fmt.Sprintf("claude mcp add-json %s '%s' --scope user\n", shellWord(name), shellSingleQuotePayload(server))
	})
	if err != nil {
		return nil, err
	}
	return []Operation{WriteFile{
		Dst:     filepath.Join(ctx.Options.AgentsDir, "generated", "claude", "mcp.commands.sh"),
		Data:    []byte(script),
		Replace: update || ctx.Force,
	}}, nil
}

type codexPlugin struct{}

func (p codexPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactMCP)
	return caps
}

func (p codexPlugin) ExtraOperations(ctx Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	if ctx.NoMCP {
		return nil, nil
	}
	manifest, err := readMCPManifest(ctx)
	if err != nil {
		return nil, err
	}
	if len(manifest.MCPServers) == 0 {
		return nil, nil
	}
	return []Operation{AppendManagedBlock{
		Dst:     filepath.Join(ctx.Home, ".codex", "config.toml"),
		Label:   "mcp",
		Content: codexMCPBlock(manifest),
		Replace: true,
	}}, nil
}

func (p codexPlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	return []string{filepath.Join(ctx.Home, ".codex", "config.toml")}
}

type aiderPlugin struct{}

func (p aiderPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactSettings, ArtifactRules)
	return caps
}

func (p aiderPlugin) ExtraOperations(ctx Context, _ AdapterSpec, _ bool) ([]Operation, error) {
	return []Operation{AppendManagedBlock{
		Dst: filepath.Join(ctx.Home, ".aider.conf.yml"), Label: "conventions",
		Content: "read: " + filepath.ToSlash(filepath.Join(ctx.Options.AgentsDir, "AGENTS.md")),
		Replace: true,
	}}, nil
}

func (p aiderPlugin) ExtraStatusPaths(ctx Context, _ AdapterSpec) []string {
	return []string{filepath.Join(ctx.Home, ".aider.conf.yml")}
}

// minimaxPlugin writes default model + region presets into
// ~/.mmx/config.json. mmx-cli does not have a user-level skills / agents /
// MCP directory concept, so the adapter only manages the JSON config file
// via MergeJSON. The same default values are documented in the bundled
// `presets/skills/minimax-cli/SKILL.md` for AI agents that invoke mmx.
type minimaxPlugin struct {
	ConfigPath string
}

func (p minimaxPlugin) ExtendCapabilities(_ AdapterSpec, caps AgentCapabilities) AgentCapabilities {
	caps.Artifacts = append(caps.Artifacts, ArtifactSettings)
	return caps
}

func (p minimaxPlugin) ExtraOperations(ctx Context, _ AdapterSpec, update bool) ([]Operation, error) {
	if p.ConfigPath == "" {
		return nil, nil
	}
	replace := update || ctx.Force
	values, err := readPresetFile(ctx, "presets/minimax/config.json")
	if err != nil {
		return nil, err
	}
	parsed := map[string]any{}
	if err := json.Unmarshal(values, &parsed); err != nil {
		return nil, fmt.Errorf("presets/minimax/config.json: %w", err)
	}
	if len(parsed) == 0 {
		return nil, nil
	}
	return []Operation{MergeJSON{Dst: p.ConfigPath, KeyPath: []string{}, Values: parsed, Replace: replace}}, nil
}

func (p minimaxPlugin) ExtraStatusPaths(_ Context, _ AdapterSpec) []string {
	if p.ConfigPath == "" {
		return nil
	}
	return []string{p.ConfigPath}
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
	ctx.Update = update
	mode := "init"
	if update {
		mode = "update"
	}
	plan, err := m.buildPlan(ctx, update)
	if err != nil {
		return err
	}
	ctx.Report.Line("%s shared agent config at %s", mode, ctx.Options.AgentsDir)
	if err := plan.Apply(ctx); err != nil {
		return err
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
	if err := writeRegistryHelpers(ctx, true); err != nil {
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
	userCfg, err := loadUserConfig(opt)
	if err != nil {
		return Context{}, err
	}
	return Context{Options: opt, Home: home, XDGConfigHome: xdg, Presets: m.Presets, UserConfig: userCfg, Report: stdoutReporter{}}, nil
}

func (m Manager) adapters(ctx Context) []AgentAdapter {
	home := ctx.Home
	xdg := ctx.XDGConfigHome
	kiroRoot := ExpandPath(os.Getenv("KIRO_HOME"))
	if kiroRoot == "" {
		kiroRoot = filepath.Join(home, ".kiro")
	}
	return []AgentAdapter{
		specAdapter{spec: AdapterSpec{ID: "claude", Tier: TierStable, Executables: []string{"claude"}, Targets: AdapterTargets{Instruction: filepath.Join(home, ".claude", "CLAUDE.md"), Skills: filepath.Join(home, ".claude", "skills"), Subagents: filepath.Join(home, ".claude", "agents"), Settings: filepath.Join(home, ".claude", "settings.json")}, Docs: []string{"https://docs.claude.com/en/docs/claude-code/settings", "https://docs.claude.com/en/docs/claude-code/mcp"}}, plugin: claudePlugin{}},
		specAdapter{spec: AdapterSpec{ID: "opencode", Tier: TierStable, Executables: []string{"opencode"}, Targets: AdapterTargets{Instruction: filepath.Join(xdg, "opencode", "AGENTS.md"), Skills: filepath.Join(xdg, "opencode", "skill"), Subagents: filepath.Join(xdg, "opencode", "agent")}, Docs: []string{"https://opencode.ai/docs/config/", "https://opencode.ai/docs/agents/", "https://opencode.ai/docs/mcp-servers/"}}, plugin: opencodePlugin{ConfigPath: filepath.Join(xdg, "opencode", "opencode.json")}},
		specAdapter{spec: AdapterSpec{ID: "grok", Tier: TierStable, Executables: []string{"grok"}, Targets: AdapterTargets{Skills: filepath.Join(home, ".grok", "skills")}, Docs: []string{"https://docs.x.ai/build/overview", "https://docs.x.ai/build/features/skills-plugins-marketplaces"}, Notes: "Grok Build reads AGENTS.md from projects and also discovers ~/.agents/skills; this adapter mirrors shared skills into ~/.grok/skills for native slash-command discovery."}},
		specAdapter{spec: AdapterSpec{ID: "kimi", Tier: TierStable, Executables: []string{"kimi"}, Targets: AdapterTargets{Instruction: filepath.Join(home, ".kimi", "AGENTS.md"), Skills: filepath.Join(home, ".kimi", "skills"), MCPPath: filepath.Join(home, ".kimi", "mcp.json"), MCPKeyPath: []string{"mcpServers"}}, Docs: []string{"https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html"}}},
		specAdapter{spec: AdapterSpec{ID: "kiro", Aliases: []string{"kiro-cli"}, Tier: TierStable, Executables: []string{"kiro", "kiro-cli"}, Targets: AdapterTargets{Instruction: filepath.Join(kiroRoot, "steering", "AGENTS.md"), Skills: filepath.Join(kiroRoot, "skills"), MCPPath: filepath.Join(kiroRoot, "settings", "mcp.json"), MCPKeyPath: []string{"mcpServers"}}, Docs: []string{"https://kiro.dev/docs/cli/chat/configuration/", "https://kiro.dev/docs/cli/mcp/", "https://kiro.dev/docs/cli/reference/settings/", "https://kiro.dev/docs/cli/skills/"}, Notes: "Kiro CLI alias: kiro-cli. Shared instructions sync to global steering; skills sync to Kiro global skills; MCP presets sync to the shared Kiro settings path."}},
		specAdapter{spec: AdapterSpec{ID: "qwen", Tier: TierStable, Executables: []string{"qwen"}, Targets: AdapterTargets{Instruction: filepath.Join(home, ".qwen", "QWEN.md"), Skills: filepath.Join(home, ".qwen", "skills"), HooksPath: filepath.Join(home, ".qwen", "settings.json"), HooksKeyPath: []string{"hooks"}, MCPPath: filepath.Join(home, ".qwen", "settings.json"), MCPKeyPath: []string{"mcpServers"}}, Docs: []string{"https://qwenlm.github.io/qwen-code-docs/en/cli/configuration/", "https://qwenlm.github.io/qwen-code-docs/en/users/features/mcp/"}}},
		specAdapter{spec: AdapterSpec{ID: "gemini", Tier: TierStable, Executables: []string{"gemini"}, Targets: AdapterTargets{Instruction: filepath.Join(home, ".gemini", "GEMINI.md"), Skills: filepath.Join(home, ".gemini", "skills"), HooksPath: filepath.Join(home, ".gemini", "settings.json"), HooksKeyPath: []string{"hooks"}, MCPPath: filepath.Join(home, ".gemini", "settings.json"), MCPKeyPath: []string{"mcpServers"}}, Docs: []string{"https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md"}}},
		specAdapter{spec: AdapterSpec{ID: "codex", Tier: TierStable, Executables: []string{"codex"}, Targets: AdapterTargets{Instruction: filepath.Join(home, ".codex", "AGENTS.md"), Skills: filepath.Join(home, ".codex", "skills")}, Docs: []string{"https://github.com/openai/codex/blob/main/docs/config.md", "https://github.com/openai/codex/blob/main/docs/agents_md.md"}}, plugin: codexPlugin{}},
		specAdapter{spec: AdapterSpec{ID: "cline", Tier: TierStable, Executables: []string{"cline"}, Targets: AdapterTargets{Skills: filepath.Join(home, ".cline", "data", "skills"), Subagents: filepath.Join(home, ".cline", "data", "agents"), MCPPath: filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"), MCPKeyPath: []string{"mcpServers"}}, Docs: []string{"https://docs.cline.bot/cline-cli/configuration"}}},
		specAdapter{spec: AdapterSpec{ID: "windsurf", Tier: TierStable, Targets: AdapterTargets{Instruction: filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md")}, Docs: []string{"https://docs.windsurf.com/windsurf/cascade/memories"}}},
		specAdapter{spec: AdapterSpec{ID: "aider", Tier: TierStable, Executables: []string{"aider"}, Docs: []string{"https://aider.chat/docs/config/aider_conf.html", "https://aider.chat/docs/usage/conventions.html"}}, plugin: aiderPlugin{}},
		specAdapter{spec: AdapterSpec{ID: "minimax", Aliases: []string{"minimax-cli", "mmx"}, Tier: TierStable, Executables: []string{"mmx"}, Docs: []string{"https://platform.minimax.io/docs/token-plan/minimax-cli", "https://github.com/MiniMax-AI/cli"}, Notes: "MiniMax CLI (mmx) is a multimodal content-generation CLI (text/image/video/speech/music). Adapter writes default model and region presets to ~/.mmx/config.json via MergeJSON. The shared skills/ and agents/ fan-out does not apply because mmx-cli does not expose a user-level skills or subagents directory; use the bundled `presets/skills/minimax-cli/SKILL.md` from a coding agent to teach it the mmx surface."}, plugin: minimaxPlugin{ConfigPath: filepath.Join(home, ".mmx", "config.json")}},
		specAdapter{spec: AdapterSpec{ID: "cursor", Tier: TierManual, Executables: []string{"cursor-agent"}, Manual: true, Docs: []string{"https://docs.cursor.com/en/context", "https://docs.cursor.com/cli/mcp"}, Notes: "Cursor user rules are stored through Cursor settings; generated helper only."}},
		specAdapter{spec: AdapterSpec{ID: "github-copilot", Tier: TierManual, Manual: true, Docs: []string{"https://code.visualstudio.com/docs/copilot/customization/custom-instructions"}, Notes: "Copilot instructions are repo/editor scoped; generated helper only."}},
		specAdapter{spec: AdapterSpec{ID: "jetbrains", Tier: TierManual, Manual: true, Docs: []string{"https://www.jetbrains.com/help/ai-assistant/mcp.html"}, Notes: "JetBrains AI MCP setup is product/version specific."}},
		specAdapter{spec: AdapterSpec{ID: "antigravity", Tier: TierExperimental, Manual: true, Notes: "No stable official user-level filesystem path confirmed yet."}},
		specAdapter{spec: AdapterSpec{ID: "trae", Tier: TierExperimental, Executables: []string{"trae"}, Manual: true, Notes: "No stable official user-level filesystem path confirmed yet."}},
		specAdapter{spec: AdapterSpec{ID: "roo", Tier: TierExperimental, Manual: true, Notes: "Roo Code support is guarded because the project status is unstable."}},
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
	if !ctx.Update {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return manifest, err
			}
			return manifest, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return manifest, err
		}
	}
	data, err := readPresetFile(ctx, "presets/mcp/servers.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func readOpenCodeConfigManifest(ctx Context) (OpenCodeConfigManifest, error) {
	var manifest OpenCodeConfigManifest
	data, err := readPresetFile(ctx, "presets/opencode/opencode.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

// readOpenCodeConfigValues returns the full opencode preset as a generic
// map so user-defined keys (timeout, provider, etc.) flow through to the
// native config alongside the canonical `mcp` and `permission` keys.
// `mcp` is intentionally stripped here because the opencode plugin layers
// the shared MCP manifest on top after this call.
func readOpenCodeConfigValues(ctx Context) (map[string]any, error) {
	data, err := readPresetFile(ctx, "presets/opencode/opencode.json")
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	delete(values, "mcp")
	return values, nil
}

func opencodeMCPManifest(manifest MCPManifest) MCPManifest {
	out := MCPManifest{MCPServers: map[string]any{}}
	for name, value := range manifest.MCPServers {
		server, ok := value.(map[string]any)
		if !ok {
			out.MCPServers[name] = value
			continue
		}
		next := map[string]any{}
		for key, serverValue := range server {
			next[key] = serverValue
		}
		// OpenCode uses "remote" for URL-backed MCP servers; shared presets keep
		// "http" for other agent config formats.
		if typ, _ := next["type"].(string); typ == "http" {
			next["type"] = "remote"
		}
		out.MCPServers[name] = next
	}
	return out
}

func readSettingsManifest(ctx Context) (SettingsManifest, error) {
	var manifest SettingsManifest
	path := filepath.Join(ctx.Options.AgentsDir, "settings.json")
	if !ctx.Update {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return manifest, err
			}
			if manifest.Hooks == nil {
				manifest.Hooks = map[string]any{}
			}
			return manifest, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return manifest, err
		}
	}
	data, err := readPresetFile(ctx, "presets/settings/default.json")
	if err != nil {
		return manifest, err
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
	manifest, err := readRegistryManifest(ctx)
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
	script.WriteString(fmt.Sprintf("# Install registry-managed skills. Custom skills live in %s.\n", filepath.Join(ctx.Options.AgentsDir, "skills")))
	for _, skill := range manifest.Skills {
		script.WriteString(registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir))
		script.WriteString("\n")
	}
	if err := writeFileManaged(ctx, filepath.Join(registryDir, "install.sh"), []byte(script.String()), replace); err != nil {
		return err
	}
	installScript := filepath.Join(ctx.Options.AgentsDir, "registry", "install.sh")
	readme := fmt.Sprintf("# Registry Skills\n\nThese skills are installed from the public Skills registry so updates can come from upstream.\n\nRun:\n\n```bash\nsh %s\n```\n", shellWord(installScript))
	return writeFileManaged(ctx, filepath.Join(registryDir, "README.md"), []byte(readme), replace)
}

func writeMCPReadme(ctx Context, replace bool) error {
	readme := "# Shared MCP Presets\n\n`servers.json` is the source of truth for personal MCP servers.\n\nStable file-based adapters merge these presets into their native user-level config. CLI-backed or UI-backed tools get generated helper files under `~/.agents/generated/<agent>/`.\n"
	return writeFileManaged(ctx, filepath.Join(ctx.Options.AgentsDir, "mcp", "README.md"), []byte(readme), replace)
}

func readRegistryManifest(ctx Context) (RegistryManifest, error) {
	var manifest RegistryManifest
	data, err := readPresetFile(ctx, "presets/registry/skills.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func installRegistrySkills(ctx Context) error {
	manifest, err := readRegistryManifest(ctx)
	if err != nil {
		return err
	}
	if len(manifest.Skills) == 0 {
		return nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		installScript := filepath.Join(ctx.Options.AgentsDir, "registry", "install.sh")
		return fmt.Errorf("npx is required to install registry skills; rerun with --no-registry or run %s later", installScript)
	}
	for _, skill := range manifest.Skills {
		args := registryCommandArgs(skill, true, ctx.CopyMode)
		ctx.Report.Line("registry: %s from %s@%s", skill.Name, skill.Source, skill.Skill)
		if ctx.DryRun {
			ctx.Report.Line("run: %s", registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir))
			continue
		}
		cmd := exec.Command("npx", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		// Keep registry installs aligned with --agents-home instead of the CLI default.
		cmd.Env = append(os.Environ(), "AGENTS_HOME="+ctx.Options.AgentsDir)
		if err := cmd.Run(); err != nil {
			ctx.Report.Line("warning: registry skill %s failed: %v", skill.Name, err)
			continue
		}
	}
	return nil
}

func registryCommand(skill RegistrySkill, global bool, copyMode bool, agentsDir string) string {
	args := registryCommandArgs(skill, global, copyMode)
	parts := []string{fmt.Sprintf("AGENTS_HOME=%s", shellWord(agentsDir)), "npx"}
	for _, arg := range args {
		parts = append(parts, shellWord(arg))
	}
	return strings.Join(parts, " ")
}

func registryCommandArgs(skill RegistrySkill, global bool, copyMode bool) []string {
	args := []string{"--yes", "skills", "add", skill.Source, "--skill", skill.Skill}
	if global {
		args = append(args, "--global")
	}
	// Install into the shared universal skills home; ns-workspace owns adapter fan-out.
	args = append(args, "--agent", registryAgentTarget, "--yes")
	if copyMode {
		args = append(args, "--copy")
	}
	return args
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

func manualReadme(a AdapterSpec) string {
	var b strings.Builder
	b.WriteString("# " + a.ID + " manual setup\n\n")
	if a.Notes != "" {
		b.WriteString(a.Notes + "\n\n")
	}
	b.WriteString("Shared source files live in `~/.agents`:\n\n")
	b.WriteString("- `~/.agents/AGENTS.md`\n")
	b.WriteString("- `~/.agents/skills/`\n")
	b.WriteString("- `~/.agents/agents/`\n")
	b.WriteString("- `~/.agents/mcp/servers.json`\n\n")
	if len(a.Docs) > 0 {
		b.WriteString("Docs:\n\n")
		for _, url := range a.Docs {
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

func replaceJSONAt(obj map[string]any, keyPath []string, values map[string]any) {
	if len(keyPath) == 0 {
		for name := range obj {
			delete(obj, name)
		}
		for name, value := range values {
			obj[name] = value
		}
		return
	}
	cursor := obj
	for _, key := range keyPath[:len(keyPath)-1] {
		next, _ := cursor[key].(map[string]any)
		if next == nil {
			next = map[string]any{}
			cursor[key] = next
		}
		cursor = next
	}
	leaf := map[string]any{}
	for name, value := range values {
		leaf[name] = value
	}
	cursor[keyPath[len(keyPath)-1]] = leaf
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
