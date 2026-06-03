# Thuật Ngữ

## Meta

- **Status**: active
- **Description**: Thuật ngữ chung cho `ns-workspace`, bao gồm adapter sync, presets, managed artifacts, preview web, spec docs, feature docs, module docs và metadata docs.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Kiến trúc tổng quan](../architecture/overview.md), [Module agentsync](../modules/agentsync.md), [Module preview](../modules/preview.md), [Aspect inventory](../research/aspect-inventory.md)

## Thuật Ngữ

`~/.agents` là thư mục nguồn cấu hình cá nhân, chứa instructions, skills, subagents, settings và MCP presets.

Adapter là lớp đồng bộ cấu hình từ `~/.agents` sang native user-level location của từng coding agent.

Preset là source config được lưu trong `presets/` và Go embed vào binary để `init/update` có thể ghi shared home hoặc native adapter output.

Managed artifact là file, directory, JSON key hoặc managed text block do `ns-workspace` tạo và có thể rewrite khi chạy `update`.

Managed block là đoạn text có label trong file native, ví dụ block MCP trong `~/.codex/config.toml` hoặc conventions trong `~/.aider.conf.yml`.

Support tier là mức ổn định của adapter: `stable` ghi native path thật, `manual` tạo helper guidance, còn `experimental` được guard vì path hoặc contract chưa đủ chắc.

Preview web là dashboard local được mở bằng lệnh `preview` để đọc, search và điều hướng tài liệu trong `docs/`.

Spec là tài liệu yêu cầu hoặc plan nằm dưới `docs/specs/`.

Feature doc là tài liệu mô tả hành vi đã implement hoặc shipped dưới `docs/features/`.

Module doc là tài liệu mô tả API, data model, quan hệ và ràng buộc của một module code dưới `docs/modules/`.

Metadata doc là section `## Meta` với các dòng `- **Status**:`, `- **Description**:`, `- **Compliance**:` và `- **Links**:` để preview scan được tóm tắt, trạng thái và quan hệ graph.

Aspect inventory là bản đồ onboarding trong `docs/research/` liệt kê các aspect chính, source paths, docs hiện có, docs gaps và target cập nhật.
