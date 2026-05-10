# Dung Thu Vien Graph Chuyen Dung Cho Preview

## Meta

- **Status**: implemented
- **Description**: Spec da trien khai renderer graph Sigma/Graphology cho preview de xem duoc nhieu file/flow, chon node va mo preview qua details panel.
- **Compliance**: planning
- **Links**: [Preview web](../../features/preview-web.md), [Module preview](../../modules/preview.md), [Quy uoc frontend preview](../../development/conventions/preview-frontend.md), [Trang Search cho Preview](./add-preview-search-page.md), [Search graphs from semantic results](./search-graphs-from-semantic-results.md)

## Boi Canh

Preview web dung Sigma/Graphology renderer cho hai be mat graph:

- `internal/preview/preview_ui_src/js/graph.ts` render Docs Graph trong tab Graph.
- `internal/preview/preview_ui_src/app.ts` render Search Graph cho Docs Graph va Code Graph panels.

Renderer dung `internal/preview/preview_ui_src/js/network_graph.ts` lam adapter chung, build Graphology graph, seed layout vong tron, chay ForceAtlas2 co gioi han iteration va render bang Sigma WebGL. UI khong edit graph; click node chi chon node va cap nhat details panel, con preview doc/file/code mo bang action trong details panel.

Research nhanh:

- `sigma.js` duoc mo ta la thu vien render va tuong tac network graph trong browser, dung WebGL va huong toi graph hang nghin nodes/edges. Thu vien nay ket hop voi `graphology` lam data model va co event `clickNode`.
- `graphology-layout-forceatlas2` cung cap layout ForceAtlas2, phu hop cho graph exploration static sau khi data da load.
- `Cytoscape.js` la phuong an thay the tot neu can nhieu graph algorithms, selectors, extension layouts, hoac editor/manipulation sau nay. Tuy nhien voi nhu cau hien tai la render nhieu node/edge va click node, Sigma nhe dung hon vi tap trung vao visualization WebGL.
- `vis-network` co clustering va events, nhung van duoc thiet ke quanh physics canvas; muc tieu "chi xem/click graph lon" khop Sigma hon.

## Muc Tieu

- Renderer graph dung `sigma.js` + `graphology`.
- Node click chi select va cap nhat details; node doc/file/code co action preview trong details panel.
- Details panel hien incoming/outgoing refs/flows.
- Search/filter, fit view, dark/light color mapping va empty/loading states duoc giu.
- Graph nhieu nodes/edges duoc render bang WebGL thay vi SVG force simulation.

## Ngoai Pham Vi

- Khong thay doi backend `/api/graph` hoac `/api/search` tru khi phat hien data thieu bat buoc cho renderer.
- Khong them graph editing, tao node, tao edge, drag-to-connect, hay persist layout.
- Khong thay the semantic search, graph expansion, hoac graphify ingestion.
- Khong doi navigation model cua preview web.

## Huong Tiep Can De Xuat

Dung `sigma.js` lam renderer WebGL va `graphology` lam in-memory graph object. Vi frontend hien tai la TypeScript compile thang sang static ESM bang `tsc`, khong co bundler, runtime dung import map trong `internal/preview/preview_ui/index.html` de resolve Sigma stack qua jsDelivr ESM.

- Dependencies `sigma`, `graphology`, `graphology-layout-forceatlas2` nam trong `package.json` de TypeScript co type/module resolution va lock version.
- Import map trong `internal/preview/preview_ui/index.html` tro bare imports toi jsDelivr ESM URLs, phu hop voi cach preview hien tai da dung CDN cho lucide, markdown-it, mermaid va svg-pan-zoom.

Adapter dung chung:

- File: `internal/preview/preview_ui_src/js/network_graph.ts`.
- Input adapter: `{ nodes, links, selectedId, container, onSelectNode, nodeColor, edgeColor }`.
- Adapter build `Graphology` graph, gan `x/y/size/color/label/type/path/specId/line`, chay ForceAtlas2 sau khi seed circular layout.
- Adapter khoi tao `Sigma`, bat `clickNode`, expose `fit`, `kill`, va `setSelected`.
- Dung `nodeReducer` va `edgeReducer` de highlight selected node va lam mem unrelated nodes/edges.

Sau do migrate tung be mat:

- `js/graph.ts` dung adapter, giu `normalizedGraphData`, `renderDetails`, `openGraphNode`, `selectGraphNode`.
- `app.ts` dung adapter trong `renderSearchResultGraph`, va `state.searchGraphRenderers` luu Sigma renderer instances.
- `types.d.ts` bo D3 globals.
- `style.css` dung container/canvas styles cho Sigma, giu shell layout va details panel.
- `index.html` bo D3 script va them import map cho Sigma stack.

Neu trong implementation phat hien Sigma ESM qua CDN khong on dinh voi browser target hien tai, fallback hop ly la `Cytoscape.js` UMD/global vi no ho tro module systems, no external dependencies, layouts, events, va canvas rendering. Fallback nay de tich hop nhanh hon voi pattern CDN globals hien tai, nhung hieu nang graph rat lon co the kem Sigma WebGL.

## Cong Viec Can Lam

- [x] Cai dependencies va cap nhat lockfile: `sigma`, `graphology`, `graphology-layout-forceatlas2`.
- [x] Them import map runtime cho Sigma stack.
- [x] Tao shared graph renderer adapter trong `internal/preview/preview_ui_src/js/network_graph.ts`.
- [x] Migrate Docs Graph trong `internal/preview/preview_ui_src/js/graph.ts`.
- [x] Migrate Search Graph trong `internal/preview/preview_ui_src/app.ts`.
- [x] Cap nhat CSS cho graph container WebGL/canvas va selected/highlight state.
- [x] Cap nhat tests string-based trong `internal/preview/preview_test.go`.
- [x] Chay `npm run check:preview`, `npm run lint:preview`, `npm run build:preview`, va Go tests lien quan.
- [ ] Manual browser preview: Docs Graph, Docs Graph search panel, Code Graph search panel, click node select-only, preview buttons, fit, theme light/dark.

## Rui Ro Va Rang Buoc

- Runtime preview hien tai phu thuoc CDN cho nhieu thu vien UI, nen them Sigma qua import map khong lam doi mo hinh runtime, nhung can lock version ro va test offline behavior khong xau hon hien tai.
- `tsc` khong bundle bare imports; neu khong co import map dung, browser se khong resolve duoc `sigma`/`graphology`.
- ForceAtlas2 can toa do ban dau va co chi phi tinh toan. Can gioi han iteration theo kich thuoc graph, hoac dung layout nhanh hon cho graph nho/search result graph.
- Sigma co label rendering khac SVG D3; can dam bao selected/highlight va path/title tooltip van du de user click dung node.
- Search Graph hien tai co details side panel va preview file on click rieng cho Code Graph. Adapter phai giu du metadata node de khong mat behavior.
- Neu graph co duplicate edge IDs hoac node IDs tu docs/search, adapter can sanitize edge keys de Graphology khong throw.

## Kiem Chung

- `npm run check:preview`
- `npm run lint:preview`
- `npm run build:preview`
- `go test ./internal/preview` hoac `go test ./...` neu thay doi test shared.
- Manual preview:
  - Mo tab Graph, graph render khong blank, pan/zoom duoc, Fit hoat dong.
  - Click doc/file/code node chi chon node va cap nhat details panel.
  - Nut preview trong details panel mo doc/file dung target va line khi co line.
  - Tim query co nhieu graphify neighbors, node khong chong qua muc, UI khong freeze.
  - Toggle light/dark va kiem tra label/edge/selected contrast.
