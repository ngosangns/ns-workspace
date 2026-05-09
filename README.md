# Personal Agent Bootstrap

`ns-workspace` la mot Go CLI nho dung de bootstrap va dong bo cau hinh cho cac AI coding agent ca nhan. Du an gom cac preset ve instructions, skills, subagents, settings, hooks va MCP servers, roi day chung sang dung vi tri native cua tung tool nhu Claude Code, OpenCode, Kimi, Qwen, Gemini, Codex, Cline, Windsurf, Aider va mot so adapter khac.

Y tuong chinh: dung `~/.agents` lam "source of truth", sau do sync sang cac agent khac de minh co cung mot bo workflow va convention tren nhieu tool.

## Luu Y

Day la du an vibe coding ca nhan, duoc tao va phat trien nhanh de phuc vu workflow rieng. Vi vay cac function, adapter path, MCP config hoac generated artifact co the khong dam bao chinh xac 100% tren moi may, moi version tool, hay moi moi truong.

Hay dung `doctor`, `status`, `--dry-run`, va doc diff/backups truoc khi apply len moi truong quan trong.

## Cai Dat Va Chay Nhanh

Khong can clone repo neu chi muon chay ban moi nhat:

```bash
go run github.com/ngosangns/ns-workspace@latest init
```

Mot so lenh hay dung:

```bash
go run github.com/ngosangns/ns-workspace@latest status
go run github.com/ngosangns/ns-workspace@latest doctor
go run github.com/ngosangns/ns-workspace@latest update
go run github.com/ngosangns/ns-workspace@latest agents
```

Neu dang lam trong checkout cua repo nay:

```bash
go run . status
go run . doctor
go run . preview --project . --addr 127.0.0.1:8787 --open
```

## Cach Dung Don Gian

1. Kiem tra trang thai hien tai:

```bash
go run github.com/ngosangns/ns-workspace@latest status
```

2. Xem truoc nhung gi se duoc ghi:

```bash
go run github.com/ngosangns/ns-workspace@latest init --dry-run
```

3. Tao cau hinh ban dau:

```bash
go run github.com/ngosangns/ns-workspace@latest init
```

4. Cap nhat lai preset da quan ly:

```bash
go run github.com/ngosangns/ns-workspace@latest update
```

5. Kiem tra JSON config va cac CLI agent da cai:

```bash
go run github.com/ngosangns/ns-workspace@latest doctor
```

## Cac Lenh Chinh

- `init`: tao cau hinh shared va link/copy sang cac adapter native. Mac dinh bo qua file da ton tai, tru khi dung `--force`.
- `update`: thay the cac phan config do tool quan ly bang preset embedded va tao backup timestamp truoc khi ghi.
- `status`: hien thi path da cai, path thieu, va link hien co.
- `doctor`: validate JSON config va report cac local agent CLI.
- `registry`: cai cac skill lay tu registry.
- `agents`: liet ke adapter duoc ho tro, support tier va artifact support.
- `preview`: chay web dashboard local de doc va search thu muc `docs/` cua mot project.

## Flag Hay Dung

```bash
--agents-home ~/.agents
--tools all
--tools stable
--tools claude,opencode,kimi,qwen,gemini,codex,cline,windsurf,aider,cursor,trae
--dry-run
--force
--copy
--no-mcp
--no-registry
```

Dung `--copy` neu khong muon tao symlink.

## Du Lieu Duoc Quan Ly

- Shared instructions: `~/.agents/AGENTS.md`
- Shared subagents: `~/.agents/agents/*.md`
- Custom/private skills: `~/.agents/skills/<name>/SKILL.md`
- Registry-managed skills: `~/.agents/registry/skills.json`
- Shared settings/hooks: `~/.agents/settings.json`
- Shared MCP presets: `~/.agents/mcp/servers.json`
- User-level adapters cho Claude Code, OpenCode, Kimi Code CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf, Aider, Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae va Roo.

## Adapter Support

Stable adapters ghi vao cac user-level path da biet:

| Agent | User-level targets |
| --- | --- |
| Claude Code | `~/.claude/CLAUDE.md`, `~/.claude/settings.json` with hooks, `~/.claude/skills`, `~/.claude/agents`, generated MCP commands |
| OpenCode | `$XDG_CONFIG_HOME/opencode/AGENTS.md`, `skill/`, `agent/`, `opencode.json` with hooks and MCP |
| Kimi Code CLI | `~/.kimi/AGENTS.md`, `~/.kimi/skills`, `~/.kimi/mcp.json` |
| Qwen Code | `~/.qwen/QWEN.md`, `~/.qwen/skills`, `~/.qwen/settings.json` with hooks and MCP |
| Gemini CLI | `~/.gemini/GEMINI.md`, `~/.gemini/skills`, `~/.gemini/settings.json` with hooks and MCP |
| Codex CLI | `~/.codex/AGENTS.md`, `~/.codex/skills`, managed MCP block in `~/.codex/config.toml` |
| Cline | `~/.cline/data/skills`, `~/.cline/data/agents`, `~/.cline/data/settings/cline_mcp_settings.json` |
| Windsurf | `~/.codeium/windsurf/memories/global_rules.md` |
| Aider | managed conventions block in `~/.aider.conf.yml` |

Manual hoac experimental adapters tao guidance trong `~/.agents/generated/<agent>/` thay vi ghi truc tiep vao native path chua chac chan. Nhom nay hien gom Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae va Roo.

## Docs Preview

Lenh `preview` chay mot localhost web server de doc thu muc `docs/` cua project. Dashboard co danh sach docs, Markdown preview, sync state, typed docs graph va trang search gom Docs Semantic, Docs Graph, Code Semantic, Code Graph. Cac graph panel dung `graphify-out/graph.json` neu file nay ton tai. Markdown code fence `mermaid` se duoc render thanh diagram trong browser.

Vi du:

```bash
go run github.com/ngosangns/ns-workspace@latest preview --project /Users/ngosangns/Github/viclass
go run github.com/ngosangns/ns-workspace@latest preview --project . --addr 127.0.0.1:8787 --open
go run . preview --project . --addr 127.0.0.1:8787 --open
```

Preview flags:

```bash
--project PATH
--docs-dir docs
--addr 127.0.0.1:8787
--open
```
