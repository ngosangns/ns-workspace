# Kiến Trúc Tổng Quan

## Meta

- **Status**: active
- **Description**: Tổng quan kiến trúc hiện tại của `ns-workspace`, bao gồm CLI, adapter sync, preview web và các quan hệ tài liệu cốt lõi.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](../modules/preview.md), [Preview web](../features/preview-web.md), [Thuật ngữ](../shared/glossary.md)

## Tổng Quan

`ns-workspace` gồm hai bề mặt chính: CLI đồng bộ cấu hình agent và preview/search web để đọc knowledge base của một project. CLI giữ preset trong repo, ghi hoặc link sang các vị trí user-level của agent, còn preview web phục vụ tài liệu trong `docs/` qua local HTTP server. Lệnh `graph` dùng cùng backend preview/search nhưng mở entry Search standalone qua HTML launcher sinh tại thư mục đang chạy.

## Thành Phần

- CLI Go trong `main.go` xử lý các lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents`, `preview` và `graph`.
- Package `internal/agentsync` gom logic adapter và operation sync cho các agent. Stable adapters hiện gồm Claude Code, OpenCode, Grok Build, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf và Aider.
- Package `internal/preview` scan docs, parse metadata, dựng graph, phục vụ API, chạy preview server và sinh launcher cho Search standalone.
- Frontend preview dùng TypeScript source trong `internal/preview/preview_ui_src/`, build ra static assets trong `internal/preview/preview_ui/`, gồm SPA preview chính và entry `search.html` cho lệnh `graph`.
- Preset agent instruction trong `presets/agents/AGENTS.md` nhận trigger skill dạng `//<tag>` cho pipeline research, search docs, plan, execution, fix, update-docs và commit. Trigger riêng `/s` gọi skill `spawn-opencode` để spawn OpenCode process như sub-agent.

## Quan Hệ

Preview đọc `docs/_index.md` và `docs/_sync.md` khi có, đồng thời scan toàn bộ file text dưới `docs/`. Bảng `## Modules` trong `_index.md`, field `**Links**` trong `## Meta`, relationship map và dependency diagram tạo thành graph điều hướng. Search page và Search standalone đọc docs corpus, source code corpus ngoài `docs/`, và Code Graph dựng symbol graph từ LSP trên source code tracked bởi Git khi language server tương ứng có sẵn.

## Quyết Định Liên Quan

Thiết kế preview hiện tại được mô tả ở [Module preview](../modules/preview.md). Thiết kế adapter hiện tại nằm trong package `internal/agentsync` và các preset trong `presets/`.
