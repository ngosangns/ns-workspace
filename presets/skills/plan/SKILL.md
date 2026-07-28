---
name: plan
description: "Tạo kế hoạch cho công việc lớn, rồi chờ user phê duyệt trước khi sửa code. Trigger: lập plan, viết spec, đề xuất thiết kế, refactor lớn."
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
- **Trình bày plan trong hội thoại** (hoặc file path user chỉ định ngoài `docs/`). Không ghi vào thư mục `docs/`.

## Từ Branch Hoặc Commit

Khi user yêu cầu tạo plan từ branch/commit:

- Dùng lệnh chỉ đọc: `git merge-base`, `git log`, `git diff --stat`, `git diff`, `git show`. Không switch branch.
- Không đưa vào plan: tên branch, hash commit, danh sách commit, tác giả, "files changed" table.
- Chuyển hóa thành: mục tiêu thiết kế, cấu trúc giải pháp, module boundary, logic nghiệp vụ, rủi ro, kiểm chứng.

## Quy Trình

1. Đảm bảo research đã xác định code path, ràng buộc, boundary. Dùng `lsp-code-graph` khi cần symbol/caller/callee context.
2. Làm rõ nguyên nhân gốc rễ và động lực thiết kế.
3. Xác định bức tranh tổng quan rồi thu hẹp: module boundary, data flow, API/contract, vùng ảnh hưởng, ngoài phạm vi.
4. Nếu từ branch/commit, đọc thay đổi bằng Git chỉ đọc.
5. Soạn kế hoạch theo mẫu `_shared/templates/plan-template.md`.
6. Nếu có impact nghiệp vụ rõ, ghi acceptance criteria / user impact trong plan (section riêng).
7. Trình bày tóm tắt kế hoạch cô đọng bằng tiếng Việt.
8. **Dừng lại và chờ user phê duyệt** trước khi sửa code.

## Ràng Buộc

- Không triển khai code cho công việc lớn trước khi user duyệt.
- Không switch branch hoặc checkout commit chỉ để đọc.
- Không đưa siêu dữ liệu Git vào kế hoạch.
- Không đọc/ghi thư mục `docs/`.
