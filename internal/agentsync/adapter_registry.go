package agentsync

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// RegistryOptions captures the per-Manger state every concrete
// adapter needs to construct itself. Pass one of these to
// NewAdapterRegistry to build the catalog.
type RegistryOptions struct {
	Home          string
	XDGConfigHome string
	KiroHome      string
	// HermesHome overrides HERMES_HOME / ~/.hermes for the hermes adapter.
	// Empty means resolve via resolveHermesHome (env then ~/.hermes).
	HermesHome string
	// PiAgentDir overrides PI_CODING_AGENT_DIR / ~/.pi/agent for the pi adapter.
	// Empty means resolve via resolvePiAgentDir (env then ~/.pi/agent).
	PiAgentDir string
	// ClaudeDesktopDir overrides the directory holding
	// claude_desktop_config.json. Empty means derive it from the running
	// OS via claudeDesktopConfigDir.
	ClaudeDesktopDir string
}

// registryGOOS is a seam so tests can exercise the per-OS Claude Desktop
// config directory without running on that OS.
var registryGOOS = runtime.GOOS

// claudeDesktopConfigDir returns the directory Claude Desktop keeps
// claude_desktop_config.json in:
//
//   - macOS:   ~/Library/Application Support/Claude
//   - Windows: %APPDATA%\Claude (fallback ~/AppData/Roaming/Claude)
//   - Linux:   $XDG_CONFIG_HOME/Claude (xdg already defaults to ~/.config)
//
// https://modelcontextprotocol.io/docs/develop/connect-local-servers
func claudeDesktopConfigDir(home, xdg string) string {
	switch registryGOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Claude")
		}
		return filepath.Join(home, "AppData", "Roaming", "Claude")
	default:
		return filepath.Join(xdg, "Claude")
	}
}

// NewAdapterRegistry builds the default catalog of stable +
// experimental + manual adapters. The factory pattern lets tests
// pass a custom catalog without touching this file.
func NewAdapterRegistry(opts RegistryOptions) *AdapterRegistry {
	r := &AdapterRegistry{}
	kiro := opts.KiroHome
	if kiro == "" {
		kiro = filepath.Join(opts.Home, ".kiro")
	}
	xdg := opts.XDGConfigHome
	if xdg == "" {
		xdg = filepath.Join(opts.Home, ".config")
	}
	home := opts.Home
	claudeDesktop := opts.ClaudeDesktopDir
	if claudeDesktop == "" {
		claudeDesktop = claudeDesktopConfigDir(home, xdg)
	}

	r.add(&ClaudeAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "claude", Tier: TierStable, Executables: []string{"claude"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".claude", "CLAUDE.md"),
				Skills:      filepath.Join(home, ".claude", "skills"),
				Subagents:   filepath.Join(home, ".claude", "agents"),
				Settings:    filepath.Join(home, ".claude", "settings.json"),
			},
			Docs: []string{"https://docs.claude.com/en/docs/claude-code/settings", "https://docs.claude.com/en/docs/claude-code/mcp"},
		},
		Plugin: ClaudePlugin{},
	}})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "claude-desktop", Aliases: []string{"claude-app", "claudedesktop"}, Tier: TierStable,
			// GUI app: no CLI binary to probe on PATH.
			Targets: AdapterTargets{
				MCPPath:    filepath.Join(claudeDesktop, "claude_desktop_config.json"),
				MCPKeyPath: []string{"mcpServers"},
			},
			Docs: []string{
				"https://modelcontextprotocol.io/docs/develop/connect-local-servers",
				"https://modelcontextprotocol.io/docs/develop/connect-remote-servers",
				"https://support.claude.com/en/articles/10949351-getting-started-with-local-mcp-servers-on-claude-desktop",
			},
			Notes: "Claude Desktop app (GUI, no CLI binary). Only MCP is synced: the shared catalog is merged under mcpServers in claude_desktop_config.json (macOS ~/Library/Application Support/Claude, Windows %APPDATA%\\Claude, Linux $XDG_CONFIG_HOME/Claude). That file reads stdio servers only, so remote HTTP/SSE entries are bridged through `npx -y mcp-remote <url>`. Instructions/skills/subagents stay with the `claude` adapter under ~/.claude (the desktop app's Claude Code side reads them there); desktop-native Skills and Extensions are managed in-app and are not file-synced. Restart Claude Desktop after a sync to pick up new servers.",
		},
		Plugin: ClaudeDesktopPlugin{},
	}})

	r.add(&OpenCodeAdapter{
		BaseAdapter: BaseAdapter{
			Spec: AdapterSpec{
				ID: "opencode", Tier: TierStable, Executables: []string{"opencode"},
				Targets: AdapterTargets{
					Instruction: filepath.Join(xdg, "opencode", "AGENTS.md"),
					// Skills: OpenCode discovers ~/.agents/skills natively
					// (https://opencode.ai/docs/skills/); do not mirror.
					Subagents:          filepath.Join(xdg, "opencode", "agent"),
					SkillsCleanupRoots: []string{filepath.Join(xdg, "opencode", "skill")},
				},
				Docs:  []string{"https://opencode.ai/docs/config/", "https://opencode.ai/docs/agents/", "https://opencode.ai/docs/mcp-servers/", "https://opencode.ai/docs/skills/"},
				Notes: "OpenCode loads skills from ~/.agents/skills (and optional ~/.config/opencode/skills / ~/.claude/skills); this adapter does not mirror skills. It still links AGENTS.md, subagents under agent/, and merges MCP into opencode.json.",
			},
			Plugin: OpenCodePlugin{ConfigPath: filepath.Join(xdg, "opencode", "opencode.json")},
		},
		ConfigPath: filepath.Join(xdg, "opencode", "opencode.json"),
	})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "grok", Tier: TierStable, Executables: []string{"grok"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".grok", "AGENTS.md"),
				// Skills: Grok discovers ~/.agents/skills natively; do not mirror.
				SkillsCleanupRoots: []string{filepath.Join(home, ".grok", "skills")},
			},
			Docs:  []string{"https://docs.x.ai/build/overview", "https://docs.x.ai/build/features/skills-plugins-marketplaces"},
			Notes: "Grok Build loads global rules from ~/.grok/ (including AGENTS.md), discovers skills from ~/.agents/skills (and optional ~/.grok/skills), and configures MCP under [mcp_servers.*] in ~/.grok/config.toml. This adapter links shared AGENTS.md and writes a managed MCP TOML block; it does not mirror skills.",
		},
		Plugin: GrokPlugin{},
	}})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "kimi", Tier: TierStable, Executables: []string{"kimi"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".kimi", "AGENTS.md"),
				MCPPath:     filepath.Join(home, ".kimi", "mcp.json"),
				MCPKeyPath:  []string{"mcpServers"},
			},
			Docs:  []string{"https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html"},
			Notes: "Kimi Code CLI reads generic cross-tool Skills directly from ~/.agents/skills/ regardless of KIMI_CODE_HOME, so this adapter does not mirror skills into a Kimi-specific folder.",
		},
		Plugin: NoopPlugin{},
	}})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "kiro", Aliases: []string{"kiro-cli"}, Tier: TierStable, Executables: []string{"kiro", "kiro-cli"},
			Targets: AdapterTargets{
				Instruction:    filepath.Join(kiro, "steering", "AGENTS.md"),
				Skills:         filepath.Join(kiro, "skills"),
				MCPPath:        filepath.Join(kiro, "settings", "mcp.json"),
				MCPKeyPath:     []string{"mcpServers"},
				AgentConfigSrc: "presets/settings/kiro.json",
				AgentConfigDst: filepath.Join(kiro, "agents", "ns-full.json"),
			},
			Docs:  []string{"https://kiro.dev/docs/cli/chat/configuration/", "https://kiro.dev/docs/cli/mcp/", "https://kiro.dev/docs/cli/reference/settings/", "https://kiro.dev/docs/cli/skills/", "https://kiro.dev/docs/cli/custom-agents/creating/"},
			Notes: "Kiro CLI alias: kiro-cli. Shared instructions sync to global steering; skills sync to Kiro global skills; MCP presets sync to the shared Kiro settings path. A full-permissions/yolo custom agent (tools:*, allowedTools:@builtin+@*, toolsSettings auto-allow shell/write/read/web/aws, model gpt-5.6-terra) is written to ~/.kiro/agents/ns-full.json so `kiro-cli chat --agent ns-full` (or --trust-all-tools) runs without per-tool approval prompts.",
		},
		Plugin: NoopPlugin{},
	}})

	r.add(&ProfileAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "qwen", Tier: TierStable, Executables: []string{"qwen"},
			Targets: AdapterTargets{
				Instruction:  filepath.Join(home, ".qwen", "QWEN.md"),
				Skills:       filepath.Join(home, ".qwen", "skills"),
				HooksPath:    filepath.Join(home, ".qwen", "settings.json"),
				HooksKeyPath: []string{"hooks"},
				MCPPath:      filepath.Join(home, ".qwen", "settings.json"),
				MCPKeyPath:   []string{"mcpServers"},
			},
			Docs: []string{"https://qwenlm.github.io/qwen-code-docs/en/cli/configuration/", "https://qwenlm.github.io/qwen-code-docs/en/users/features/mcp/"},
		},
		Plugin: QwenPlugin{},
	}})

	r.add(&ProfileAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "antigravity", Tier: TierStable, Executables: []string{"agy"},
			Targets: AdapterTargets{
				// Global context lives at ~/.gemini/GEMINI.md.
				// https://antigravity.google/docs/cli/gcli-migration
				Instruction: filepath.Join(home, ".gemini", "GEMINI.md"),
				// Global skills: ~/.gemini/antigravity-cli/skills/ (workspace: .agents/skills/).
				// https://antigravity.google/docs/cli/plugins
				Skills: filepath.Join(home, ".gemini", "antigravity-cli", "skills"),
				// Sparse CLI settings (toolPermission, sandbox, …).
				// https://antigravity.google/docs/cli/settings
				Settings: filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"),
				// MCP is a standalone profile (not nested in settings.json).
				// Remote servers use serverUrl (not url/httpUrl).
				// https://antigravity.google/docs/mcp
				MCPPath:    filepath.Join(home, ".gemini", "config", "mcp_config.json"),
				MCPKeyPath: []string{"mcpServers"},
			},
			Docs: []string{
				"https://antigravity.google/docs/cli/settings",
				"https://antigravity.google/docs/mcp",
				"https://antigravity.google/docs/cli/plugins",
				"https://antigravity.google/docs/cli/gcli-migration",
			},
			Notes: "Antigravity CLI (agy). Instructions at ~/.gemini/GEMINI.md; settings at ~/.gemini/antigravity-cli/settings.json; skills mirrored to ~/.gemini/antigravity-cli/skills; MCP at ~/.gemini/config/mcp_config.json with serverUrl for remote servers.",
		},
		Plugin: AntigravityPlugin{},
	}})

	r.add(&CodexAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "codex", Tier: TierStable, Executables: []string{"codex"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".codex", "AGENTS.md"),
			},
			Docs:  []string{"https://github.com/openai/codex/blob/main/docs/config.md", "https://github.com/openai/codex/blob/main/docs/agents_md.md"},
			Notes: "Codex CLI has no ~/.codex/skills path at all — it only discovers Agent Skills from .agents/skills (repo, walking up to the repo root) and $HOME/.agents/skills (user), so this adapter does not mirror skills anywhere; the shared ~/.agents/skills directory is picked up natively.",
		},
		Plugin: CodexPlugin{},
	}})

	r.add(&ProfileAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "cline", Tier: TierStable, Executables: []string{"cline"},
			Targets: AdapterTargets{
				// Docs: global skills/agents live under ~/.cline/ (not data/).
				// https://docs.cline.bot/customization/skills
				// https://docs.cline.bot/getting-started/config
				Skills:    filepath.Join(home, ".cline", "skills"),
				Subagents: filepath.Join(home, ".cline", "agents"),
				MCPPath:   filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
				MCPKeyPath: []string{"mcpServers"},
				// Previous ns-workspace path used data/skills and data/agents.
				SkillsCleanupRoots: []string{
					filepath.Join(home, ".cline", "data", "skills"),
					filepath.Join(home, ".cline", "data", "agents"),
				},
			},
			Docs:  []string{"https://docs.cline.bot/cline-cli/configuration", "https://docs.cline.bot/customization/skills", "https://docs.cline.bot/getting-started/config"},
			Notes: "Cline discovers global skills at ~/.cline/skills and agents at ~/.cline/agents; MCP settings stay under ~/.cline/data/settings/cline_mcp_settings.json. Stale managed links under the former data/skills and data/agents paths are cleaned on apply.",
		},
		Plugin: ClinePlugin{},
	}})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "zcode", Aliases: []string{"zcode-cli"}, Tier: TierStable, Executables: []string{"zcode"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".zcode", "AGENTS.md"),
				// Skills: ZCode skill-creator docs list ~/.agents/skills (and
				// project .agents/skills) with default install there; do not mirror.
				SkillsCleanupRoots: []string{filepath.Join(home, ".zcode", "skills")},
			},
			Docs:  []string{},
			Notes: "ZCode discovers skills from ~/.agents/skills (preferred) and optional ~/.zcode/skills; this adapter does not mirror skills. Shared ~/.agents/AGENTS.md is file-linked into ~/.zcode/AGENTS.md. There is no first-party user-level MCP config in this ZCode release (MCP lives per-plugin under the plugin cache), so the adapter does not write an MCP file yet.",
		},
		Plugin: ZCodePlugin{},
	}})

	hermesHome := resolveHermesHome(home, opts.HermesHome)
	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "hermes", Tier: TierStable, Executables: []string{"hermes"},
			Targets: AdapterTargets{
				// No Instruction: SOUL.md is identity, not shared AGENTS.md.
				// No Skills mirror: skills.external_dirs points at shared home.
			},
			Docs: []string{
				"https://hermes-agent.nousresearch.com/docs/user-guide/configuration",
				"https://hermes-agent.nousresearch.com/docs/user-guide/features/skills",
				"https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp",
			},
			Notes: "Hermes Agent (CLI hermes). Skills via skills.external_dirs → <agents-home>/skills (no mirror into ~/.hermes/skills). MCP merged into $HERMES_HOME/config.yaml under mcp_servers (default ~/.hermes). Respects HERMES_HOME. Does not modify SOUL.md or .env. Reload Hermes after sync to pick up MCP. YAML rewrite may drop comments in config.yaml.",
		},
		Plugin: HermesPlugin{Home: hermesHome},
	}})

	piAgent := resolvePiAgentDir(home, opts.PiAgentDir)
	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "pi", Aliases: []string{"pi-coding-agent", "pi-agent"}, Tier: TierStable, Executables: []string{"pi"},
			Targets: AdapterTargets{
				// Global context: ~/.pi/agent/AGENTS.md (or $PI_CODING_AGENT_DIR/AGENTS.md).
				// https://github.com/earendil-works/pi-mono / docs skills + context files
				Instruction: filepath.Join(piAgent, "AGENTS.md"),
				// Skills: pi discovers ~/.agents/skills and ~/.pi/agent/skills natively.
				// Do not mirror — leave ~/.pi/agent/skills for user/package-owned skills.
			},
			Docs: []string{
				"https://github.com/earendil-works/pi-mono",
				"https://www.npmjs.com/package/@earendil-works/pi-coding-agent",
			},
			Notes: "Pi coding agent (CLI pi, package @earendil-works/pi-coding-agent). Links shared AGENTS.md into $PI_CODING_AGENT_DIR/AGENTS.md (default ~/.pi/agent). Skills load natively from ~/.agents/skills and optional ~/.pi/agent/skills — this adapter does not mirror skills. No first-party MCP config (pi philosophy: CLI tools/skills instead); settings.json / models.json / extensions are user-managed and not overwritten.",
		},
		Plugin: NoopPlugin{},
	}})

	return r
}

// AdapterRegistry is the resolved catalog of adapters the Manager
// iterates over for Apply / Status / Doctor / Catalog. The factory
// pattern lets tests inject a smaller catalog without rewriting call
// sites.
type AdapterRegistry struct {
	adapters []Adapter
	byID     map[string]Adapter
}

// add registers a new adapter in the registry. id collisions panic
// loudly so a typo in a provider id fails the build, not the runtime.
func (r *AdapterRegistry) add(a Adapter) {
	if r.byID == nil {
		r.byID = map[string]Adapter{}
	}
	name := a.Name()
	if _, exists := r.byID[name]; exists {
		panic("agentsync: duplicate adapter id " + name)
	}
	r.adapters = append(r.adapters, a)
	r.byID[name] = a
}

// All returns the catalog in registration order. Doctor and Catalog
// iterate this slice directly.
func (r *AdapterRegistry) All() []Adapter {
	return append([]Adapter(nil), r.adapters...)
}

// Lookup returns the adapter with the given id or alias. The match is
// case-insensitive on the lowercased name and aliases. Returns nil
// when no match is found.
func (r *AdapterRegistry) Lookup(id string) Adapter {
	needle := strings.ToLower(strings.TrimSpace(id))
	if a, ok := r.byID[needle]; ok {
		return a
	}
	for _, a := range r.adapters {
		for _, alias := range a.Aliases() {
			if alias == needle {
				return a
			}
		}
	}
	return nil
}

// Ids returns the sorted set of registered adapter ids and aliases.
// Useful for --help output and CLI validation.
func (r *AdapterRegistry) Ids() []string {
	out := map[string]bool{}
	for _, a := range r.adapters {
		out[strings.ToLower(a.Name())] = true
		for _, alias := range a.Aliases() {
			out[alias] = true
		}
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
