# Personal Agent Instructions

These instructions are shared across local coding agents through `~/.agents`.

## Collaboration

- Be concise, direct, and practical.
- Prefer reading the existing codebase before making assumptions.
- Preserve user changes and avoid destructive git operations unless explicitly requested.
- Explain risky changes before making them.

## Engineering Defaults

- Use the repository's existing patterns before adding new abstractions.
- Keep changes scoped to the requested behavior.
- Run focused tests or checks when code changes are made.
- Treat secrets, credentials, `.env` files, and private keys as sensitive.

## Personal Workflow

- Prefer `rg` for search and fast local inspection.
- Prefer small, reviewable commits when asked to commit.
- For frontend work, verify responsive layout and avoid decorative UI that does not serve the task.

## Shared Skills And Agents

- Use installed skills from `~/.agents/skills` when a request matches a skill description.
- Use shared subagents from `~/.agents/agents` when the active tool supports external agent definitions.
- When the user types `/graphify`, invoke the `graphify` skill before doing anything else.

## Viclass Workflow

- In the Viclass repository, read `graphify-out/GRAPH_REPORT.md` before broad codebase exploration when it exists.
- Prefer repo-local specs and commands before ad hoc investigation.
- Keep implementation notes and specs synchronized when changing business rules or architecture.
