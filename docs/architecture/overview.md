# Kiến Trúc Tổng Quan

## Meta

- **Status**: active
- **Description**: Tổng quan kiến trúc hiện tại của `ns-workspace`, bao gồm CLI, adapter sync, preview web và các quan hệ tài liệu cốt lõi.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](../modules/preview.md), [Preview web](../features/preview-web.md), [Adapter agent user-level](../specs/planning/user-level-agent-adapter-framework.md), [Thuật ngữ](../shared/glossary.md)

## Tổng Quan

`ns-workspace` gồm hai bề mặt chính: CLI đồng bộ cấu hình agent và preview web để đọc knowledge base của một project. CLI giữ preset trong repo, ghi hoặc link sang các vị trí user-level của agent, còn preview web phục vụ tài liệu trong `docs/` qua local HTTP server.

## Thành Phần

- CLI Go trong `main.go` xử lý các lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents` và `preview`.
- Package `internal/agentsync` gom logic adapter và operation sync cho các agent. Stable adapters hiện gồm Claude Code, OpenCode, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf và Aider.
- Package `internal/preview` scan docs, parse metadata, dựng graph, phục vụ API và embed frontend.
- Frontend preview dùng TypeScript source trong `internal/preview/preview_ui_src/`, build ra static assets trong `internal/preview/preview_ui/`.
- Preset agent instruction trong `presets/agents/AGENTS.md` nhận trigger skill dạng `//<tag>` cho pipeline research, search docs, plan, execution, fix và update-docs. Trigger riêng `/s` gọi skill `spawn-claude-code` để spawn Claude Code process như sub-agent.

## Quan Hệ

Preview đọc `docs/_index.md` và `docs/_sync.md` khi có, đồng thời scan toàn bộ file text dưới `docs/`. Bảng `## Modules` trong `_index.md`, field `**Links**` trong `## Meta`, relationship map và dependency diagram tạo thành graph điều hướng. Search page đọc docs corpus, source code corpus ngoài `docs/`, và `graphify-out/graph.json` nếu tồn tại.

## Quyết Định Liên Quan

Thiết kế preview hiện tại được mô tả ở [Module preview](../modules/preview.md). Kế hoạch mở rộng adapter nằm trong [Adapter agent user-level](../specs/planning/user-level-agent-adapter-framework.md).
