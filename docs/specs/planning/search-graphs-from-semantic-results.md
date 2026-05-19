# Search Graphs Directly From Query

## Meta

- **Status**: implemented
- **Description**: Spec đã triển khai cho Search tab để Docs Graph và Code Graph search trực tiếp trên graph data bằng query hiện tại, độc lập với semantic results.
- **Compliance**: current-state
- **Links**: [Trang search preview](./add-preview-search-page.md), [Preview web](../../features/preview-web.md), [Module preview](../../modules/preview.md), [Kiến trúc tổng quan](../../architecture/overview.md)

## Bối Cảnh

Search tab trả bốn panel độc lập: Docs Semantic, Docs Graph, Code Semantic và Code Graph. Backend tính semantic panels từ docs/code text, còn graph panels search trực tiếp trên typed docs graph và `graphify-out/graph.json` bằng cùng query đã normalize. Graph panels không cần semantic hit làm anchor trước khi trả kết quả.

Docs Graph match trên node metadata của `project.Graph.Nodes` và doc-like nodes trong graphify. Code Graph match trên code nodes trong graphify. Kết quả graph vẫn kèm neighbors để UI hiển thị quan hệ quanh node match và mở preview đúng target.

## Mục Tiêu

- Docs Graph search trực tiếp theo `node.ID`, `label`, `path`, `category` và `status` trong docs graph.
- Docs Graph cũng search doc nodes từ graphify khi graphify data có sẵn.
- Code Graph search trực tiếp theo `id`, `label`, normalized label, owner label, file type, normalized source file, source location và community trong graphify, nhưng chỉ trả callable code nodes có source file nằm trong `git ls-files`. File-only nodes, doc nodes, class/container nodes, untracked code nodes và mọi code node khi không đọc được Git tracked files đều bị loại khỏi result và neighbor.
- Graph panels không phụ thuộc `DocsSemantic` hoặc `CodeSemantic`; nếu semantic panels rỗng nhưng graph node match query, graph panels vẫn có kết quả.
- Kết quả graph trả `nodeId`, `path`, `line`, `matchedBy`, confidence/community khi có và danh sách neighbors đã cap để UI render ổn định. Code Graph dùng `matchedBy: ["graph"]` cho node match trực tiếp, `matchedBy: ["graph-caller", "graph-flow"]` cho direct incoming `calls`, `matchedBy: ["graph-root-caller", "graph-caller", "graph-flow"]` cho root caller của directed flow và `matchedBy: ["graph-callee", "graph-flow"]` cho outgoing `calls`. UI tự suy ra caller node ở đầu nguồn từ graph đang render để vẽ border đen trong light theme và trắng trong dark theme.
- Keyword difference vẫn loại graph nodes match các keyword loại trừ.

## Ngoài Phạm Vi

- Không thay đổi nguồn tạo graphify hoặc schema `graphify-out/graph.json`.
- Không thêm dependency runtime mới cho preview server.
- Không biến graph panels thành full project graph không giới hạn; vẫn cần cap để tránh response quá lớn và UI graph khó đọc.
- Không thay đổi contract mở file/doc preview hiện có ngoài metadata graph đã có.

## Hướng Tiếp Cận

### 1. Search graph bằng query

`buildPreviewSearchResponse` vẫn tính semantic panels khi mode cho phép, nhưng Docs Graph và Code Graph gọi trực tiếp query search:

- `searchDocsGraph` gọi `searchDocsGraphByQuery`.
- `searchCodeGraph` gọi `searchCodeGraphByQuery`.
- `searchDocsGraphByQuery` match typed docs graph trước, sau đó merge thêm doc nodes từ graphify.
- `searchCodeGraphByQuery` match callable code symbol nodes từ graphify, gồm cả owner label được suy ra từ `method` hoặc symbol-level `contains`, sort direct matches theo score/evidence/title/path, rồi mở rộng từng anchor qua directed `calls` để trả caller/callee liên quan mà không cần file-only hoặc class/container node làm trung gian.

### 2. Result và neighbors

Docs graph result dùng `docGraphNeighbors` để lấy incoming/outgoing edge quanh node match. Graphify result dùng adjacency đã load từ graphify links và giữ `Path`, `Line`, `Community`, `Confidence` để preview file đúng vị trí. Code Graph dùng `calls` làm cạnh render chính; `method` hoặc `contains` chỉ enrich title/search key cho callable node để method hiển thị kèm owner class mà class không thành node riêng.

Neighbors chỉ bị cap khi đưa vào từng result. Search result list vẫn sort và dedupe deterministic trước khi apply limit.

### 3. Quan hệ với semantic panels

Semantic panels và graph panels độc lập về nguồn match. Sau khi graph panels có kết quả, `boostSemanticWithGraph` vẫn có thể tăng điểm semantic result cùng path/spec để giúp panel semantic phản ánh tín hiệu graph, nhưng graph panels không lấy input từ semantic results.

Các helper anchor expansion cũ không nằm trên đường search chính. Nếu cần anchor expansion trở lại, nó phải là mode/tùy chọn riêng thay vì hành vi mặc định của Search tab.

## Công Việc Đã Làm

1. Backend: `buildPreviewSearchResponse` gọi graph search trực tiếp bằng query.
2. Backend: Docs Graph và Code Graph không truyền semantic results vào graph search.
3. Backend: query graph search vẫn giữ exclusion keyword handling, result dedupe, sorting và neighbors.
4. Tests: graph panels trả result khi semantic panels rỗng nhưng graph node match query.
5. Docs: feature doc và spec phản ánh behavior shipped.

## Rủi Ro Và Ràng Buộc

- Graphify hiện là optional, nên Code Graph có thể rỗng khi project chưa có `graphify-out/graph.json`.
- Query direct chỉ match graph metadata hiện có; nếu một khái niệm chỉ xuất hiện trong nội dung file nhưng không xuất hiện trong graph node metadata, graph panel sẽ không tự mở rộng từ semantic result.
- Graphify adjacency cần được giữ đủ trong loader; neighbor cap chỉ áp dụng trên result đưa về UI.

## Kiểm Chứng

- Chạy targeted tests cho preview search, ví dụ `go test ./internal/preview -run 'PreviewSearch'`.
- Kiểm tra `/api/search?q=...` trả graph results có `MatchedBy` chứa `graph`, có `nodeId` và neighbors khi graph data có cạnh liên quan.
- Nếu có dev server preview sẵn, mở Search tab và xác nhận Docs Graph/Code Graph chọn được node, neighbor và preview action đúng path/line.
