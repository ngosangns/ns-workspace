---
name: init
description: Khởi tạo knowledge base cho repo mới: quét codebase, lập aspect inventory, rồi cập nhật docs/specs với 2 phiên bản business và developer trong các thư mục riêng biệt.
---

# Init Knowledge Base

Dùng khi user muốn khởi tạo hoặc làm mới knowledge base từ gần như không biết gì.

## Kết Quả

- Aspect inventory markdown trong `docs/business/research/` hoặc `docs/developer/research/` tùy audience.
- Docs được tạo/cập nhật từ inventory:
  - `docs/README.md`, `docs/_index.md`, `docs/_sync.md`
  - `docs/business/README.md`, `docs/business/_index.md`
  - `docs/developer/README.md`, `docs/developer/_index.md`
  - `docs/business/features/`, `docs/business/modules/`, `docs/business/shared/`
  - `docs/developer/architecture/`, `docs/developer/modules/`, `docs/developer/features/`, `docs/developer/development/conventions/`
- Không sửa source code trừ khi user yêu cầu rõ.

## Workflow

1. **Search (`//rp`):**
   - `read-search-docs`: đọc `AGENTS.md`, README, docs index/sync, specs hiện có.
   - `rg --files` quét entrypoints, commands/API, packages, config, data model, tests, scripts, integration boundaries.
   - `lsp-code-graph` khi cần symbol/caller/callee context.
   - `plan` tạo aspect inventory markdown.

2. **Aspect inventory:** Mỗi aspect cần: tên, lý do quan trọng, source paths, docs hiện có, khoảng trống, doc target, priority, **audience** (`business` | `developer` | `both`). Bao phủ tối thiểu:
   - **Business:** domain purpose, user workflows, business rules, acceptance criteria, vocabulary, user personas.
   - **Developer:** module boundaries, public API, data models, integrations, invariants, failure modes, security, dev workflow, generated artifacts, architecture decisions, conventions.
   - **Both:** features/modules có cả góc nhìn nghiệp vụ và kỹ thuật.

3. **Update docs (`//ru`):**
   - `read-search-docs` đọc lại inventory.
   - `update-docs` cập nhật docs nhỏ nhất đủ mô tả hiện tại, theo đúng phân chia `business/` và `developer/`.
   - Với mỗi feature/module quan trọng (P0/P1), tạo **cả hai** phiên bản nếu ảnh hưởng cả hai audience.
   - Tạo link tương đối thật, cập nhật `docs/_index.md`, `docs/business/_index.md`, `docs/developer/_index.md`, và `docs/_sync.md`.

4. **Review:** Đối chiếu docs với inventory, đảm bảo aspect P0/P1 có doc target đúng audience. Chạy validation docs hoặc `git diff --check`.

## Nguyên Tắc

- Docs-first. Source code chỉ đọc để hiểu, không sửa.
- OKF-first: mọi doc tạo mới theo Open Knowledge Format — markdown + YAML frontmatter với `type` bắt buộc, nên có `audience` (xem `_shared/templates/frontmatter-schema.md`). Cross-link dùng dạng bundle-relative `/business/...` hoặc `/developer/...`.
- Audience-first: mỗi doc target phải rõ thuộc `business/` hay `developer/`; feature/module cross-cutting thì có cả hai.
- Docs stale → coi là bối cảnh, không phải chân lý.
- Không tạo placeholder rỗng. Mỗi doc phải giúp ngườimới hiểu aspect cụ thể.
