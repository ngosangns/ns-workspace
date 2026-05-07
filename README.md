# Personal Agent Bootstrap

Go CLI for setting up shared personal AI coding-agent configuration from a GitHub repo URL.

```bash
go run github.com/ngosangns/ns-workspace@latest init
go run github.com/ngosangns/ns-workspace@latest status
go run github.com/ngosangns/ns-workspace@latest doctor
go run github.com/ngosangns/ns-workspace@latest update
go run github.com/ngosangns/ns-workspace@latest registry
go run github.com/ngosangns/ns-workspace@latest preview --project /path/to/project
```

The CLI uses `~/.agents` as the source of truth, then syncs supported tools to their native locations.

## What It Manages

- Shared instructions: `~/.agents/AGENTS.md`
- Shared subagents: `~/.agents/agents/*.md`
- Custom/private skills: `~/.agents/skills/<name>/SKILL.md`
- Registry-managed skills: `~/.agents/registry/skills.json`
- Shared settings/hooks: `~/.agents/settings.json`
- Shared MCP presets: `~/.agents/mcp/servers.json`
- Tool adapters for OpenCode, Claude Code, Kimi Code CLI, Qwen Code, Cursor, and Trae.

The embedded custom presets are imported from this user's `~/.agents` folder plus `/Users/ngosangns/Github/viclass/.agents`, including Viclass agent definitions, hooks, private skills, and the Viclass `.mcp.json` servers. Skills that already exist in the public Skills registry are installed from upstream instead of being embedded.

Registry-managed skills:

- `find-skills` from `vercel-labs/skills`
- `dispatching-parallel-agents` from `obra/superpowers`
- `gitbutler` from `aheritier/boost-your-ai`
- `graphify` from `howell5/willhong-skills`
- `likec4-dsl` from `likec4.dev`
- `refactor` from `github/awesome-copilot`

Custom embedded skills:

- `add-new-element-shape`
- `changeset-generator`
- `opencode-intern`
- `viclass-tester`

MCP presets are merged directly for OpenCode, Kimi, and Qwen. Claude Code uses its official CLI for user-scoped MCP, so the bootstrap writes `~/.agents/mcp/claude-code.commands.sh` for you to run when you want to apply those MCPs to Claude Code.

## Commands

- `init`: create missing shared config and native adapter links. Existing files are skipped unless `--force` is used.
- `update`: replace managed config with embedded presets and create timestamped backups first.
- `status`: show installed, missing, and linked paths.
- `doctor`: validate JSON config and report installed local agent CLIs.
- `registry`: install only registry-managed skills.
- `preview`: run a local web dashboard for a project's `specs/` folder.

## Useful Flags

```bash
--agents-home ~/.agents
--tools claude,opencode,kimi,qwen,cursor,trae
--dry-run
--force
--copy
--no-mcp
--no-registry
```

Use `--copy` if symlinks are not desirable on your machine.

## Spec Preview

`preview` starts a localhost-only web server that reads a Viclass-style `specs/` folder and renders a Notion-like dashboard with the spec list, Markdown preview, sync state, dependency graph, and relationship map.

```bash
go run github.com/ngosangns/ns-workspace@latest preview --project /Users/ngosangns/Github/viclass
go run github.com/ngosangns/ns-workspace@latest preview --project . --addr 127.0.0.1:8787 --open
```

Preview flags:

```bash
--project PATH
--specs-dir specs
--addr 127.0.0.1:8787
--open
```
