package agentsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// writeRegistryHelpers writes the registry skills manifest, the install
// script, and the README into <AgentsDir>/registry/. Used by the core
// phase so the user can re-run install later without re-initializing.
func writeRegistryHelpers(ctx Context, replace bool) error {
	manifest, err := readRegistryManifest(ctx)
	if err != nil {
		return err
	}
	registryDir := filepath.Join(ctx.Options.AgentsDir, "registry")
	if err := ensureDir(ctx, registryDir); err != nil {
		return err
	}
	data, err := encodeRegistryManifest(manifest)
	if err != nil {
		return err
	}
	if err := writeFileManaged(ctx, filepath.Join(registryDir, "skills.json"), data, replace); err != nil {
		return err
	}
	var script string
	script += "#!/usr/bin/env sh\nset -eu\n\n"
	script += "# Get GitHub token from gh CLI to avoid rate limits\n"
	script += "GITHUB_TOKEN=$(gh auth token 2>/dev/null) || GITHUB_TOKEN=\"\"\n\n"
	script += fmt.Sprintf("# Install registry-managed skills. Custom skills live in %s.\n", filepath.Join(ctx.Options.AgentsDir, "skills"))
	for _, skill := range manifest.Skills {
		script += registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir)
		script += "\n"
	}
	if err := writeFileManaged(ctx, filepath.Join(registryDir, "install.sh"), []byte(script), replace); err != nil {
		return err
	}
	installScript := filepath.Join(ctx.Options.AgentsDir, "registry", "install.sh")
	readme := fmt.Sprintf("# Registry Skills\n\nThese skills are installed from the public Skills registry so updates can come from upstream.\n\nRun:\n\n```bash\nsh %s\n```\n", shellWord(installScript))
	return writeFileManaged(ctx, filepath.Join(registryDir, "README.md"), []byte(readme), replace)
}

// writeMCPReadme writes a short README into <AgentsDir>/mcp/ explaining
// that the directory is the shared MCP source of truth.
func writeMCPReadme(ctx Context, replace bool) error {
	readme := "# Shared MCP Presets\n\n`servers.json` is the source of truth for personal MCP servers.\n\nStable file-based adapters merge these presets into their native user-level config. CLI-backed or UI-backed tools get generated helper files under `~/.agents/generated/<agent>/`.\n"
	return writeFileManaged(ctx, filepath.Join(ctx.Options.AgentsDir, "mcp", "README.md"), []byte(readme), replace)
}

// installRegistrySkills runs each entry of the registry manifest
// against the shared `npx --yes skills add ... --agent universal` flow.
// Skipped when ctx.DryRun is true or when the registry manifest is
// empty. Each npx invocation gets AGENTS_HOME (so the upstream installer
// targets the shared universal home) and GITHUB_TOKEN (to avoid rate
// limits on the registry fetch).
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
	baseEnv := append(os.Environ(), "AGENTS_HOME="+ctx.Options.AgentsDir)
	var ghToken string
	if token, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		ghToken = strings.TrimSpace(string(token))
		baseEnv = append(baseEnv, "GITHUB_TOKEN="+ghToken)
	}
	for _, skill := range manifest.Skills {
		args := registryCommandArgs(skill, true, ctx.CopyMode)
		ctx.Report.Line("registry: %s from %s@%s", skill.Name, skill.Source, skill.Skill)
		if ctx.DryRun {
			ctx.Report.Line("run: %s", registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir))
			continue
		}
		c := exec.Command("npx", args...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
		c.Env = baseEnv
		if err := c.Run(); err != nil {
			msg := fmt.Sprintf("%v", err)
			if ghToken != "" {
				msg = strings.ReplaceAll(msg, ghToken, "***")
			}
			ctx.Report.Line("warning: registry skill %s failed: %s", skill.Name, msg)
			continue
		}
	}
	return nil
}

// registryCommand renders the npx invocation for one skill. It is
// shared between the static install.sh helper and the runtime
// installer so both surfaces stay in lock-step.
func registryCommand(skill RegistrySkill, global bool, copyMode bool, agentsDir string) string {
	args := registryCommandArgs(skill, global, copyMode)
	parts := []string{fmt.Sprintf("AGENTS_HOME=%s", shellWord(agentsDir)), "npx"}
	for _, arg := range args {
		parts = append(parts, shellWord(arg))
	}
	return strings.Join(parts, " ")
}

// registryCommandArgs builds the argv for `npx --yes skills add`. The
// installer targets the shared universal skills home; ns-workspace
// owns adapter fan-out afterwards.
func registryCommandArgs(skill RegistrySkill, global bool, copyMode bool) []string {
	args := []string{"--yes", "skills", "add", skill.Source, "--skill", skill.Skill}
	if global {
		args = append(args, "--global")
	}
	args = append(args, "--agent", registryAgentTarget, "--yes")
	if copyMode {
		args = append(args, "--copy")
	}
	return args
}

// encodeRegistryManifest is the seam used by writeRegistryHelpers so tests
// can inject encoding errors (the production encoding cannot fail because
// RegistryManifest is a typed struct of strings/slices, but the seam
// keeps the error branch covered for future schema evolution).
var encodeRegistryManifest = func(manifest RegistryManifest) ([]byte, error) {
	return encodeJSONIndent(manifest)
}
