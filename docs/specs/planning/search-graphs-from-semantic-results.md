# Search Graphs From Semantic Results

## Meta

- **Status**: implemented
- **Description**: Spec đã triển khai cho Search tab để Docs Graph và Code Graph được neo từ kết quả semantic search, sau đó mở rộng sâu và rộng theo graph context.
- **Compliance**: current-state
- **Links**: [Trang search preview](./add-preview-search-page.md), [Preview web](../../features/preview-web.md), [Module preview](../../modules/preview.md), [Kiến trúc tổng quan](../../architecture/overview.md)

## Bối Cảnh

Search tab trả bốn panel độc lập: Docs Semantic, Docs Graph, Code Semantic và Code Graph. Backend tính semantic results trước trong `buildPreviewSearchResponse`, rồi dùng các semantic hits đó làm anchor cho graph panels. Vì vậy graph results không chỉ match text trong node metadata mà còn mở rộng từ tài liệu hoặc file code semantic liên quan.

Semantic search tìm các tài liệu hoặc file code có liên quan trước; Docs Graph và Code Graph sau đó lấy các kết quả đó làm điểm bắt đầu, rồi đào sâu và rộng trong docs graph hoặc graphify graph để hiển thị context liên quan.

## Mục Tiêu

- Docs Graph được seed từ `DocsSemantic` results bằng `specId`, docs-relative `path`, `title` và các alias graph node có sẵn.
- Code Graph được seed từ `CodeSemantic` results bằng project-relative `path`, symbol/path match trong graphify nodes và fallback query match khi semantic không đủ anchor.
- Graph expansion trả về anchor nodes, neighbor nodes nhiều tầng, relation direction, confidence, path và line để UI vẫn mở preview đúng target.
- Kết quả graph ưu tiên semantic anchors và nodes gần anchors hơn query-only graph matches.
- Hành vi vẫn degrade an toàn khi docs thiếu, graphify thiếu hoặc embedding runtime không khả dụng.

## Ngoài Phạm Vi

- Không thay đổi nguồn tạo graphify hoặc schema `graphify-out/graph.json`.
- Không thêm dependency runtime mới cho preview server.
- Không biến graph panels thành full project graph không giới hạn; vẫn cần cap để tránh response quá lớn và UI graph khó đọc.
- Không thay đổi contract mở file/doc preview hiện có ngoài việc bổ sung metadata nếu cần.

## Hướng Tiếp Cận

### 1. Tách graph search thành anchor expansion

`internal/preview/preview_search.go` có các abstraction nhỏ:

- `graphSearchAnchor` gom anchors từ semantic results.
- `docsGraphIndex` map `specId`, `path`, `nodeId`, normalized title sang docs graph nodes.
- `graphifyIndex` map source path, node id, normalized label và neighbor adjacency sang graphify nodes.
- `expandDocsGraphAnchor` và `expandGraphifyAnchor` duyệt theo breadth-first search với depth và result cap.

Depth dùng để đi nhiều tầng quan hệ từ anchor. Breadth dùng để giới hạn số neighbor mỗi tầng. Plan này hiểu yêu cầu "full deep và bread" là cần đào đủ sâu và đủ rộng nhưng vẫn có guardrail.

### 2. Docs Graph dựa trên Docs Semantic

Sau khi `response.Panels.DocsSemantic` có kết quả:

- Tạo anchors từ `SpecID`, `Path`, `Title` và `ID`.
- Match anchors vào `project.Graph.Nodes`.
- Expand qua `project.Graph.Edges` cả incoming và outgoing.
- Gắn `MatchedBy` gồm `semantic-anchor` cho anchor node và `graph-expansion` cho neighbor.
- Score theo semantic result score, graph distance và relation confidence. Node càng gần anchor càng cao.
- Nếu không có anchor match, fallback sang query graph search hiện tại để không làm panel rỗng bất ngờ.

### 3. Code Graph dựa trên Code Semantic

Sau khi `response.Panels.CodeSemantic` có kết quả:

- Tạo anchors từ `Path`, `Title`, `ID`, line và symbol tokens trong result.
- Match anchors vào graphify nodes bằng `SourceFile` rel path trước, sau đó label/node id fuzzy match.
- Expand qua `graphify.Neighbors` hoặc adjacency nội bộ đầy đủ.
- Giữ line/path của từng graphify node để preview file đúng vị trí.
- Nếu semantic code result match file nhưng không match symbol, source path vẫn có thể map vào graphify nodes trong file đó.
- Nếu graphify không tồn tại, giữ warning hiện tại và để Code Graph empty.

### 4. API metadata cho UI

`previewSearchResult` được mở rộng tương thích JSON hiện tại bằng các field optional:

- `anchorId` để biết node được mở rộng từ semantic result nào.
- `depth` để UI hoặc tests xác nhận mức mở rộng.
- `anchor` boolean để tô/ưu tiên anchor nodes.

UI hiện render graph từ `results + neighbors`, nên backend có thể cải thiện graph context mà không đổi layout search grid. Metadata mới vẫn sẵn sàng để UI hiển thị anchor/expanded badge nếu cần.

## Công Việc Cần Làm

1. Backend: helper index và anchor extraction nằm trong `preview_search.go`.
2. Backend: `buildPreviewSearchResponse` truyền semantic results đã tính vào graph panels.
3. Backend: `searchDocsGraph` và `searchCodeGraph` search theo anchors, giữ fallback query search.
4. Backend: graphify adjacency được giữ đầy đủ cho expansion; neighbor cap chỉ áp dụng khi đưa vào result.
5. Tests: docs semantic result không match query graph node trực tiếp nhưng vẫn kéo được Docs Graph qua `specId/path`.
6. Tests: code semantic result match file/symbol và Code Graph expand sang neighbor dù neighbor không match query token.
7. Tests: graphify missing/invalid và docs missing vẫn không fail.
8. Frontend: chưa cần đổi layout; TypeScript type có thể bổ sung nếu UI bắt đầu hiển thị anchor metadata.
9. Docs: search spec, feature doc và module doc phản ánh behavior shipped.

## Rủi Ro Và Ràng Buộc

- Graphify hiện là optional, nên Code Graph không được phụ thuộc graphify để search tổng thể thành công.
- BFS full depth có thể làm response lớn trên repo nhiều node; cần cap rõ ràng và deterministic sort để tests ổn định.
- Graphify adjacency cần được giữ đầy đủ cho expansion; neighbor cap chỉ áp dụng trên result đưa về UI.
- Semantic search có thể là embedding thật hoặc lexical fallback; anchor extraction phải hoạt động với cả hai.
- Worktree hiện có nhiều thay đổi đang mở trong preview UI/backend/docs, nên implementation cần đọc diff kỹ và không revert thay đổi không liên quan.

## Kiểm Chứng

- Chạy targeted tests cho preview search, ví dụ `go test ./internal/preview -run 'PreviewSearch'`.
- Nếu sửa frontend TypeScript, chạy command build preview frontend theo `docs/development/conventions/preview-frontend.md`.
- Kiểm tra `/api/search?q=...` trả graph results có `MatchedBy`/metadata thể hiện semantic anchor và expansion.
- Nếu có dev server preview sẵn, mở Search tab và xác nhận Docs Graph/Code Graph chọn được anchor node, neighbor và preview action đúng path/line.
