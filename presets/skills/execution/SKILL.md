---
name: execution
description: Triển khai thay đổi code đã được duyệt (sau `plan`) hoặc task nhỏ rõ ràng (sau `research`) theo hướng tiến thẳng, scope chặt, ưu tiên kiến trúc sạch hơn tương thích ngược vô điều kiện. Trigger: viết code, triển khai tính năng, refactor đã duyệt, code change theo plan.
---

# Thực Thi Code Rõ Ràng

Dùng skill này khi đã đến bước sửa code. Với task lớn, chỉ dùng sau khi user đã duyệt plan được tạo bởi `plan`. Với task nhỏ đã rõ, có thể dùng ngay sau `research`. Giọng làm việc là chắc tay, thẳng thắn và gọn: làm đến nơi, giữ scope sạch, không lẩn vào refactor thừa.

**Phân biệt với `fix`:** `execution` triển khai thay đổi đã có thiết kế/hướng đi rõ (qua `plan` hoặc `research`), có thể chạm nhiều file, và được phép phá tương thích ngược nội bộ để giữ kiến trúc sạch. `fix` bắt đầu từ triệu chứng/bug report, ưu tiên diff nhỏ nhất đúng nguyên nhân và validation sát lỗi.

## Nguyên Tắc Bắt Buộc

- **Nhìn tổng quát trước, tập trung khi sửa:** Xác định nguyên nhân gốc rễ, bối cảnh hệ thống, module boundary và đường thay đổi nhỏ nhất. Không triển khai chỉ từ triệu chứng hoặc yêu cầu bề mặt nếu chưa hiểu vì sao cần sửa.
- **Tuân thủ requirements của feature/module liên quan:** Trước khi triển khai, xác định docs liên quan trong `docs/features/` và `docs/modules/`. Nếu có `requirements.md` trong folder thuộc phạm vi ảnh hưởng, đọc toàn bộ và coi chúng là acceptance constraints bắt buộc.
- **Không giữ tương thích ngược vô điều kiện:** Khi code mới cần đổi contract nội bộ để đúng kiến trúc hơn, được quyền thay đổi. Chỉ giữ tương thích khi user yêu cầu rõ hoặc public contract thật sự bắt buộc.
- **Giữ scope chặt:** Theo sát yêu cầu, Git diff và module boundary. Không kéo thêm việc phụ.
- **Review toàn bộ changes sau khi edit:** Sau mỗi lượt sửa file, đọc lại toàn bộ diff vừa tạo, xóa phần thừa, gom/đơn giản hóa logic, đảm bảo không còn thay đổi cơ học không cần thiết.
- **Tối ưu diff trước khi kết thúc:** Code cuối cùng phải rõ, ít nhánh thừa, ít duplication, đúng helper/pattern hiện có, không chứa dead code, log/debug tạm, TODO vô căn cứ, import không dùng.
- **Comment bằng tiếng Anh ở vùng đã sửa:** Thêm/cập nhật comment cho logic không tự giải thích, edge case quan trọng, contract nội bộ hoặc quyết định kỹ thuật đáng lưu lại. Không thêm comment hiển nhiên chỉ để lấp chỗ.
- **Báo cáo có diễn giải:** Nói thẳng việc đã làm, vì sao làm, validation nào đã chạy, và giải thích chi tiết ý nghĩa của từng nhóm thay đổi để user hiểu tác động lên behavior, architecture, docs, workflow.
- **Liệt kê việc còn lại:** Nếu task chưa xử lý trọn vẹn, nêu rõ phần còn lại, lý do dang dở, bước tiếp theo ngắn gọn.
- **Không build chỉ để kết thúc:** Không chạy build rộng nếu repo guidance không yêu cầu.
- **Tự review liên tục:** Lặp lại review và cleanup đến khi diff không còn vấn đề rõ ràng về scope, chất lượng, comment, validation.
- **Hỏi lại khi vướng mắc:** Khi đang triển khai phát hiện plan thiếu sót thật sự (thiếu file, thiếu dependency, design assumption sai), khi cần đổi hướng/expand scope so với plan đã duyệt, khi requirements mâu thuẫn với plan/prompt, hoặc khi có quyết định kiến trúc phụ mà plan chưa nói tới → **dừng lại và hỏi user** trước khi sửa, kèm phần đã làm, phần phát hiện, các lựa chọn. Không tự ý đổi hướng plan đã duyệt vì "thấy hợp lý hơn".

## Nguyên Tắc Thực Thi

- Triển khai hành vi được yêu cầu theo plan đã duyệt hoặc theo bối cảnh đã research đủ rõ.
- Ưu tiên kiến trúc hiện tại sạch và đúng hơn các lớp vá để giữ tương thích ngược không cần thiết.
- Xóa hoặc thay thế hành vi lỗi thời khi thiết kế mới khiến nó không còn cần thiết.
- Giữ thay đổi đúng phạm vi yêu cầu và các module bị ảnh hưởng.
- Giữ nguyên các thay đổi không liên quan của user trong worktree.

## Quy Trình

1. Đọc lại plan đã duyệt hoặc ghi chú research liên quan trước khi sửa.
2. Xác định nguyên nhân gốc rễ và bức tranh tổng quan vừa đủ: hành vi hiện tại, module liên quan, contract, call site, dữ liệu vào/ra, lý do thay đổi cần thiết.
3. Tìm docs feature/module liên quan đến phạm vi sửa. Đọc toàn bộ `requirements.md` tương ứng nếu tồn tại trong `docs/features/**/requirements.md` hoặc `docs/modules/**/requirements.md`; coi chúng là acceptance constraints bắt buộc.
4. Nếu requirements mâu thuẫn với plan, prompt của user hoặc trạng thái code hiện tại, **dừng lại** và báo rõ mâu thuẫn thay vì triển khai âm thầm theo một phía.
5. Thu hẹp thành phạm vi sửa tập trung: file cần sửa, pattern code lân cận, test/validation phù hợp, phần rõ ràng ngoài scope.
6. Triển khai thay đổi theo style, helper, kiến trúc hiện có của repo và toàn bộ requirements liên quan.
7. Không giữ tương thích ngược trừ khi user yêu cầu rõ hoặc public contract hiện tại bắt buộc phải giữ.
8. Sau mỗi lượt sửa file, review toàn bộ diff: logic, imports, tests, docs, comment.
9. Cleanup ngay phần thừa: code chết, duplication, naming lệch pattern, debug output, TODO không chủ đích, whitespace churn, thay đổi ngoài scope.
10. Rà comment trong vùng code vừa chạm; bổ sung hoặc chuyển sang tiếng Anh ở điểm cần ngữ cảnh để người sau đọc nhanh hơn, đồng thời giữ comment ngắn, chính xác, không mô tả điều code đã nói rõ.
11. Lặp lại review và cleanup cho đến khi không còn vấn đề rõ ràng cần sửa.
12. Chạy validation có mục tiêu khi có sẵn và phù hợp, nhưng không chạy full build chỉ để kết thúc nếu guidance của repo nói không cần build.
13. Tổng kết bằng cách giải thích ý nghĩa từng nhóm thay đổi: vấn đề/nhu cầu nào được xử lý, requirements nào đã tuân thủ, hành vi/contract nào đổi/giữ, rủi ro nào giảm, user cần lưu ý gì khi tiếp tục.
14. Nếu thay đổi code ảnh hưởng flow, business rule, architecture, quan hệ module hoặc constraint, dùng `update-docs` để cập nhật docs/specs liên quan.
15. Trước khi kết thúc, nếu chưa hoàn thành toàn bộ yêu cầu, liệt kê rõ phần còn lại, trạng thái hiện tại, bước tiếp theo đề xuất.

## Ràng Buộc

- Không dùng browser tools.
- Không chạy build chỉ để kết thúc task.
- Không bỏ qua `requirements.md` của feature/module liên quan khi triển khai thay đổi code trong phạm vi đó.
- Không để nội dung docs/spec cũ nằm cạnh hành vi mới; thay thế mô tả đã lỗi thời.
- Không để lại comment tiếng Việt trong code mới hoặc code vừa chạm nếu thuộc phần mình sửa.
- Không thêm comment hiển nhiên, verbose hoặc sai lệch với implementation.
- Không revert các thay đổi worktree không liên quan.
