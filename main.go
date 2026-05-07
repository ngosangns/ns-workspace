package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
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

//go:embed presets/agents presets/mcp presets/registry presets/settings presets/skills/* presets/subagents
var presetFS embed.FS

type options struct {
	command    string
	agentsDir  string
	dryRun     bool
	yes        bool
	force      bool
	copyMode   bool
	noMCP      bool
	noRegistry bool
	toolFilter map[string]bool
}

type mcpManifest struct {
	MCPServers map[string]any `json:"mcpServers"`
}

type registryManifest struct {
	Skills []registrySkill `json:"skills"`
}

type registrySkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Skill  string `json:"skill"`
}

type toolAdapter struct {
	Name             string
	InstructionPath  string
	SkillPath        string
	NativeReadsAgent bool
	MCPPath          string
	MCPFormat        string
}

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
	case "init", "update", "status", "doctor", "registry", "preview":
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", cmd)
	}
	if cmd == "preview" {
		return runPreview(args[1:])
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	homeDefault, err := defaultAgentsDir()
	if err != nil {
		return err
	}
	opt := options{command: cmd, agentsDir: homeDefault}
	tools := fs.String("tools", "all", "comma-separated tools: all,claude,opencode,kimi,qwen,cursor,trae")
	fs.StringVar(&opt.agentsDir, "agents-home", homeDefault, "shared agents home")
	fs.BoolVar(&opt.dryRun, "dry-run", false, "show planned writes without changing files")
	fs.BoolVar(&opt.yes, "yes", false, "skip interactive confirmations")
	fs.BoolVar(&opt.force, "force", false, "replace existing files during init")
	fs.BoolVar(&opt.copyMode, "copy", false, "copy files instead of creating symlinks")
	fs.BoolVar(&opt.noMCP, "no-mcp", false, "skip MCP configuration")
	fs.BoolVar(&opt.noRegistry, "no-registry", false, "skip skills registry installation")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	opt.toolFilter = parseTools(*tools)

	switch cmd {
	case "init":
		return apply(opt, false)
	case "update":
		return apply(opt, true)
	case "status":
		return status(opt)
	case "doctor":
		return doctor(opt)
	case "registry":
		return installRegistrySkills(opt)
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
  go run github.com/ngosangns/ns-workspace@latest preview [flags]

Flags:
  --agents-home PATH   shared home, default ~/.agents
  --tools LIST         all,claude,opencode,kimi,qwen,cursor,trae
  --dry-run           print actions without writing
  --force             replace existing files during init
  --copy              copy instead of symlink
  --no-mcp            skip MCP config
  --no-registry       skip skills registry installation

Preview flags:
  --project PATH      project root to inspect, default current directory
  --specs-dir PATH    specs directory, default specs
  --addr HOST:PORT    local server address, default 127.0.0.1:8787
  --open              open browser after the server starts`)
}

func apply(opt options, update bool) error {
	opt.agentsDir = expandPath(opt.agentsDir)
	mode := "init"
	if update {
		mode = "update"
	}
	fmt.Printf("%s shared agent config at %s\n", mode, opt.agentsDir)

	agentsFile := filepath.Join(opt.agentsDir, "AGENTS.md")
	skillsDir := filepath.Join(opt.agentsDir, "skills")
	subagentsDir := filepath.Join(opt.agentsDir, "agents")
	mcpFile := filepath.Join(opt.agentsDir, "mcp", "servers.json")
	settingsFile := filepath.Join(opt.agentsDir, "settings.json")

	if err := ensureDir(opt, opt.agentsDir); err != nil {
		return err
	}
	if err := ensureDir(opt, skillsDir); err != nil {
		return err
	}
	if err := ensureDir(opt, subagentsDir); err != nil {
		return err
	}
	if err := ensureDir(opt, filepath.Dir(mcpFile)); err != nil {
		return err
	}

	if err := installPresetFile(opt, "presets/agents/AGENTS.md", agentsFile, update); err != nil {
		return err
	}
	if err := installPresetTree(opt, "presets/skills", skillsDir, update); err != nil {
		return err
	}
	if err := installPresetTree(opt, "presets/subagents", subagentsDir, update); err != nil {
		return err
	}
	if err := installPresetFile(opt, "presets/settings/settings.json", settingsFile, update); err != nil {
		return err
	}
	if err := writeRegistryHelpers(opt, update); err != nil {
		return err
	}
	if !opt.noRegistry {
		if err := installRegistrySkills(opt); err != nil {
			return err
		}
	}
	if !opt.noMCP {
		if err := installPresetFile(opt, "presets/mcp/servers.json", mcpFile, update); err != nil {
			return err
		}
		if err := writeMCPHelpers(opt, update); err != nil {
			return err
		}
	}

	adapters, err := adapters(opt.agentsDir)
	if err != nil {
		return err
	}
	for _, adapter := range adapters {
		if !selected(opt, adapter.Name) {
			continue
		}
		if err := installAdapter(opt, adapter, update); err != nil {
			return err
		}
	}

	fmt.Println("done")
	return nil
}

func installAdapter(opt options, adapter toolAdapter, update bool) error {
	sourceAgents := filepath.Join(opt.agentsDir, "AGENTS.md")
	sourceSkills := filepath.Join(opt.agentsDir, "skills")

	if adapter.InstructionPath != "" {
		if err := ensureDir(opt, filepath.Dir(adapter.InstructionPath)); err != nil {
			return err
		}
		if err := linkOrCopy(opt, sourceAgents, adapter.InstructionPath, update || opt.force); err != nil {
			return fmt.Errorf("%s instructions: %w", adapter.Name, err)
		}
	}

	if adapter.SkillPath != "" && !adapter.NativeReadsAgent {
		if err := ensureDir(opt, adapter.SkillPath); err != nil {
			return err
		}
		if err := linkSkillDirs(opt, sourceSkills, adapter.SkillPath, update || opt.force); err != nil {
			return fmt.Errorf("%s skills: %w", adapter.Name, err)
		}
	}

	if !opt.noMCP && adapter.MCPPath != "" {
		if err := mergeMCP(opt, adapter); err != nil {
			return fmt.Errorf("%s MCP: %w", adapter.Name, err)
		}
	}
	return nil
}

func status(opt options) error {
	opt.agentsDir = expandPath(opt.agentsDir)
	paths := []string{
		filepath.Join(opt.agentsDir, "AGENTS.md"),
		filepath.Join(opt.agentsDir, "agents"),
		filepath.Join(opt.agentsDir, "registry", "skills.json"),
		filepath.Join(opt.agentsDir, "skills"),
		filepath.Join(opt.agentsDir, "settings.json"),
		filepath.Join(opt.agentsDir, "mcp", "servers.json"),
	}
	adapters, err := adapters(opt.agentsDir)
	if err != nil {
		return err
	}
	for _, adapter := range adapters {
		if selected(opt, adapter.Name) {
			paths = append(paths, adapter.InstructionPath, adapter.MCPPath)
			if !adapter.NativeReadsAgent {
				paths = append(paths, adapter.SkillPath)
			}
		}
	}
	for _, path := range compact(paths) {
		if path == "" {
			continue
		}
		printPathStatus(path)
	}
	return nil
}

func doctor(opt options) error {
	opt.agentsDir = expandPath(opt.agentsDir)
	fmt.Printf("os: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("agents home: %s\n", opt.agentsDir)
	if _, err := os.Stat(opt.agentsDir); err != nil {
		fmt.Printf("missing: %s\n", opt.agentsDir)
	} else {
		fmt.Printf("ok: %s\n", opt.agentsDir)
	}

	checkJSON(filepath.Join(opt.agentsDir, "mcp", "servers.json"))
	checkJSON(filepath.Join(opt.agentsDir, "settings.json"))
	checkJSON(filepath.Join(opt.agentsDir, "registry", "skills.json"))
	for _, exe := range []string{"claude", "opencode", "kimi", "qwen", "cursor-agent", "trae"} {
		if path, err := exec.LookPath(exe); err == nil {
			fmt.Printf("found %-12s %s\n", exe, path)
		} else {
			fmt.Printf("missing %-10s not on PATH\n", exe)
		}
	}
	adapters, err := adapters(opt.agentsDir)
	if err != nil {
		return err
	}
	for _, adapter := range adapters {
		if selected(opt, adapter.Name) {
			printPathStatus(adapter.InstructionPath)
			if adapter.NativeReadsAgent {
				fmt.Printf("ok skills %s reads %s directly\n", adapter.Name, filepath.Join(opt.agentsDir, "skills"))
			} else {
				printPathStatus(adapter.SkillPath)
			}
			if adapter.MCPPath != "" {
				printPathStatus(adapter.MCPPath)
				if strings.HasSuffix(adapter.MCPPath, ".json") {
					checkJSON(adapter.MCPPath)
				}
			}
		}
	}
	return nil
}

func adapters(agentsDir string) ([]toolAdapter, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return []toolAdapter{
		{
			Name:             "opencode",
			InstructionPath:  filepath.Join(xdg, "opencode", "AGENTS.md"),
			SkillPath:        filepath.Join(xdg, "opencode", "skills"),
			NativeReadsAgent: true,
			MCPPath:          filepath.Join(xdg, "opencode", ".opencode.json"),
			MCPFormat:        "opencode",
		},
		{
			Name:            "claude",
			InstructionPath: filepath.Join(home, ".claude", "CLAUDE.md"),
			SkillPath:       filepath.Join(home, ".claude", "skills"),
			MCPPath:         "",
			MCPFormat:       "manual",
		},
		{
			Name:            "kimi",
			InstructionPath: filepath.Join(home, ".kimi", "AGENTS.md"),
			SkillPath:       filepath.Join(home, ".kimi", "skills"),
			MCPPath:         filepath.Join(home, ".kimi", "mcp.json"),
			MCPFormat:       "mcpServers",
		},
		{
			Name:            "qwen",
			InstructionPath: filepath.Join(home, ".qwen", "AGENTS.md"),
			SkillPath:       filepath.Join(home, ".qwen", "skills"),
			MCPPath:         filepath.Join(home, ".qwen", "settings.json"),
			MCPFormat:       "qwen-settings",
		},
		{
			Name:            "cursor",
			InstructionPath: filepath.Join(home, ".cursor", "rules", "AGENTS.md"),
			SkillPath:       filepath.Join(home, ".cursor", "skills"),
			MCPPath:         "",
			MCPFormat:       "manual",
		},
		{
			Name:            "trae",
			InstructionPath: filepath.Join(home, ".trae", "user_rules", "AGENTS.md"),
			SkillPath:       "",
			MCPPath:         "",
			MCPFormat:       "manual",
		},
	}, nil
}

func installPresetFile(opt options, src, dst string, update bool) error {
	data, err := presetFS.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFileManaged(opt, dst, data, update || opt.force)
}

func installPresetTree(opt options, srcRoot, dstRoot string, update bool) error {
	return fs.WalkDir(presetFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil || rel == "." {
			return err
		}
		dst := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return ensureDir(opt, dst)
		}
		data, err := presetFS.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFileManaged(opt, dst, data, update || opt.force)
	})
}

func mergeMCP(opt options, adapter toolAdapter) error {
	manifest, err := readMCPManifest(opt, filepath.Join(opt.agentsDir, "mcp", "servers.json"))
	if err != nil {
		return err
	}
	if len(manifest.MCPServers) == 0 {
		return nil
	}
	if err := ensureDir(opt, filepath.Dir(adapter.MCPPath)); err != nil {
		return err
	}
	obj := map[string]any{}
	if data, err := os.ReadFile(adapter.MCPPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", adapter.MCPPath, err)
		}
	}
	switch adapter.MCPFormat {
	case "mcpServers":
		mergeObject(obj, "mcpServers", manifest.MCPServers)
	case "qwen-settings":
		mergeObject(obj, "mcpServers", manifest.MCPServers)
	case "opencode":
		mergeObject(obj, "mcp", manifest.MCPServers)
	default:
		return nil
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileManaged(opt, adapter.MCPPath, data, true)
}

func writeMCPHelpers(opt options, update bool) error {
	manifest, err := readMCPManifest(opt, filepath.Join(opt.agentsDir, "mcp", "servers.json"))
	if err != nil {
		return err
	}
	names := make([]string, 0, len(manifest.MCPServers))
	for name := range manifest.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)

	var claude strings.Builder
	claude.WriteString("#!/usr/bin/env sh\nset -eu\n\n")
	claude.WriteString("# Apply shared MCP presets to Claude Code user scope.\n")
	for _, name := range names {
		server, err := json.Marshal(manifest.MCPServers[name])
		if err != nil {
			return err
		}
		claude.WriteString(fmt.Sprintf("claude mcp add-json %s '%s' --scope user\n", shellWord(name), shellSingleQuotePayload(string(server))))
	}
	if err := writeFileManaged(opt, filepath.Join(opt.agentsDir, "mcp", "claude-code.commands.sh"), []byte(claude.String()), update || opt.force); err != nil {
		return err
	}

	readme := `# Shared MCP Presets

` + "`servers.json`" + ` is the source of truth for personal MCP servers.

This CLI merges the manifest directly into file-based configs for OpenCode, Kimi, and Qwen.

Claude Code stores user-scoped MCP through its own CLI. Run:

` + "```bash\nsh ~/.agents/mcp/claude-code.commands.sh\n```\n\n" + `Cursor and Trae do not currently expose a stable shared user-level MCP config path in this project, so keep their MCP setup manual and tracked here until their native config format is clear.
`
	return writeFileManaged(opt, filepath.Join(opt.agentsDir, "mcp", "README.md"), []byte(readme), update || opt.force)
}

func writeRegistryHelpers(opt options, update bool) error {
	manifest, err := readRegistryManifest()
	if err != nil {
		return err
	}
	registryDir := filepath.Join(opt.agentsDir, "registry")
	if err := ensureDir(opt, registryDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := writeFileManaged(opt, filepath.Join(registryDir, "skills.json"), data, update || opt.force); err != nil {
		return err
	}

	var script strings.Builder
	script.WriteString("#!/usr/bin/env sh\nset -eu\n\n")
	script.WriteString("# Install registry-managed skills. Custom skills live in ~/.agents/skills.\n")
	for _, skill := range manifest.Skills {
		script.WriteString(registryCommand(skill, true, opt.copyMode))
		script.WriteString("\n")
	}
	if err := writeFileManaged(opt, filepath.Join(registryDir, "install.sh"), []byte(script.String()), update || opt.force); err != nil {
		return err
	}

	readme := `# Registry Skills

These skills are intentionally not embedded in this repository. They are installed from the public Skills registry so updates can come from their upstream source.

Run:

` + "```bash\nsh ~/.agents/registry/install.sh\n```\n\n" + `Custom/private skills remain in ` + "`~/.agents/skills`" + ` and are installed from this bootstrap's embedded presets.
`
	return writeFileManaged(opt, filepath.Join(registryDir, "README.md"), []byte(readme), update || opt.force)
}

func installRegistrySkills(opt options) error {
	manifest, err := readRegistryManifest()
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
		if opt.copyMode {
			args = append(args, "--copy")
		}
		fmt.Printf("registry: %s from %s@%s\n", skill.Name, skill.Source, skill.Skill)
		if opt.dryRun {
			fmt.Printf("run: npx %s\n", strings.Join(args, " "))
			continue
		}
		cmd := exec.Command("npx", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install registry skill %s: %w", skill.Name, err)
		}
	}
	return nil
}

func readRegistryManifest() (registryManifest, error) {
	var manifest registryManifest
	data, err := presetFS.ReadFile("presets/registry/skills.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func registryCommand(skill registrySkill, global bool, copyMode bool) string {
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

func readMCPManifest(opt options, path string) (mcpManifest, error) {
	var manifest mcpManifest
	data, err := os.ReadFile(path)
	if err != nil {
		if !opt.dryRun {
			return manifest, err
		}
		data, err = presetFS.ReadFile("presets/mcp/servers.json")
		if err != nil {
			return manifest, err
		}
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func mergeObject(obj map[string]any, key string, values map[string]any) {
	nested, _ := obj[key].(map[string]any)
	if nested == nil {
		nested = map[string]any{}
		obj[key] = nested
	}
	for name, value := range values {
		nested[name] = value
	}
}

func linkSkillDirs(opt options, sourceSkills, targetSkills string, replace bool) error {
	entries, err := os.ReadDir(sourceSkills)
	if err != nil {
		if !opt.dryRun {
			return err
		}
		names, err := embeddedSkillNames()
		if err != nil {
			return err
		}
		for _, name := range names {
			src := filepath.Join(sourceSkills, name)
			dst := filepath.Join(targetSkills, name)
			if err := linkOrCopy(opt, src, dst, replace); err != nil {
				return err
			}
		}
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		src := filepath.Join(sourceSkills, entry.Name())
		dst := filepath.Join(targetSkills, entry.Name())
		if err := linkOrCopy(opt, src, dst, replace); err != nil {
			return err
		}
	}
	return nil
}

func linkOrCopy(opt options, src, dst string, replace bool) error {
	if _, err := os.Lstat(dst); err == nil {
		if sameLink(dst, src) {
			fmt.Printf("ok: %s -> %s\n", dst, src)
			return nil
		}
		if !replace {
			fmt.Printf("skip existing: %s\n", dst)
			return nil
		}
		if err := backupAndRemove(opt, dst); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if opt.dryRun {
		if opt.copyMode || runtime.GOOS == "windows" {
			fmt.Printf("copy: %s -> %s\n", src, dst)
			return nil
		}
		fmt.Printf("link: %s -> %s\n", dst, src)
		return nil
	}
	if opt.copyMode || runtime.GOOS == "windows" {
		return copyAny(opt, src, dst)
	}
	fmt.Printf("link: %s -> %s\n", dst, src)
	return os.Symlink(src, dst)
}

func embeddedSkillNames() ([]string, error) {
	entries, err := presetFS.ReadDir("presets/skills")
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func copyAny(opt options, src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(opt, src, dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFileManaged(opt, dst, data, true)
}

func copyDir(opt options, src, dst string) error {
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
			return ensureDir(opt, target)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFileManaged(opt, target, data, true)
	})
}

func writeFileManaged(opt options, path string, data []byte, replace bool) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == string(data) {
			fmt.Printf("ok: %s\n", path)
			return nil
		}
		if !replace {
			fmt.Printf("skip existing: %s\n", path)
			return nil
		}
		if err := backupPath(opt, path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureDir(opt, filepath.Dir(path)); err != nil {
		return err
	}
	fmt.Printf("write: %s\n", path)
	if opt.dryRun {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

func backupAndRemove(opt options, path string) error {
	if err := backupPath(opt, path); err != nil {
		return err
	}
	fmt.Printf("remove: %s\n", path)
	if opt.dryRun {
		return nil
	}
	return os.RemoveAll(path)
}

func backupPath(opt options, path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	backup := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
	fmt.Printf("backup: %s -> %s\n", path, backup)
	if opt.dryRun {
		return nil
	}
	return os.Rename(path, backup)
}

func ensureDir(opt options, path string) error {
	if path == "" || path == "." {
		return nil
	}
	fmt.Printf("mkdir: %s\n", path)
	if opt.dryRun {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func printPathStatus(path string) {
	if path == "" {
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		fmt.Printf("missing: %s\n", path)
		return
	}
	kind := "file"
	if info.IsDir() {
		kind = "dir"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(path)
		fmt.Printf("link: %s -> %s\n", path, target)
		return
	}
	fmt.Printf("ok %-4s %s\n", kind, path)
}

func checkJSON(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		fmt.Printf("invalid json: %s: %v\n", path, err)
		return
	}
	fmt.Printf("valid json: %s\n", path)
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

func defaultAgentsDir() (string, error) {
	if env := os.Getenv("AGENTS_HOME"); env != "" {
		return expandPath(env), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents"), nil
}

func expandPath(path string) string {
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

func parseTools(value string) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func selected(opt options, name string) bool {
	return opt.toolFilter["all"] || opt.toolFilter[strings.ToLower(name)]
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
