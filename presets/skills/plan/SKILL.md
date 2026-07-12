---
name: plan
description: Tạo file kế hoạch trong `docs/developer/specs/planning/` và business spec tương ứng trong `docs/business/specs/planning/` cho công việc lớn, rồi chờ user phê duyệt trước khi sửa code. Trigger: lập plan, viết spec, đề xuất thiết kế, refactor lớn.
---

# Lập Kế Hoạch Và Xin Phép

Dùng sau `research` khi yêu cầu lớn, phức tạp, liên quan kiến trúc hoặc rủi ro. Công việc nhỏ và rõ ràng có thể bỏ qua.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Ngôn Ngữ

- Viết kế hoạch bằng tiếng Việt có dấu. Pha tiếng Anh chỉ cho tên riêng, thuật ngữ kỹ thuật, tên API/module/field.
- Viết như tài liệu thiết kế, không như changelog hay nhật ký Git.
- Thêm Mermaid khi cấu trúc, luồng dữ liệu, quan hệ module khó hiểu bằng chữ.

## Nguyên Tắc

- **Tìm nguyên nhân gốc rễ trước:** Kế hoạch phải thể hiện vì sao vấn đề tồn tại, phân biệt triệu chứng vs nguyên nhân gốc rễ.
- **Nhìn tổng quát, giữ trọng tâm:** Bao quát context, module boundary, contract, rủi ro; chỉ đề xuất công việc trong phạm vi mục tiêu.
- **Hai audience:** Plan kỹ thuật đặt trong `docs/developer/specs/planning/`. Nếu plan có tác động nghiệp vụ (user workflow, acceptance criteria, business rule), tạo thêm business spec ở `docs/business/specs/planning/`.

## Vị Trí

```text
docs/developer/specs/planning/<kebab-case-name>.md
docs/business/specs/planning/<kebab-case-name>.md     # khi có business impact
```

## Từ Branch Hoặc Commit

Khi user yêu cầu tạo plan từ branch/commit:

- Dùng lệnh chỉ đọc: `git merge-base`, `git log`, `git diff --stat`, `git diff`, `git show`. Không switch branch.
- Không đưa vào plan: tên branch, hash commit, danh sách commit, tác giả, "files changed" table.
- Chuyển hóa thành: mục tiêu thiết kế, cấu trúc giải pháp, module boundary, logic nghiệp vụ, rủi ro, kiểm chứng.

## Quy Trình

1. Đảm bảo research đã xác định docs/specs, code path, ràng buộc. Dùng `lsp-code-graph` khi cần symbol/caller/callee context.
2. Làm rõ nguyên nhân gốc rễ và động lực thiết kế.
3. Xác định bức tranh tổng quan rồi thu hẹp: module boundary, data flow, API/contract, vùng ảnh hưởng, ngoài phạm vi.
4. Nếu từ branch/commit, đọc thay đổi bằng Git chỉ đọc.
5. Tạo file kế hoạch kỹ thuật trong `docs/developer/specs/planning/` theo mẫu `_shared/templates/plan-template.md`.
6. Nếu plan ảnh hưởng đến business (user-facing behavior, acceptance criteria, business rules), tạo business spec tương ứng trong `docs/business/specs/planning/` theo mẫu `_shared/templates/spec-template.md`. Liên kết hai file qua lại.
7. Trình bày tóm tắt kế hoạch cô đọng bằng tiếng Việt.
8. **Dừng lại và chờ user phê duyệt** trước khi sửa code.

## Ràng Buộc

- Không triển khai code cho công việc lớn trước khi user duyệt.
- Không tạo placeholder docs không có nội dung hữu ích.
- Không switch branch hoặc checkout commit chỉ để đọc.
- Không đưa siêu dữ liệu Git vào kế hoạch.
