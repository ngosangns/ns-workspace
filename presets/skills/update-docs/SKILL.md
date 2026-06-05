---
name: update-docs
description: Cập nhật knowledge base của repo trong `./docs` cho khớp codebase hiện tại: specs, features, modules, architecture, research, learnings, requirements.md của feature/module folder, và `docs/_sync.md`. Dùng sau `execution`/`fix` khi thay đổi code cần phản ánh vào docs. Trigger: cập nhật tài liệu, sync docs, làm mới spec, ghi lại implementation, cập nhật requirements.
---

# Cập Nhật Tài Liệu

Dùng skill này để cập nhật knowledge base của repo sau nghiên cứu hoặc implementation. Ưu tiên hướng dẫn của chính repo trước: đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` khi có, sau đó làm theo kiến trúc docs/specs bên dưới trừ khi repo quy định khác. Giọng làm việc là sạch sẽ và hiện tại: docs phải nói đúng trạng thái mới nhất, không cất giữ lịch sử thừa trong góc tối.

## Nguyên Tắc Bắt Buộc

- **Chỉ mô tả trạng thái hiện tại:** Không lưu lịch sử sync, incremental history, changelog, bảng commit history hay version snapshot trong docs/specs. Docs/specs mô tả trạng thái/thiết kế hiện tại; `docs/_sync.md` chỉ giữ metadata sync hiện tại.
- **Root docs cố định:** Luôn tạo, sửa, move hoặc xóa tài liệu dự án bên trong `./docs` tính từ project root hiện tại. Không ghi docs vào `./specs`, thư mục docs ngoài repo, package-local docs hoặc docs root nào khác, kể cả khi repo hoặc user prompt gợi ý nơi khác.
- **Requirements theo feature/module:** Mỗi feature folder hoặc module folder nên có `requirements.md` chứa critical requirements phải luôn follow cho phạm vi đó. Khi user yêu cầu cập nhật requirements, critical rules, constraints, acceptance criteria hoặc always-follow rules cho feature/module, tạo hoặc cập nhật `requirements.md` tương ứng.
- **Báo cáo cô đọng:** Nói thẳng docs nào đã đổi và validation nào đã chạy.
- **Cập nhật tinh gọn:** Nội dung phải phản ánh thiết kế/logic mới nhất. Xóa nội dung cũ không còn chính xác, không giữ lại rồi thêm correction bên cạnh.
- **Hỏi lại khi vướng mắc:** Khi user yêu cầu cập nhật docs mà scope chưa rõ (chỉ feature đang đổi, hay toàn bộ module/architecture chịu ảnh hưởng), khi docs cũ và code hiện tại mâu thuẫn mà không rõ phía nào đúng (docs lỗi thời hay behavior đã đổi có chủ đích), khi có nhiều cách diễn giải thiết kế hiện tại, hoặc khi cần quyết định có migrate flat doc sang folder feature/module hay không → **dừng lại và hỏi user** trước khi viết, kèm diff context, các lựa chọn và rủi ro. Không tự quyết phạm vi docs khi ý user chưa rõ.

## Bộ Quy Tắc Viết Docs

Áp dụng cho mọi docs mới hoặc docs đang chỉnh, trừ khi repo có contract riêng rõ ràng hơn. Mỗi file chọn nhóm quy tắc theo format của nó (Markdown hay HTML); khi tạo/chuẩn hóa generator, xác định format đích trước rồi áp dụng đầy đủ nhóm đó.

### Nội Dung Và Quan Hệ

- Viết như tài liệu vận hành hiện tại, không như nhật ký thay đổi. Tránh các đoạn "trước đây", "vừa thêm", "sẽ đổi sau" nếu không phải spec/plan đang draft.
- Mỗi doc phải có phạm vi rõ: feature doc mô tả hành vi người dùng/workflow đã shipped; module doc mô tả boundary, API, data model, business rules, quan hệ module; spec/plan mô tả yêu cầu chưa hoặc đang triển khai.
- Ưu tiên câu ngắn, heading rõ, danh sách có ý nghĩa. Xóa mô tả lặp, wrapper văn xuông, câu chung chung và mọi phần không giúp người đọc quyết định hoặc implement.
- Khi behavior thay đổi, sửa statement cũ tại chỗ. Không thêm "Correction"/"Update"/"Note mới" bên cạnh nội dung stale.
- Ghi rõ constraint, assumption, failure mode, security/compliance rule và business rule nếu ảnh hưởng đến cách hệ thống vận hành.
- Source references và docs references phải nằm trong metadata (frontmatter `source_paths`/`related` cho Markdown; semantic tags `doc-meta`/`doc-relation` cho HTML) chứ không tạo section tham khảo riêng trong body. Label phải ngắn, target phải thật và ổn định, chỉ liệt kê nguồn trực tiếp — không dump mọi file từng đọc.
- Dùng link tương đối thật tới tài liệu tồn tại: Markdown `[Tên](../path/doc.md)`; HTML `<a href="../path/doc.html">Tên</a>` với label trong content, target trong `href`. Không tạo link placeholder; nếu target chưa tồn tại thì tạo trong cùng scope hoặc ghi known-unsynced ngắn trong `docs/_sync.md`.
- Giữ quan hệ hai chiều ở mức cần thiết: khi module doc link tới feature/shared model quan trọng, kiểm tra doc đích có cần link ngược hoặc cập nhật quan hệ không.
- Nếu target chỉ là path tham khảo chưa chắc tồn tại trong checkout, giữ trong metadata dạng text/code thay vì tạo link placeholder. Không dùng path label khác href một cách gây hiểu nhầm.

### Markdown Docs

- Dùng Markdown cho docs thủ công hoặc repo chưa có HTML contract. Frontmatter và `## Meta` ngắn, nhất quán, không chứa history.
- Một doc nhỏ không cần đủ mọi heading template; chỉ giữ section giúp mô tả trạng thái hiện tại.
- Code path dùng inline code; snippet dài dùng fenced code block có language khi biết.
- Mermaid/diagram chỉ thêm khi giúp giải thích dependency, flow hoặc state machine rõ hơn văn bản ngắn.

### HTML Docs

- Output là fragment nội dung, không phải full document shell: không `<!doctype>`, `<html>`, `<head>`, `<body>` khi preview chỉ cần fragment.
- Dùng custom semantic metadata tags tối thiểu khi repo hỗ trợ HTML: `doc-meta`, `doc-title`, `doc-description`, và các custom semantic tags repo preview support. Các tag nằm trực tiếp trong fragment, không nằm trong `<head>`.
- Internal navigation chỉ dùng `<a href="...">label</a>`. Không dùng custom tag riêng cho link.
- Output ngắn gọn và ổn định: không inline `<script>`, `<style>`, event handler, framework attributes, id tự sinh, class rỗng, class trùng, wrapper chỉ trang trí, hoặc attribute không phục vụ semantic/rendering thật.
- Tailwind/class chỉ dùng khi tạo khác biệt layout hoặc meaning rõ ràng. Baseline custom tag styling phải đủ đọc khi class bị bỏ.
- Metadata không được lặp thành section body chỉ để parser đọc. Không tạo `Source References`/`Docs Refs`/`Checked Sources` trong body nếu cùng dữ liệu đã nằm trong semantic metadata tags.
- Diagram/code trong HTML dùng contract repo hỗ trợ, ví dụ `doc-diagram`, `doc-graph`, `doc-code`, hoặc `<pre><code class="language-*">`.

### Chất Lượng Diff

- Giữ diff nhỏ và có chủ đích. Không rewrite hàng loạt chỉ đổi style nếu task không yêu cầu.
- Không trộn normalize docs với thay đổi behavior lớn nếu có thể tách riêng.
- Sau khi edit, đọc lại diff để bắt link sai cấp thư mục, stale statement, duplicate section, whitespace churn, metadata không khớp nội dung.
- Sau khi sửa Markdown/HTML docs, chạy formatter và lint bằng script của repo khi có: `npm run format:docs` rồi `npm run lint:docs`. Nếu chỉ cần kiểm tra không ghi file, dùng `npm run format:docs:check` trước hoặc thay write-mode khi scope bẩn ngoài task.

## Quy Ước Thư Mục Và Quy Tắc Đặt File

Dùng duy nhất `./docs` làm root của knowledge base. Không tạo cây `specs/` ở root, không dùng docs root ở package con, không ghi docs ra ngoài `./docs` vì bất kỳ ngoại lệ nào.

```text
docs/
├── README.md
├── overview.md
├── _index.md
├── _sync.md
├── specs/
│   └── planning/
├── features/
│   └── <feature>/
│       └── requirements.md
├── architecture/
│   ├── overview.md
│   ├── decisions/
│   └── patterns/
├── modules/
│   └── <module>/
│       └── requirements.md
├── shared/
├── development/
│   └── conventions/
├── research/
├── learnings/
└── compliance/
```

Quy tắc đặt file (nhanh):

- Trước khi code: ghi vào `docs/specs/`; trong lúc triển khai: spec/plan chưa duyệt đặt ở `docs/specs/planning/`.
- Sau khi code shipped: tài liệu hành vi đã implement đặt ở `docs/features/<feature>/`; thiết kế module hiện tại (boundary, API, business rules, quan hệ) ở `docs/modules/<module>/`.
- Pattern kiến trúc tái sử dụng ở `docs/architecture/patterns/`. Quyết định kiến trúc và trade-off ở `docs/architecture/decisions/`.
- Shared models, glossary, quy ước API, project context ở `docs/shared/`.
- Investigation tạm thời, benchmark, bug report ở `docs/research/`. Lesson có thể tái sử dụng ở `docs/learnings/`. Báo cáo orphan-code hoặc design-compliance ở `docs/compliance/`.

## Requirements Theo Feature/Module

`requirements.md` là file critical cho từng feature folder hoặc module folder. File này không thay thế feature/module docs đầy đủ; nó giữ các yêu cầu phải luôn được agent và người implement bám theo khi sửa phạm vi đó.

Áp dụng khi:

- User yêu cầu cập nhật requirements, critical rules, constraints, acceptance criteria, invariant, guardrail hoặc always-follow rule cho feature/module.
- Implementation hoặc docs update làm thay đổi business rule, security/compliance rule, API contract, data invariant, failure mode hoặc acceptance criteria của feature/module.
- Tạo feature/module folder mới trong `docs/features/<feature>/` hoặc `docs/modules/<module>/`.

Quy tắc:

- Nếu folder feature/module đã tồn tại, đảm bảo có `requirements.md` trong folder đó.
- Nếu feature/module đang là flat doc (`docs/features/foo.md` hoặc `docs/modules/bar.md`), chỉ migrate sang folder khi task yêu cầu hoặc khi cần để đặt `requirements.md` mà không tạo duplicate. Khi migrate, cập nhật `_index.md`, links hai chiều và references liên quan.
- `requirements.md` chỉ ghi yêu cầu hiện tại. Không ghi lịch sử thay đổi, migration note, commit note hay timeline.
- Requirements phải cụ thể và kiểm chứng được: mỗi item mô tả constraint, expected behavior, acceptance criteria hoặc failure mode rõ ràng.
- Khi requirements thay đổi, cập nhật feature/module/spec docs liên quan nếu chúng chứa statement cũ hoặc cần link tới requirements mới.
- Khi user yêu cầu "update requirements" mà không nêu folder, tự tìm feature/module liên quan từ docs/code context; chỉ hỏi lại nếu vẫn không xác định được phạm vi an toàn.

## Quy Tắc Quan Hệ

- Theo dõi phụ thuộc dữ liệu (`reads`), API calls (`calls`), shared models, events phát ra/tiêu thụ, và chuỗi business rules khi chúng quan trọng với architecture hoặc behavior.
- Giữ quan hệ trực tiếp hai chiều khi cập nhật docs. Nếu module doc link đến shared model hoặc feature doc, cập nhật doc liên quan khi cần để graph vẫn điều hướng được.
- Dùng field `Links` trong `## Meta` cho quan hệ trực tiếp khi target doc có ý nghĩa ổn định.
- Dùng Markdown link tương đối thật tới file `.md` hiện có, ví dụ `[Data Models](../shared/data-models.md)` từ `docs/modules/` hoặc `[Auth Spec](../specs/auth.md)` từ `docs/features/`.
- Nếu target được link chưa tồn tại, hoặc tạo khi task yêu cầu, hoặc để lại note known-unsynced ngắn trong `docs/_sync.md`. Không tạo docs placeholder rỗng.

## Metadata Chuẩn

Metadata giúp agent hiểu tài liệu đang mô tả cái gì, trạng thái nào, liên quan trực tiếp tới phần nào của codebase. Metadata không phải changelog; chỉ ghi trạng thái hiện tại tại thời điểm sync. Sync commit thuộc về `docs/_sync.md`, không bao giờ vào frontmatter.

### Frontmatter Markdown

Dùng YAML frontmatter cho docs mới hoặc docs đã có frontmatter. Với docs cũ chưa có, chỉ thêm khi giúp index/search/sync rõ hơn; không rewrite hàng loạt chỉ để chuẩn hóa.

```yaml
---
title: "[Tên ngắn, rõ nghĩa — nên trùng hoặc gần trùng H1]"
description: "[Một câu mô tả phạm vi hiện tại]"
type: spec | feature | module | architecture | decision | pattern | shared | development | research | learning | compliance | index | sync
status: draft | proposed | approved | implemented | active | deprecated | archived
tags: ["tag-ngan", "domain"] # lowercase/kebab-case
owners: ["team-or-area"] # bỏ qua nếu repo không có ownership rõ
source_paths: # code path chính mà doc mô tả, ổn định, trực tiếp
  - "internal/example/example.go"
related: # Markdown path tương đối tới docs liên quan trực tiếp
  - "../modules/example.md"
---
```

Không dùng frontmatter để lưu `last_updated`, commit history, version timeline, migration notes hay danh sách commit.

### Khối `## Meta`

Với module, feature, architecture, shared, research và compliance docs, thêm `## Meta` ngay sau H1 khi doc cần quan hệ rõ cho agent. Dễ đọc hơn frontmatter, dùng cho metadata giàu ngữ nghĩa.

```markdown
## Meta

- Trạng thái: active
- Phạm vi: [module/capability/workflow mà doc mô tả]
- Nguồn code: `path/to/file.go`, `path/to/dir` # inline code, danh sách ngắn
- Tuân thủ: [policy/spec/pattern liên quan hoặc "Không áp dụng"]
- Links: [Tên Doc](../path/doc.md), [Doc Khác](../path/other.md)
```

Nếu có cả frontmatter và `## Meta`, các trường tương ứng (đặc biệt `status`) phải nhất quán. Không thêm field nếu không có giá trị thật; không tạo bảng metadata lớn cho docs nhỏ — ưu tiên một khối ngắn và ổn định.

## Quy Trình

1. Kiểm tra trạng thái hiện tại bằng `git status --short` và định vị docs hiện có bằng `rg --files ./docs` khi `./docs` tồn tại. Nếu cần tạo docs mới, tạo bên trong `./docs`.
2. Đọc `./docs/_sync.md` trước nếu file này tồn tại. Trích xuất synced commit/HEAD từ đó. Nếu user nêu target commit, dùng commit đó; nếu không thì dùng `HEAD`. Nếu không có sync state, xem docs là chưa sync và dùng commit liên quan cũ nhất hoặc diff hiện tại của worktree làm nguồn so sánh.
3. So sánh sync-state commit với target commit bằng `git log --oneline`, `git diff --name-status`, `git diff` (và `git diff --staged` khi worktree dirty).
4. Nếu range có nhiều commit, duyệt từng commit theo thứ tự thời gian qua `git rev-list --reverse <synced-commit>..<target-commit>`; với mỗi commit, inspect `git show --stat --oneline <commit>` và targeted diffs để tích lũy final behavior, renamed paths, deleted concepts, module mới, relationship đã thay đổi. Không cập nhật docs như journal theo từng commit.
5. Đọc `./docs/overview.md` và `./docs/specs` bị chạm bởi các module đã đổi. Khi cần code graph context (symbol, caller/callee, references), dùng skill `lsp-code-graph`; nếu command báo thiếu language server hoặc không đủ kết quả, ghi rõ fallback sang diff và code inspection. Đồng thời đọc code path đã đổi vừa đủ để hiểu final behavior tại target commit.
6. Quyết định tập docs nhỏ nhất cần cập nhật từ commit walk và final diff. Tránh rewrite rộng và duplicate docs.
7. Cập nhật docs để mô tả thiết kế hiện tại tại target commit, không mô tả chuỗi commit đã dẫn tới trạng thái đó. Xóa statement stale thay vì thêm correction bên cạnh.
8. Khi scope là feature/module và có requirements critical, tạo hoặc cập nhật `requirements.md` trong folder feature/module tương ứng, nhất là khi user yêu cầu trực tiếp.
9. Duy trì link hai chiều khi document relationships. Cập nhật `./docs/_index.md` khi thêm, move hoặc xóa docs có ý nghĩa.
10. Cập nhật `./docs/_sync.md` như snapshot sync cuối cùng sau khi docs đã phản ánh target commit.
11. Chạy formatter cho Markdown/HTML docs đã sửa khi repo có script, ưu tiên `npm run format:docs`. Nếu worktree có thay đổi ngoài scope và formatter toàn repo sẽ rewrite file không liên quan, không chạy write-mode toàn repo; dùng `npm run format:docs:check` để báo rõ file nào chưa format.
12. Chạy lint docs khi repo có script, ưu tiên `npm run lint:docs` hoặc script hẹp hơn như `npm run lint:docs:markdown` và `npm run lint:docs:html`. Chạy `git diff --check` cho docs đã sửa. Nếu repo có doc validation khác, chạy nó trừ khi user yêu cầu không chạy.

## Sync State

Luôn duy trì sync state trong `./docs/_sync.md` khi cây docs tồn tại hoặc được tạo. File này là nguồn sự thật cho điểm sync trước đó và là nơi ghi điểm sync mới sau khi cập nhật. `docs/_sync.md` nên ngắn và chỉ mô tả trạng thái hiện tại; các trường:

- **Synced commit**: commit hoặc `HEAD` mà docs hiện phản ánh. Đây là mốc để lần sau chạy `git log` và `git diff`.
- **Synced at**: timestamp ISO-8601 nếu hữu ích, timezone rõ ràng. Bỏ qua nếu repo không muốn timestamp.
- **Scope**: phạm vi docs/code đã kiểm tra hoặc cập nhật trong lần sync hiện tại, ví dụ `preview UI`, `agent presets`, `docs tree`.
- **Status**: `synced` khi docs phản ánh target commit; `partially-synced` khi còn known-unsynced có chủ ý; `unsynced` khi mới bootstrap hoặc chưa thể xác minh.
- **Known unsynced**: note ngắn về phần chưa sync, có path/area cụ thể. Ghi `Không có` nếu không còn khu vực lệch đã biết.

Cấu trúc gợi ý:

```markdown
# Docs Sync State

## Meta

- Synced commit: `<commit-sha-or-HEAD>`
- Synced at: `YYYY-MM-DDTHH:MM:SSZ`
- Scope: [docs/code area đã kiểm tra]
- Status: synced | partially-synced | unsynced
- Known unsynced: Không có | [note ngắn]
```

Khi cập nhật docs, dùng previous synced commit từ `docs/_sync.md` để inspect `git log` và `git diff` đến target commit. Nếu range có nhiều commit, duyệt chúng theo thứ tự thời gian từ `<synced-commit>+1` đến `<target-commit>` để hiểu intermediate renames, removals và semantic shifts. Chỉ dùng history đó làm input để tổng hợp. Không copy commit list, diff timeline, incremental notes, legacy notes, history, migration logs hay changelogs vào docs.

Không đưa changelogs, bảng commit-history, migration history, incremental sync logs, legacy sections hay narrative "what changed over time" vào `docs/_sync.md` hay docs khác trừ khi user yêu cầu rõ changelog.

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

## Mẫu Requirements Doc

Dùng cấu trúc này cho `docs/features/<feature>/requirements.md` hoặc `docs/modules/<module>/requirements.md`:

```markdown
---
title: "[Feature/Module] Requirements"
description: "Critical requirements that must always be followed for [feature/module]."
type: feature | module
status: active
tags: ["requirements", "[domain]"]
related:
  - "./overview.md"
---

# [Feature/Module] Requirements

## Meta

- Trạng thái: active
- Phạm vi: [feature/module boundary]
- Links: [Overview](./overview.md)

## Critical Requirements

### REQ-1: [Requirement]

- Acceptance criteria: [observable outcome]
- Applies to: [workflow/API/module area]
- Failure mode: [what must not happen]
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
