# Trạng Thái Sync Tài Liệu

## Meta

- **Status**: active
- **Description**: Snapshot sync hiện tại của knowledge base, ghi commit phản ánh, phạm vi kiểm tra và trạng thái worktree còn chưa commit.
- **Compliance**: current-state
- **Links**: [Chỉ mục](./_index.md), [Tài liệu dự án](./README.md)

## Current Sync

- **Last Synced Commit**: ffaacad1a15f6a4072ca1f36b6d988d485b5f730
- **Branch**: current worktree
- **Sync Date**: 2026-05-19T12:48:09Z
- **Scope**: preview search corpus and Code Graph behavior, including strict Git-tracked Code Graph files, keyword-backed Code Semantic embedding and fallback hits, graphify source path normalization, deterministic Code Graph direct-match anchor ranking, per-anchor directed Code Graph expansion through `calls`, graphify file-only node filtering, callable-only Code Graph nodes, owner-prefixed method labels, class/container node filtering, broader code symbol extraction, root caller node theme-aware border, Search Graph focused-node file opening, existing preview web behavior, agent preset trigger skills, OpenCode sub-agent skill, commit skill trigger, commit lint guidance, merge-request-style commit descriptions, bootstrap skill sync docs.
- **Known Unsynced**: Includes current working-tree preview search changes in `internal/preview/preview_search.go`, `internal/preview/preview_test.go`, `internal/preview/preview_ui_src/components/SearchPanel.vue`, `internal/preview/preview_ui_src/js/network_graph.ts`, `internal/preview/spec_project_test.go` and matching docs updates.
