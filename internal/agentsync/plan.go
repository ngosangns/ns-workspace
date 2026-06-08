package agentsync

import (
	"fmt"
	"path/filepath"
	"strings"
)

type PlanPhaseName string

const (
	PhaseCore            PlanPhaseName = "core"
	PhaseRegistryHelpers PlanPhaseName = "registry_helpers"
	PhaseRegistryInstall PlanPhaseName = "registry_install"
	PhaseMCP             PlanPhaseName = "mcp"
	PhaseAdapters        PlanPhaseName = "adapters"
)

type SyncPlan struct {
	Mode      string
	AgentsDir string
	Phases    []PlanPhase
}

type PlanPhase struct {
	Name       PlanPhaseName
	Operations []PlannedOperation
}

type PlannedOperation struct {
	Owner    string
	Artifact ArtifactKind
	Op       Operation
}

func (m Manager) BuildPlan(opt Options, update bool) (SyncPlan, error) {
	ctx, err := m.context(opt)
	if err != nil {
		return SyncPlan{}, err
	}
	ctx.Update = update
	return m.buildPlan(ctx, update)
}

func (m Manager) buildPlan(ctx Context, update bool) (SyncPlan, error) {
	mode := "init"
	if update {
		mode = "update"
	}
	replace := update || ctx.Force
	plan := SyncPlan{Mode: mode, AgentsDir: ctx.Options.AgentsDir}

	coreDirs := []string{
		ctx.Options.AgentsDir,
		filepath.Join(ctx.Options.AgentsDir, "skills"),
		filepath.Join(ctx.Options.AgentsDir, "agents"),
		filepath.Join(ctx.Options.AgentsDir, "mcp"),
		filepath.Join(ctx.Options.AgentsDir, "generated"),
	}
	for _, dir := range coreDirs {
		plan.Add(PhaseCore, "core", ArtifactDirectory, EnsureDir{Dir: dir})
	}
	plan.Add(PhaseCore, "core", ArtifactInstructions, InstallPresetFile{Src: "presets/agents/AGENTS.md", Dst: filepath.Join(ctx.Options.AgentsDir, "AGENTS.md"), Replace: replace})
	plan.Add(PhaseCore, "core", ArtifactSkills, InstallPresetTree{SrcRoot: "presets/skills", DstRoot: filepath.Join(ctx.Options.AgentsDir, "skills"), Replace: replace})
	plan.Add(PhaseCore, "core", ArtifactSubagents, InstallPresetTree{SrcRoot: "presets/subagents", DstRoot: filepath.Join(ctx.Options.AgentsDir, "agents"), Replace: replace})
	plan.Add(PhaseCore, "core", ArtifactSettings, InstallPresetFile{Src: "presets/settings/default.json", Dst: filepath.Join(ctx.Options.AgentsDir, "settings.json"), Replace: replace})

	plan.Add(PhaseRegistryHelpers, "registry", ArtifactSkills, WriteRegistryHelpers{Replace: replace})
	if !ctx.NoRegistry {
		plan.Add(PhaseRegistryInstall, "registry", ArtifactSkills, RegistryInstall{})
	}
	if !ctx.NoMCP {
		plan.Add(PhaseMCP, "core", ArtifactMCP, InstallPresetFile{Src: "presets/mcp/servers.json", Dst: filepath.Join(ctx.Options.AgentsDir, "mcp", "servers.json"), Replace: replace})
		plan.Add(PhaseMCP, "core", ArtifactMCP, WriteMCPReadme{Replace: replace})
	}

	for _, adapter := range m.adapters(ctx) {
		if !selected(ctx.Options, adapter) {
			continue
		}
		ops, err := adapter.Plan(ctx, update)
		if err != nil {
			return SyncPlan{}, fmt.Errorf("%s adapter: %w", adapter.Name(), err)
		}
		for _, op := range ops {
			plan.Add(PhaseAdapters, adapter.Name(), operationArtifact(op), op)
		}
	}
	return plan, nil
}

func (p *SyncPlan) Add(phase PlanPhaseName, owner string, artifact ArtifactKind, op Operation) {
	if op == nil {
		return
	}
	planned := PlannedOperation{Owner: owner, Artifact: artifact, Op: op}
	for i := range p.Phases {
		if p.Phases[i].Name == phase {
			p.Phases[i].Operations = append(p.Phases[i].Operations, planned)
			return
		}
	}
	p.Phases = append(p.Phases, PlanPhase{Name: phase, Operations: []PlannedOperation{planned}})
}

func (p SyncPlan) Apply(ctx Context) error {
	for _, phase := range p.Phases {
		for _, planned := range phase.Operations {
			if err := planned.Op.Apply(ctx); err != nil {
				owner := planned.Owner
				if owner == "" {
					owner = string(phase.Name)
				}
				return fmt.Errorf("%s %s: %w", owner, planned.Op.Path(), err)
			}
		}
	}
	return nil
}

func operationArtifact(op Operation) ArtifactKind {
	switch op := op.(type) {
	case EnsureDir:
		return ArtifactDirectory
	case InstallPresetFile:
		return artifactFromPath(op.Src)
	case InstallPresetTree:
		return artifactFromPath(op.SrcRoot)
	case LinkOrCopy:
		return artifactFromLink(op.Src, op.Dst)
	case LinkSkillDirs:
		return artifactFromPath(op.SrcRoot)
	case MergeJSON:
		if len(op.KeyPath) > 0 && strings.EqualFold(op.KeyPath[len(op.KeyPath)-1], "mcpServers") {
			return ArtifactMCP
		}
		if len(op.KeyPath) > 0 && strings.EqualFold(op.KeyPath[len(op.KeyPath)-1], "hooks") {
			return ArtifactHooks
		}
		if strings.Contains(filepath.ToSlash(op.Dst), "mcp") {
			return ArtifactMCP
		}
		return ArtifactSettings
	case AppendManagedBlock:
		if op.Label == "mcp" {
			return ArtifactMCP
		}
		return ArtifactRules
	case ManualStep:
		return ArtifactCommands
	case WriteFile:
		return artifactFromPath(op.Dst)
	case WriteRegistryHelpers, RegistryInstall:
		return ArtifactSkills
	case WriteMCPReadme:
		return ArtifactMCP
	default:
		return ArtifactSettings
	}
}

func artifactFromLink(src string, dst string) ArtifactKind {
	if strings.Contains(filepath.ToSlash(src), "AGENTS.md") {
		return ArtifactInstructions
	}
	return artifactFromPath(dst)
}

func artifactFromPath(path string) ArtifactKind {
	path = filepath.ToSlash(path)
	switch {
	case strings.Contains(path, "/skills"):
		return ArtifactSkills
	case strings.Contains(path, "AGENTS.md"), strings.Contains(path, "CLAUDE.md"), strings.Contains(path, "QWEN.md"), strings.Contains(path, "GEMINI.md"):
		return ArtifactInstructions
	case strings.Contains(path, "/subagents"), strings.Contains(path, "/agents"):
		return ArtifactSubagents
	case strings.Contains(path, "/mcp"), strings.Contains(path, "mcp.json"):
		return ArtifactMCP
	case strings.Contains(path, "settings.json"), strings.Contains(path, "opencode.json"):
		return ArtifactSettings
	default:
		return ArtifactRules
	}
}
