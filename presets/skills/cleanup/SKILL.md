---
name: cleanup
description: Audit worktree, uncommitted changes, branch hoặc commit/ref để đánh giá dead code, dead flows, dead docs, legacy code/docs. Output là một plan trong `docs/specs/planning/` để user duyệt, không tự xóa. Trigger: dọn dẹp, audit branch, tìm dead code, lập cleanup plan, rút gọn trước khi tiếp tục.
---

# Cleanup Audit Và Plan

Dùng skill này khi cần đánh giá cleanup trước khi xóa hoặc refactor. Mục tiêu là gom bằng chứng từ diff, code và docs, rồi dùng `plan` để tạo kế hoạch cleanup có thể duyệt. Không tự xóa code/docs trong skill này nếu user chưa duyệt plan rõ ràng.

## Kết Quả Mong Đợi

- Một inventory có bằng chứng cho dead code, dead flows, dead docs, legacy compatibility, duplicate logic hoặc docs stale.
- Một file plan trong `docs/specs/planning/` mô tả phạm vi cleanup, thứ tự xử lý, rủi ro và validation.
- Danh sách các phần không đủ bằng chứng để xóa, kèm cách kiểm chứng tiếp theo.

## Workflow

1. Xác định nguồn cần audit:
   - Với worktree hiện tại, đọc `git status --short`, `git diff --stat`, `git diff` và `git diff --staged` nếu có staged changes.
   - Với branch, dùng lệnh chỉ đọc như `git merge-base`, `git log`, `git diff --stat` và `git diff`; không switch branch.
   - Với commit/ref, dùng `git show --stat`, `git show` hoặc `git diff <ref>^..<ref>`; nếu là merge commit, xác định parent phù hợp trước.
   - Nếu user nói chung "những gì đã triển khai", mặc định audit worktree hiện tại và bối cảnh `HEAD`; nếu không có diff, đọc các commit gần nhất vừa đủ để hiểu scope.
2. Chạy pha `read-search-docs` như pipeline `//r`:
   - Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md`, `docs/README.md`, `docs/_index.md` và `docs/_sync.md` khi có.
   - Nếu docs stale so với `HEAD` hoặc target ref, coi docs là bối cảnh cần verify bằng code/diff.
   - Đọc docs/specs/features/modules liên quan đến file hoặc behavior trong diff.
3. Kiểm chứng reachability:
   - Dùng `rg --files` và `rg -n` trước để tìm path, symbol, route, command, config key, docs link và test coverage.
   - Khi cần caller/callee/reference context, dùng `lsp-code-graph` hoặc `graph --query`; nếu graph thiếu kết quả thì nói rõ fallback sang `rg` và code inspection.
   - Phân biệt rõ "không tìm thấy reference" với "đủ bằng chứng để xóa".
4. Phân loại cleanup candidate:
   - **Dead code:** symbol/helper/type/import/test fixture không còn được gọi, duplicate branch bị thay thế, fallback không còn reachable.
   - **Dead flows:** CLI/UI/API/state path không còn entrypoint, feature flag/config không còn tác dụng, test mô phỏng flow cũ đã bị bỏ khỏi behavior.
   - **Dead docs:** docs link tới path đã mất, spec mô tả behavior cũ, plan implemented nhưng còn viết như future work, `_index.md` hoặc graph docs lệch thực tế.
   - **Legacy code/docs:** compatibility layer, migration note, adapter branch hoặc docs wording chỉ phục vụ kiến trúc cũ và làm nhiễu behavior hiện tại.
5. Đánh giá rủi ro trước khi đề xuất xóa:
   - Xác định public contract, config user-level, data migration, adapter/native path, generated artifact và test surface có thể bị ảnh hưởng.
   - Nếu cleanup cần nhiều module hoặc có rủi ro behavior, tách thành phase nhỏ.
   - Nếu bằng chứng yếu, đưa vào nhóm "cần xác minh thêm" thay vì xóa trong plan chính.
6. Dùng `plan` như pipeline `//p` để tạo file trong `docs/specs/planning/`:
   - Tên file nên dạng `cleanup-<scope>.md`.
   - Plan phải viết bằng tiếng Việt, không kể lịch sử commit, không dump file list nếu không giúp quyết định.
   - Plan phải nêu bối cảnh, nguyên nhân cleanup, phạm vi, ngoài phạm vi, candidate theo nhóm, thứ tự xử lý, rủi ro, validation và tiêu chí chấp nhận.
7. Dừng lại sau plan và chờ user duyệt trước khi triển khai cleanup.

## Ràng Buộc

- Không dùng `git switch`, `git checkout`, `git reset`, `git clean` hoặc command destructive để đọc branch/commit.
- Không xóa code/docs chỉ vì tên có vẻ cũ; cần bằng chứng về reachability, ownership hoặc docs mismatch.
- Không tạo cleanup plan kiểu changelog, commit summary hoặc danh sách diff thô.
- Không cập nhật docs hiện trạng thành "đã cleanup" trước khi cleanup thật sự được duyệt và triển khai.
- Không chạy full build chỉ để kết thúc audit; chọn validation mục tiêu theo candidate đã xác định.
