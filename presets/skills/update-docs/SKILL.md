---
name: update-docs
description: Giữ docs/specs của dự án đồng bộ với codebase hiện tại. Dùng khi user yêu cầu cập nhật tài liệu, sync specs, làm mới tài liệu kiến trúc, ghi lại implementation đã hoàn thành, tạo hoặc cập nhật docs/specs/features/research/learnings, hoặc khi thay đổi code cần được phản ánh vào knowledge base và trạng thái sync của repo.
---

# Cập Nhật Tài Liệu

Dùng skill này để cập nhật knowledge base của repo sau nghiên cứu hoặc implementation. Ưu tiên hướng dẫn của chính repo trước: đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` khi có, sau đó làm theo kiến trúc docs/specs bên dưới trừ khi repo quy định khác. Giọng làm việc của skill này là sạch sẽ và hiện tại: docs phải nói đúng trạng thái mới nhất, không cất giữ lịch sử thừa trong góc tối.

## Nguyên Tắc Bắt Buộc

- **Chỉ mô tả trạng thái hiện tại:** Không lưu lịch sử sync, incremental history, changelog, bảng commit history hoặc version snapshot trong docs/specs. Docs/specs phải mô tả trạng thái/thiết kế hiện tại; `docs/_sync.md` chỉ giữ metadata sync hiện tại.
- **Báo cáo cô đọng:** Giao tiếp thẳng thắn, đi vào trọng tâm, nêu rõ docs nào đã đổi và validation nào đã chạy.
- **Cập nhật tinh gọn:** Khi cập nhật specs, nội dung phải phản ánh thiết kế/logic mới nhất. Bắt buộc xóa nội dung cũ không còn chính xác, không giữ lại rồi thêm correction bên cạnh.

## Bộ Quy Tắc Viết Docs

Áp dụng các quy tắc này cho mọi docs mới hoặc docs đang được chỉnh, trừ khi repo có contract riêng rõ ràng hơn.

Luôn giữ đủ cả hai nhóm quy tắc `Markdown Docs` và `HTML Docs`. Khi cập nhật một file cụ thể, chọn nhóm quy tắc theo format của file đó; khi tạo hoặc chuẩn hóa generator docs, xác định format đích trước rồi áp dụng đầy đủ nhóm tương ứng.

### Nội Dung

- Viết như tài liệu vận hành hiện tại, không như nhật ký thay đổi. Tránh các đoạn “trước đây”, “vừa thêm”, “sẽ đổi sau” nếu không phải là spec/plan đang draft.
- Mỗi doc phải có phạm vi rõ: feature doc mô tả hành vi người dùng hoặc workflow đã shipped; module doc mô tả boundary, API, data model, business rules và quan hệ module; spec/plan mô tả yêu cầu chưa hoặc đang triển khai.
- Ưu tiên câu ngắn, heading rõ, danh sách có ý nghĩa. Xóa mô tả lặp, wrapper văn bản rỗng, câu chung chung và mọi phần không giúp người đọc quyết định hoặc implement.
- Khi behavior thay đổi, sửa statement cũ tại chỗ. Không thêm “Correction”, “Update”, “Note mới” bên cạnh nội dung stale.
- Ghi rõ constraint, assumption, failure mode, security/compliance rule và business rule nếu chúng ảnh hưởng đến cách hệ thống vận hành.
- Source references phải trỏ tới path thật và ổn định. Chỉ liệt kê nguồn trực tiếp; không biến phần tham khảo thành dump mọi file từng đọc.

### Link Và Quan Hệ

- Dùng link tương đối thật tới tài liệu tồn tại. Với Markdown dùng `[Tên](../path/doc.md)`; với HTML chỉ dùng thẻ `<a href="../path/doc.html">Tên</a>`: label là nội dung bên trong thẻ `<a>`, target luôn đặt trong `href`.
- Không dùng custom tag cho internal navigation nếu thẻ HTML chuẩn đã đủ. Trong HTML docs, dùng `<a>` cho internal links; chỉ dùng custom tag khi tag đó mang semantic mà HTML chuẩn không thể hiện được.
- Không tạo link placeholder. Nếu target chưa tồn tại, hoặc tạo doc đó trong cùng scope, hoặc ghi known-unsynced ngắn trong `docs/_sync.md` khi thật sự cần.
- Giữ quan hệ hai chiều ở mức cần thiết: khi một module doc link tới feature/shared model quan trọng, kiểm tra doc đích có cần link ngược hoặc cập nhật quan hệ không.
- Source References có thể là `<a href="..."><code>path</code></a>` khi target mở được, hoặc `<code>path</code>` khi chỉ là path tham khảo chưa chắc tồn tại trong checkout. Không dùng path label khác href một cách gây hiểu nhầm.

### Markdown Docs

- Dùng Markdown cho docs thủ công hoặc repo chưa có HTML contract. Frontmatter và `## Meta` phải ngắn, nhất quán, không chứa history.
- Một doc nhỏ không cần đủ mọi heading template. Chỉ giữ các section giúp mô tả trạng thái hiện tại.
- Code path dùng inline code; snippet dài dùng fenced code block có language khi biết.
- Mermaid/diagram chỉ thêm khi giúp giải thích dependency, flow hoặc state machine rõ hơn văn bản ngắn.

### HTML Docs

- Generated HTML nên là fragment nội dung, không phải full document shell: không `<!doctype>`, `<html>`, `<head>`, `<body>` nếu preview chỉ cần fragment.
- Dùng custom metadata tags tối thiểu khi repo hỗ trợ HTML docs: `doc-meta`, `doc-title`, `doc-description`, và các custom semantic tags đã được repo preview support.
- Internal navigation trong `doc-meta` hoặc body chỉ dùng `<a href="...">label</a>`. Không dùng custom tag riêng cho link; label nằm trong content của `<a>`, link nằm trong `href`.
- Output phải ngắn gọn và ổn định: không inline `<script>`, `<style>`, event handler, framework attributes, id tự sinh, class rỗng, class trùng, wrapper chỉ để trang trí, hoặc attribute không phục vụ semantic/rendering thật.
- Tailwind/class chỉ dùng khi tạo khác biệt layout hoặc meaning rõ ràng. Baseline custom tag styling của preview phải đủ đọc khi class bị bỏ.
- Metadata không được lặp lại thành một phần body chỉ để parser đọc. Nếu cần hiển thị metadata, preview nên render từ metadata source.
- Diagram/code trong HTML dùng contract repo hỗ trợ, ví dụ `doc-diagram`, `doc-graph`, `doc-code`, hoặc `<pre><code class="language-*">`.

### Chất Lượng Diff

- Giữ diff nhỏ và có chủ đích. Không rewrite hàng loạt chỉ để đổi style nếu task không yêu cầu.
- Không trộn normalize docs với thay đổi behavior lớn nếu có thể tách riêng.
- Sau khi edit, đọc lại diff để bắt link sai cấp thư mục, stale statement, duplicate section, whitespace churn và metadata không khớp nội dung.

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

## Metadata Chuẩn

Metadata giúp agent hiểu tài liệu hiện đang mô tả cái gì, trạng thái nào, và liên quan trực tiếp tới phần nào của codebase. Metadata không phải changelog; chỉ ghi trạng thái hiện tại của doc tại thời điểm sync.

### Frontmatter Markdown

Dùng YAML frontmatter cho docs mới hoặc docs đã có frontmatter. Với docs cũ không có frontmatter, chỉ thêm khi việc đó giúp index/search/sync rõ hơn; không rewrite hàng loạt chỉ để chuẩn hóa.

```yaml
---
title: "[Tên ngắn, rõ nghĩa]"
description: "[Một câu mô tả phạm vi tài liệu]"
type: spec | feature | module | architecture | decision | pattern | shared | development | research | learning | compliance | index | sync
status: draft | proposed | approved | implemented | active | deprecated | archived
tags: ["tag-ngan", "domain"]
owners: ["team-or-area"]
source_paths:
  - "internal/example/example.go"
related:
  - "../modules/example.md"
---
```

Các field:

- `title`: Tên hiển thị của doc. Nên trùng hoặc gần trùng với H1.
- `description`: Một câu mô tả phạm vi hiện tại của doc, không ghi lịch sử thay đổi.
- `type`: Loại tài liệu. Chọn đúng một giá trị trong danh sách chuẩn để index và search ổn định.
- `status`: Trạng thái hiện tại của nội dung. Với docs shipped dùng `implemented` hoặc `active`; với planning/spec chưa duyệt dùng `draft` hoặc `proposed`; với nội dung không còn dùng nữa chỉ đặt `deprecated` hoặc `archived` khi doc vẫn cần giữ.
- `tags`: Nhãn ngắn cho domain, module, capability hoặc workflow. Dùng lowercase/kebab-case khi có thể.
- `owners`: Nhóm, module hoặc area chịu trách nhiệm. Bỏ qua nếu repo không có ownership rõ.
- `source_paths`: Code path chính mà doc mô tả. Chỉ liệt kê path ổn định và trực tiếp, không liệt kê mọi file phụ.
- `related`: Markdown path tương đối tới docs liên quan trực tiếp. Chỉ link tài liệu tồn tại.

Không dùng frontmatter để lưu `last_updated`, commit history, version timeline, migration notes hoặc danh sách commit. Sync commit thuộc về `docs/_sync.md`.

### Khối `## Meta`

Với module, feature, architecture, shared, research và compliance docs, thêm `## Meta` ngay sau H1 khi doc cần quan hệ rõ cho agent. Khối này dễ đọc bằng mắt hơn frontmatter và nên dùng cho metadata giàu ngữ nghĩa.

```markdown
## Meta

- Trạng thái: active | implemented | draft | deprecated | archived
- Phạm vi: [module/capability/workflow mà doc mô tả]
- Nguồn code: `path/to/file.go`, `path/to/dir`
- Tuân thủ: [policy/spec/pattern liên quan hoặc "Không áp dụng"]
- Links: [Tên Doc](../path/doc.md), [Doc Khác](../path/other.md)
```

Các field:

- `Trạng thái`: Trạng thái hiện tại của doc, cùng nghĩa với `status` trong frontmatter. Nếu có cả hai, phải nhất quán.
- `Phạm vi`: Ranh giới nội dung mà doc chịu trách nhiệm. Dùng để tránh duplicate hoặc viết lan sang module khác.
- `Nguồn code`: Path code chính đang được mô tả. Dùng inline code path; giữ danh sách ngắn.
- `Tuân thủ`: Quy định, quyết định kiến trúc, security/compliance rule hoặc convention mà doc cần bám theo. Ghi `Không áp dụng` nếu không có.
- `Links`: Quan hệ trực tiếp tới docs hiện có. Dùng Markdown link tương đối thật; không ghi link placeholder.

Không thêm field nếu không có giá trị thật. Không tạo bảng metadata lớn cho docs nhỏ; ưu tiên một khối ngắn và ổn định.

## Quy Trình

1. Kiểm tra trạng thái hiện tại bằng `git status --short` và định vị docs hiện có bằng `rg --files docs` khi `docs/` tồn tại.
2. Nếu `graphify-out/GRAPH_REPORT.md` tồn tại, đọc file này trước khi search raw code để nắm god nodes, community structure và các knowledge gaps hiện tại.
3. Đọc `docs/_sync.md` trước nếu file này tồn tại. Trích xuất synced commit/HEAD từ đó. Nếu user nêu target commit, dùng commit đó làm target; nếu không thì dùng `HEAD`. Nếu không có sync state, xem docs là chưa sync và dùng commit liên quan cũ nhất hoặc diff hiện tại của worktree làm nguồn so sánh.
4. So sánh sync-state commit với target commit. Dùng cả commit summaries và diffs:
   - `git log --oneline <synced-commit>..<target-commit>`
   - `git diff --name-status <synced-commit>..<target-commit>`
   - `git diff <synced-commit>..<target-commit> -- <relevant paths>`
   - Thêm `git diff --name-status` và `git diff -- <relevant paths>` cho thay đổi chưa commit khi worktree dirty.
5. Nếu có hơn một commit giữa synced commit và target commit, duyệt từng commit theo thứ tự thời gian:
   - Lấy danh sách commit theo thứ tự bằng `git rev-list --reverse <synced-commit>..<target-commit>`.
   - Với mỗi commit, inspect `git show --stat --oneline <commit>` và targeted diffs như `git show --name-status <commit>` hoặc `git show <commit> -- <relevant paths>`.
   - Tích lũy final behavior, renamed paths, deleted concepts, module mới và relationship đã thay đổi.
   - Không cập nhật docs như journal theo từng commit; dùng việc duyệt commit để không bỏ sót rename, removal hoặc semantic change ở giữa.
6. Đọc `docs/overview.md` và docs/specs bị chạm bởi các module đã đổi. Đồng thời đọc các code path đã đổi vừa đủ để hiểu final behavior tại target commit.
7. Quyết định tập docs nhỏ nhất cần cập nhật từ commit walk và final diff. Tránh rewrite rộng và duplicate docs.
8. Cập nhật docs để mô tả thiết kế hiện tại tại target commit, không mô tả chuỗi commit đã dẫn tới trạng thái đó. Xóa statement stale thay vì thêm correction bên cạnh.
9. Duy trì link hai chiều khi document relationships. Dùng Markdown link tương đối thật tới file `.md`.
10. Cập nhật `docs/_index.md` khi thêm, move hoặc xóa docs có ý nghĩa.
11. Cập nhật `docs/_sync.md` như snapshot sync cuối cùng sau khi docs đã phản ánh target commit.
12. Nếu repo có `graphify-out/graph.json`, refresh graph sau khi cập nhật bằng `graphify auto-update .`. Nếu CLI không có sẵn hoặc command lỗi, báo rõ trong phản hồi cuối thay vì bỏ qua âm thầm. Khi chỉ muốn refresh các file cụ thể và đã biết danh sách file code bị chạm, có thể dùng `graphify update graphify-out/graph.json <file...>`.
13. Chạy `git diff --check` cho docs đã sửa. Nếu repo có doc validation, chạy nó trừ khi user yêu cầu không chạy.

## Sync State

Luôn duy trì sync state trong `docs/_sync.md` khi cây docs tồn tại hoặc được tạo. File này là nguồn sự thật cho điểm sync docs trước đó và là nơi ghi điểm sync mới sau khi cập nhật.

`docs/_sync.md` nên ngắn và chỉ mô tả trạng thái hiện tại. Bao gồm:

- Commit hiện tại hoặc HEAD đang được docs phản ánh.
- Sync timestamp nếu hữu ích.
- Phạm vi docs đã kiểm tra hoặc cập nhật.
- Bất kỳ khu vực known-unsynced nào dưới dạng note ngắn.

Khuyến nghị dùng cấu trúc sau:

```markdown
# Docs Sync State

## Meta

- Synced commit: `<commit-sha-or-HEAD>`
- Synced at: `YYYY-MM-DDTHH:MM:SSZ`
- Scope: [docs/code area đã kiểm tra]
- Status: synced | partially-synced | unsynced
- Known unsynced: Không có | [note ngắn]
```

Các field:

- `Synced commit`: Commit hoặc `HEAD` mà docs hiện phản ánh. Đây là mốc để lần sau chạy `git log` và `git diff`.
- `Synced at`: Timestamp ISO-8601 nếu hữu ích. Dùng timezone rõ ràng; bỏ qua nếu repo không muốn timestamp.
- `Scope`: Phạm vi docs/code đã kiểm tra hoặc cập nhật trong lần sync hiện tại, ví dụ `preview UI`, `agent presets`, hoặc `docs tree`.
- `Status`: `synced` khi docs phản ánh target commit; `partially-synced` khi còn known-unsynced có chủ ý; `unsynced` khi mới bootstrap hoặc chưa thể xác minh.
- `Known unsynced`: Note ngắn về phần chưa sync, có path hoặc area cụ thể. Ghi `Không có` nếu không còn khu vực lệch đã biết.

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

Báo cáo docs đã thay đổi, kết quả sync state, kết quả refresh `graphify` nếu graph tồn tại, và validation đã chạy. Giữ câu trả lời cô đọng.
