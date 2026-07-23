package cli

import (
	"flag"
	"io/fs"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

func IsAgentSyncCommand(cmd string) bool {
	switch cmd {
	case "init", "update", "status", "doctor", "registry", "agents", "catalog":
		return true
	default:
		return false
	}
}

func RunAgentSync(cmd string, args []string, presets fs.FS) error {
	flagSet := flag.NewFlagSet(cmd, flag.ContinueOnError)
	homeDefault, err := agentsync.DefaultAgentsDir()
	if err != nil {
		return err
	}
	opt := agentsync.Options{Command: cmd, AgentsDir: homeDefault}
	tools := flagSet.String("tools", "all", "comma-separated tools: all,stable,manual,experimental,claude,opencode,grok,kimi,kiro,kiro-cli,qwen,antigravity,codex,cline")
	flagSet.StringVar(&opt.AgentsDir, "agents-home", homeDefault, "shared agents home")
	configDefault, err := agentsync.DefaultUserConfigPath()
	if err != nil {
		return err
	}
	flagSet.StringVar(&opt.ConfigPath, "config", configDefault, "path to user config file overriding embedded presets; empty disables overlay")
	flagSet.BoolVar(&opt.DryRun, "dry-run", false, "show planned writes without changing files")
	flagSet.BoolVar(&opt.Yes, "yes", false, "skip interactive confirmations")
	flagSet.BoolVar(&opt.Force, "force", false, "replace existing files during init")
	flagSet.BoolVar(&opt.CopyMode, "copy", false, "copy files instead of creating symlinks (always on for update)")
	flagSet.BoolVar(&opt.NoMCP, "no-mcp", false, "skip MCP configuration")
	flagSet.BoolVar(&opt.NoRegistry, "no-registry", false, "skip skills registry installation")
	flagSet.BoolVar(&opt.RefreshSkills, "refresh-skills", false, "force re-install registry skills even when catalog fingerprint is unchanged")
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	opt.ToolFilter = agentsync.ParseTools(*tools)

	manager := agentsync.Manager{Presets: presets}
	switch cmd {
	case "init":
		return manager.Apply(opt, false)
	case "update":
		// update always copies (see Manager.apply); --copy is redundant but accepted
		return manager.Apply(opt, true)
	case "status":
		return manager.Status(opt)
	case "doctor":
		return manager.Doctor(opt)
	case "registry":
		return manager.InstallRegistrySkills(opt)
	case "agents", "catalog":
		return manager.Catalog(opt)
	default:
		return flag.ErrHelp
	}
}
