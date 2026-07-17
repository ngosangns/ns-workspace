---
name: cleanup
description: Audit worktree/branch/commit để đánh giá dead code, dead flows, dead docs, legacy artifacts. Output là plan trong `docs/specs/planning/`. Trigger: dọn dẹp, audit branch, tìm dead code.
---

# Cleanup Audit Và Plan

Dùng khi cần đánh giá cleanup trước khi xóa hoặc refactor. Gom bằng chứng từ diff, code và docs, rồi dùng `plan` tạo kế hoạch cleanup. Không tự xóa nếu user chưa duyệt.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Kết Quả

- Inventory có bằng chứng cho dead code/flows/docs, legacy compatibility, duplicate logic.
- File plan trong `docs/specs/planning/cleanup-<scope>.md`.
- Danh sách phần không đủ bằng chứng để xóa + cách kiểm chứng tiếp.

## Workflow

1. Xác định nguồn audit:
   - Worktree: `git status --short`, `git diff --stat`, `git diff`, `git diff --staged`.
   - Branch: `git merge-base`, `git log`, `git diff --stat`, `git diff` (chỉ đọc, không switch).
   - Commit/ref: `git show --stat`, `git show`, `git diff <ref>^..<ref>`.
2. Chạy `read-search-docs`: đọc `AGENTS.md`, `docs/README.md`, `docs/_index.md`, `docs/_sync.md`, specs/features/modules liên quan trong cây flat `docs/`.
3. Kiểm chứng reachability: `rg --files` + `rg -n` trước. Dùng `lsp-code-graph` khi cần caller/callee context. Phân biệt "không tìm thấy reference" vs "đủ bằng chứng để xóa".
4. Phân loại candidate:
   - **Dead code:** symbol/helper/type/import không còn được gọi, duplicate branch, fallback không reachable.
   - **Dead flows:** CLI/UI/API path không còn entrypoint, feature flag không tác dụng.
   - **Dead docs:** docs link path đã mất, spec mô tả behavior cũ, `_index.md` lệch thực tế.
   - **Legacy:** compatibility layer, migration note, adapter branch chỉ phục vụ kiến trúc cũ.
5. Đánh giá rủi ro: public contract, config user-level, data migration, generated artifact, test surface. Tách phase nếu cần. Bằng chứng yếu → "cần xác minh thêm".
6. Dùng `plan` tạo file `cleanup-<scope>.md` trong `docs/specs/planning/`. Plan bằng tiếng Việt, nêu bối cảnh, phạm vi, candidate, thứ tự, rủi ro, validation.
7. **Dừng và chờ user duyệt.**

## Ràng Buộc

- Không dùng `git switch`, `git checkout`, `git reset`, `git clean` để đọc.
- Không xóa chỉ vì tên có vẻ cũ; cần bằng chứng reachability.
- Không tạo plan kiểu changelog hoặc diff thô.
