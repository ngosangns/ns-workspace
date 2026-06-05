---
name: fix
description: Chẩn đoán và sửa bug, test fail, regression hoặc lỗi runtime theo hướng tái hiện trước, sửa nguyên nhân gốc, có validation mục tiêu. Dùng khi đã có triệu chứng rõ (failing test, log lỗi, bug report). Trigger: sửa bug, fix lỗi, điều tra regression, làm ổn định hành vi.
---

# Fix Bug Có Bằng Chứng

Dùng skill này khi nhiệm vụ chính là sửa lỗi đã có triệu chứng rõ: test fail, lỗi runtime, regression, hành vi sai, log lỗi hoặc bug report cụ thể. Giọng làm việc là bình tĩnh, có chứng cứ: tái hiện trước, sửa đúng nguyên nhân, rồi xác minh bằng command sát lỗi nhất.

**Phân biệt với `execution`:** `fix` bắt đầu từ triệu chứng/bug report, dùng tái hiện làm bằng chứng, và ưu tiên diff nhỏ nhất đúng nguyên nhân. `execution` triển khai thay đổi đã được duyệt (qua `plan` hoặc `research`) và có thể chạm vào nhiều file hơn.

Nếu lỗi còn mơ hồ, dùng `research` trước để gom bối cảnh. Nếu fix làm đổi business rule, kiến trúc hoặc docs/specs liên quan, dùng `update-docs` sau khi sửa.

## Nguyên Tắc Bắt Buộc

- **Tái hiện trước:** Cố gắng tái hiện lỗi bằng test, command, log hoặc đọc code path cụ thể trước khi sửa. Nếu không tái hiện được nhưng đã có bằng chứng đủ từ code/log, ghi rõ lý do.
- **Sửa nguyên nhân gốc, không vá triệu chứng:** Phân biệt triệu chứng, nguyên nhân trực tiếp, nguyên nhân gốc rễ trong bối cảnh hệ thống. Nhìn đủ rộng để hiểu module boundary, contract và luồng dữ liệu, rồi thu hẹp vào fix nhỏ nhất đúng nguyên nhân.
- **Tuân thủ requirements của feature/module liên quan:** Trước khi sửa, xác định docs liên quan trong `docs/features/` và `docs/modules/`. Nếu có `requirements.md` trong folder thuộc phạm vi ảnh hưởng, đọc toàn bộ và coi chúng là acceptance constraints bắt buộc.
- **Thay đổi nhỏ nhưng trọn vẹn:** Diff nhỏ nhất có thể, nhưng sạch, đúng kiến trúc hiện tại, không tạo workaround khó bảo trì.
- **Chặn regression:** Khi phù hợp, thêm hoặc cập nhật test để lỗi không quay lại.
- **Validation mục tiêu:** Chạy command sát với lỗi đã sửa; không cần full build nếu repo guidance không yêu cầu.
- **Review toàn bộ changes sau khi edit:** Sau mỗi lượt sửa file, đọc lại toàn bộ diff vừa tạo, loại bỏ phần thừa, tối ưu logic và đảm bảo fix không kéo theo thay đổi ngoài nguyên nhân lỗi.
- **Comment bằng tiếng Anh ở vùng đã sửa:** Thêm/cập nhật comment cho edge case, regression guard, invariant hoặc workaround bắt buộc. Không thêm comment hiển nhiên hoặc thay cho code rõ ràng.
- **Giải thích ý nghĩa sau fix:** Khi báo cáo, giải thích lỗi gốc được loại bỏ ra sao, regression nào được chặn, hành vi nào đổi/giữ, phần nào còn rủi ro.
- **Liệt kê việc còn lại:** Nếu bug chưa sửa/xác minh trọn vẹn, nêu rõ phần còn lại, bằng chứng hiện có, lý do dang dở, bước tiếp theo.
- **Hỏi lại khi vướng mắc:** Khi không tái hiện được lỗi và chưa đủ bằng chứng để chốt nguyên nhân, khi bug có thể do nhiều nguyên nhân hợp lý, khi fix theo hướng này có thể đụng contract/business rule khác, hoặc khi requirements mâu thuẫn nhau và cần user chọn phía nào → **dừng lại và hỏi cụ thể**, kèm triệu chứng đã thu thập, các giả thuyết đang cân nhắc và lựa chọn khả dĩ. Không đoán nguyên nhân rồi sửa mù.
- **Tôn trọng worktree:** Không revert hoặc chạm vào thay đổi không liên quan của user.

## Quy Trình

1. Đọc bug report, failing output hoặc triệu chứng user đưa.
2. Kiểm tra `git status` để nhận diện thay đổi đang có và tránh đè việc của user.
3. Xác định code path liên quan bằng `rg`, test hiện có, docs/specs hoặc call site gần nhất. Khi phụ thuộc symbol/caller/callee/reference, dùng skill `lsp-code-graph`; nếu command báo thiếu language server hoặc kết quả không đủ, ghi rõ fallback sang `rg` và code inspection.
4. Tìm docs feature/module liên quan đến phạm vi lỗi. Đọc toàn bộ `requirements.md` tương ứng nếu tồn tại trong `docs/features/**/requirements.md` hoặc `docs/modules/**/requirements.md`; coi chúng là acceptance constraints bắt buộc.
5. Nếu requirements mâu thuẫn với bug report hoặc trạng thái code hiện tại, **dừng lại** và báo rõ mâu thuẫn thay vì sửa âm thầm theo một phía.
6. Dựng giả thuyết nguyên nhân gốc rễ: vì sao lỗi phát sinh, contract/invariant nào bị phá, dữ liệu đi qua đâu, requirements nào bị ảnh hưởng, khu vực nào không nên chạm.
7. Tái hiện lỗi bằng command nhỏ nhất có thể, hoặc ghi rõ nếu không tái hiện được nhưng đã có bằng chứng đủ từ code/log.
8. Sửa nguyên nhân gốc theo pattern hiện có của repo và toàn bộ requirements liên quan.
9. Thêm hoặc cập nhật test/regression guard nếu bug có bề mặt test hợp lý.
10. Sau mỗi lượt sửa file, review toàn bộ diff: kiểm tra scope, imports, naming, duplication, test coverage, dead code, debug output, docs/comment lệch.
11. Cleanup ngay phần thừa hoặc kém tối ưu; ưu tiên fix nhỏ, trực tiếp, đọc được, phù hợp kiến trúc hiện tại.
12. Rà comment trong vùng code vừa chạm; bổ sung hoặc chuyển sang tiếng Anh nếu fix phụ thuộc edge case, invariant hoặc ràng buộc khó thấy từ code.
13. Chạy lại command tái hiện lỗi và validation mục tiêu.
14. Review diff lần cuối, cleanup thay đổi thừa, rồi báo nguyên nhân gốc rễ, cách sửa, requirements nào đã tuân thủ, cách xác minh, ý nghĩa thực tế của từng nhóm thay đổi.
15. Nếu bug chưa xử lý/xác minh hết, liệt kê rõ công việc còn lại, trạng thái hiện tại, bước tiếp theo đề xuất.

## Ràng Buộc

- Không mở rộng scope sang refactor lớn nếu không cần để fix lỗi.
- Không đổi hành vi public ngoài phần cần sửa, trừ khi bug fix bắt buộc và cần nêu rõ.
- Không dùng browser tools trừ khi bug chỉ xác minh được qua UI hoặc user yêu cầu rõ.
- Không chạy build rộng chỉ để kết thúc task.
- Không bỏ qua `requirements.md` của feature/module liên quan khi sửa lỗi trong phạm vi đó.
- Không để lại comment tiếng Việt trong code mới hoặc code vừa chạm nếu thuộc phần mình sửa.
- Không thêm comment verbose, sai sự thật hoặc mô tả lại code hiển nhiên.
