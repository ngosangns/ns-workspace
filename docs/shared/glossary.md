---
type: shared
title: "Thuật Ngữ"
description: "Thuật ngữ chung cho `ns-workspace`, bao gồm adapter sync, presets, managed artifacts, preview web, spec docs, feature docs, module docs và metadata docs."
tags: ["shared", "glossary"]
timestamp: 2026-07-17T00:00:00Z
status: active
compliance: current-state
---

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

Managed artifact là file, directory, JSON key hoặc managed text block do `ns-workspace` tạo và có thể rewrite khi chạy `update` (replace-in-place, không backup-before-write).

Managed block là đoạn text có label trong file native, ví dụ block MCP trong `~/.codex/config.toml`.

Support tier là mức ổn định của adapter: `stable` ghi native path thật, `manual` tạo helper guidance, còn `experimental` được guard vì path hoặc contract chưa đủ chắc.

Portal là web UI local (`go run . portal`) để quản lý skills (installed + discover/catalog), MCP servers, registry entries, adapters và chạy sync.

Preview web là dashboard local được mở bằng lệnh `preview` (SolidJS SPA) để đọc, search và điều hướng tài liệu trong `docs/`.

Spec là tài liệu yêu cầu hoặc plan nằm dưới `docs/specs/`.

Feature doc là tài liệu mô tả hành vi đã implement hoặc shipped dưới `docs/features/`.

Module doc là tài liệu mô tả API, data model, quan hệ và ràng buộc của một module code dưới `docs/modules/`.

Metadata doc theo Open Knowledge Format (OKF): YAML frontmatter đầu file với `type` (bắt buộc) cùng `title`/`description`/`tags`/`timestamp`/`status`/`compliance`. Block `## Meta` cũ (các dòng `- **Status**:`, `- **Description**:`, `- **Compliance**:`, `- **Links**:`) vẫn được giữ để tương thích ngược và cung cấp `**Links**` cho preview graph; khi có cả hai, frontmatter thắng ở key trùng.

Aspect inventory là bản đồ onboarding trong `docs/research/` liệt kê các aspect chính, source paths, docs hiện có, docs gaps và target cập nhật.

Harness là bộ kiểm chứng gồm task file, evaluator, loop controller và memory store để tự động hóa workflow dev.

Task file là file YAML/JSON trong `.harness/tasks/` định nghĩa requirements, scope, acceptance criteria, routing và stopping rules cho một harness task.

Looping agentic là vòng lặp tự động `plan → execute → verify → diagnose` cho đến khi đạt điều kiện dừng dựa trên tiến triển thay vì timeout hay max iterations.

Subagent dispatcher là abstraction gọi các coding agent backend (OpenCode, Claude Code, Codex, ...) để thực hiện phase của loop.

Dual memory store là cơ chế lưu harness state ở cả project path và shared path để resume và share giữa các môi trường.
