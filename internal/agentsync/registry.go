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
	// Materialize only installable rows — never persist placeholder sources
	// like org/repo into ~/.agents/registry/skills.json or install.sh.
	rawCount := len(manifest.Skills)
	manifest.Skills = SanitizeRegistrySkills(manifest.Skills)
	if skipped := rawCount - len(manifest.Skills); skipped > 0 {
		ctx.Report.Line("warning: omitted %d invalid/placeholder registry skill(s) from helpers", skipped)
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
	// Never emit install commands for placeholder / invalid sources (e.g. org/repo).
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
// Default update skips installers when each skill has a matching
// per-skill stamp and SKILL.md is present. Use Options.RefreshSkills
// to force re-install. Per-skill failures are warnings so one bad
// entry does not block the rest of update.
func installRegistrySkills(ctx Context) error {
	manifest, err := readRegistryManifest(ctx)
	if err != nil {
		return err
	}
	// Drop placeholder sources (org/repo, …) so they never reach npx/git clone.
	raw := manifest.Skills
	manifest.Skills = SanitizeRegistrySkills(raw)
	if skipped := len(raw) - len(manifest.Skills); skipped > 0 {
		ctx.Report.Line("warning: skipped %d registry skill(s) with invalid/placeholder source (e.g. org/repo)", skipped)
	}
	// Portal-disabled top-level skill names are not re-installed.
	disabled := loadDisabledSkills(ctx)
	if len(disabled) > 0 {
		filtered := make([]RegistrySkill, 0, len(manifest.Skills))
		for _, sk := range manifest.Skills {
			if disabled[skillInstallID(sk)] || disabled[strings.TrimSpace(sk.Name)] {
				ctx.Report.Line("registry: skip %s (portal disabled)", skillInstallID(sk))
				continue
			}
			filtered = append(filtered, sk)
		}
		manifest.Skills = filtered
	}
	if len(manifest.Skills) == 0 {
		return nil
	}

	state := loadRegistrySyncState(ctx.Options.AgentsDir)
	plan := planRegistryInstalls(ctx.Options.AgentsDir, manifest.Skills, state, ctx.RefreshSkills)
	if plan.SkipAll {
		ctx.Report.Line("registry: ok (unchanged, %d skills) — skip installers", len(manifest.Skills))
		return nil
	}
	if len(plan.Skipped) > 0 {
		ctx.Report.Line("registry: skip %d unchanged skill(s)", len(plan.Skipped))
	}

	needNpx := false
	needBut := false
	for _, skill := range plan.ToInstall {
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

	// Track success for state: skipped (unchanged+present) count as OK;
	// installs that succeed count as OK.
	installedOK := map[string]bool{}
	for _, id := range plan.Skipped {
		installedOK[id] = true
	}

	for _, skill := range plan.ToInstall {
		id := skillInstallID(skill)
		ctx.Report.Line("registry: %s (%s)", skill.Name, skill.installerKind())
		if ctx.DryRun {
			ctx.Report.Line("run: %s", registryCommand(skill, true, ctx.CopyMode, ctx.Options.AgentsDir))
			continue
		}
		if err := installOneRegistrySkill(ctx, skill, baseEnv, ghToken); err != nil {
			ctx.Report.Line("warning: registry skill %s failed: %s", skill.Name, err.Error())
			continue
		}
		// Stamp only when SKILL.md is on disk (real installers and test
		// stubs that materialize the skill layout).
		if registrySkillPresent(ctx.Options.AgentsDir, skill) {
			installedOK[id] = true
		}
	}

	if ctx.DryRun {
		return nil
	}

	if err := saveRegistrySyncState(ctx.Options.AgentsDir, buildRegistrySyncState(manifest.Skills, installedOK)); err != nil {
		ctx.Report.Line("warning: registry sync state save failed: %v", err)
		return fmt.Errorf("save registry sync state: %w", err)
	}
	return nil
}

// installOneRegistrySkill is the seam used by bulk/portal install so
// tests can count/stub network installers without calling npx/but.
// Production default is installOneRegistrySkillDefault.
var installOneRegistrySkill = installOneRegistrySkillDefault

// installOneRegistrySkillDefault is the production installer (npx / but).
func installOneRegistrySkillDefault(ctx Context, skill RegistrySkill, baseEnv []string, ghToken string) error {
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
		if err := validateRegistrySource(skill.Source); err != nil {
			return fmt.Errorf("npx-skills entry %q: %w", skill.Name, err)
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

// InstallRegistrySkill installs one registry entry into agentsDir using the
// same installers as update/registry (npx skills add or but skill install).
// Intended for portal one-shot installs. Uses copy mode so agent skill dirs
// that do not follow symlinks (e.g. Kiro IDE) still see the skill.
// On success, stamps registry/.sync-state.json so a later unchanged
// update skips re-installing this skill.
func InstallRegistrySkill(agentsDir string, skill RegistrySkill) error {
	if strings.TrimSpace(agentsDir) == "" {
		return fmt.Errorf("agents home is required")
	}
	if strings.TrimSpace(skill.Skill) == "" && skill.installerKind() != installerButSkill {
		return fmt.Errorf("skill id is required")
	}
	if skill.Name == "" {
		skill.Name = skill.Skill
	}
	baseEnv := append(os.Environ(), "AGENTS_HOME="+agentsDir)
	var ghToken string
	if token, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		ghToken = strings.TrimSpace(string(token))
		baseEnv = append(baseEnv, "GITHUB_TOKEN="+ghToken)
	}
	ctx := Context{
		Options: Options{AgentsDir: agentsDir, CopyMode: true},
		Report:  stdoutReporter{},
	}
	if err := installOneRegistrySkill(ctx, skill, baseEnv, ghToken); err != nil {
		return err
	}
	if err := stampOneRegistrySkill(agentsDir, skill); err != nil {
		return fmt.Errorf("stamp registry skill after install: %w", err)
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

// placeholderRegistrySources are documentation/test-only values that must
// never be cloned or installed. Keep this list authoritative for install,
// install.sh generation, and portal write validation.
var placeholderRegistrySources = map[string]bool{
	"org/repo":            true,
	"owner/repo":          true,
	"user/repo":           true,
	"example/repo":        true,
	"your-org/your-repo":  true,
	"your-org/your-skill": true,
	"foo/bar":             true,
	"acme/example":        true,
}

// NormalizeGitHubSource strips common GitHub URL prefixes and .git suffix,
// returning owner/repo form when possible.
func NormalizeGitHubSource(source string) string {
	s := strings.TrimSpace(source)
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.TrimPrefix(s, "http://github.com/")
	s = strings.TrimPrefix(s, "git@github.com:")
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	return s
}

// IsPlaceholderRegistrySource reports whether source is a known docs/test
// placeholder (e.g. org/repo) and must never be used for install or clone.
func IsPlaceholderRegistrySource(source string) bool {
	n := strings.ToLower(NormalizeGitHubSource(source))
	if n == "" {
		return false
	}
	if placeholderRegistrySources[n] {
		return true
	}
	// Also reject obvious template patterns.
	if strings.Contains(n, "your-") || strings.Contains(n, "example-org") {
		return true
	}
	return false
}

// ValidateRegistrySource rejects empty or placeholder GitHub sources that
// come from docs/tests (e.g. "org/repo") so update does not spend time
// cloning a non-existent repo and flood the portal log with auth noise.
func ValidateRegistrySource(source string) error {
	return validateRegistrySource(source)
}

func validateRegistrySource(source string) error {
	s := strings.TrimSpace(source)
	if s == "" {
		return fmt.Errorf("empty source")
	}
	normalized := NormalizeGitHubSource(s)
	if IsPlaceholderRegistrySource(s) {
		return fmt.Errorf("placeholder source %q is not a real repository; remove it from registry skills.json (portal Registry tab or reset overlay)", source)
	}
	if !strings.Contains(normalized, "/") {
		return fmt.Errorf("source %q must look like owner/repo (got no '/')", source)
	}
	parts := strings.Split(normalized, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("source %q must look like owner/repo", source)
	}
	return nil
}

// ValidateRegistrySkill validates one registry entry for installability.
// but-skill entries do not require Source; npx-skills require a real source.
func ValidateRegistrySkill(skill RegistrySkill) error {
	name := strings.TrimSpace(skill.Name)
	if name == "" {
		name = strings.TrimSpace(skill.Skill)
	}
	switch skill.installerKind() {
	case installerButSkill:
		if strings.TrimSpace(skill.Skill) == "" && strings.TrimSpace(skill.Name) == "" {
			return fmt.Errorf("but-skill entry missing skill id")
		}
		return nil
	default:
		if strings.TrimSpace(skill.Skill) == "" {
			return fmt.Errorf("registry skill %q missing skill id", name)
		}
		if err := validateRegistrySource(skill.Source); err != nil {
			return fmt.Errorf("registry skill %q: %w", name, err)
		}
		return nil
	}
}

// SanitizeRegistrySkills returns only entries safe to install / write into
// install.sh. Placeholder sources (org/repo, …) are dropped.
func SanitizeRegistrySkills(skills []RegistrySkill) []RegistrySkill {
	if len(skills) == 0 {
		return skills
	}
	out := make([]RegistrySkill, 0, len(skills))
	for _, sk := range skills {
		if err := ValidateRegistrySkill(sk); err != nil {
			continue
		}
		out = append(out, sk)
	}
	return out
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
