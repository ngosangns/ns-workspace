# Kiến Trúc Tổng Quan

## Meta

- **Status**: active
- **Description**: Tổng quan kiến trúc hiện tại của `ns-workspace`, bao gồm CLI, adapter sync, preview web và các quan hệ tài liệu cốt lõi.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Module graph query](../modules/graphquery.md), [Preview web](../features/preview-web.md), [Aspect inventory](../research/aspect-inventory.md), [Thuật ngữ](../shared/glossary.md)

## Tổng Quan

`ns-workspace` gồm hai bề mặt chính: CLI đồng bộ cấu hình agent và preview/search web để đọc knowledge base của một project. CLI giữ preset trong repo, ghi hoặc link sang các vị trí user-level của agent, còn preview web phục vụ tài liệu trong `docs/` qua local HTTP server. Lệnh `search` dùng cùng backend preview/search nhưng mở entry Search standalone qua HTML launcher sinh tại thư mục đang chạy. Lệnh `graph` chỉ chạy query terminal bằng Search/LSP Code Graph, còn nhóm `lsp` quản lý language server dùng cho Code Graph.

## Thành Phần

- CLI Go trong `main.go` route các lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents`, `preview`, `search`, `graph` và `lsp`.
- Package `internal/cli` parse flags và dispatch nhóm command agentsync.
- Package `internal/agentsync` gom logic adapter, `SyncPlan` và operation sync cho các agent. Stable adapters hiện gồm Claude Code, OpenCode, Grok Build, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf và Aider; module này được mô tả trong [Module agentsync](../modules/agentsync.md).
- Lệnh `update` rewrite artifact do `internal/agentsync` quản lý từ preset hiện tại. File, tree và JSON key managed được backup trước khi ghi; entry hoặc key đã bị xóa khỏi preset sẽ không được giữ lại trong output hiện tại.
- Package `internal/preview` scan docs, parse metadata, dựng graph, phục vụ API, chạy preview server, sinh launcher cho Search standalone và query LSP Code Graph. Package `internal/graphquery` sở hữu registry/setup/cache LSP cho Code Graph, chạy CLI `lsp`, và được `graph --query`/preview adapter dùng lại khi cần chuẩn bị language server; `graph --no-ensure-lsp` bỏ qua bước cài tự động.
- Frontend preview dùng TypeScript source trong `internal/preview/preview_ui_src/`, build ra static assets trong `internal/preview/preview_ui/`, gồm SPA preview chính và entry `search.html` cho lệnh `search`.
- Preset agent instruction trong `presets/agents/AGENTS.md` nhận trigger skill dạng `//<tag>` cho pipeline research, search docs, init knowledge base, plan, execution, fix, update-docs và commit. Trigger riêng `/s` gọi skill `spawn-opencode` để spawn OpenCode process như sub-agent.

## Quan Hệ

Preview đọc `docs/_index.md` và `docs/_sync.md` khi có, đồng thời scan toàn bộ file text dưới `docs/`. Bảng `## Modules` trong `_index.md`, field `**Links**` trong `## Meta`, relationship map và dependency diagram tạo thành graph điều hướng. Search page và Search standalone đọc docs corpus, source code corpus ngoài `docs/`, và Code Graph dựng symbol graph từ LSP trên source code tracked bởi Git khi language server tương ứng có sẵn. Resolver LSP ưu tiên môi trường user/project trước cache `ns-workspace`, còn web request chỉ fail-open với warning; side effect cài package chỉ xảy ra khi CLI setup được gọi rõ ràng.

## Quyết Định Liên Quan

Thiết kế adapter hiện tại nằm trong [Module agentsync](../modules/agentsync.md). Thiết kế preview hiện tại được mô tả ở [Module preview](../modules/preview.md). Thiết kế setup/cache LSP cho graph query nằm trong [Module graph query](../modules/graphquery.md). Inventory aspect để onboarding repo nằm trong [Aspect inventory](../research/aspect-inventory.md).
