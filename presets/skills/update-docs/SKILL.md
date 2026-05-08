---
name: update-docs
description: Giữ docs/specs của dự án đồng bộ với codebase hiện tại. Dùng khi user yêu cầu cập nhật docs, sync specs, refresh tài liệu kiến trúc, document implementation đã hoàn thành, tạo hoặc cập nhật docs/specs/features/research/learnings, hoặc khi thay đổi code cần được phản ánh vào knowledge base và sync state của repo.
---

# Cập Nhật Docs

Dùng skill này để cập nhật knowledge base của repo sau research hoặc implementation. Ưu tiên guidance của chính repo trước: đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` khi có, sau đó làm theo kiến trúc docs/specs bên dưới trừ khi repo quy định khác.

## Nguyên Tắc Bắt Buộc

- **Current-state only:** Không lưu sync history, increment history, changelog, commit-history table hoặc version snapshots trong docs/specs. Docs/specs phải mô tả trạng thái/thiết kế hiện tại; `docs/_sync.md` chỉ giữ metadata sync hiện tại.
- **Transparent-compact:** Giao tiếp thẳng thắn, báo cáo cô đọng, đi vào trọng tâm.
- **Concise-update:** Khi cập nhật specs phải đảm bảo nội dung tinh gọn, phản ánh thiết kế/logic mới nhất. BẮT BUỘC xóa bỏ các nội dung cũ không còn chính xác, KHÔNG ĐƯỢC giữ lại thông tin cũ rồi gióng ngoặc hoặc ghi thêm cập nhật bên cạnh.

## Quy Ước Thư Mục

Dùng `docs/` làm root của knowledge base. Không tạo cây `specs/` ở root trừ khi user yêu cầu rõ hoặc repo đã yêu cầu như vậy.

```text
docs/
├── README.md
├── overview.md
├── _index.md
├── _sync.md
├── specs/
│   └── planning/
├── features/
├── architecture/
│   ├── overview.md
│   ├── decisions/
│   └── patterns/
├── modules/
├── shared/
├── development/
│   └── conventions/
├── research/
├── learnings/
└── compliance/
```

## Quy Tắc Đặt File

- Đặt requirements trước implementation, acceptance criteria, scenarios và technical plans trong `docs/specs/`.
- Đặt plan task lớn cần user phê duyệt trong `docs/specs/planning/`.
- Đặt tài liệu cho hành vi đã implement hoặc shipped trong `docs/features/`.
- Đặt thiết kế module hiện tại, APIs, quan hệ, ràng buộc và business rules trong `docs/modules/`.
- Đặt pattern kiến trúc tái sử dụng trong `docs/architecture/patterns/`.
- Đặt quyết định kiến trúc và trade-off trong `docs/architecture/decisions/`.
- Đặt shared models, glossary, quy ước API và project context trong `docs/shared/`.
- Đặt investigation tạm thời, benchmark và bug report trong `docs/research/`.
- Đặt lesson có thể tái sử dụng từ debugging hoặc implementation trong `docs/learnings/`.
- Đặt báo cáo orphan-code hoặc design-compliance trong `docs/compliance/`.

Quy tắc nhanh: trước khi code thì ghi vào `docs/specs/`; sau khi code shipped thì ghi vào `docs/features/`; investigation chưa chắc chắn thì ghi vào `docs/research/`.

## Quy Tắc Quan Hệ

- Theo dõi phụ thuộc dữ liệu (`reads`), API calls (`calls`), shared models, events phát ra/tiêu thụ, và chuỗi business rules khi chúng quan trọng với architecture hoặc behavior.
- Giữ quan hệ trực tiếp hai chiều khi cập nhật docs. Nếu module doc link đến shared model hoặc feature doc, cập nhật doc liên quan khi cần để graph vẫn điều hướng được.
- Dùng field `Links` trong `## Meta` cho quan hệ trực tiếp khi target doc có ý nghĩa ổn định.
- Dùng Markdown link tương đối thật tới file `.md` hiện có, ví dụ `[Data Models](../shared/data-models.md)` từ `docs/modules/` hoặc `[Auth Spec](../specs/auth.md)` từ `docs/features/`.
- Nếu target được link chưa tồn tại, hoặc tạo nó khi task yêu cầu, hoặc để lại note known-unsynced ngắn trong `docs/_sync.md`. Không tạo docs placeholder rỗng.

## Quy Trình

1. Kiểm tra trạng thái hiện tại bằng `git status --short` và định vị docs hiện có bằng `rg --files docs` khi `docs/` tồn tại.
2. Đọc `docs/_sync.md` trước nếu file này tồn tại. Trích xuất synced commit/HEAD từ đó. Nếu user nêu target commit, dùng commit đó làm target; nếu không thì dùng `HEAD`. Nếu không có sync state, xem docs là chưa sync và dùng commit liên quan cũ nhất hoặc diff hiện tại của worktree làm nguồn so sánh.
3. So sánh sync-state commit với target commit. Dùng cả commit summaries và diffs:
   - `git log --oneline <synced-commit>..<target-commit>`
   - `git diff --name-status <synced-commit>..<target-commit>`
   - `git diff <synced-commit>..<target-commit> -- <relevant paths>`
   - Thêm `git diff --name-status` và `git diff -- <relevant paths>` cho thay đổi chưa commit khi worktree dirty.
4. Nếu có hơn một commit giữa synced commit và target commit, duyệt từng commit theo thứ tự thời gian:
   - Lấy danh sách commit theo thứ tự bằng `git rev-list --reverse <synced-commit>..<target-commit>`.
   - Với mỗi commit, inspect `git show --stat --oneline <commit>` và targeted diffs như `git show --name-status <commit>` hoặc `git show <commit> -- <relevant paths>`.
   - Tích lũy final behavior, renamed paths, deleted concepts, module mới và relationship đã thay đổi.
   - Không cập nhật docs như journal theo từng commit; dùng việc duyệt commit để không bỏ sót rename, removal hoặc semantic change ở giữa.
5. Đọc `docs/overview.md` và docs/specs bị chạm bởi các module đã đổi. Đồng thời đọc các code path đã đổi vừa đủ để hiểu final behavior tại target commit.
6. Quyết định tập docs nhỏ nhất cần cập nhật từ commit walk và final diff. Tránh rewrite rộng và duplicate docs.
7. Cập nhật docs để mô tả thiết kế hiện tại tại target commit, không mô tả chuỗi commit đã dẫn tới trạng thái đó. Xóa statement stale thay vì thêm correction bên cạnh.
8. Duy trì link hai chiều khi document relationships. Dùng Markdown link tương đối thật tới file `.md`.
9. Cập nhật `docs/_index.md` khi thêm, move hoặc xóa docs có ý nghĩa.
10. Cập nhật `docs/_sync.md` như snapshot sync cuối cùng sau khi docs đã phản ánh target commit.
11. Chạy `git diff --check` cho docs đã sửa. Nếu repo có doc validation, chạy nó trừ khi user yêu cầu không chạy.

## Sync State

Luôn duy trì sync state trong `docs/_sync.md` khi cây docs tồn tại hoặc được tạo. File này là nguồn sự thật cho điểm sync docs trước đó và là nơi ghi điểm sync mới sau khi cập nhật.

`docs/_sync.md` nên ngắn và chỉ mô tả trạng thái hiện tại. Bao gồm:

- Commit hiện tại hoặc HEAD đang được docs phản ánh.
- Sync timestamp nếu hữu ích.
- Phạm vi docs đã kiểm tra hoặc cập nhật.
- Bất kỳ khu vực known-unsynced nào dưới dạng note ngắn.

Khi cập nhật docs, dùng previous synced commit từ `docs/_sync.md` để inspect `git log` và `git diff` đến target commit. Nếu range có nhiều commit, duyệt chúng theo thứ tự thời gian từ `<synced-commit>+1` đến `<target-commit>` để hiểu intermediate renames, removals và semantic shifts. Chỉ dùng history đó làm input để tổng hợp. Không copy commit list, diff timeline, incremental notes, legacy notes, history, migration logs hoặc changelogs vào docs.

Không đưa changelogs, bảng commit-history, migration history, incremental sync logs, legacy sections hoặc narrative "what changed over time" vào `docs/_sync.md` hay docs khác trừ khi user yêu cầu rõ changelog.

## Mẫu Spec

Dùng cấu trúc này cho `docs/specs/*.md`:

```markdown
---
title: "[Tính năng hoặc thay đổi]"
description: "[Mô tả ngắn]"
type: spec
status: draft | approved | implemented
tags: [spec]
---

# [Tính năng hoặc thay đổi]

## Tổng Quan

## Yêu Cầu

### REQ-1: [Yêu cầu]

**Tiêu Chí Chấp Nhận:**
- [ ] AC-1.1: [Tiêu chí]

**Kịch Bản:**
GIVEN [bối cảnh]
WHEN [hành động]
THEN [kết quả mong đợi]

## Ghi Chú Triển Khai

## Tham Chiếu

## Ghi Chú
```

## Mẫu Module Doc

Dùng cấu trúc này cho `docs/modules/*.md`:

```markdown
# [Tên Module]

## Meta

- Trạng thái:
- Tuân thủ:
- Links:

## Tổng Quan

## Yêu Cầu Chức Năng Và Phi Chức Năng

## Data Models Và APIs

## Quy Tắc Nghiệp Vụ

## Ràng Buộc Và Giả Định

## Quan Hệ

## Quyết Định Liên Quan
```

## Phản Hồi Cuối

Báo cáo docs đã thay đổi, kết quả sync state và validation đã chạy. Giữ câu trả lời cô đọng.
