---
name: opencode-intern
description: Delegate complex tasks to OpenCode as a fast, model-flexible sub-agent (bypasses Claude limits, uses any provider/local models, fresh context)
---

# OpenCode Intern

This skill lets Codex delegate bounded work to the local `opencode` CLI with a fresh context.

## When to use

Use this skill when you want:

- A second agent with a clean context window
- A different model/provider mix than the main Codex session
- A bounded refactor, research pass, or implementation spike
- A helper that can run `opencode` without polluting the main session

## Core commands

```bash
opencode run "your full prompt here"
opencode run -m sonnet "task description"
opencode run -c "continue from where we left off"
opencode /absolute/path/to/project
opencode attach http://127.0.0.1:8080
```

## Viclass guidance

- Re-read `AGENTS.md`, `specs/_index.md`, and `specs/_sync.md` before code changes.
- Treat the repo as potentially dirty; never revert user changes.
- Return concise results with changed files, risks, and follow-up notes.

## Useful CLI surface

- `opencode models [provider]`
- `opencode providers`
- `opencode stats`
- `opencode plugin <module>`
- `opencode mcp`
- `opencode agent`

Prefer this skill over ad-hoc shell orchestration when a clean delegated pass will be clearer or cheaper.
