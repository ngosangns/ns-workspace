---
name: execution
description: Triển khai thay đổi code đã được duyệt hoặc các task nhỏ đã rõ theo hướng tiến thẳng. Dùng sau research, hoặc sau plan khi task lớn đã được user duyệt.
---

# Thực Thi Code Rõ Ràng

Dùng skill này khi đã đến bước sửa code.

Với task lớn, chỉ dùng skill này sau khi user đã duyệt plan được tạo bởi `plan`. Với task nhỏ đã rõ, có thể dùng ngay sau `research`. Giọng làm việc của skill này là chắc tay, thẳng thắn và gọn: làm đến nơi, giữ scope sạch, không lẩn vào refactor thừa.

## Nguyên Tắc Bắt Buộc

- **Nhìn tổng quát trước, tập trung khi sửa:** Trước khi edit, phải xác định nguyên nhân gốc rễ, bối cảnh hệ thống, module boundary và đường thay đổi nhỏ nhất. Không triển khai chỉ từ triệu chứng hoặc yêu cầu bề mặt nếu chưa hiểu vì sao cần sửa.
- **Không giữ tương thích ngược vô điều kiện:** Khi code mới cần thay đổi contract nội bộ để đúng kiến trúc hơn, được quyền thay đổi. Chỉ giữ tương thích khi user yêu cầu rõ hoặc public contract thật sự bắt buộc.
- **Giữ scope chặt:** Theo sát yêu cầu, Git diff và module boundary. Không kéo thêm việc phụ.
- **Review toàn bộ changes sau khi edit:** Sau mỗi lượt sửa file, đọc lại toàn bộ diff mình vừa tạo, xóa phần thừa, gom hoặc đơn giản hóa logic nếu có thể, và đảm bảo không còn thay đổi cơ học không cần thiết.
- **Tối ưu diff trước khi kết thúc:** Code cuối cùng phải rõ, ít nhánh thừa, ít duplication, đúng helper/pattern hiện có và không chứa dead code, log/debug tạm, TODO vô căn cứ hoặc import không dùng.
- **Comment bằng tiếng Anh đầy đủ ở vùng đã sửa:** Với code mới hoặc code vừa chạm, thêm/cập nhật comment tiếng Anh cho logic không tự giải thích, edge case quan trọng, contract nội bộ hoặc quyết định kỹ thuật đáng lưu lại. Không thêm comment hiển nhiên chỉ để lấp chỗ.
- **Báo cáo có diễn giải:** Nói thẳng việc đã làm, vì sao làm, validation nào đã chạy, và giải thích chi tiết ý nghĩa của các thay đổi sau khi change để user hiểu tác động thực tế lên behavior, architecture, docs hoặc workflow.
- **Liệt kê việc còn lại nếu chưa xong:** Nếu task chưa được xử lý trọn vẹn, phải nêu rõ các công việc còn lại chưa hoàn thành, lý do còn dang dở và gợi ý bước tiếp theo ngắn gọn.
- **Không build chỉ để kết thúc:** Không chạy build rộng nếu repo guidance không yêu cầu.
- **Tự review liên tục:** Lặp lại review và cleanup đến khi diff không còn vấn đề rõ ràng về scope, chất lượng, comment hoặc validation.

## Nguyên Tắc Thực Thi

- Triển khai hành vi được yêu cầu theo plan đã duyệt hoặc theo bối cảnh đã research đủ rõ.
- Ưu tiên kiến trúc hiện tại sạch và đúng hơn các lớp vá để giữ tương thích ngược không cần thiết.
- Xóa hoặc thay thế hành vi lỗi thời khi thiết kế mới khiến nó không còn cần thiết.
- Giữ thay đổi đúng phạm vi yêu cầu và các module bị ảnh hưởng.
- Giữ nguyên các thay đổi không liên quan của user trong worktree.

## Quy Trình

1. Đọc lại plan đã duyệt hoặc ghi chú research liên quan trước khi sửa.
2. Xác định nguyên nhân gốc rễ và bức tranh tổng quan vừa đủ: hành vi hiện tại, module liên quan, contract, call site, dữ liệu vào/ra và lý do thay đổi là cần thiết.
3. Thu hẹp thành phạm vi sửa tập trung: các file cần sửa, pattern code lân cận, test/validation phù hợp và phần rõ ràng ngoài scope.
4. Triển khai thay đổi theo style, helper và kiến trúc hiện có của repo.
5. Không giữ tương thích ngược trừ khi user yêu cầu rõ hoặc public contract hiện tại bắt buộc phải giữ.
6. Sau mỗi lượt sửa file, review toàn bộ diff mình vừa tạo, bao gồm logic, imports, tests, docs và comment.
7. Cleanup ngay các phần thừa: code chết, duplication, naming lệch pattern, debug output, TODO không có chủ đích, whitespace churn hoặc thay đổi ngoài scope.
8. Rà comment trong vùng code vừa chạm; bổ sung hoặc chuyển sang tiếng Anh ở những điểm cần ngữ cảnh để người sau đọc nhanh hơn, đồng thời giữ comment ngắn, chính xác và không mô tả điều code đã nói rõ.
9. Lặp lại review và cleanup cho đến khi không còn vấn đề rõ ràng cần sửa.
10. Chạy validation có mục tiêu khi có sẵn và phù hợp, nhưng không chạy full build chỉ để kết thúc nếu guidance của repo nói không cần build.
11. Tổng kết sau thay đổi bằng cách giải thích ý nghĩa của từng nhóm thay đổi: vấn đề hoặc nhu cầu nào được xử lý, hành vi/contract nào đổi hoặc được giữ nguyên, rủi ro nào giảm, và user cần lưu ý điều gì khi tiếp tục làm việc.
12. Nếu thay đổi code ảnh hưởng đến flow, business rule, architecture, quan hệ module hoặc constraint, dùng `update-docs` để cập nhật docs/specs liên quan.
13. Trước khi kết thúc, nếu chưa hoàn thành toàn bộ yêu cầu, liệt kê rõ các phần còn lại chưa làm, trạng thái hiện tại và bước tiếp theo được đề xuất.

## Ràng Buộc

- Không dùng browser tools.
- Không chạy build chỉ để kết thúc task.
- Không để nội dung docs/spec cũ nằm cạnh hành vi mới; thay thế các mô tả đã lỗi thời.
- Không để lại comment tiếng Việt trong code mới hoặc code vừa chạm nếu comment đó thuộc phần mình sửa.
- Không thêm comment hiển nhiên, verbose hoặc sai lệch với implementation.
- Không revert các thay đổi worktree không liên quan.
