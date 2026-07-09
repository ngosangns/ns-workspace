package agentsync

import (
	"path/filepath"
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
			Notes: "Kiro CLI alias: kiro-cli. Shared instructions sync to global steering; skills sync to Kiro global skills; MCP presets sync to the shared Kiro settings path. A full-permissions custom agent (tools:* + permissions allow capability:all) is written to ~/.kiro/agents/ns-full.json so `kiro --agent ns-full` runs without per-tool approval prompts.",
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
			ID: "gemini", Tier: TierStable, Executables: []string{"gemini"},
			Targets: AdapterTargets{
				Instruction:  filepath.Join(home, ".gemini", "GEMINI.md"),
				HooksPath:    filepath.Join(home, ".gemini", "settings.json"),
				HooksKeyPath: []string{"hooks"},
				MCPPath:      filepath.Join(home, ".gemini", "settings.json"),
				MCPKeyPath:   []string{"mcpServers"},
			},
			Docs:  []string{"https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md", "https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/skills.md"},
			Notes: "Gemini CLI resolves a `.agents/skills/` alias (user and workspace tiers) that takes precedence over `.gemini/skills/`, so it reads the shared ~/.agents/skills/ directly and this adapter does not mirror skills into ~/.gemini/skills.",
		},
		Plugin: GeminiPlugin{},
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
			Docs:  []string{""},
			Notes: "ZCode discovers skills from ~/.agents/skills (preferred) and optional ~/.zcode/skills; this adapter does not mirror skills. Shared ~/.agents/AGENTS.md is file-linked into ~/.zcode/AGENTS.md. There is no first-party user-level MCP config in this ZCode release (MCP lives per-plugin under the plugin cache), so the adapter does not write an MCP file yet.",
		},
		Plugin: ZCodePlugin{},
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
