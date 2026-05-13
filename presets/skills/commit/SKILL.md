---
name: commit
description: Chuẩn bị và tạo git commit an toàn theo commit lint/Conventional Commits, với phạm vi staged rõ ràng và commit description chi tiết như merge request.
---

# Commit Có Kiểm Soát

Dùng skill này khi user yêu cầu commit, tạo commit, chuẩn bị commit message,
stage thay đổi hoặc dùng trigger `//c`. Giọng làm việc của skill này là gọn,
cẩn thận và tôn trọng worktree: chỉ commit đúng phần user muốn, viết header
đúng commit lint và luôn có commit description chi tiết như một merge request.

## Nguyên Tắc Bắt Buộc

- **Bảo vệ thay đổi của user:** Luôn kiểm tra `git status --short` trước khi stage hoặc commit. Không revert, discard, checkout hoặc reset thay đổi không liên quan.
- **Stage có chủ đích:** Chỉ stage file hoặc hunk thuộc phạm vi task hiện tại. Nếu worktree có thay đổi không liên quan và không thể tách an toàn bằng command không tương tác, hỏi user thay vì đoán.
- **Review trước commit:** Đọc `git diff` và `git diff --staged` để bảo đảm commit chỉ chứa thay đổi mong muốn, không có debug output, secret, generated noise hoặc file tạm.
- **Commit lint trước hết:** Message phải theo Conventional Commits trừ khi repo có rule cụ thể khác: `type(scope): subject`. Dùng type hợp lệ như `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, `ci`, `perf`, `style` hoặc `revert`.
- **Subject đúng chuẩn:** Subject dùng imperative mood, chữ thường sau dấu `:`, không kết thúc bằng dấu chấm, mô tả kết quả hiện tại và giữ ngắn gọn.
- **Description như merge request:** Luôn viết commit body sau một dòng trống, có cấu trúc như MR nhỏ để reviewer hiểu context, thay đổi, validation và rủi ro. Không kể lịch sử thao tác.
- **Validation phù hợp:** Chạy test, lint hoặc command mục tiêu nếu repo/task có bề mặt xác minh rõ. Nếu không chạy được hoặc không phù hợp, nói rõ trong phản hồi cuối.
- **Không tự push:** Không push, tạo PR hoặc tag trừ khi user yêu cầu rõ.

## Commit Message

Commit message phải có header lint-safe và body chi tiết. Body nên đọc được như
một merge request thu nhỏ, không chỉ là một câu mô tả ngắn.

```text
type(scope): short imperative subject

Context:
Explain the user-facing or maintainer-facing reason for this change. Mention
the previous gap, failing behavior or requested capability.

Changes:
- Describe the main implementation or docs changes.
- Mention important files, modules or behavior affected.
- Call out any intentionally excluded work when it could matter to review.

Validation:
- command or check that passed, with enough detail to understand coverage
- Not run: reason, when no targeted validation was appropriate

Risks:
- Note compatibility, rollout, migration or follow-up risk.
- Use "None known." only after reviewing the staged diff.
```

Chọn `scope` theo module hoặc area rõ nhất, ví dụ `agents`, `preview`, `docs`,
`skills`, `mcp` hoặc `deps`. Nếu scope không rõ hoặc repo không dùng scope, có
thể bỏ scope và dùng `type: subject`. Với breaking changes, dùng `!` trong
header và thêm body `BREAKING CHANGE: ...`.

Có thể thêm section `Notes:` khi reviewer cần biết trade-off, generated output,
dependency behavior hoặc lý do chọn một hướng triển khai. Với commit rất nhỏ,
vẫn giữ ít nhất `Context`, `Changes` và `Validation`; `Risks` có thể ghi
`None known.` nếu đã kiểm tra staged diff.

## Quy Trình

1. Kiểm tra trạng thái bằng `git status --short` và xác định thay đổi nào thuộc yêu cầu của user.
2. Đọc diff liên quan bằng `git diff -- <path>` hoặc `git diff --stat` trước khi stage.
3. Tìm commit lint rule của repo nếu có, ví dụ `commitlint.config.*`, `package.json`, hooks hoặc docs. Nếu không có rule riêng, dùng Conventional Commits như trên.
4. Chạy validation mục tiêu khi có lệnh phù hợp và không quá rộng so với task.
5. Soạn commit message có header đúng lint và body chi tiết như merge request. Với body nhiều dòng, dùng file message tạm ngoài repo thay vì nhồi nhiều `-m` khó đọc.
6. Stage đúng phạm vi bằng command không tương tác, ưu tiên `git add -- <path>` cho file rõ ràng.
7. Kiểm tra `git diff --staged --stat` và `git diff --staged` trước khi commit.
8. Nếu repo có commitlint command sẵn, kiểm tra message trước commit, ví dụ pipe file message vào `npx commitlint --edit <message-file>` hoặc command tương đương của repo.
9. Tạo commit bằng `git commit -F <message-file>` cho description chi tiết, hoặc `git commit -m "<header>" -m "<body>"` nếu body ngắn.
10. Sau commit, đọc `git status --short` và `git log -1 --format=%B` để xác nhận message đầy đủ và báo lại kết quả.

## Ràng Buộc

- Không dùng `git add .` khi worktree có file không liên quan hoặc phạm vi chưa rõ.
- Không dùng command destructive như `git reset --hard`, `git checkout --`, `git clean` hoặc amend/rebase nếu user chưa yêu cầu rõ.
- Không commit secret, credential, file `.env` cá nhân, artifact tạm hoặc output build không được repo quản lý.
- Không tạo commit chỉ có header cụt. Commit body kiểu merge request là mặc định của skill này, kể cả khi diff nhỏ.
- Không sửa source code chỉ để commit trừ khi user đồng thời yêu cầu fix hoặc cleanup cụ thể.
