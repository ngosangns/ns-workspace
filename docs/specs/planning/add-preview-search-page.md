# Add Preview Search Page

## Boi Canh

`preview` hien tai la local web dashboard doc specs. Backend Go expose `/api/project`, `/api/specs`, `/api/specs/{id}`, `/api/graph`, va `/api/events`. Frontend trong `preview_ui/` co sidebar filter text, tab Overview, Graph, Spec, va graph D3 rieng cho quan he docs.

Nguoi dung yeu cau cap nhat preview web, them searching page, search bang ca semantic va graph tren ca code lan docs. Tham khao Knowns cho semantic search: Knowns dung mode keyword, semantic, hybrid; neu semantic runtime khong san sang thi search fallback an toan ve keyword; hybrid merge keyword va semantic bang Reciprocal Rank Fusion roi rerank. Repo nay hien khong co dependency embedding/runtime, nen implementation nen bat dau bang local semantic fallback nhe, co cau truc de sau nay thay bang embedding thuc.

Repo da co `graphify-out/graph.json` va `graphify-out/GRAPH_REPORT.md`. Graphify output dung `nodes` va `links` voi fields nhu `label`, `id`, `community`, `source_file`, `source_location`, `relation`, `confidence`, `source`, `target`. Plan se doc graphify output neu ton tai trong project root va ket hop voi typed docs graph hien co.

## Muc Tieu

- Them tab/page Search rieng trong preview UI.
- Search docs bang hybrid semantic + keyword theo tinh than Knowns, co fallback khong crash.
- Search code bang semantic fallback dua tren symbols, path, comments, function/type names, va excerpts tu source.
- Search docs graph bang typed docs graph hien co va graphify doc/file nodes.
- Search code graph bang graphify code graph, gom symbols, functions, types, files, va neighbor relationships.
- Hien thi 4 panel ket qua rieng biet: Docs Semantic, Docs Graph, Code Semantic, Code Graph.
- Ket qua search phai mo duoc spec khi co `specId`, va hien graph neighbors/context khi chi la graphify node.
- Giu server localhost-only, khong them external service bat buoc.

## Ngoai Pham Vi

- Khong tai model embedding, khong them ONNX runtime trong task dau tien.
- Khong generate lai `graphify-out`; chi doc output co san neu project da chay graphify.
- Khong thay the graph tab hien tai.
- Khong refactor lon CLI init/update.

## Huong Tiep Can De Xuat

1. Them backend search API:
   - `GET /api/search?q=...&mode=hybrid|semantic|keyword|graph&limit=...`
   - Response gom `query`, `mode`, `panels`, `stats`, va warning neu semantic/graphify fallback.
   - `panels.docsSemantic`: ket qua Markdown specs/docs theo hybrid keyword + semantic fallback.
   - `panels.docsGraph`: ket qua node/edge doc graph tu `specGraph` va graphify doc/file nodes.
   - `panels.codeSemantic`: ket qua source code theo semantic fallback tren filename, symbol names, comments, string snippets, va nearby source excerpts.
   - `panels.codeGraph`: ket qua code graph tu graphify nodes/links, gom symbols va neighbors.
   - `keyword`: exact token/title/path/content matching, co title/path boost.
   - `semantic`: local semantic score dua tren normalized token overlap, path/title/header/symbol boost, fuzzy-ish partial tokens, va heading-aware/source-aware excerpts. Ten field nen la semantic fallback de trung thuc voi viec chua co embeddings.
   - `hybrid`: chay keyword + semantic rieng cho docs va code, merge bang RRF nhu Knowns, sau do boost item co graph neighbors match query.
   - `graph`: uu tien graph panels, nhung van tra semantic panels rong/co warning de UI giu layout on dinh.

2. Them corpus scanners rieng cho docs va code:
   - Docs corpus: dung `scanSpecProject` de lay specs/docs Markdown hien co, raw content, metadata, va typed doc graph.
   - Code corpus: walk project root, bo qua `.git`, `node_modules`, `graphify-out`, binary/media, va generated cache; index cac file text/source pho bien (`.go`, `.js`, `.ts`, `.tsx`, `.css`, `.html`, `.md` ngoai specs neu can).
   - Code candidate nen co `path`, `title`, `kind` (`file`, `function`, `type`, `symbol` neu co), `excerpt`, va `line` khi co the.
   - Khong doc file qua lon vao response; cap excerpt va limit bytes/file de preview khong cham.

3. Them graphify reader:
   - Doc `<projectRoot>/graphify-out/graph.json` neu ton tai.
   - Ho tro ca `links` va `edges` de chiu duoc schema khac nhau.
   - Normalize graphify node thanh search candidate voi label, source file, line, community, relation summary.
   - Phan loai graphify nodes thanh docs/code bang `file_type`, `source_file`, extension, label, va relationship context.
   - Build adjacency map de tra neighbors cho ca docs graph va code graph.
   - Khong loi neu file khong ton tai hoac JSON invalid; API tra warning.

4. Cap nhat frontend:
   - Them tab `Search` vao header va route `/search`.
   - Them search input lon, mode segmented buttons, summary chips.
   - Render 4 panel co dinh trong grid responsive:
     - `Docs Semantic`: docs/specs matches voi excerpt va Open spec.
     - `Docs Graph`: doc graph / graphify doc nodes voi neighbor chips.
     - `Code Semantic`: source matches voi path, line, symbol/excerpt.
     - `Code Graph`: graphify code nodes voi relation, confidence, source location, neighbors.
   - Moi panel co empty state rieng, count rieng, va warning/fallback badge neu can.
   - Ket qua spec co nut Open spec; code/graphify result co source path/location va neighbor list.
   - Debounce input de khong spam API.

5. Cap nhat tests:
   - Unit tests cho docs semantic scoring, code semantic scoring, hybrid RRF merge, graphify reader/classifier.
   - HTTP test cho `/api/search`.
   - UI string tests cho tab Search, route `/search`, `fetchJSON("/api/search...")`, mode controls, va 4 panel IDs/classes.

## Cong Viec Can Lam

- Tao `preview_search.go` cho models, scoring, graphify loading, code/docs corpus indexing, va `handleSearch`.
- Dang ky route `/api/search` trong `newPreviewServer`.
- Cap nhat `preview_ui/index.html` them tab va panel Search.
- Cap nhat `preview_ui/app.js` them state/els/search page rendering, debounced fetch, route handling.
- Cap nhat `preview_ui/style.css` cho 4-panel result grid, score badges, source metadata, neighbor chips.
- Them/bo sung tests trong `preview_test.go` hoac file test moi.
- Cap nhat README preview section neu can de nho endpoint/search tab.

## Rui Ro Va Rang Buoc

- Goi la semantic nhung chua co embedding model: UI/API can hien ro la local semantic fallback/hybrid heuristic de tranh hieu nham.
- `graphify-out/graph.json` co schema khac nhau giua version; reader can defensive.
- Full raw Markdown docs va source files co the lon; search nen cap excerpt, limit bytes/file, va limit result de preview van nhanh.
- Code search khong nen leak binary/generated/cache content; scanner can ignore patterns ro rang.
- Graphify co the duoc chay tren repo khac/thoi diem cu; UI nen hien warning neu source path trong graphify khong con ton tai.
- Worktree dang co thay doi unrelated trong `presets/agents/AGENTS.md`; khong duoc revert.
- `go test ./...` hien dang fail truoc task vi `TestInitCreatesSharedAndNativeLayout` ky vong `spec-guardian.md` nhung preset hien chi co `opencode-intern.md`. Validation sau implementation can neu ro fail baseline nay neu chua duoc sua rieng.

## Kiem Chung

- `go test ./...` hoac subset focused:
  - `go test ./... -run 'TestPreview|TestSearch|TestScanSpecProject'`
- Manual:
  - `go run . preview --project . --addr 127.0.0.1:8787`
  - Mo `/search`, query `parseSpecGraph`, `semantic search`, mot spec title/path, va mot code symbol.
  - Xac nhan ca 4 panels render, co empty states rieng, khong collapse layout.
  - Kiem tra khi co va khong co `graphify-out/graph.json`.

## Acceptance Criteria

- `/api/search` tra ket qua JSON on dinh cho query rong, query match spec, query match graph node, va project khong co graphify output.
- UI co tab Search route-able bang `/search`.
- Search page hien dung 4 panel: Docs Semantic, Docs Graph, Code Semantic, Code Graph.
- Hybrid search hien matched methods (`keyword`, `semantic`, `graph`) va score tren ca docs lan code.
- Graphify matches hien source file/location/community/relation/confidence khi co.
- Click Open spec tu search result load dung Spec tab.
