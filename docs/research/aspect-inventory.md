---
type: research
title: "Aspect Inventory"
description: "Inventory các aspect chính của `ns-workspace` để người mới hiểu domain, module boundary, workflow, invariants và docs gaps hiện tại."
tags: ["research", "aspect-inventory"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Aspect Inventory

## Meta

- **Status**: active
- **Description**: Inventory các aspect chính của `ns-workspace` để người mới hiểu domain, module boundary, workflow, invariants và docs gaps hiện tại.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Tài liệu dự án](../README.md), [Kiến trúc tổng quan](../architecture/overview.md), [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md), [Preview web](../features/preview-web.md), [Thuật ngữ](../shared/glossary.md)

## Cách Đọc

Inventory này là bản đồ current-state cho lần khởi tạo knowledge base. Mỗi aspect ghi lý do quan trọng với người mới, source paths chính, docs hiện có, khoảng trống còn lại, doc target và priority. P0 là các boundary cần hiểu trước khi sửa repo; P1 là workflow hoặc constraint quan trọng nhưng có thể đọc sau P0.

## P0 Aspects

### Domain Và Project Purpose

- **Lý do**: `ns-workspace` là công cụ cá nhân để đồng bộ cấu hình nhiều AI coding agent từ một nguồn chung; nếu hiểu sai mục tiêu này, người sửa rất dễ coi repo như preview app đơn thuần.
- **Source paths**: `README.md`, `main.go`, `presets/agents/AGENTS.md`, `presets/skills/`.
- **Docs hiện có**: [Tài liệu dự án](../README.md), [Kiến trúc tổng quan](../architecture/overview.md), [Thuật ngữ](../shared/glossary.md).
- **Khoảng trống**: Đã đủ cho purpose cấp cao; inventory này giữ bản đồ aspect để onboarding nhanh hơn.
- **Doc target**: [Tài liệu dự án](../README.md), file này.
- **Priority**: P0.

### Public Commands Và User Workflows

- **Lý do**: CLI là public surface chính; các lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents`, `preview`, `search`, `graph` và `lsp` có side effect khác nhau.
- **Source paths**: `main.go`, `internal/cli/agentsync.go`, `internal/agentsync/`, `internal/preview/preview.go`, `internal/preview/graph.go`, `internal/graphquery/lsp.go`, `main_test.go`, `internal/cli/agentsync_test.go`, `internal/preview/preview_test.go`.
- **Docs hiện có**: [Tài liệu dự án](../README.md), [Kiến trúc tổng quan](../architecture/overview.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md).
- **Khoảng trống**: Adapter workflow cần module doc riêng để người mới không phải ghép thông tin từ README, architecture và code.
- **Doc target**: [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md).
- **Priority**: P0.

### Adapter Sync Boundary

- **Lý do**: `internal/agentsync` là lõi bootstrap/sync cấu hình agent, bao gồm native targets, backup, symlink/copy, JSON merge, managed blocks và adapter support tiers.
- **Source paths**: `internal/agentsync/`, `internal/cli/agentsync.go`, `internal/agentsync/agentsync_test.go`, `internal/cli/agentsync_test.go`, `presets/agents/AGENTS.md`, `presets/settings/settings.json`, `presets/mcp/servers.json`, `presets/registry/skills.json`, `presets/opencode/opencode.json`.
- **Docs hiện có**: [Kiến trúc tổng quan](../architecture/overview.md), `README.md`, `DEVELOPER.md`.
- **Khoảng trống**: Cần module doc riêng để mô tả operation model, adapter catalog, safety rules và failure modes.
- **Doc target**: [Module agentsync](../modules/agentsync.md).
- **Priority**: P0.

### Preset Source Và Config Model

- **Lý do**: Presets là source of truth được Go embed, build thành `SyncPlan`, rồi materialize vào `~/.agents` trước khi link/copy sang native paths.
- **Source paths**: `presets/agents/AGENTS.md`, `presets/skills/*/SKILL.md`, `presets/subagents/opencode-intern.md`, `presets/settings/settings.json`, `presets/mcp/servers.json`, `presets/registry/skills.json`, `presets/opencode/opencode.json`, `main.go`, `internal/agentsync/plan.go`.
- **Docs hiện có**: [Tài liệu dự án](../README.md), [Module agentsync](../modules/agentsync.md), [Kiến trúc tổng quan](../architecture/overview.md).
- **Khoảng trống**: Nếu preset surface tiếp tục lớn lên, tách thêm `docs/shared/preset-model.md` để tránh dồn quá nhiều vào module agentsync.
- **Doc target**: [Module agentsync](../modules/agentsync.md), [Thuật ngữ](../shared/glossary.md).
- **Priority**: P0.

### Preview, Search Và Docs Graph

- **Lý do**: Preview web là workflow đọc knowledge base, search docs/code, mở graph và validate metadata docs.
- **Source paths**: `internal/preview/preview.go`, `internal/preview/preview_api.go`, `internal/preview/spec_project.go`, `internal/preview/preview_search.go`, `internal/preview/preview_ui_src/`, `internal/preview/preview_ui/`, `vite.config.ts`, `package.json`.
- **Docs hiện có**: [Preview web](../features/preview-web.md), [Module preview](../modules/preview.md), [Quy ước frontend preview](../development/conventions/preview-frontend.md).
- **Khoảng trống**: Planning docs còn mô tả một số context lịch sử; shipped behavior đã nằm trong feature/module docs.
- **Doc target**: [Preview web](../features/preview-web.md), [Module preview](../modules/preview.md).
- **Priority**: P0.

### LSP Code Graph Và Graph Query

- **Lý do**: Code Graph thay thế graphify runtime bằng LSP, có installer/cache riêng và phải fail-open khi thiếu binary hoặc capability.
- **Source paths**: `internal/preview/preview_lsp.go`, `internal/preview/preview_lsp_setup.go`, `internal/graphquery/`, `internal/preview/graph.go`, `presets/skills/lsp-code-graph/SKILL.md`.
- **Docs hiện có**: [Module graph query](../modules/graphquery.md), [Module preview](../modules/preview.md), [Preview web](../features/preview-web.md), planning docs trong `docs/specs/planning/`.
- **Khoảng trống**: Không cần doc mới; cần giữ sync khi thêm language server hoặc thay đổi side effect cài đặt.
- **Doc target**: [Module graph query](../modules/graphquery.md), [Module preview](../modules/preview.md).
- **Priority**: P0.

### Docs Metadata Và Knowledge Graph Contract

- **Lý do**: Preview scan `_index.md`, `_sync.md`, `## Meta`, frontmatter, HTML `doc-meta`, relationship map và Mermaid dependency diagram để dựng graph điều hướng.
- **Source paths**: `docs/_index.md`, `docs/_sync.md`, `docs/**/*.md`, `internal/preview/spec_project.go`, `internal/preview/spec_project_test.go`.
- **Docs hiện có**: [Chỉ mục](../_index.md), [Tài liệu dự án](../README.md), [Module preview](../modules/preview.md).
- **Khoảng trống**: Chỉ mục cần liệt kê mọi doc quan trọng đang tồn tại; inventory này được thêm vào index để graph thấy được scope bootstrap.
- **Doc target**: [Chỉ mục](../_index.md), file này.
- **Priority**: P0.

## P1 Aspects

### Data Models Và Managed Artifacts

- **Lý do**: Người sửa cần biết data structs nào là API nội bộ ổn định và artifact nào là generated output.
- **Source paths**: `internal/agentsync/agentsync.go`, `internal/preview/spec_project.go`, `internal/preview/preview_search.go`, `internal/graphquery/lsp.go`, `internal/preview/preview_ui/`, `internal/preview/preview_ui_src/`.
- **Docs hiện có**: [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md), [Quy ước frontend preview](../development/conventions/preview-frontend.md).
- **Khoảng trống**: Nếu API JSON preview trở thành external contract, tách thêm doc shared/API contract riêng.
- **Doc target**: [Module preview](../modules/preview.md), [Module agentsync](../modules/agentsync.md).
- **Priority**: P1.

### External Integrations

- **Lý do**: Repo chạm tới agent CLIs, native config paths, registry install qua `npx`, MCP server presets, browser preview, npm/Vite và language server installers.
- **Source paths**: `internal/agentsync/agentsync.go`, `internal/graphquery/`, `internal/preview/preview.go`, `package.json`, `presets/mcp/servers.json`, `presets/registry/skills.json`.
- **Docs hiện có**: [Tài liệu dự án](../README.md), [Module agentsync](../modules/agentsync.md), [Module graph query](../modules/graphquery.md), `DEVELOPER.md`.
- **Khoảng trống**: External URLs trong adapter catalog nên được kiểm tra khi thêm/sửa adapter vì chúng có thể đổi theo tool vendor.
- **Doc target**: [Module agentsync](../modules/agentsync.md), [Module graph query](../modules/graphquery.md).
- **Priority**: P1.

### Domain Vocabulary

- **Lý do**: Thuật ngữ như `~/.agents`, adapter, preset, managed block, spec, feature doc, module doc và metadata doc xuất hiện khắp code/docs.
- **Source paths**: `README.md`, `docs/shared/glossary.md`, `internal/agentsync/agentsync.go`, `internal/preview/spec_project.go`.
- **Docs hiện có**: [Thuật ngữ](../shared/glossary.md).
- **Khoảng trống**: Glossary cần giữ terms agentsync/preset để docs và code dùng cùng vocabulary.
- **Doc target**: [Thuật ngữ](../shared/glossary.md).
- **Priority**: P1.

### Invariants Và Business Rules

- **Lý do**: Các rule như `init` không overwrite nếu không `--force`, `update` rewrite managed output có backup, preview/search read-only, HTTP không auto-install LSP và generated preview UI không được index bởi LSP là guardrail quan trọng.
- **Source paths**: `internal/agentsync/agentsync.go`, `internal/preview/preview_lsp.go`, `internal/preview/preview_api.go`, `internal/preview/preview_search.go`, tests trong `main_test.go`, `internal/agentsync/agentsync_test.go`, `internal/preview/preview_test.go`.
- **Docs hiện có**: [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md), [Preview web](../features/preview-web.md).
- **Khoảng trống**: Không cần doc requirements riêng lúc này vì repo đang dùng flat module docs; tạo `requirements.md` nếu sau này migrate module folders.
- **Doc target**: [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md).
- **Priority**: P1.

### Failure Modes

- **Lý do**: CLI có thể ghi user-level config thật; registry cần `npx`; JSON native có thể invalid; LSP install/network/checksum/server capability có thể fail; docs dir có thể thiếu.
- **Source paths**: `internal/agentsync/agentsync.go`, `internal/graphquery/lsp.go`, `internal/preview/graph.go`, `internal/preview/preview_lsp.go`, `README.md`, `DEVELOPER.md`.
- **Docs hiện có**: [Tài liệu dự án](../README.md), [Module agentsync](../modules/agentsync.md), [Module graph query](../modules/graphquery.md), [Module preview](../modules/preview.md).
- **Khoảng trống**: Safety notes đang đủ cho current-state nhưng cần cập nhật khi thêm side effect mới.
- **Doc target**: [Module agentsync](../modules/agentsync.md), [Module graph query](../modules/graphquery.md).
- **Priority**: P1.

### Security Và Compliance Constraints

- **Lý do**: Tool này ghi vào home directory, tạo symlink/copy, append managed block, install registry skills và có thể tải LSP vào cache user. Các side effect phải rõ, có dry-run hoặc fail-open phù hợp.
- **Source paths**: `internal/agentsync/agentsync.go`, `internal/graphquery/`, `presets/mcp/servers.json`, `presets/registry/skills.json`, `COPYRIGHT.md`.
- **Docs hiện có**: [Tài liệu dự án](../README.md), [Module agentsync](../modules/agentsync.md), [Module graph query](../modules/graphquery.md).
- **Khoảng trống**: Không có compliance policy riêng ngoài current-state/safety docs.
- **Doc target**: [Module agentsync](../modules/agentsync.md), [Module graph query](../modules/graphquery.md).
- **Priority**: P1.

### Dev, Test Và Build Workflow

- **Lý do**: Repo kết hợp Go CLI, Vue/Vite preview, generated static assets, markdown/html lint và docs sync; validation phải chọn đúng phạm vi.
- **Source paths**: `DEVELOPER.md`, `package.json`, `go.mod`, `vite.config.ts`, `internal/preview/preview_ui_src/`, `internal/preview/preview_ui/`.
- **Docs hiện có**: `DEVELOPER.md`, [Quy ước frontend preview](../development/conventions/preview-frontend.md), [Module preview](../modules/preview.md).
- **Khoảng trống**: Không cần doc mới; developer guide là source vận hành chính.
- **Doc target**: [Quy ước frontend preview](../development/conventions/preview-frontend.md), `DEVELOPER.md`.
- **Priority**: P1.

### Generated Artifacts

- **Lý do**: `internal/preview/preview_ui/` là static build output được Go embed; graph/search phải ưu tiên source thật trong `preview_ui_src` và bỏ artifact generated khi index LSP Code Graph.
- **Source paths**: `internal/preview/preview_ui/`, `internal/preview/preview_ui_src/`, `internal/preview/preview_lsp.go`, `internal/preview/preview_search.go`, `vite.config.ts`.
- **Docs hiện có**: [Module preview](../modules/preview.md), [Quy ước frontend preview](../development/conventions/preview-frontend.md), [Preview web](../features/preview-web.md).
- **Khoảng trống**: Search Semantic generated filtering còn là planning scope trong [Tối ưu và rút gọn Preview Web](../specs/planning/optimize-preview-web-surface.md); LSP Code Graph filtering đã shipped.
- **Doc target**: [Module preview](../modules/preview.md), [Quy ước frontend preview](../development/conventions/preview-frontend.md).
- **Priority**: P1.

## Docs Gaps

- [Module agentsync](../modules/agentsync.md) xử lý gap P0 về adapter sync, native targets, operations và safety rules.
- [Chỉ mục](../_index.md) cần liệt kê inventory, module agentsync và toàn bộ planning docs đang tồn tại để preview graph không bỏ rơi tài liệu.
- Nếu preset/config surface tăng thêm nhiều rule, tạo doc shared riêng cho preset model thay vì nhồi tiếp vào module agentsync.
- Nếu module docs được migrate từ flat file sang folder, thêm `requirements.md` tương ứng cho invariant critical của `agentsync`, `preview` và `graphquery`.
