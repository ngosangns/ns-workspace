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
					Skills:      filepath.Join(xdg, "opencode", "skill"),
					Subagents:   filepath.Join(xdg, "opencode", "agent"),
				},
				Docs: []string{"https://opencode.ai/docs/config/", "https://opencode.ai/docs/agents/", "https://opencode.ai/docs/mcp-servers/"},
			},
			Plugin: OpenCodePlugin{ConfigPath: filepath.Join(xdg, "opencode", "opencode.json")},
		},
		ConfigPath: filepath.Join(xdg, "opencode", "opencode.json"),
	})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "grok", Tier: TierStable, Executables: []string{"grok"},
			Targets: AdapterTargets{Skills: filepath.Join(home, ".grok", "skills")},
			Docs:  []string{"https://docs.x.ai/build/overview", "https://docs.x.ai/build/features/skills-plugins-marketplaces"},
			Notes: "Grok Build reads AGENTS.md from projects and also discovers ~/.agents/skills; this adapter mirrors shared skills into ~/.grok/skills for native slash-command discovery.",
		},
	}})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "kimi", Tier: TierStable, Executables: []string{"kimi"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".kimi", "AGENTS.md"),
				Skills:      filepath.Join(home, ".kimi", "skills"),
				MCPPath:     filepath.Join(home, ".kimi", "mcp.json"),
				MCPKeyPath:  []string{"mcpServers"},
			},
			Docs: []string{"https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html"},
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
			Docs: []string{"https://kiro.dev/docs/cli/chat/configuration/", "https://kiro.dev/docs/cli/mcp/", "https://kiro.dev/docs/cli/reference/settings/", "https://kiro.dev/docs/cli/skills/", "https://kiro.dev/docs/cli/custom-agents/creating/"},
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
				Skills:       filepath.Join(home, ".gemini", "skills"),
				HooksPath:    filepath.Join(home, ".gemini", "settings.json"),
				HooksKeyPath: []string{"hooks"},
				MCPPath:      filepath.Join(home, ".gemini", "settings.json"),
				MCPKeyPath:   []string{"mcpServers"},
			},
			Docs: []string{"https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md"},
		},
		Plugin: GeminiPlugin{},
	}})

	r.add(&CodexAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "codex", Tier: TierStable, Executables: []string{"codex"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".codex", "AGENTS.md"),
				Skills:      filepath.Join(home, ".codex", "skills"),
			},
			Docs: []string{"https://github.com/openai/codex/blob/main/docs/config.md", "https://github.com/openai/codex/blob/main/docs/agents_md.md"},
		},
		Plugin: CodexPlugin{},
	}})

	r.add(&ProfileAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "cline", Tier: TierStable, Executables: []string{"cline"},
			Targets: AdapterTargets{
				Skills:     filepath.Join(home, ".cline", "data", "skills"),
				Subagents:  filepath.Join(home, ".cline", "data", "agents"),
				MCPPath:    filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
				MCPKeyPath: []string{"mcpServers"},
			},
			Docs: []string{"https://docs.cline.bot/cline-cli/configuration"},
		},
		Plugin: ClinePlugin{},
	}})

	r.add(&ProfileAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "qoder", Aliases: []string{"qodercli", "qoder-cli"}, Tier: TierStable, Executables: []string{"qodercli", "qoder"},
			Targets: AdapterTargets{
				Instruction: filepath.Join(home, ".qoder", "AGENTS.md"),
				Skills:      filepath.Join(home, ".qoder", "skills"),
				Subagents:   filepath.Join(home, ".qoder", "agents"),
				MCPPath:     filepath.Join(home, ".qoder", "settings.json"),
				MCPKeyPath:  []string{"mcpServers"},
			},
			Docs: []string{"https://docs.qoder.com/en/cli/Skills", "https://docs.qoder.com/en/cli/subagent", "https://docs.qoder.com/en/cli/mcp-servers", "https://docs.qoder.com/en/cli/permissions"},
			Notes: "Qoder CLI (qodercli) reads global AGENTS.md, skills, and subagents from ~/.qoder, and stores MCP servers + full-bypass permission mode (general.defaultPermissionMode=bypass_permissions) in ~/.qoder/settings.json. MCP servers keep the Claude-style {type:http,url} shape.",
		},
		Plugin: QoderPlugin{},
	}})

	r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "windsurf", Tier: TierStable,
			Targets: AdapterTargets{Instruction: filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md")},
			Docs:    []string{"https://docs.windsurf.com/windsurf/cascade/memories"},
		},
	}})

	r.add(&AiderAdapter{BaseAdapter: BaseAdapter{
		Spec: AdapterSpec{
			ID: "aider", Tier: TierStable, Executables: []string{"aider"},
			Docs: []string{"https://aider.chat/docs/config/aider_conf.html", "https://aider.chat/docs/usage/conventions.html"},
		},
		Plugin: AiderPlugin{},
	}})

	r.add(&MiniMaxAdapter{
		BaseAdapter: BaseAdapter{
			Spec: AdapterSpec{
				ID: "minimax", Aliases: []string{"minimax-cli", "mmx"}, Tier: TierStable, Executables: []string{"mmx"},
				Docs: []string{"https://platform.minimax.io/docs/token-plan/minimax-cli", "https://github.com/MiniMax-AI/cli"},
				Notes: "MiniMax CLI (mmx) is a multimodal content-generation CLI (text/image/video/speech/music). Adapter writes default model and region presets to ~/.mmx/config.json via MergeJSON. The shared skills/ and agents/ fan-out does not apply because mmx-cli does not expose a user-level skills or subagents directory; use the bundled `presets/skills/minimax-cli/SKILL.md` from a coding agent to teach it the mmx surface.",
			},
			Plugin: MiniMaxPlugin{ConfigPath: filepath.Join(home, ".mmx", "config.json")},
		},
	})

	// Manual / experimental adapters share the same ManualPlan path.
	manualNotes := map[string]string{
		"cursor":         "Cursor user rules are stored through Cursor settings; generated helper only.",
		"github-copilot": "Copilot instructions are repo/editor scoped; generated helper only.",
		"jetbrains":      "JetBrains AI MCP setup is product/version specific.",
		"antigravity":    "No stable official user-level filesystem path confirmed yet.",
		"trae":           "No stable official user-level filesystem path confirmed yet.",
		"roo":            "Roo Code support is guarded because the project status is unstable.",
	}
	for _, entry := range []struct {
		id      string
		tier    SupportTier
		exe     []string
		docs    []string
		note    string
	}{
		{"cursor", TierManual, []string{"cursor-agent"}, []string{"https://docs.cursor.com/en/context", "https://docs.cursor.com/cli/mcp"}, manualNotes["cursor"]},
		{"github-copilot", TierManual, nil, []string{"https://code.visualstudio.com/docs/copilot/customization/custom-instructions"}, manualNotes["github-copilot"]},
		{"jetbrains", TierManual, nil, []string{"https://www.jetbrains.com/help/ai-assistant/mcp.html"}, manualNotes["jetbrains"]},
		{"antigravity", TierExperimental, nil, nil, manualNotes["antigravity"]},
		{"trae", TierExperimental, []string{"trae"}, nil, manualNotes["trae"]},
		{"roo", TierExperimental, nil, nil, manualNotes["roo"]},
	} {
		r.add(&SimpleAdapter{BaseAdapter: BaseAdapter{
			Spec: AdapterSpec{
				ID: entry.id, Tier: entry.tier, Executables: entry.exe, Docs: entry.docs, Notes: entry.note, Manual: true,
			},
		}})
	}

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
