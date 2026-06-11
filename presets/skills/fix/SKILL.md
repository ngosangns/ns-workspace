---
name: fix
description: Chẩn đoán và sửa bug, test fail, regression đã có triệu chứng rõ. Trigger: sửa bug, fix lỗi, điều tra regression.
---

# Fix Bug Có Bằng Chứng

Dùng khi nhiệm vụ là sửa lỗi đã có triệu chứng rõ: test fail, lỗi runtime, regression, bug report.

**Khác `execution`:** `fix` bắt đầu từ triệu chứng, dùng tái hiện làm bằng chứng, ưu tiên diff nhỏ nhất đúng nguyên nhân. `execution` triển khai từ thiết kế đã duyệt.

Nếu lỗi mơ hồ, dùng `research` trước. Nếu fix đổi business rule/architecture → dùng `update-docs` sau.

Quy tắc chung: đọc `_shared/CONVENTIONS.md` (hỏi khi vướng, requirements.md, diff review loop, comment, worktree).

## Nguyên Tắc Riêng

- **Tái hiện trước:** Cố tái hiện lỗi bằng test/command/log trước khi sửa. Nếu không, ghi rõ lý do và bằng chứng thay thế.
- **Sửa nguyên nhân gốc:** Phân biệt triệu chứng → nguyên nhân trực tiếp → nguyên nhân gốc rễ. Fix nhỏ nhất đúng nguyên nhân, không vá triệu chứng.
- **Chặn regression:** Thêm/cập nhật test khi bug có bề mặt test hợp lý.
- **Giải thích sau fix:** Lỗi gốc bị loại bỏ ra sao, regression nào chặn, behavior đổi/giữ, rủi ro còn.

## Quy Trình

1. Đọc bug report/triệu chứng. Kiểm tra `git status` tránh đè việc user.
2. Xác định code path bằng `rg`, test, docs. Dùng `lsp-code-graph` khi cần caller/callee context.
3. Đọc `requirements.md` của feature/module liên quan (xem CONVENTIONS.md).
4. Dựng giả thuyết nguyên nhân gốc rễ.
5. Tái hiện lỗi bằng command nhỏ nhất.
6. Sửa nguyên nhân gốc theo pattern hiện có.
7. Thêm regression guard nếu phù hợp.
8. Diff review loop (xem CONVENTIONS.md).
9. Chạy lại command tái hiện + validation mục tiêu.
10. Báo nguyên nhân, cách sửa, requirements tuân thủ, xác minh, việc còn lại.

## Ràng Buộc

- Không mở rộng scope sang refactor lớn nếu không cần để fix.
- Không đổi behavior public ngoài phần cần sửa.
