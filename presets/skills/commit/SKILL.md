---
name: commit
description: Tạo git commit an toàn theo Conventional Commits, với commit description chi tiết như merge request.
---

# Commit Có Kiểm Soát

Dùng khi user yêu cầu commit, stage thay đổi, hoặc trigger `//c`.

## Nguyên Tắc

- **Kiểm tra trước:** Luôn `git status --short` trước khi stage/commit.
- **Stage có chủ đích:** Chỉ stage file/hunk thuộc task. Nếu có thay đổi không liên quan, hỏi user.
- **Review trước commit:** Đọc `git diff` và `git diff --staged`. Không commit debug output, secret, file tạm.
- **Commit lint:** `type(scope): subject` theo Conventional Commits. Subject imperative, chữ thường sau `:`, không dấu chấm.
- **Description như MR:** Luôn có body sau dòng trống.
- **Không tự push** trừ khi user yêu cầu.

## Commit Message

```text
type(scope): short imperative subject

Context:
Lý do thay đổi, gap hoặc failing behavior trước đó.

Changes:
- Implementation chính, file/module ảnh hưởng.
- Phần cố ý loại trừ nếu cần cho review.

Validation:
- Command/check đã chạy hoặc "Not run: reason".

Risks:
- Compatibility, rollout, migration risk. "None known." nếu đã review diff.
```

Scope theo module/area: `agents`, `preview`, `docs`, `skills`, etc. Breaking change: `!` trong header + `BREAKING CHANGE:` trong body.

## Quy Trình

1. `git status --short` → xác định thay đổi thuộc task.
2. `git diff` → review trước khi stage.
3. Tìm commitlint rule của repo nếu có.
4. Chạy validation mục tiêu khi phù hợp.
5. Soạn message, dùng temp file cho body dài.
6. `git add -- <path>` cho file rõ ràng.
7. `git diff --staged` → review lần cuối.
8. Kiểm tra commitlint nếu repo có.
9. `git commit -F <message-file>`.
10. Xác nhận bằng `git log -1 --format=%B`.

## Ràng Buộc

- Không `git add .` khi worktree có file không liên quan.
- Không commit secret, `.env`, artifact tạm.
- Không tạo commit chỉ có header. Body kiểu MR là mặc định.
- Không sửa source code chỉ để commit.
