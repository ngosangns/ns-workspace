# Agent Instructions

## Trigger Skills

Agent phải nhận diện trigger skill được viết ở đầu message của user theo cú pháp:

```text
//<short-tag-of-skill>
```

Trigger có thể chứa một tag hoặc nhiều tag ghép liền nhau. Khi có nhiều tag, áp
dụng các skill tương ứng theo đúng thứ tự chữ cái trong trigger.

Ví dụ:

```text
//spe add account notifications
```

Nghĩa là: chạy `read-search-docs` như bước search, sau đó chạy `plan`, rồi chạy
`execution`.

## Short Tags Cho Skill Local

| Trigger | Skill | Khi Dùng |
| --- | --- | --- |
| `//r` | `research` | Research codebase/docs trước khi sửa code, đặc biệt khi yêu cầu mơ hồ, liên quan kiến trúc hoặc có rủi ro. |
| `//s` | `read-search-docs` | Search/đọc docs và specs, không sửa file. |
| `//p` | `plan` | Tạo hoặc cập nhật file planning cho task lớn và chờ user duyệt trước khi sửa source. |
| `//e` | `execution` | Triển khai thay đổi đã được duyệt hoặc task nhỏ đã rõ theo kiến trúc hiện tại của repo. |
| `//u` | `update-docs` | Cập nhật docs/specs để phản ánh trạng thái hiện tại của codebase. |

## Trigger Ghép

Các trigger ghép thường dùng:

| Trigger | Pipeline |
| --- | --- |
| `//sp` | Search docs, rồi tạo plan. |
| `//spe` | Search docs, tạo plan, rồi execution sau khi được duyệt nếu task cần. |
| `//re` | Research, rồi execution cho thay đổi nhỏ đã rõ. |
| `//rpe` | Research, tạo plan, rồi execution sau khi được duyệt. |
| `//eu` | Execution thay đổi code, rồi update docs nếu behavior, architecture, business rules hoặc quan hệ module thay đổi. |

Nếu trigger ghép có `plan` đứng trước `execution` cho task lớn, dừng lại sau
bước plan và chờ user duyệt rõ ràng trước khi sửa source code.

## Registry-Managed Skills

Project cũng cài các registry-managed skills sau từ
`presets/registry/skills.json`:

- `find-skills`
- `dispatching-parallel-agents`
- `gitbutler`
- `graphify`
- `refactor`

Các registry-managed skills này giữ nguyên tên upstream. Dùng chúng khi user
gọi đích danh skill hoặc khi mô tả skill đã cài khớp rõ với task.
