package agentsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// registrySkillInstallTimeout bounds each registry skill installer
// invocation so a bad or unreachable source cannot hang a sync job forever.
const registrySkillInstallTimeout = 2 * time.Minute

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
	script += "# Get GitHub token from gh CLI to avoid rate limits (npx-skills only)\n"
	script += "GITHUB_TOKEN=$(gh auth token 2>/dev/null) || GITHUB_TOKEN=\"\"\n"
	script += "export GITHUB_TOKEN\n\n"
	script += fmt.Sprintf("# Install registry-managed skills into %s.\n", filepath.Join(ctx.Options.AgentsDir, "skills"))
	script += fmt.Sprintf("export AGENTS_HOME=%s\n\n", shellWord(ctx.Options.AgentsDir))
	for _, skill := range manifest.Skills {
		script += registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir)
		script += "\n"
	}
	if err := writeFileManaged(ctx, filepath.Join(registryDir, "install.sh"), []byte(script), replace); err != nil {
		return err
	}
	installScript := filepath.Join(ctx.Options.AgentsDir, "registry", "install.sh")
	readme := fmt.Sprintf("# Registry Skills\n\nThese skills are installed from upstream sources (npx skills registry and GitButler `but skill install`) so updates can come from the vendor CLIs.\n\nRun:\n\n```bash\nsh %s\n```\n", shellWord(installScript))
	return writeFileManaged(ctx, filepath.Join(registryDir, "README.md"), []byte(readme), replace)
}

// writeMCPReadme writes a short README into <AgentsDir>/mcp/ explaining
// that the directory is the shared MCP source of truth.
func writeMCPReadme(ctx Context, replace bool) error {
	readme := "# Shared MCP Presets\n\n`servers.json` is the source of truth for personal MCP servers.\n\nStable file-based adapters merge these presets into their native user-level config. CLI-backed or UI-backed tools get generated helper files under `~/.agents/generated/<agent>/`.\n"
	return writeFileManaged(ctx, filepath.Join(ctx.Options.AgentsDir, "mcp", "README.md"), []byte(readme), replace)
}

// installRegistrySkills installs each entry of the registry manifest
// into the shared agents home. Default installer is
// `npx --yes skills add ... --agent universal`. Entries with
// installer "but-skill" use `but skill install --path ...` per
// https://docs.gitbutler.com/commands/but-skill.
//
// Skipped when ctx.DryRun is true or when the registry manifest is empty.
// Per-skill failures are reported as warnings so one bad entry does not
// block the rest of update.
func installRegistrySkills(ctx Context) error {
	manifest, err := readRegistryManifest(ctx)
	if err != nil {
		return err
	}
	if len(manifest.Skills) == 0 {
		return nil
	}

	needNpx := false
	needBut := false
	for _, skill := range manifest.Skills {
		switch skill.installerKind() {
		case installerButSkill:
			needBut = true
		default:
			needNpx = true
		}
	}
	if needNpx {
		if _, err := exec.LookPath("npx"); err != nil {
			installScript := filepath.Join(ctx.Options.AgentsDir, "registry", "install.sh")
			return fmt.Errorf("npx is required to install registry skills; rerun with --no-registry or run %s later", installScript)
		}
	}
	if needBut {
		if _, err := exec.LookPath("but"); err != nil {
			// Soft dependency: warn once and skip only but-skill entries
			// later; other installers still run.
			ctx.Report.Line("warning: but CLI not on PATH; GitButler skill install (but skill install) will be skipped — see https://docs.gitbutler.com/commands/but-skill")
		}
	}

	baseEnv := append(os.Environ(), "AGENTS_HOME="+ctx.Options.AgentsDir)
	var ghToken string
	if token, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		ghToken = strings.TrimSpace(string(token))
		baseEnv = append(baseEnv, "GITHUB_TOKEN="+ghToken)
	}

	for _, skill := range manifest.Skills {
		ctx.Report.Line("registry: %s (%s)", skill.Name, skill.installerKind())
		if ctx.DryRun {
			ctx.Report.Line("run: %s", registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir))
			continue
		}
		if err := installOneRegistrySkill(ctx, skill, baseEnv, ghToken); err != nil {
			ctx.Report.Line("warning: registry skill %s failed: %s", skill.Name, err.Error())
		}
	}
	return nil
}

// installOneRegistrySkill runs a single registry entry with a timeout.
func installOneRegistrySkill(ctx Context, skill RegistrySkill, baseEnv []string, ghToken string) error {
	cmdCtx, cancel := context.WithTimeout(context.Background(), registrySkillInstallTimeout)
	defer cancel()

	var c *exec.Cmd
	switch skill.installerKind() {
	case installerButSkill:
		if _, err := exec.LookPath("but"); err != nil {
			return fmt.Errorf("but CLI not found on PATH (install GitButler CLI, then re-run update)")
		}
		path := butSkillInstallPath(ctx.Options.AgentsDir, skill)
		// Non-interactive: --path is required (docs: "In non-interactive
		// mode, specify --path or --detect"). --format none keeps portal
		// / CI logs quiet.
		c = exec.CommandContext(cmdCtx, "but", "skill", "install", "--path", path, "--format", "none")
	default:
		if skill.Source == "" {
			return fmt.Errorf("npx-skills entry %q missing source", skill.Name)
		}
		args := registryCommandArgs(skill, true, ctx.CopyMode)
		c = exec.CommandContext(cmdCtx, "npx", args...)
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	// No stdin: headless portal / CI; installers must not prompt.
	c.Env = baseEnv
	err := c.Run()
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		if ghToken != "" {
			msg = strings.ReplaceAll(msg, ghToken, "***")
		}
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			msg = fmt.Sprintf("timed out after %s", registrySkillInstallTimeout)
		}
		return errors.New(msg)
	}
	return nil
}

// butSkillInstallPath is the destination directory for
// `but skill install --path`. The folder name is the skill id (default
// "but") under the shared skills home.
func butSkillInstallPath(agentsDir string, skill RegistrySkill) string {
	name := strings.TrimSpace(skill.Skill)
	if name == "" {
		name = "but"
	}
	return filepath.Join(agentsDir, "skills", name)
}

// registryCommand renders a shell line for one skill (install.sh + dry-run).
func registryCommand(skill RegistrySkill, global bool, copyMode bool, agentsDir string) string {
	switch skill.installerKind() {
	case installerButSkill:
		path := butSkillInstallPath(agentsDir, skill)
		return strings.Join([]string{
			"but", "skill", "install",
			"--path", shellWord(path),
			"--format", "none",
		}, " ")
	default:
		args := registryCommandArgs(skill, global, copyMode)
		parts := []string{fmt.Sprintf("AGENTS_HOME=%s", shellWord(agentsDir)), "npx"}
		for _, arg := range args {
			parts = append(parts, shellWord(arg))
		}
		return strings.Join(parts, " ")
	}
}

// registryCommandArgs builds the argv for `npx --yes skills add` (npx-skills only).
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
