# Agent Instructions

## Trigger Skills

Agent phải nhận diện trigger skill được viết ở đầu message của user theo cú pháp:

```text
//<short-tag-of-skill>
```

Riêng trigger `//s` hoặc `/s` ở đầu message là trigger tắt cho skill
`spawn-opencode`, dùng để spawn OpenCode process như sub-agent.

Trigger có thể chứa một tag hoặc nhiều tag ghép liền nhau. Khi có nhiều tag, áp
dụng các skill tương ứng theo đúng thứ tự chữ cái trong trigger.

Ví dụ:

```text
//rpe add account notifications
```

Nghĩa là: chạy `read-search-docs` như bước search, sau đó chạy `plan`, rồi chạy
`execution`.

## Short Tags Cho Skill Local

| Trigger | Skill | Khi Dùng |
| --- | --- | --- |
| `//f` | `fix` | Chẩn đoán và sửa bug, failing test, regression hoặc lỗi runtime đã có triệu chứng cụ thể. |
| `//r` | `read-search-docs` | Search/đọc docs và specs, không sửa file. |
| `//s` | `spawn-opencode` | Spawn OpenCode process như sub-agent cho research, review, triển khai hoặc làm việc song song có phạm vi rõ. |
| `//p` | `plan` | Tạo hoặc cập nhật file planning cho task lớn và chờ user duyệt trước khi sửa source. |
| `//e` | `execution` | Triển khai thay đổi đã được duyệt hoặc task nhỏ đã rõ theo kiến trúc hiện tại của repo. |
| `//u` | `update-docs` | Cập nhật docs/specs để phản ánh trạng thái hiện tại của codebase. |
| `/s` | `spawn-opencode` | Spawn OpenCode process như sub-agent cho research, review, triển khai hoặc làm việc song song có phạm vi rõ. |

## Trigger Ghép

Các trigger ghép thường dùng:

| Trigger | Pipeline |
| --- | --- |
| `//rf` | Search docs/specs liên quan, rồi fix theo nguồn tham chiếu hiện có. |
| `//sf` | Spawn OpenCode sub-agent, rồi fix khi đã đủ bối cảnh. |
| `//fu` | Fix lỗi, rồi update docs nếu behavior, architecture, business rules hoặc quan hệ module thay đổi. |
| `//rp` | Search docs, rồi tạo plan. |
| `//rpe` | Search docs, tạo plan, rồi execution sau khi được duyệt nếu task cần. |
| `//re` | Search docs, rồi execution cho thay đổi nhỏ đã rõ. |
| `//spe` | Spawn OpenCode sub-agent, tạo plan, rồi execution sau khi được duyệt nếu task cần. |
| `//eu` | Execution thay đổi code, rồi update docs nếu behavior, architecture, business rules hoặc quan hệ module thay đổi. |

Nếu trigger ghép có `plan` đứng trước `execution` cho task lớn, dừng lại sau
bước plan và chờ user duyệt rõ ràng trước khi sửa source code.
