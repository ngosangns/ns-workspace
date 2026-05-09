---
name: plan
description: Tạo file kế hoạch/task cho công việc lớn hoặc phức tạp và chờ user phê duyệt trước khi sửa source code. Dùng sau bước nghiên cứu khi task lớn, liên quan kiến trúc, rủi ro hoặc nhiều bước.
---

# Lập Kế Hoạch Và Xin Phép

Dùng skill này sau `research` khi yêu cầu lớn, phức tạp, liên quan kiến trúc, rủi ro hoặc cần phối hợp thay đổi trên nhiều module. Giọng làm việc của skill này là điềm tĩnh và có cấu trúc: biến mơ hồ thành đường đi rõ, rồi dừng đúng lúc để user duyệt.

Các task nhỏ và rõ ràng có thể bỏ qua skill này.

## Vị Trí Lưu Kế Hoạch

Tạo file planning trong:

```text
docs/specs/planning/
```

Dùng tên task kebab-case rõ nghĩa, ví dụ:

```text
docs/specs/planning/add-account-notifications.md
```

## Quy Trình

1. Đảm bảo bước research đã xác định docs/specs liên quan, code paths, constraints và các giả định chưa được giải quyết.
2. Tạo hoặc cập nhật một file planning tập trung trong `docs/specs/planning/`.
3. Bao gồm cấu trúc logic của giải pháp, khu vực bị ảnh hưởng, task triển khai, rủi ro, acceptance criteria và cách validation.
4. Giữ plan theo trạng thái hiện tại. Không viết changelog, bảng lịch sử commit, migration history hoặc incremental sync log.
5. Trình bày tóm tắt plan cho user một cách cô đọng.
6. Dừng lại và chờ user phê duyệt rõ ràng trước khi sửa source code cho task lớn.

## Mẫu Kế Hoạch Gợi Ý

```markdown
# [Tên Task]

## Bối Cảnh

## Mục Tiêu

## Ngoài Phạm Vi

## Hướng Tiếp Cận Đề Xuất

## Công Việc Cần Làm

## Rủi Ro Và Ràng Buộc

## Kiểm Chứng
```

## Ràng Buộc

- Không bắt đầu triển khai source code cho task lớn trước khi user duyệt plan.
- Không tạo placeholder docs không có nội dung hữu ích.
- Không dùng browser tools.
- Không chạy build chỉ để hoàn tất bước planning.
