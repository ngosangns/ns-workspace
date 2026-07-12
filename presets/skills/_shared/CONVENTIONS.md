# Quy Tắc Chung

File này chứa quy tắc dùng chung cho nhiều skill. Mỗi skill reference file này thay vì lặp lại nội dung.

## Hỏi Lại Khi Vướng Mắc

Khi gặp tình huống sau, **dừng lại và hỏi user** kèm bằng chứng đã thu thập + các lựa chọn khả dĩ:

- Scope chưa rõ hoặc có nhiều cách diễn giải
- Requirements/docs/code mâu thuẫn nhau mà không rõ phía nào đúng
- Nhiều hướng kiến trúc/thiết kế hợp lý, không biết user prefer hướng nào
- Plan thiếu sót thật sự (thiếu file, dependency, design assumption sai)
- Cần expand scope hoặc đổi hướng so với plan đã duyệt
- Rủi ro cao (public contract, config user-level, generated artifact, data migration)
- Bằng chứng yếu, chỉ là "không thấy reference trong grep"

Không tự đoán hoặc tự chọn hướng có rủi ro sai ý user.

## Không Build Chỉ Để Kết Thúc

Không chạy full build rộng nếu repo guidance không yêu cầu. Chọn validation mục tiêu theo phạm vi thay đổi.

## Đọc `requirements.md`

Trước khi sửa code trong phạm vi feature/module:

1. Tìm requirements thuộc phạm vi ảnh hưởng trong cả hai cây audience:
   - `docs/business/features/**/requirements.md` — acceptance criteria, business rules, user impact.
   - `docs/developer/features/**/requirements.md` — technical constraints, implementation requirements.
   - `docs/business/modules/**/requirements.md` — business view của module (contract, rules).
   - `docs/developer/modules/**/requirements.md` — technical view của module (API, boundary, invariants).
2. Đọc toàn bộ, coi chúng là acceptance constraints bắt buộc.
3. Nếu requirements mâu thuẫn với plan/prompt/code hiện tại → **dừng lại** và báo rõ mâu thuẫn.

## Diff Review Loop

Sau mỗi lượt sửa file:

1. Đọc lại toàn bộ diff vừa tạo.
2. Xóa phần thừa: dead code, duplication, debug output, TODO không chủ đích, import không dùng, whitespace churn.
3. Gom/đơn giản hóa logic nếu có thể.
4. Rà comment trong vùng đã chạm; bổ sung tiếng Anh cho edge case, invariant, contract.
5. Lặp lại đến khi diff không còn vấn đề rõ ràng.

## Comment Trong Code

- Comment bằng tiếng Anh ở vùng đã sửa.
- Chỉ comment cho logic không tự giải thích, edge case, contract, quyết định kỹ thuật đáng lưu.
- Không thêm comment hiển nhiên hoặc mô tả lại code.

## Không Dùng Browser Tools

Trừ khi task chỉ xác minh được qua UI hoặc user yêu cầu rõ.

## Tôn Trọng Worktree

Không revert, discard hoặc chạm vào thay đổi không liên quan của user.
