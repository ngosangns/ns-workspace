---
name: update-docs
description: Cập nhật knowledge base trong `./docs` cho khớp codebase, đảm bảo có phiên bản cho BA (business) và phiên bản cho developer trong các thư mục riêng biệt. Trigger: cập nhật tài liệu, sync docs, làm mới spec, ghi lại implementation, cập nhật requirements.
---

# Cập Nhật Tài Liệu

Dùng skill này để cập nhật knowledge base sau nghiên cứu hoặc implementation. Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` trước, rồi làm theo kiến trúc docs bên dưới trừ khi repo quy định khác.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Nguyên Tắc

- **Chỉ mô tả trạng thái hiện tại.** Không lưu history, changelog, version snapshot trong docs.
- **Root docs cố định:** Mọi tài liệu nằm trong `./docs` tính từ project root. Không ghi ra ngoài.
- **OKF-first:** Docs theo Open Knowledge Format — markdown + YAML frontmatter. Mọi doc mới hoặc doc đang sửa metadata **bắt buộc** có frontmatter với `type` (xem `_shared/templates/frontmatter-schema.md`). Không tạo mới block `## Meta`; chỉ giữ `## Meta` cũ để tương thích ngược.
- **Hai audience rõ ràng:** Mỗi tài liệu thuộc về `business/` (cho BA/stakeholder) hoặc `developer/` (cho developer). Các doc feature/module/spec ảnh hưởng đến cả hai bên phải có **cả hai phiên bản**, mỗi phiên bản viết theo góc nhìn audience tương ứng.
- **Cập nhật tinh gọn:** Sửa statement cũ tại chỗ khi behavior thay đổi. Không thêm correction bên cạnh nội dung stale.
- **Requirements theo feature/module:** Mỗi feature/module folder nên có `requirements.md` chứa critical requirements. Tạo/cập nhật khi user yêu cầu hoặc khi implementation thay đổi business rule, contract, invariant.

## Cấu Trúc Thư Mục

```text
docs/
├── README.md                 # Giải thích cấu trúc docs 2 audience
├── _index.md                 # Chỉ mục root, link đến business/_index.md và developer/_index.md
├── _sync.md                  # Sync state chung cho toàn bộ docs
├── business/                 # Tài liệu cho BA / business stakeholders
│   ├── README.md             # Giới thiệu docs business
│   ├── _index.md             # Chỉ mục business docs
│   ├── features/<feature>/   # Behavior đã shipped, requirements, acceptance criteria
│   ├── modules/<module>/     # Business view của module: contract, business rules
│   ├── specs/planning/       # Business specs, user stories, acceptance specs
│   ├── shared/               # Glossary, domain vocabulary, user personas
│   └── research/
└── developer/                # Tài liệu cho developer
    ├── README.md             # Giới thiệu docs developer
    ├── _index.md             # Chỉ mục developer docs
    ├── architecture/         # Overview, decisions/, patterns/
    ├── modules/<module>/     # Technical view: boundary, API, dependencies
    ├── features/<feature>/   # Technical implementation notes cho feature
    ├── specs/planning/       # Technical design specs, plans
    ├── development/conventions/
    ├── research/
    ├── learnings/
    └── compliance/
```

## Phân Loại Audience

| Loại nội dung                                      | Audience    | Ví dụ vị trí                                                                                     |
| -------------------------------------------------- | ----------- | ------------------------------------------------------------------------------------------------ |
| User workflow, acceptance criteria, business rules | `business`  | `docs/business/features/<feature>/requirements.md`                                               |
| Technical implementation, API, module boundary     | `developer` | `docs/developer/modules/<module>/overview.md`                                                    |
| Feature ảnh hưởng cả business và dev               | Cả hai      | `docs/business/features/<feature>/overview.md` + `docs/developer/features/<feature>/overview.md` |
| Architecture, patterns, dev conventions            | `developer` | `docs/developer/architecture/...`                                                                |
| Domain glossary, user personas                     | `business`  | `docs/business/shared/...`                                                                       |

Góc nhìn viết:

- **Business:** tập trung vào _ngườidùng làm gì_, _kết quả mong đợi_, _ràng buộc nghiệp vụ_, _acceptance criteria_. Tránh implementation detail, tên class/function, công nghệ.
- **Developer:** tập trung vào _kiến trúc_, _contract_, _API_, _dependency_, _failure mode kỹ thuật_, _cách triển khai_. Giữ ngắn gọn về motivation nghiệp vụ nhưng link đến business doc khi cần.

## Quy Tắc Viết Docs

### Nội Dung

- Viết như tài liệu vận hành hiện tại. Tránh "trước đây", "vừa thêm", "sẽ đổi".
- Mỗi doc có phạm vi và audience rõ: business = góc nhìn nghiệp vụ; developer = góc nhìn kỹ thuật.
- Ưu tiên câu ngắn, heading rõ. Xóa mô tả lặp, wrapper văn xuôi, câu chung chung.
- Ghi rõ constraint, assumption, failure mode, security/compliance rule nếu ảnh hưởng vận hành.
- Khi một topic có cả hai phiên bản, đảm bảo nội dung không copy-paste nguyên xi mà diễn đạt phù hợp audience. Giữ fact đồng nhất giữa hai phiên bản (tên feature, boundary, contract).

### Liên Kết

- Dùng link tương đối thật: `[Tên](../path/doc.md)`. Không tạo link placeholder.
- Ưu tiên dạng OKF bundle-relative `[Tên](/business/features/preview.md)` hoặc `[Tên](/developer/modules/preview.md)` (bắt đầu bằng `/`, tính từ docs root) cho cross-link giữa các doc — ổn định khi di chuyển file và được viewer/export hiểu để điều hướng nội bộ.
- Link hai chiều giữa business và developer khi chúng mô tả cùng một concept: business doc link đến developer doc ("Chi tiết kỹ thuật") và ngược lại ("Bối cảnh nghiệp vụ").
- Source/related references nằm trong metadata frontmatter (`resource`, `tags`) hoặc link trong body, không tạo section tham khảo riêng. Citation nguồn ngoài đặt dưới heading `# Citations` ở cuối doc khi cần.
- Giữ quan hệ hai chiều khi cần: kiểm tra doc đích có cần link ngược.

### Markdown

- **Frontmatter YAML là bắt buộc** cho doc mới, theo `_shared/templates/frontmatter-schema.md`: `type` (required) + `title`/`description`/`tags`/`timestamp` (+ `resource` khi mô tả asset có URI). Nên thêm `audience: business | developer` để lọc/viewer dễ phân biệt. Không tạo mới `## Meta`.
- Reserved filenames theo OKF: `index.md` (directory listing, không frontmatter trừ `okf_version` ở root), `log.md` (history — không dùng trong repo này vì docs chỉ mô tả hiện trạng).
- Mermaid/diagram chỉ khi giúp giải thích nhanh hơn văn bản.

### HTML

- Output là fragment, không full document shell.
- Dùng custom semantic tags (`doc-meta`, `doc-title`, `doc-description`) khi repo hỗ trợ.
- Không inline script/style, event handler, framework attributes, id tự sinh, class rỗng.

### Chất Lượng Diff

- Giữ diff nhỏ, có chủ đích. Không rewrite hàng loạt chỉ đổi style.
- Sau khi edit, đọc lại diff bắt link sai, stale statement, duplicate section.
- Chạy `npm run lint:doc:fix` khi repo có script (lint fix đã bao gồm format).
- Khi repo có CLI `ns-workspace`, chạy `kb validate` để xác nhận docs còn OKF-conformant (mọi doc có frontmatter + `type`).

## Quy Trình

1. `git status --short` + `rg --files ./docs` để định vị docs hiện có.
2. Đọc `docs/_sync.md` để lấy synced commit. Nếu không có, xem docs là chưa sync.
3. So sánh sync-state commit với target commit bằng `git log --oneline`, `git diff --name-status`, `git diff`.
4. Nếu nhiều commit, duyệt theo thứ tự thờigian qua `git rev-list --reverse` để hiểu final behavior.
5. Đọc docs/specs bị chạm (cả `business/` và `developer/`) + code path đã đổi vừa đủ hiểu final behavior. Dùng `lsp-code-graph` khi cần code graph context.
6. Quyết định tập docs nhỏ nhất cần cập nhật. Tránh rewrite rộng.
7. Cập nhật docs mô tả thiết kế hiện tại. Xóa statement stale.
8. Nếu một feature/module/spec thay đổi và ảnh hưởng cả hai audience, cập nhật **cả hai** phiên bản: `docs/business/...` và `docs/developer/...`.
9. Tạo/cập nhật `requirements.md` trong cả `business/` và `developer/` khi scope là feature/module và có requirements critical (mỗi phiên bản ghi requirements phù hợp audience).
10. Cập nhật `_index.md` ở root và trong `business/`/`developer/` khi thêm/move/xóa docs. Duy trì link hai chiều.
11. Cập nhật `docs/_sync.md` như snapshot sync cuối cùng. Xem `_shared/templates/sync-state-template.md`.
12. Chạy formatter + lint docs khi repo có script.

## Templates

Khi tạo docs mới, dùng templates trong `_shared/templates/`:

- `spec-template.md` cho specs (business + developer)
- `requirements-template.md` cho requirements.md (business + developer)
- `module-template.md` cho module docs (business + developer)
- `frontmatter-schema.md` cho metadata, bao gồm `audience`
- `sync-state-template.md` cho \_sync.md

## Phản Hồi Cuối

Báo cáo docs đã thay đổi, kết quả sync state và validation đã chạy. Giữ cô đọng. Nêu rõ những doc nào được cập nhật ở `business/`, những doc nào ở `developer/`, và những doc nào có cả hai phiên bản.
