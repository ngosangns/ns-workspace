---
type: architecture
title: "Kiến Trúc Tổng Quan"
description: "Tổng quan kiến trúc hiện tại của `ns-workspace`, bao gồm CLI, adapter sync, preview web và các quan hệ tài liệu cốt lõi."
tags: ["architecture", "overview"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Kiến Trúc Tổng Quan

## Meta

- **Status**: active
- **Description**: Tổng quan kiến trúc hiện tại của `ns-workspace`, bao gồm CLI, adapter sync, preview web và các quan hệ tài liệu cốt lõi.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module kbmcp](../modules/kbmcp.md), [Module graph query](../modules/graphquery.md), [Preview web](../features/preview-web.md), [Aspect inventory](../research/aspect-inventory.md), [Thuật ngữ](../shared/glossary.md)

## Tổng Quan

`ns-workspace` gồm hai bề mặt chính: CLI đồng bộ cấu hình agent và preview/search web để đọc knowledge base của một project. CLI giữ preset trong repo, ghi hoặc link sang các vị trí user-level của agent, còn preview web phục vụ tài liệu trong `docs/` qua local HTTP server (SolidJS SPA + PreviewHandler). Lệnh `search` dùng backend preview/search để serve API/SPA và mở HTML launcher. Lệnh `graph` chỉ chạy query terminal bằng Search/LSP Code Graph, `export` xuất docs + graph thành một file HTML tĩnh self-contained (viewer SolidJS), `mcp` cung cấp command-line truy cập `docs/` dưới dạng JSON, `preview` serve docs SPA local, còn nhóm `lsp` quản lý language server dùng cho Code Graph.

## Thành Phần

- CLI Go trong `main.go` route các lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents`, `preview`, `search`, `graph`, `export`, `mcp`, `kb` và `lsp`.
- Package `internal/cli` parse flags và dispatch nhóm command agentsync.
- Package `internal/agentsync` gom logic adapter, `SyncPlan` và operation sync cho các agent. Stable adapters hiện gồm Claude Code, OpenCode, Grok Build, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI và Cline; module này được mô tả trong [Module agentsync](../modules/agentsync.md).
- Lệnh `update` rewrite artifact do `internal/agentsync` quản lý từ preset hiện tại. File, tree và JSON key managed được backup trước khi ghi; entry hoặc key đã bị xóa khỏi preset sẽ không được giữ lại trong output hiện tại.
- Package `internal/preview` scan docs, parse metadata (YAML frontmatter OKF + `## Meta` prose), dựng graph, phục vụ API cho `search`/`graph`, xuất static HTML (`export` với viewer SolidJS), chạy `preview` bằng SolidJS SPA embed + PreviewHandler, và query LSP Code Graph. Package `internal/kbmcp` cung cấp command-line truy cập `docs/` qua `Knowledge` façade của preview; mỗi lệnh chạy một lần và trả JSON. Package `internal/graphquery` sở hữu registry/setup/cache LSP cho Code Graph, chạy CLI `lsp`, và được `graph --query` dùng lại khi cần chuẩn bị language server; `graph --no-ensure-lsp` bỏ qua bước cài tự động.
- Portal và preview frontend dùng SolidJS + TypeScript 7: `internal/portal/portal_ui_src/` → `portal_ui/`, `internal/preview/preview_ui_src/` → `preview_ui/`.
- Preset agent instruction trong `presets/agents/AGENTS.md` nhận trigger skill dạng `//<tag>` cho pipeline research, search docs, init knowledge base, plan, execution, fix, cleanup audit, update-docs và commit (`//c` → registry skill `git-commit`). Trigger riêng `/s` gọi skill `spawn-opencode` để spawn OpenCode process như sub-agent.

## Quan Hệ

Preview đọc `docs/_index.md` và `docs/_sync.md` khi có, đồng thời scan toàn bộ file text dưới `docs/`. Bảng `## Modules` trong `_index.md`, field `**Links**` trong `## Meta`, relationship map và dependency diagram tạo thành graph điều hướng. Search page và Search standalone đọc docs corpus, source code corpus ngoài `docs/`, và Code Graph dựng symbol graph từ LSP trên source code tracked bởi Git khi language server tương ứng có sẵn. Resolver LSP ưu tiên môi trường user/project trước cache `ns-workspace`, còn web request chỉ fail-open với warning; side effect cài package chỉ xảy ra khi CLI setup được gọi rõ ràng.

## Quyết Định Liên Quan

Thiết kế adapter hiện tại nằm trong [Module agentsync](../modules/agentsync.md). Thiết kế preview hiện tại được mô tả ở [Module preview](../modules/preview.md). Thiết kế knowledge base nằm trong [Module preview](../modules/preview.md) (SolidJS docs preview + search/graph/export/kb) và [Module kbmcp](../modules/kbmcp.md) (command-line docs query). Thiết kế setup/cache LSP cho graph query nằm trong [Module graph query](../modules/graphquery.md). Inventory aspect để onboarding repo nằm trong [Aspect inventory](../research/aspect-inventory.md).
