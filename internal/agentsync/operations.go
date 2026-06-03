package agentsync

import "path/filepath"

type EnsureDir struct {
	Dir string
}

func (op EnsureDir) Apply(ctx Context) error { return ensureDir(ctx, op.Dir) }
func (op EnsureDir) Describe(ctx Context)    { ctx.Report.Line("mkdir: %s", op.Dir) }
func (op EnsureDir) Path() string            { return op.Dir }

type WriteRegistryHelpers struct {
	Replace bool
}

func (op WriteRegistryHelpers) Apply(ctx Context) error {
	return writeRegistryHelpers(ctx, op.Replace)
}

func (op WriteRegistryHelpers) Describe(ctx Context) {
	ctx.Report.Line("registry helpers: %s", filepath.Join(ctx.Options.AgentsDir, "registry"))
}
func (op WriteRegistryHelpers) Path() string {
	return "registry"
}

type RegistryInstall struct{}

func (op RegistryInstall) Apply(ctx Context) error { return installRegistrySkills(ctx) }
func (op RegistryInstall) Describe(ctx Context)    { ctx.Report.Line("registry install") }
func (op RegistryInstall) Path() string {
	return "registry/skills.json"
}

type WriteMCPReadme struct {
	Replace bool
}

func (op WriteMCPReadme) Apply(ctx Context) error { return writeMCPReadme(ctx, op.Replace) }
func (op WriteMCPReadme) Describe(ctx Context) {
	ctx.Report.Line("mcp readme: %s", filepath.Join(ctx.Options.AgentsDir, "mcp", "README.md"))
}
func (op WriteMCPReadme) Path() string {
	return "mcp/README.md"
}
