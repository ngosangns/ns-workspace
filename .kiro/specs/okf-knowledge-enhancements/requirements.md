# Requirements Document

## Glossary

- **OKF (Open Knowledge Format)**: định dạng biểu diễn knowledge bằng markdown + YAML frontmatter của dự án knowledge-catalog; ở đây chỉ mượn convention, không copy nguyên spec.
- **Knowledge core**: tập hàm đọc/dựng dữ liệu chung trong `internal/preview` (`scanSpecProject`, `specGraph`, `buildPreviewSearchResponse`).
- **Frontmatter**: block YAML đặt đầu file markdown, phân tách bằng `---`.
- **`## Meta`**: section metadata dạng prose hiện đang dùng trong docs (tương thích ngược).
- **Static export**: file HTML self-contained nhúng toàn bộ knowledge base, render client-side, không cần backend.
- **Hard caps**: giới hạn code-enforced cho enrichment (`max_pages`, `allowed_hosts`, `max_depth`, timeout).
- **Docs root**: thư mục docs của project (mặc định `docs/`), phạm vi cho mọi thao tác đọc/ghi.
- **MCP (Model Context Protocol)**: giao thức tool cho agent; ở đây là server stdio local expose docs.
- **Permissive consumer**: nguyên tắc parser bỏ qua key lạ / type lạ / link gãy mà không lỗi.

## Introduction

Tài liệu này mô tả yêu cầu cho việc áp dụng một số pattern từ dự án [GoogleCloudPlatform/knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) (Apache 2.0, định dạng OKF — Open Knowledge Format) vào Go CLI `ns-workspace`. Mục tiêu là làm knowledge base trong `docs/` dễ chia sẻ, dễ query/index, và để agent đọc/sửa trực tiếp — **không** kéo theo bất kỳ tầng cloud nào (Dataplex, BigQuery, GCS). Mọi thứ chạy local-only.

Các requirement dưới đây được **dẫn xuất từ** [design.md](./design.md) (workflow design-first) và bao gồm bốn feature độc lập, phased, ship riêng được:

1. **Static HTML export** (ưu tiên cao nhất): dump docs + graph thành một file HTML self-contained, offline.
2. **Chuẩn hoá metadata docs theo tinh thần OKF**: thêm YAML frontmatter chuẩn tối thiểu, giữ tương thích ngược với `## Meta`.
3. **Enrichment agent với hard caps**: task type `enrich-docs` trong `harness` với guardrails code-enforced.
4. **MCP tools cho knowledge base**: expose `docs/` qua MCP server (list/lookup/search/modify).

Ký hiệu tham chiếu: mỗi acceptance criterion liên kết tới Correctness Property tương ứng trong design dưới dạng `(Design: Property N)`.

## Requirements

### Requirement 1: Static HTML Export — Sinh File Tĩnh Self-Contained

**User Story:** Là người dùng `ns-workspace`, tôi muốn xuất toàn bộ knowledge base ra một file HTML duy nhất, để tôi có thể chia sẻ và xem offline mà không cần chạy server.

#### Acceptance Criteria

1. WHEN người dùng chạy `export --out <path>` với một project hợp lệ THEN hệ thống SHALL ghi một file HTML tại `<path>`.
2. WHEN không truyền `--out` THEN hệ thống SHALL ghi ra tên file mặc định (`ns-workspace-kb.html`) trong current working directory.
3. WHEN file HTML được sinh với `--inline-assets=true` (mặc định) THEN file SHALL mở được bằng `file://` và render đầy đủ docs + graph mà không cần bất kỳ request mạng nào. (Design: Property 1)
4. WHEN `--inline-assets=false` THEN hệ thống SHALL tham chiếu thư viện render qua CDN thay vì nhúng inline.
5. WHEN project root không hợp lệ hoặc docs dir không tồn tại THEN hệ thống SHALL trả lỗi rõ ràng và KHÔNG ghi file.

### Requirement 2: Static HTML Export — Tính Toàn Vẹn Nội Dung

**User Story:** Là người dùng, tôi muốn file export chứa đúng và đủ docs + graph của project, để bản chia sẻ phản ánh trung thực knowledge base.

#### Acceptance Criteria

1. WHEN export một project THEN mọi `specDocument` trong project SHALL xuất hiện trong bundle của file HTML. (Design: Property 2)
2. WHEN export với cờ mặc định (graph bật) THEN graph nhúng trong file SHALL bằng đúng `project.Graph`. (Design: Property 3)
3. WHEN export với `--no-graph` THEN graph trong file SHALL rỗng và phần docs vẫn đầy đủ. (Design: Property 3)
4. WHEN một doc render lỗi THEN hệ thống SHALL chèn nội dung placeholder cho doc đó, ghi warning, và vẫn export các doc còn lại (fail-open). (Design: Property 4)
5. WHERE dữ liệu được nhúng vào file THE hệ thống SHALL chỉ nhúng docs + graph + metadata của project, KHÔNG nhúng bất kỳ dữ liệu nào ngoài project. (Design: Property 5)

### Requirement 3: Chuẩn Hoá Metadata Docs — Hỗ Trợ Frontmatter OKF

**User Story:** Là người viết docs, tôi muốn khai báo metadata bằng YAML frontmatter chuẩn (`type`, `description`, `tags`, `timestamp`), để metadata dễ query/filter/index và tương thích với Obsidian/Notion/MkDocs.

#### Acceptance Criteria

1. WHEN một doc bắt đầu bằng block YAML frontmatter (`---`) THEN hệ thống SHALL parse các key `type`, `description`, `tags`, `timestamp` cùng các key tương thích (`status`, `version`, `compliance`, `priority`, `links`).
2. WHEN `tags` được khai báo dạng string đơn hoặc dạng array THEN hệ thống SHALL normalize về `[]string`. (Design: Property 10)
3. WHEN frontmatter chứa key lạ hoặc `type` không nằm trong tập biết THEN hệ thống SHALL chấp nhận và KHÔNG báo lỗi parse (permissive consumer). (Design: Property 8)
4. WHEN một `links`/reference trỏ tới doc không tồn tại THEN hệ thống SHALL bỏ qua edge đó và KHÔNG crash.

### Requirement 4: Chuẩn Hoá Metadata Docs — Tương Thích Ngược Và Fail-Open

**User Story:** Là người bảo trì repo, tôi muốn các doc hiện có dùng `## Meta` vẫn hoạt động nguyên vẹn, để việc thêm frontmatter là tùy chọn và migration chi phí thấp.

#### Acceptance Criteria

1. WHEN một doc chỉ có section `## Meta` (không có frontmatter) THEN hệ thống SHALL parse ra metadata y hệt hành vi trước thay đổi. (Design: Property 6)
2. WHEN một doc có cả frontmatter và `## Meta` THEN với key trùng, giá trị frontmatter SHALL thắng; các field còn trống SHALL được điền từ `## Meta`. (Design: Property 7)
3. IF frontmatter YAML lỗi cú pháp THEN hệ thống SHALL fallback sang parse `## Meta`, ghi warning, và KHÔNG panic. (Design: Property 9)
4. WHEN không có doc nào dùng frontmatter THEN toàn bộ behavior của preview/search SHALL không đổi.

### Requirement 5: Enrichment Agent — Task `enrich-docs` Với Hard Caps

**User Story:** Là người dùng harness, tôi muốn một task tự enrich docs từ seed URL với giới hạn cứng, để LLM có thể bổ sung kiến thức từ nguồn ngoài mà không crawl tràn.

#### Acceptance Criteria

1. WHEN một task có `type: enrich-docs` được chạy qua `harness run` THEN hệ thống SHALL kích hoạt nhánh enrichment thay vì task dev generic.
2. WHILE đang fetch trong một lần chạy THE số trang fetch SHALL không vượt quá `max_pages` (đếm code-enforced). (Design: Property 11)
3. WHEN một URL có host ngoài `allowed_hosts` (∪ host của seeds) THEN hệ thống SHALL từ chối fetch URL đó. (Design: Property 12)
4. WHEN một redirect dẫn tới host ngoài allowlist THEN hệ thống SHALL chặn redirect đó. (Design: Property 12)
5. WHEN một fetch lỗi hoặc timeout THEN hệ thống SHALL ghi warning và tiếp tục loop (fail-open), KHÔNG dừng toàn bộ task. (Design: Property 14)
6. WHERE guardrails được áp dụng THE giới hạn (host allowlist, page budget, depth, timeout) SHALL được enforce bằng code, KHÔNG dựa vào LLM tự giới hạn.

### Requirement 6: Enrichment Agent — Ghi Kết Quả An Toàn

**User Story:** Là người bảo trì repo, tôi muốn enrichment chỉ ghi file bên trong docs root, để không có side-effect ngoài ý muốn lên phần còn lại của repo.

#### Acceptance Criteria

1. WHEN target mode là `references` THEN hệ thống SHALL tạo file mới trong `references_dir` (bên trong docs root) với frontmatter chuẩn (`type: reference`).
2. WHEN target mode là `enrich` THEN hệ thống SHALL chỉ sửa các doc đã tồn tại bên trong docs root.
3. WHERE bất kỳ file enrichment nào được ghi THE đường dẫn SHALL nằm trong docs root. (Design: Property 13)
4. WHEN phase verify chạy acceptance command (vd markdownlint) THEN hệ thống SHALL báo pass/fail theo kết quả command.

### Requirement 7: MCP Tools — Đọc Knowledge Base

**User Story:** Là một AI agent, tôi muốn liệt kê, tra cứu và search docs qua MCP, để thao tác knowledge base trực tiếp mà không cần preview server UI.

#### Acceptance Criteria

1. WHEN người dùng chạy `mcp --project <path>` THEN hệ thống SHALL khởi động một MCP server giao tiếp JSON-RPC qua stdio (local-only, không bind network).
2. WHEN client gọi `tools/list` THEN server SHALL trả descriptor của `list_docs`, `lookup_doc`, `search_docs`, `modify_doc`.
3. WHEN client gọi `list_docs` THEN server SHALL chỉ trả docs trong docs root (id, title, type, tags, path), có hỗ trợ filter theo `type`/`tag`. (Design: Property 16)
4. WHEN client gọi `lookup_doc` với `id` không tồn tại THEN server SHALL trả lỗi rõ ràng và KHÔNG panic. (Design: Property 16)
5. WHEN client gọi `search_docs` với một query THEN kết quả SHALL khớp với kết quả của `buildPreviewSearchResponse` cho cùng query (single contract). (Design: Property 17)
6. WHEN tool args không hợp lệ hoặc handler panic THEN server SHALL trả JSON-RPC error response và tiếp tục phục vụ (không crash server).

### Requirement 8: MCP Tools — Ghi An Toàn (Chống Path Traversal)

**User Story:** Là người bảo trì repo, tôi muốn tool ghi của MCP chỉ tác động trong docs root, để agent không thể ghi đè file ngoài phạm vi knowledge base.

#### Acceptance Criteria

1. WHEN client gọi `modify_doc` với `id` hợp lệ nằm trong docs root THEN server SHALL tạo/sửa file đó và trả kết quả thành công kèm path.
2. IF `id` resolve ra đường dẫn thoát khỏi docs root (path traversal) THEN server SHALL từ chối thao tác và trả lỗi. (Design: Property 15)
3. WHEN file đích nằm trong thư mục chưa tồn tại (vẫn trong docs root) THEN server SHALL tạo thư mục cha trước khi ghi.

### Requirement 9: Ràng Buộc Chung — Local-Only Và Phased

**User Story:** Là chủ dự án, tôi muốn toàn bộ tính năng chạy local và mỗi feature ship độc lập, để giữ dự án nhẹ và triển khai an toàn theo từng giai đoạn.

#### Acceptance Criteria

1. WHERE bất kỳ feature nào được triển khai THE hệ thống SHALL KHÔNG thêm dependency hay lời gọi tới Google Cloud / Dataplex / BigQuery.
2. WHEN một feature được build THEN nó SHALL không phụ thuộc cứng vào ba feature còn lại (ship độc lập được).
3. WHERE tái dùng logic đọc/search THE các feature SHALL đi qua cùng "knowledge core" (`scanSpecProject`, `specGraph`, `buildPreviewSearchResponse`) thay vì nhân đôi logic.
4. WHEN Feature 1 (export) được triển khai trước các feature khác THEN nó SHALL hoạt động mà không cần Feature 2/3/4.
