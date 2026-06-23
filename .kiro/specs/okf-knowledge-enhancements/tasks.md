# Implementation Plan: OKF Knowledge Enhancements

## Overview

Kế hoạch chia theo bốn feature độc lập, phased, ship riêng được, theo đúng thứ tự rollout trong design:

- **Phase 1 (ưu tiên cao nhất)**: Feature 1 — Static HTML export (`internal/preview/export.go` + `export_ui/`).
- **Phase 2**: Feature 2 — Frontmatter metadata OKF (`internal/preview/spec_project.go`).
- **Phase 3**: Feature 4 — MCP tools cho knowledge base (`internal/kbmcp/`).
- **Phase 4**: Feature 3 — Enrichment agent với hard caps (`internal/harness/enrich.go` + `task.go` + `loop.go`).

Nguyên tắc xuyên suốt: tái dùng "knowledge core" (`scanSpecProject`, `specGraph`, `buildPreviewSearchResponse`) thay vì nhân đôi logic; mọi thứ local-only, không thêm dependency cloud. Ngôn ngữ triển khai: **Go** (theo design).

Mỗi phase kết thúc bằng một checkpoint và có thể build/test/ship độc lập. Các sub-task gắn hậu tố `*` là test tùy chọn, có thể bỏ qua khi cần MVP nhanh.

## Tasks

- [x] 1. Phase 1 — Static HTML export (Feature 1)
  - [x] 1.1 Tạo bộ asset export UI tĩnh và vendor embed
    - Tạo thư mục `internal/preview/export_ui/` với `export.html.tmpl`, `export.js`, `export.css`
    - Thêm `export_ui/vendor/` chứa Cytoscape.js + marked để inline offline; thêm `//go:embed export_ui` trong `export.go`
    - `export.js` (vanilla) hydrate sidebar/doc view/graph từ `window.__NS_KB__`, routing bằng `location.hash`; không phụ thuộc pipeline Vite của `preview_ui_src/`
    - _Requirements: 1.3, 1.4_

  - [x] 1.2 Định nghĩa data model bundle và renderer doc trong `export.go`
    - Khai báo `exportBundle`, `exportProjectMeta`, `exportDocument` (graph tái dùng nguyên `specGraph`)
    - Viết `renderDocumentHTML(doc specDocument) (string, error)`: markdown render bằng `goldmark` (đã có), HTML doc qua sanitizer `golang.org/x/net/html` hiện có; lỗi render một doc trả placeholder thay vì panic
    - Viết `collectDocMeta(doc)` map metadata (status/version/tags...) cho viewer tĩnh
    - _Requirements: 2.1, 2.4_

  - [x] 1.3 Implement `exportStaticBundle` và `injectBundle`
    - `exportStaticBundle(project specProject, opt exportOptions) ([]byte, error)`: build bundle từ `project.Documents`, gắn `project.Graph` khi `includeGraph`, ngược lại để rỗng
    - `injectBundle(tmpl, bundle, opt)`: marshal JSON blob vào `window.__NS_KB__`, inline asset từ vendor embed khi `inlineAssets=true`, tham chiếu CDN khi `false`
    - Chỉ nhúng docs + graph + meta của project (không dữ liệu ngoài project)
    - _Requirements: 2.1, 2.2, 2.3, 2.5_

  - [x] 1.4 Implement `RunExport` + parse flags
    - `RunExport(args []string) error`: parse `--project`, `--docs`, `--out` (mặc định `ns-workspace-kb.html`), `--no-graph`, `--inline-assets` (mặc định true), `--open`
    - Tái dùng `normalizePreviewProjectRoot`, `scanSpecProject`, `docsRoot`, `openURL`
    - Validate project root / docs dir: lỗi rõ ràng và KHÔNG ghi file khi không hợp lệ; `os.WriteFile(out, html, 0o644)` khi hợp lệ
    - _Requirements: 1.1, 1.2, 1.4, 1.5_

  - [x] 1.5 Đăng ký command `export` trong `main.go`
    - Thêm `case "export": return preview.RunExport(args[1:])`
    - _Requirements: 1.1_

  - [x]* 1.6 Viết unit test cho export
    - Fixture project nhiều doc + graph; assert mọi doc có mặt trong bundle, graph đúng/rỗng theo flag, HTML chứa `window.__NS_KB__`
    - **Property 2: Export không mất doc** — Validates: Requirements 2.1
    - **Property 3: Export graph theo flag** — Validates: Requirements 2.2, 2.3
    - **Property 4: Export render permissive** — Validates: Requirements 2.4
    - **Property 5: Export không rò dữ liệu ngoài project** — Validates: Requirements 2.5

  - [x]* 1.7 Viết integration test cho export self-contained
    - Chạy `exportStaticBundle` với `inlineAssets=true`, assert HTML không chứa reference `http://`/`https://` ra ngoài (mở được bằng `file://`, không request mạng)
    - **Property 1: Export self-contained offline** — Validates: Requirements 1.3

- [x] 2. Checkpoint Phase 1
  - Ensure all tests pass, ask the user if questions arise.
  - Chạy `go test ./internal/preview` và `go build ./...`

- [x] 3. Phase 2 — Frontmatter metadata OKF (Feature 2)
  - [x] 3.1 Mở rộng struct metadata trong `spec_project.go`
    - Thêm `Type string`, `Tags []string`, `Timestamp string` vào `specDocument` và `moduleMeta`
    - Giữ nguyên các field hiện có; không đổi behavior khi field mới rỗng
    - _Requirements: 3.1, 4.4_

  - [x] 3.2 Implement `parseFrontmatter` + normalize tags
    - `parseFrontmatter(raw string) (meta moduleMeta, ok bool, err error)`: tái dùng `metadataBlock` để lấy block giữa `---`, `yaml.Unmarshal` (`gopkg.in/yaml.v3`, đã có) vào struct trung gian
    - Parse `type`, `description`, `tags`, `timestamp` + key tương thích (`status`, `version`, `compliance`, `priority`, `links`); key lạ → bỏ qua, không lỗi
    - Normalize `tags` từ string đơn HOẶC array về `[]string`
    - _Requirements: 3.1, 3.2, 3.3_

  - [x] 3.3 Sửa `parseDocumentMeta` merge frontmatter + `## Meta`
    - Thử `parseFrontmatter` trước (ưu tiên cao nhất); fallback parse `## Meta` prose để điền field còn trống (`fillEmpty`)
    - Frontmatter YAML lỗi cú pháp → fallback `## Meta`, ghi warning, KHÔNG panic
    - `links`/reference trỏ doc không tồn tại → giữ hành vi `resolveSpecReference` (bỏ edge, không crash)
    - _Requirements: 3.4, 4.1, 4.2, 4.3_

  - [x]* 3.4 Viết unit test cho frontmatter (table-driven)
    - Case: chỉ `## Meta`, chỉ frontmatter, cả hai, frontmatter hỏng cú pháp, `tags` string vs array, `type` lạ
    - **Property 6: Frontmatter tương thích ngược** — Validates: Requirements 4.1
    - **Property 7: Frontmatter ưu tiên** — Validates: Requirements 4.2
    - **Property 8: Permissive consumer** — Validates: Requirements 3.3
    - **Property 9: Frontmatter fail-open** — Validates: Requirements 4.3
    - **Property 10: Tags normalize** — Validates: Requirements 3.2

- [x] 4. Checkpoint Phase 2
  - Ensure all tests pass, ask the user if questions arise.
  - Chạy `go test ./internal/preview` xác nhận preview/search không đổi behavior khi không có frontmatter.

- [x] 5. Phase 3 — MCP tools cho knowledge base (Feature 4)
  - [x] 5.1 Thêm façade public `preview.OpenKnowledge`
    - Tạo `internal/preview/knowledge.go`: API mỏng `OpenKnowledge(projectRoot, docsDir)` trả snapshot (`scanSpecProject`) + search runner (`buildPreviewSearchResponse`) để `kbmcp` không phụ thuộc symbol private
    - _Requirements: 7.5, 9.3_

  - [x] 5.2 Implement MCP stdio server `internal/kbmcp/server.go`
    - `Run(args []string) error` parse `--project`, `--docs`; `NewServer`, `Serve(ctx)` đọc/ghi JSON-RPC 2.0 qua stdin/stdout (local-only, không bind network)
    - `dispatch` xử lý `tools/list` và `tools/call`; recover panic trong handler thành JSON-RPC error; method/tool lạ trả error nhưng KHÔNG crash server
    - _Requirements: 7.1, 7.2, 7.6_

  - [x] 5.3 Implement tool handler đọc trong `internal/kbmcp/tools.go`
    - `handleListDocs` (filter `type`/`tag`, chỉ docs trong docs root), `handleLookupDoc` (id không tồn tại → lỗi rõ ràng, không panic), `handleSearchDocs` gọi `preview.OpenKnowledge`/`buildPreviewSearchResponse` (single contract)
    - Khai báo tool descriptor (`list_docs`, `lookup_doc`, `search_docs`, `modify_doc`) theo schema design
    - _Requirements: 7.2, 7.3, 7.4, 7.5_

  - [x] 5.4 Implement `resolveDocPath` + `handleModifyDoc` (ghi an toàn)
    - `resolveDocPath(id)` clean + join với docs root, từ chối path thoát docs root (path traversal)
    - `handleModifyDoc`: `MkdirAll` thư mục cha (trong docs root) rồi `os.WriteFile`; trả `{ok, path}`
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 5.5 Đăng ký command `mcp` trong `main.go`
    - Thêm `case "mcp": return kbmcp.Run(args[1:])`
    - _Requirements: 7.1_

  - [x]* 5.6 Viết unit test cho MCP handlers
    - Fixture docs; test `resolveDocPath` các case path traversal (`../`, absolute, symlink-style), từng tool handler, contract `search_docs` khớp `buildPreviewSearchResponse`, `lookup_doc` id không tồn tại trả lỗi, args không hợp lệ → rpc error không crash
    - **Property 15: MCP chống path traversal** — Validates: Requirements 8.2
    - **Property 16: MCP phạm vi docs root** — Validates: Requirements 7.3, 7.4
    - **Property 17: MCP search single contract** — Validates: Requirements 7.5

  - [x]* 5.7 Viết property test cho `resolveDocPath`
    - Dùng `testing/quick`: random `id` → `resolveDocPath` không bao giờ trả path ngoài docs root
    - **Property 15: MCP chống path traversal** — Validates: Requirements 8.2

- [x] 6. Checkpoint Phase 3
  - Ensure all tests pass, ask the user if questions arise.
  - Chạy `go test ./internal/kbmcp ./internal/preview`; test thủ công bằng integration test gửi JSON-RPC `tools/list`/`tools/call` qua stdin.

- [x] 7. Phase 4 — Enrichment agent với hard caps (Feature 3)
  - [x] 7.1 Thêm `EnrichConfig` vào `task.go`
    - Khai báo `EnrichConfig`, `EnrichSeed`, `EnrichCaps`, `EnrichTarget` (json+yaml tag) và field `Enrich EnrichConfig` trong `Task`
    - _Requirements: 5.1_

  - [x] 7.2 Implement `enrichGuard` + `fetchTool` (hard caps code-enforced) trong `enrich.go`
    - `newEnrichGuard(caps)` build host allowlist từ `allowed_hosts` ∪ seed hosts; `allow(rawURL) (ok, reason)` check host + page budget; đếm `fetched` dừng tại `max_pages`
    - `fetchTool(ctx, rawURL, timeout)` set `http.Client{Timeout}` + `CheckRedirect` chặn redirect ra host ngoài allowlist; strip HTML trả text
    - _Requirements: 5.2, 5.3, 5.4, 5.6_

  - [x] 7.3 Implement `runEnrich` loop + prompt builders + ghi file an toàn
    - `runEnrich(ctx, task, state)`: plan (LLM đề xuất URL) → fetch qua guard (fail-open khi lỗi/timeout, ghi warning, tiếp tục loop) → execute (LLM tổng hợp) → ghi `references/<slug>.md` (frontmatter `type: reference`) hoặc patch doc đã tồn tại
    - `buildEnrichPlanPrompt`, `buildEnrichExecutePrompt`, `parseDocChanges`, `writeReferenceDoc`, `patchExistingDoc`; mọi side-effect ghi giới hạn trong docs root; chạy acceptance command verify pass/fail
    - _Requirements: 5.5, 6.1, 6.2, 6.3, 6.4_

  - [x] 7.4 Rẽ nhánh `task.Type == "enrich-docs"` trong `loop.go`
    - Trong `LoopController` (runPlan/runExecute), khi `task.Type == "enrich-docs"` gọi `runEnrich` thay vì task dev generic
    - _Requirements: 5.1_

  - [x]* 7.5 Viết unit test cho guard + fetchTool
    - `enrichGuard.allow` (host in/out allowlist, budget); `fetchTool` với `httptest` server (timeout, redirect ra host ngoài bị chặn); ghi file chỉ trong docs root; fetch lỗi không dừng loop
    - **Property 11: Hard cap số trang** — Validates: Requirements 5.2
    - **Property 12: Host allowlist** — Validates: Requirements 5.3, 5.4
    - **Property 13: Ghi giới hạn docs root** — Validates: Requirements 6.3
    - **Property 14: Fetch fail-open** — Validates: Requirements 5.5

  - [x]* 7.6 Viết property test cho guard
    - Dùng `testing/quick`: random danh sách URL/host → guard không bao giờ vượt `max_pages` hoặc cho qua host ngoài allowlist
    - **Property 11: Hard cap số trang** — Validates: Requirements 5.2
    - **Property 12: Host allowlist** — Validates: Requirements 5.3

- [x] 8. Checkpoint cuối — đảm bảo toàn bộ test pass
  - Ensure all tests pass, ask the user if questions arise.
  - Chạy `go test ./...` và `go build ./...` xác nhận bốn feature build độc lập, không thêm dependency cloud.

## Notes

- Task gắn `*` là test tùy chọn, có thể bỏ để MVP nhanh; task không có `*` là core, phải triển khai.
- Mỗi task tham chiếu requirement cụ thể để truy vết; property test tham chiếu Correctness Property trong design.
- Checkpoint sau mỗi phase đảm bảo từng feature ship độc lập (Requirements 9.2, 9.4).
- Mọi feature đi qua knowledge core (`scanSpecProject`/`specGraph`/`buildPreviewSearchResponse`) — không nhân đôi logic (Requirements 9.3).
- Không thêm dependency cloud (Google Cloud / Dataplex / BigQuery) ở bất kỳ task nào (Requirements 9.1).
- Hai điểm bảo mật trọng yếu: hard caps enrichment (max_pages/allowed_hosts/redirect — task 7.2, 7.5, 7.6) và path-traversal MCP (`resolveDocPath` — task 5.4, 5.6, 5.7).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "3.1", "5.1", "7.1"] },
    { "id": 1, "tasks": ["1.2", "3.2", "5.2", "7.2"] },
    { "id": 2, "tasks": ["1.3", "3.3", "5.3", "7.3"] },
    { "id": 3, "tasks": ["1.4", "3.4", "5.4", "7.4"] },
    { "id": 4, "tasks": ["1.5", "7.5"] },
    { "id": 5, "tasks": ["5.5", "1.6"] },
    { "id": 6, "tasks": ["5.6", "1.7", "7.6"] },
    { "id": 7, "tasks": ["5.7"] }
  ]
}
```
