---
type: working-document
title: "Working Document: Kiro Agent tự động load synced skills và steering"
description: "Background shipped: custom agent ns-full loads synced skills và steering từ agentsync materialization."
tags: ["working-document", "kiro"]
timestamp: 2026-07-17T00:00:00Z
status: implemented
compliance: current-state
---

# Working Document: Kiro Agent tự động load synced skills và steering

## Meta

- **Status**: implemented (shipped)
- **Description**: Custom agent `ns-full` load synced skills và steering.
- **Compliance**: current-state
- **Links**: [Module agentsync](../modules/agentsync.md), [Chỉ mục](../_index.md)

## Trạng thái hiện tại

Behavior đã shipped trong preset `presets/settings/kiro.json` và adapter Kiro:

- `resources` gồm `skill://~/.kiro/skills/*/SKILL.md` và `file://~/.kiro/steering/**/*.md`.
- Agentsync materialize `~/.kiro/agents/ns-full.json`, steering `AGENTS.md`, skills, và MCP `~/.kiro/settings/mcp.json` (force `disabled: false` cho server còn trong catalog).
- Chi tiết current-state: [Module agentsync](../modules/agentsync.md) (mục Adapter Catalog / Kiro).

Working document này chỉ giữ bối cảnh thiết kế; không mô tả worktree uncommitted.
