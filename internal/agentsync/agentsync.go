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
	"strings"
)

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
	// Materialize MCP servers as pure JSON (enabled only). Legacy portal
	// overlays may still store disabled servers as // comments; strip them.
	// Disabled entries now live in presets/mcp/servers.disabled.json and are
	// never written to ~/.agents/mcp/servers.json.
	if filepath.ToSlash(op.Src) == MCPEnabledPath {
		enabled, _, _, err := ParseMCPServersJSONC(data)
		if err != nil {
			return err
		}
		data, err = json.MarshalIndent(MCPManifest{MCPServers: enabled}, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
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
	managed := map[string]bool{}
	// Portal-disabled skill top-level dirs are skipped (and removed on update).
	disabledSkills := map[string]bool{}
	if filepath.ToSlash(op.SrcRoot) == "presets/skills" {
		disabledSkills = loadDisabledSkills(ctx)
	}
	walkErr := fs.WalkDir(ctx.Presets, op.SrcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(op.SrcRoot, path)
		if err != nil || rel == "." {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if skillTop := skillTopLevelName(relSlash); skillTop != "" && disabledSkills[skillTop] {
			if d.IsDir() && skillTop == relSlash {
				return fs.SkipDir
			}
			return nil
		}
		dst := filepath.Join(op.DstRoot, rel)
		managed[relSlash] = true
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
	for _, rel := range ctx.UserConfig.EntriesUnder(op.SrcRoot) {
		if managed[rel] {
			continue
		}
		if skillTop := skillTopLevelName(rel); skillTop != "" && disabledSkills[skillTop] {
			continue
		}
		managed[rel] = true
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
	if op.Replace {
		if err := removeStaleEntries(ctx, op.DstRoot, managed); err != nil {
			return err
		}
	}
	return nil
}

// skillTopLevelName returns the first path segment for skill tree entries
// (e.g. "commit/SKILL.md" → "commit"). Empty for root-only names that are
// not skill dirs (e.g. catalog files could be skipped by callers).
func skillTopLevelName(rel string) string {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return ""
	}
	parts := strings.SplitN(rel, "/", 2)
	name := parts[0]
	if name == "_shared" || strings.HasPrefix(name, ".") {
		return ""
	}
	return name
}

func removeStaleEntries(ctx Context, dstRoot string, managed map[string]bool) error {
	entries, err := os.ReadDir(dstRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return removeStaleRecursive(ctx, dstRoot, "", entries, managed)
}

func removeStaleRecursive(ctx Context, root, relPrefix string, entries []os.DirEntry, managed map[string]bool) error {
	for _, entry := range entries {
		rel := entry.Name()
		if relPrefix != "" {
			rel = relPrefix + "/" + entry.Name()
		}
		fullPath := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			subEntries, err := os.ReadDir(fullPath)
			if err != nil {
				return err
			}
			if err := removeStaleRecursive(ctx, fullPath, rel, subEntries, managed); err != nil {
				return err
			}
			remaining, _ := os.ReadDir(fullPath)
			if len(remaining) == 0 {
				hasManagedChild := false
				for key := range managed {
					if strings.HasPrefix(key, rel+"/") {
						hasManagedChild = true
						break
					}
				}
				if !hasManagedChild {
					ctx.Report.Line("remove empty dir: %s", fullPath)
					if !ctx.DryRun {
						if err := os.Remove(fullPath); err != nil {
							ctx.Report.Line("warning: remove empty dir failed: %v", err)
						}
					}
				}
			}
			continue
		}
		if !managed[rel] {
			ctx.Report.Line("remove stale: %s", fullPath)
			if !ctx.DryRun {
				if err := os.Remove(fullPath); err != nil {
					ctx.Report.Line("warning: remove stale failed: %v", err)
				}
			}
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
		if err := removePath(ctx, op.DstRoot); err != nil {
			return err
		}
	}
	if err := ensureDir(ctx, op.DstRoot); err != nil {
		return err
	}
	// Every preset skill/subagent must override the matching entry in the
	// provider target, regardless of init vs update mode. In update mode the
	// whole DstRoot is wiped above, so the per-entry replace is a no-op there;
	// in init mode it guarantees a stale provider-target skill with the same
	// name is replaced by the preset version instead of skipped.
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
			if err := linkOrCopy(ctx, filepath.Join(op.SrcRoot, name), filepath.Join(op.DstRoot, name), true); err != nil {
				return err
			}
		}
		return nil
	}
	for _, entry := range entries {
		if err := linkOrCopy(ctx, filepath.Join(op.SrcRoot, entry.Name()), filepath.Join(op.DstRoot, entry.Name()), true); err != nil {
			return err
		}
	}
	return nil
}

func (op LinkSkillDirs) Describe(ctx Context) {
	ctx.Report.Line("skills: %s -> %s", op.SrcRoot, op.DstRoot)
}
func (op LinkSkillDirs) Path() string { return op.DstRoot }

// CleanupManagedLinks removes directory entries under DstRoot that are
// symlinks pointing into SharedRoot (the shared ~/.agents/skills or
// ~/.agents/agents tree). Real directories and unrelated links are left
// alone so user-owned content is never deleted.
type CleanupManagedLinks struct {
	DstRoot    string
	SharedRoot string
}

func (op CleanupManagedLinks) Apply(ctx Context) error {
	if op.DstRoot == "" || op.SharedRoot == "" {
		return nil
	}
	entries, err := os.ReadDir(op.DstRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	shared := filepath.Clean(op.SharedRoot)
	for _, entry := range entries {
		path := filepath.Join(op.DstRoot, entry.Name())
		target, err := os.Readlink(path)
		if err != nil {
			// Not a symlink (or unreadable): leave in place.
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		target = filepath.Clean(target)
		if target != shared && !strings.HasPrefix(target, shared+string(os.PathSeparator)) {
			continue
		}
		if ctx.DryRun {
			ctx.Report.Line("cleanup managed link: %s", path)
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		ctx.Report.Line("cleanup managed link: %s", path)
	}
	return nil
}

func (op CleanupManagedLinks) Describe(ctx Context) {
	ctx.Report.Line("cleanup managed links: %s (from %s)", op.DstRoot, op.SharedRoot)
}
func (op CleanupManagedLinks) Path() string { return op.DstRoot }

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
	// CleanupTables, when non-empty, removes any existing TOML table
	// sections whose header matches [mcp_servers.<name>] (or
	// [mcp_servers."<name>"]) for the listed names before the managed
	// block is inserted. This prevents duplicate-key errors when a
	// previous config (user-written or vendor-generated) already defines
	// the same MCP servers outside the managed block.
	CleanupTables []string
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
	if len(op.CleanupTables) > 0 {
		current = removeTOMLTables(current, "mcp_servers", op.CleanupTables)
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

// adapterSettingsProfile trả về profile path cho adapter nếu có trong manifest.

// adapterSettingsHomeDir trả về user home directory để áp dụng settings
// profile. Target path trong profile là relative to home (vd
// ), nên khi apply cần resolve từ user home.

type claudePlugin struct{}

type codexPlugin struct{}

func (m Manager) Apply(opt Options, update bool) error {
	ctx, err := m.context(opt)
	if err != nil {
		return err
	}
	return m.apply(ctx, update)
}

func (m Manager) apply(ctx Context, update bool) error {
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
	return m.status(ctx)
}

func (m Manager) status(ctx Context) error {
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
	return m.doctor(ctx)
}

func (m Manager) doctor(ctx Context) error {
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
	return m.installRegistrySkills(ctx)
}

func (m Manager) installRegistrySkills(ctx Context) error {
	if err := writeRegistryHelpers(ctx, true); err != nil {
		return err
	}
	return installRegistrySkills(ctx)
}

// ContextWithReporter builds a Context for the given options with a custom
// status reporter. It is exported for the portal UI so sync output can be
// streamed to the frontend instead of stdout.
func (m Manager) ContextWithReporter(opt Options, report StatusReporter) (Context, error) {
	ctx, err := m.context(opt)
	if err != nil {
		return Context{}, err
	}
	ctx.Report = report
	return ctx, nil
}

// ApplyWithContext runs Apply using an already-built Context.
func (m Manager) ApplyWithContext(ctx Context, update bool) error {
	return m.apply(ctx, update)
}

// StatusWithContext runs Status using an already-built Context.
func (m Manager) StatusWithContext(ctx Context) error {
	return m.status(ctx)
}

// DoctorWithContext runs Doctor using an already-built Context.
func (m Manager) DoctorWithContext(ctx Context) error {
	return m.doctor(ctx)
}

// InstallRegistrySkillsWithContext runs registry skill installation using an
// already-built Context.
func (m Manager) InstallRegistrySkillsWithContext(ctx Context) error {
	return m.installRegistrySkills(ctx)
}

func (m Manager) context(opt Options) (Context, error) {
	home, err := userHomeDir()
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
	ctx := Context{Options: opt, Home: home, XDGConfigHome: xdg, Presets: m.Presets, UserConfig: userCfg, Report: stdoutReporter{}, manifestCache: map[string]any{}, seenDirs: map[string]bool{}}
	applyPortalToggles(&ctx)
	return ctx, nil
}

// managerAdaptersFn is a seam test: lets tests inject a custom adapter
// catalog (e.g. one with duplicate executables) to cover dedup branches.
var managerAdaptersFn = defaultManagerAdapters

func defaultManagerAdapters(ctx Context) []Adapter {
	kiroRoot := ExpandPath(os.Getenv("KIRO_HOME"))
	return NewAdapterRegistry(RegistryOptions{
		Home:          ctx.Home,
		XDGConfigHome: ctx.XDGConfigHome,
		KiroHome:      kiroRoot,
	}).All()
}

func (m Manager) adapters(ctx Context) []Adapter {
	return managerAdaptersFn(ctx)
}

// readOpenCodeConfigValues returns the full opencode preset as a generic
// map so user-defined keys (timeout, provider, etc.) flow through to the
// native config alongside the canonical `mcp` and `permission` keys.
// `mcp` is intentionally stripped here because the opencode plugin layers
// the shared MCP manifest on top after this call.

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
