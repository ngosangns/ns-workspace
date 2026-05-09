# Thuật Ngữ

## Meta

- **Status**: active
- **Description**: Thuật ngữ chung cho `ns-workspace`, bao gồm adapter sync, preview web, spec docs, feature docs, module docs và metadata docs.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Kiến trúc tổng quan](../architecture/overview.md), [Module preview](../modules/preview.md)

## Thuật Ngữ

`~/.agents` là thư mục nguồn cấu hình cá nhân, chứa instructions, skills, subagents, settings và MCP presets.

Adapter là lớp đồng bộ cấu hình từ `~/.agents` sang native user-level location của từng coding agent.

Preview web là dashboard local được mở bằng lệnh `preview` để đọc, search và điều hướng tài liệu trong `docs/`.

Spec là tài liệu yêu cầu hoặc plan nằm dưới `docs/specs/`.

Feature doc là tài liệu mô tả hành vi đã implement hoặc shipped dưới `docs/features/`.

Module doc là tài liệu mô tả API, data model, quan hệ và ràng buộc của một module code dưới `docs/modules/`.

Metadata doc là section `## Meta` với các dòng `- **Status**:`, `- **Description**:`, `- **Compliance**:` và `- **Links**:` để preview scan được tóm tắt, trạng thái và quan hệ graph.
