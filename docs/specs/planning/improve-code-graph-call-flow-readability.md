# Cải Thiện Độ Đọc Directed Code Graph Search

## Meta

- **Status**: implemented
- **Description**: Thiết kế hiện tại cho Code Graph trong Search tab: một directed graph duy nhất, chỉ chứa callable nodes và label method kèm owner class.
- **Compliance**: current-state
- **Links**: [Preview web](../../features/preview-web.md), [Module preview](../../modules/preview.md), [Trang search preview](./add-preview-search-page.md), [Search graphs directly from query](./search-graphs-from-semantic-results.md), [Renderer graph preview](./use-specialized-graph-renderer.md)

## Bối Cảnh

Code Graph Search dùng `graphify-out/graph.json` để tìm symbol code và đọc quan hệ `calls`. Khi graph lẫn class/component nodes với method nodes, màn hình dễ bị rối: người dùng thấy nhiều nhãn class đứng độc lập nhưng không biết đâu là call direction thật.

Thiết kế hiện tại giữ Code Graph là một directed graph duy nhất thay vì tách thêm Flow/Network mode. Phần giảm nhiễu nằm ở dữ liệu: chỉ callable nodes được render, class/container nodes chỉ làm metadata để đặt tên method.

## Hành Vi Hiện Tại

- Code Graph chỉ index graphify code nodes có label callable, tức label có dạng function/method với `()`, và normalized source file phải nằm trong `git ls-files`.
- File-only nodes, doc nodes, untracked code nodes, class/container nodes và nodes thuộc project không đọc được Git tracked files không xuất hiện trong result list hoặc neighbor list.
- Query theo class vẫn có thể tìm method thuộc class đó vì backend dùng quan hệ `method` hoặc symbol-level `contains` để thêm owner label vào search haystack.
- Method title hiển thị dạng `ClassName.methodName()` khi graphify cung cấp owner. Class không trở thành node riêng trong graph.
- Directed `calls` là cạnh chính để render graph. Neighbor metadata giữ `direction`, `sourceId` và `targetId` để frontend nối cạnh đúng chiều.
- Renderer vẽ border tương phản theo theme cho caller node ở đầu nguồn của graph đang render, tức node có outgoing `calls` nhưng không có incoming `calls`.
- Search tab Code Graph không có Flow/Network toggle; panel luôn render bằng Sigma/Graphology directed graph.

## Backend Contract

`/api/search` trả Code Graph results với các field chính:

- `nodeId`: ID graphify của callable node.
- `title`: label hiển thị, có owner prefix cho method khi có thể.
- `path` và `line`: target mở file preview.
- `matchedBy`: phân biệt direct match, caller, root caller và callee.
- `neighbors`: danh sách neighbor đã filter chỉ còn callable code nodes.
- `neighbors[].direction`, `sourceId`, `targetId`: hướng thật của cạnh graphify.

Backend dùng `method` và `contains` để xác định owner label, nhưng không đưa các cạnh này thành class/member context nodes trong Code Graph. Khi nhiều callable nodes match cùng query, backend sort direct matches theo score, evidence exactness, title, path và node id trước khi mở rộng từng anchor qua directed `calls`; điều này giữ call-flow ổn định giữa các lần search cùng input.

## Frontend Contract

`SearchPanel.vue` chuyển Code Graph results thành `NetworkGraphData` và giữ hướng cạnh bằng `sourceId`/`targetId`. `network_graph.ts` tạo graphology graph kiểu `directed`, render edge type `arrow`, tăng độ dày cạnh `calls` và giữ details panel cho incoming/outgoing edge rows.

## Ràng Buộc

- Graphify và Git tracked files đều là điều kiện của Code Graph; khi thiếu `graphify-out/graph.json` hoặc không đọc được `git ls-files`, Code Graph có thể rỗng và Search tab hiển thị warning hiện có nếu graphify thiếu.
- Nếu graphify không trích xuất được quan hệ owner cho method, method dùng label gốc thay vì owner-prefixed label.
- Layout vẫn dùng ForceAtlas2 nên không biểu diễn thứ tự thực thi tuyệt đối, nhưng dữ liệu đã bớt nhiễu vì class/container nodes không còn chen vào call graph.

## Kiểm Chứng

- `go test ./internal/preview -run 'PreviewSearch|CodeGraph'`
- `npm run check:preview`
- `npm run lint:preview`
- `npm run build:preview`
- Kiểm tra Search tab với query match class/method: graph chỉ hiển thị callable nodes, method có owner prefix, và cạnh `calls` có arrow đúng chiều.
