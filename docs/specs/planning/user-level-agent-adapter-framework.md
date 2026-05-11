# Framework Adapter Agent Cấp Người Dùng

## Meta

- **Status**: draft
- **Description**: Plan thiết kế framework adapter agent cấp người dùng, mô tả registry, operation engine, tier an toàn và chiến lược sync.
- **Compliance**: current-state
- **Links**: [Kiến trúc tổng quan](../../architecture/overview.md), [Thuật ngữ](../../shared/glossary.md), [Chỉ mục](../../_index.md)

## Tổng Quan

CLI hiện dùng `~/.agents` làm source of truth rồi đồng bộ sang native user-level folder của nhiều coding agent. Logic adapter cần được tách khỏi `main.go` để mỗi agent tự mô tả path, capability, format config, merge strategy và bước manual khi không có surface ổn định.

## Mục Tiêu

- Tạo interface adapter chung cho install, update, status và doctor.
- Chuẩn hóa operation engine để write file, link/copy tree, merge JSON/TOML, quản lý backup và dry-run.
- Phân tier agent theo mức an toàn: stable, guarded/manual và catalog-only.
- Giữ `~/.agents` là nguồn chính, không ghi vào path suy đoán khi docs chính thức chưa đủ chắc.

## Yêu Cầu

### REQ-1: Adapter registry

**Tiêu Chí Chấp Nhận:**

- [ ] Có registry liệt kê agent name, aliases, tier, capability và docs URL.
- [ ] `--tools stable`, `--tools all` và danh sách tên cụ thể chọn đúng adapter.
- [ ] Manual adapter chỉ tạo helper/readme, không ghi native path rủi ro.

### REQ-2: Operation engine

**Tiêu Chí Chấp Nhận:**

- [ ] Operations dùng chung cho write, link/copy, merge JSON, merge TOML và managed block.
- [ ] Dry-run mô tả plan mà không ghi filesystem.
- [ ] Update tạo backup cho artifact được quản lý trước khi thay thế.

### REQ-3: Adapter stable

**Tiêu Chí Chấp Nhận:**

- [ ] Claude, OpenCode, Kimi, Kiro/Kiro CLI, Qwen, Gemini, Codex, Cline, Windsurf và Aider có adapter stable hoặc guarded theo capability đã xác minh.
- [ ] Cursor, GitHub Copilot, JetBrains, Antigravity, Trae và Roo không ghi tự động nếu chưa có path user-level ổn định.

## Ghi Chú Thiết Kế

`internal/agentsync` nên chứa domain model, operation plan và adapter implementations. `main.go` chỉ nên parse CLI flags, chọn adapter registry và gọi plan/apply. MCP config có thể chứa secret reference nên merge phải preserve placeholder/env reference thay vì inline secret.

## Quan Hệ

Plan này mở rộng phần CLI được mô tả trong [Kiến trúc tổng quan](../../architecture/overview.md). Các thuật ngữ như source of truth và adapter được định nghĩa trong [Thuật ngữ](../../shared/glossary.md).
