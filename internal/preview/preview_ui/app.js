import { createDocsGraph } from "./js/graph.js";
import { renderNetworkGraph } from "./js/network_graph.js";
const state = {
    project: null,
    specs: [],
    graph: null,
    graphRenderer: null,
    searchGraphRenderers: new Map(),
    searchGraphSelections: new Map(),
    graphSelectedId: "",
    searchData: null,
    searchController: null,
    searchLoading: false,
    currentSpec: null,
    codeGraphController: null,
    codeGraphLoading: false,
    searchTimer: null,
    hotReloadToken: "",
    theme: getInitialTheme(),
    diagramPanZoomInstances: new Map(),
    diagramPanZoomTargets: new Map(),
    expandedPaths: new Set(),
    selectedId: "",
    routeSpecId: "",
    tab: "spec",
    applyingRoute: false,
    previewRoute: null,
    previewTitle: "",
    diagramSerial: 0,
    showRawMarkdown: false,
    selectionCopyTarget: null,
};
const els = {
    projectName: document.querySelector("#projectName"),
    search: document.querySelector("#search"),
    specList: document.querySelector("#specList"),
    pageTitle: document.querySelector("#pageTitle"),
    pagePath: document.querySelector("#pagePath"),
    specContent: document.querySelector("#specContent"),
    graphCanvas: document.querySelector("#graphCanvas"),
    graphDetails: document.querySelector("#graphDetails"),
    graphSearch: document.querySelector("#graphSearch"),
    graphStats: document.querySelector("#graphStats"),
    graphFit: document.querySelector("#graphFit"),
    globalSearch: document.querySelector("#globalSearch"),
    searchSummary: document.querySelector("#searchSummary"),
    docsSemanticResults: document.querySelector("#docsSemanticResults"),
    docsGraphResults: document.querySelector("#docsGraphResults"),
    codeSemanticResults: document.querySelector("#codeSemanticResults"),
    codeGraphResults: document.querySelector("#codeGraphResults"),
    docsSemanticCount: document.querySelector("#docsSemanticCount"),
    docsGraphCount: document.querySelector("#docsGraphCount"),
    codeSemanticCount: document.querySelector("#codeSemanticCount"),
    codeGraphCount: document.querySelector("#codeGraphCount"),
    codeGraphReload: document.querySelector("#codeGraphReload"),
    previewDialog: document.querySelector("#previewDialog"),
    previewDialogTitle: document.querySelector("#previewDialogTitle"),
    previewDialogPath: document.querySelector("#previewDialogPath"),
    previewDialogBody: document.querySelector("#previewDialogBody"),
    themeToggle: document.querySelector("#themeToggle"),
    rawMarkdownToggle: document.querySelector("#rawMarkdownToggle"),
    selectionContextMenu: document.querySelector("#selectionContextMenu"),
    selectionCopyButton: document.querySelector("#selectionCopyButton"),
};
const markdownRenderer = window.markdownit({
    html: false,
    linkify: true,
    typographer: false,
    highlight: (source, lang) => {
        const language = normalizeHighlightLanguage(lang);
        if (window.hljs && language && window.hljs.getLanguage(language)) {
            return `<pre class="hljs"><code class="language-${escapeHTML(language)} is-highlighted" data-highlighted="yes">${window.hljs.highlight(source, { language, ignoreIllegals: true }).value}</code></pre>`;
        }
        return `<pre class="hljs"><code class="is-highlighted" data-highlighted="yes">${escapeHTML(source)}</code></pre>`;
    },
});
markdownRenderer.enable("table");
applyTheme(state.theme, { persist: false, rerender: false });
const diagramSanitizeConfig = {
    USE_PROFILES: { html: true, svg: true, svgFilters: true },
    ADD_TAGS: ["foreignObject", "marker", "defs", "text", "tspan", "div", "span", "p", "br"],
    ADD_ATTR: [
        "viewBox",
        "xmlns",
        "d",
        "x",
        "y",
        "x1",
        "x2",
        "y1",
        "y2",
        "cx",
        "cy",
        "rx",
        "ry",
        "r",
        "points",
        "marker-end",
        "marker-start",
        "text-anchor",
        "dominant-baseline",
        "transform",
        "width",
        "height",
        "fill",
        "stroke",
        "stroke-width",
        "class",
        "id",
        "style",
        "dominant-baseline",
        "alignment-baseline",
    ],
};
const graphView = createDocsGraph({ state, els, escapeHTML, refreshIcons, openSpecPreview, openFilePreview });
async function load() {
    const [project, specs, graph] = await Promise.all([fetchJSON("/api/project"), fetchJSON("/api/docs"), fetchJSON("/api/graph")]);
    state.project = project;
    state.specs = specs;
    state.graph = graph;
    els.projectName.textContent = project.name;
    const route = routeFromLocation();
    state.selectedId = validSpecId(route.spec) || defaultSpecId();
    syncRouteSpecFromURL(route);
    applySearchRoute(route);
    graphView.render();
    renderSpecList();
    if (state.selectedId) {
        await selectSpec(state.selectedId, false);
    }
    if (!route.tab && !route.spec && state.selectedId) {
        replaceSpecRoute(state.selectedId, route.fragment || "");
    }
    switchTab(route.tab || "spec", { updateURL: false });
    await applyPreviewRoute(route);
}
async function reloadPreviewData() {
    const previousSelection = state.selectedId;
    const route = routeFromLocation();
    const [project, specs, graph] = await Promise.all([fetchJSON("/api/project"), fetchJSON("/api/docs"), fetchJSON("/api/graph")]);
    state.project = project;
    state.specs = specs;
    state.graph = graph;
    els.projectName.textContent = project.name;
    state.selectedId = validSpecId(route.spec) || validSpecId(previousSelection) || defaultSpecId();
    syncRouteSpecFromURL(route);
    applySearchRoute(route);
    graphView.render();
    renderSpecList();
    if (state.selectedId) {
        await selectSpec(state.selectedId, false);
    }
    if (!route.tab && !route.spec && state.selectedId) {
        replaceSpecRoute(state.selectedId, route.fragment || "");
    }
    switchTab(route.tab || state.tab || "spec", { updateURL: false });
    await applyPreviewRoute(route);
}
function defaultSpecId() {
    return state.specs[0]?.id || "";
}
function validSpecId(id) {
    if (!id)
        return "";
    return state.specs.some((spec) => spec.id === id) ? id : "";
}
async function fetchJSON(path) {
    const res = await fetch(path);
    if (!res.ok)
        throw new Error(await res.text());
    return res.json();
}
function scheduleSearch() {
    window.clearTimeout(state.searchTimer);
    const query = els.globalSearch?.value.trim() || "";
    setPageChromeForTab("search");
    updateSearchRouteURL({ replace: true });
    if (query) {
        state.searchLoading = true;
        renderSearchPanels();
    }
    state.searchTimer = window.setTimeout(() => {
        runSearch().catch((error) => renderSearchError(error));
    }, 180);
}
async function runSearch() {
    if (!els.globalSearch)
        return;
    const query = els.globalSearch.value.trim();
    if (state.searchController) {
        state.searchController.abort();
    }
    if (!query) {
        state.searchData = null;
        state.searchLoading = false;
        renderSearchPanels();
        return;
    }
    state.searchLoading = true;
    renderSearchPanels();
    state.searchController = new AbortController();
    const params = new URLSearchParams({ q: query, limit: "8" });
    const res = await fetch(`/api/search?${params.toString()}`, { signal: state.searchController.signal });
    if (!res.ok)
        throw new Error(await res.text());
    state.searchData = await res.json();
    state.searchLoading = false;
    renderSearchPanels();
}
function renderSearchPanels() {
    if (!els.searchSummary)
        return;
    const data = state.searchData;
    const panels = data?.panels || {};
    renderSearchSummary(data);
    renderSearchPanel("docsSemantic", panels.docsSemantic || [], "No document semantic matches.", state.searchLoading);
    renderSearchPanel("docsGraph", panels.docsGraph || [], "No document graph matches.", state.searchLoading);
    renderSearchPanel("codeSemantic", panels.codeSemantic || [], "No code semantic matches.", state.searchLoading);
    renderSearchPanel("codeGraph", panels.codeGraph || [], "No code graph matches.", state.searchLoading || state.codeGraphLoading);
    updateCodeGraphReloadControl();
    refreshIcons();
}
async function reloadCodeGraph() {
    const query = els.globalSearch?.value.trim() || state.searchData?.query || "";
    if (!query || state.codeGraphLoading)
        return;
    if (state.codeGraphController) {
        state.codeGraphController.abort();
    }
    state.codeGraphLoading = true;
    updateCodeGraphReloadControl();
    const panels = state.searchData?.panels || {};
    renderSearchPanel("codeGraph", panels.codeGraph || [], "No code graph matches.", true);
    state.codeGraphController = new AbortController();
    try {
        const params = new URLSearchParams({ q: query, limit: "8" });
        const res = await fetch(`/api/search?${params.toString()}`, { signal: state.codeGraphController.signal });
        if (!res.ok)
            throw new Error(await res.text());
        const next = await res.json();
        const currentPanels = state.searchData?.panels || {};
        state.searchData = {
            ...next,
            panels: {
                ...currentPanels,
                codeGraph: next.panels?.codeGraph || [],
            },
        };
        state.codeGraphLoading = false;
        renderSearchSummary(state.searchData);
        renderSearchPanel("codeGraph", state.searchData.panels.codeGraph || [], "No code graph matches.", false);
        updateCodeGraphReloadControl();
        refreshIcons();
    }
    catch (error) {
        if (error.name !== "AbortError") {
            state.codeGraphLoading = false;
            updateCodeGraphReloadControl();
            renderSearchPanel("codeGraph", panels.codeGraph || [], "No code graph matches.", false);
            els.searchSummary.innerHTML = `<div class="alert alert-error py-2 text-sm">${escapeHTML(error.message || String(error))}</div>`;
        }
    }
}
function updateCodeGraphReloadControl() {
    if (!els.codeGraphReload)
        return;
    const hasQuery = Boolean(els.globalSearch?.value.trim() || state.searchData?.query);
    els.codeGraphReload.disabled = state.searchLoading || state.codeGraphLoading || !hasQuery;
    els.codeGraphReload.innerHTML = state.codeGraphLoading
        ? '<span class="loading loading-spinner loading-xs"></span>'
        : '<i data-lucide="refresh-cw" class="h-3.5 w-3.5"></i>';
}
function renderSearchSummary(data) {
    if (state.searchLoading) {
        els.searchSummary.innerHTML = `
      <div class="flex flex-wrap items-center gap-2" aria-live="polite">
        <span class="loading loading-spinner loading-sm text-primary"></span>
        <span class="text-sm text-base-content/70">Searching docs, code, and graphs...</span>
      </div>
    `;
        return;
    }
    if (!data) {
        els.searchSummary.innerHTML = '<span class="text-sm text-base-content/60">Search across docs, code, and graph context.</span>';
        return;
    }
    const stats = data.stats || {};
    const total = Object.values(stats).reduce((sum, value) => sum + Number(value || 0), 0);
    const warnings = data.warnings || [];
    els.searchSummary.innerHTML = `
      <div class="flex flex-wrap items-center gap-2">
      <span class="badge badge-primary badge-sm">${escapeHTML(data.mode || "hybrid")}</span>
      <span class="badge badge-ghost badge-sm">${total} results</span>
      ${warnings
        .slice(0, 2)
        .map((warning) => `<span class="badge badge-warning badge-sm max-w-full truncate">${escapeHTML(warning)}</span>`)
        .join("")}
    </div>
  `;
}
function renderSearchPanel(name, results, emptyText, loading) {
    const list = els[`${name}Results`];
    const count = els[`${name}Count`];
    if (!list || !count)
        return;
    stopSearchGraph(name);
    if (loading) {
        count.textContent = "...";
        list.innerHTML = renderSearchLoading();
        return;
    }
    count.textContent = String(results.length);
    if (!results.length) {
        list.innerHTML = `<div class="search-empty">${escapeHTML(emptyText)}</div>`;
        return;
    }
    if (name === "docsGraph" || name === "codeGraph") {
        renderSearchGraphPanel(name, results, list);
        return;
    }
    list.innerHTML = results.map((result) => renderSearchResult(result, name)).join("");
    list.querySelectorAll("[data-preview-spec]").forEach((button) => {
        button.addEventListener("click", () => openSpecPreview(button.dataset.previewSpec, { updateURL: true }));
    });
    list.querySelectorAll("[data-preview-file]").forEach((button) => {
        button.addEventListener("click", () => openFilePreview(button.dataset.previewFile, Number(button.dataset.previewLine || 0), { updateURL: true }));
    });
}
function renderSearchGraphPanel(name, results, list) {
    const graph = searchResultsToGraph(results, name);
    list.innerHTML = `
    <div class="search-graph-shell">
      <div class="search-graph-canvas" data-search-graph="${escapeHTML(name)}" role="img" aria-label="${escapeHTML(name)} search graph"></div>
      <aside class="search-graph-details" data-search-graph-details="${escapeHTML(name)}"></aside>
    </div>
  `;
    const canvas = list.querySelector(`[data-search-graph="${name}"]`);
    const details = list.querySelector(`[data-search-graph-details="${name}"]`);
    renderSearchResultGraph(name, graph, canvas, details);
}
function searchResultsToGraph(results, panelName) {
    const nodes = new Map();
    const links = [];
    const ensureNode = (node) => {
        const existing = nodes.get(node.id) || {};
        nodes.set(node.id, { ...existing, ...node, label: node.label || existing.label || node.id });
        return nodes.get(node.id);
    };
    const addLink = (source, target, type, confidence) => {
        if (!source || !target || source === target)
            return;
        links.push({ source, target, type: type || "references", confidence: confidence || "" });
    };
    results.forEach((result, index) => {
        const resultID = result.nodeId || result.id || `${panelName}:${index}`;
        const resultType = panelName === "codeGraph" ? "code" : "doc";
        const fileName = result.path ? result.path.split("/").pop() : "";
        ensureNode({
            id: resultID,
            label: panelName === "codeGraph" ? codeGraphNodeLabel(result, fileName) : result.title || result.nodeId || result.path || result.id,
            type: resultType,
            path: result.path || "",
            previewPath: result.path || "",
            previewLine: result.line || 0,
            line: result.line || 0,
            specId: result.specId || "",
            community: result.community || "",
            score: result.score || 0,
            result,
        });
        if (result.path) {
            const fileID = `file:${result.path}`;
            ensureNode({
                id: fileID,
                label: result.path.split("/").pop(),
                type: result.specId ? "doc-file" : "file",
                path: result.path,
                previewPath: result.path,
                previewLine: result.line || 0,
                line: result.line || 0,
                specId: result.specId || "",
            });
            addLink(fileID, resultID, result.specId ? "documents" : "defines", result.confidence);
        }
        (result.neighbors || []).forEach((neighbor) => {
            const neighborID = neighbor.id || neighbor.label;
            const neighborPath = neighbor.path || (panelName === "codeGraph" ? result.path || "" : "");
            const neighborLine = Number(neighbor.line || (panelName === "codeGraph" ? result.line || 0 : 0));
            ensureNode({
                id: neighborID,
                label: neighbor.label || neighbor.id,
                type: "flow",
                path: neighborPath,
                previewPath: neighborPath,
                previewLine: neighborLine,
                line: neighborLine,
                confidence: neighbor.confidence || "",
            });
            addLink(resultID, neighborID, neighbor.relation, neighbor.confidence);
        });
    });
    return { nodes: [...nodes.values()], links: dedupeGraphLinks(links) };
}
function dedupeGraphLinks(links) {
    const seen = new Set();
    return links.filter((link) => {
        const key = `${link.source}->${link.target}:${link.type}`;
        if (seen.has(key))
            return false;
        seen.add(key);
        return true;
    });
}
function renderSearchResultGraph(name, graph, canvas, details) {
    if (!canvas || !details)
        return;
    stopSearchGraph(name);
    const selected = state.searchGraphSelections.get(name);
    const selectedNode = graph.nodes.find((node) => node.id === selected) || graph.nodes[0];
    if (selectedNode) {
        state.searchGraphSelections.set(name, selectedNode.id);
    }
    renderSearchGraphDetails(name, graph, details);
    canvas.innerHTML = "";
    const renderer = renderNetworkGraph({
        container: canvas,
        graph,
        selectedId: selectedNode?.id || "",
        nodeColor: searchNodeColor,
        edgeColor: searchEdgeColor,
        labelColor: state.theme === "dark" ? "#f8fafc" : "#0f172a",
        onSelectNode: (item) => {
            state.searchGraphSelections.set(name, item.id);
            renderSearchGraphDetails(name, graph, details);
        },
    });
    state.searchGraphRenderers.set(name, renderer);
}
function renderSearchGraphDetails(name, graph, details) {
    const selected = state.searchGraphSelections.get(name);
    const node = graph.nodes.find((item) => item.id === selected) || graph.nodes[0];
    if (!node) {
        details.innerHTML = '<div class="p-4 text-sm text-base-content/60">No graph results.</div>';
        return;
    }
    const incoming = graph.links.filter((edge) => graphEndpointID(edge.target) === node.id);
    const outgoing = graph.links.filter((edge) => graphEndpointID(edge.source) === node.id);
    details.innerHTML = `
    <div class="grid gap-3 p-3">
      <div>
        <div class="text-xs uppercase tracking-wide text-base-content/50">${escapeHTML(node.type || "node")}</div>
        <h3 class="mt-1 text-sm font-semibold">${escapeHTML(node.label || node.id)}</h3>
        <p class="break-words text-xs text-base-content/60">${escapeHTML(node.path || node.id)}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        ${node.specId ? `<button class="btn btn-primary btn-xs" type="button" data-preview-spec="${escapeHTML(node.specId)}"><i data-lucide="file-text" class="h-3.5 w-3.5"></i>Preview doc</button>` : ""}
        ${node.path ? `<button class="btn btn-ghost btn-xs" type="button" data-preview-file="${escapeHTML(node.path)}" data-preview-line="${escapeHTML(String(node.line || 0))}"><i data-lucide="file-code" class="h-3.5 w-3.5"></i>Preview file</button>` : ""}
      </div>
      <div>
        <h4 class="mb-1 text-xs font-semibold">Outgoing flows (${outgoing.length})</h4>
        ${renderSearchGraphEdgeList(outgoing, "target")}
      </div>
      <div>
        <h4 class="mb-1 text-xs font-semibold">Incoming flows (${incoming.length})</h4>
        ${renderSearchGraphEdgeList(incoming, "source")}
      </div>
    </div>
  `;
    details.querySelectorAll("[data-preview-spec]").forEach((button) => {
        button.addEventListener("click", () => openSpecPreview(button.dataset.previewSpec, { updateURL: true }));
    });
    details.querySelectorAll("[data-preview-file]").forEach((button) => {
        button.addEventListener("click", () => openFilePreview(button.dataset.previewFile, Number(button.dataset.previewLine || 0), { updateURL: true }));
    });
    refreshIcons();
}
function codeGraphNodeLabel(result, fileName) {
    const title = result.title || result.nodeId || result.id || fileName || "code";
    if (!fileName || title.includes(fileName)) {
        return title;
    }
    return `${title} · ${fileName}`;
}
function renderSearchGraphEdgeList(edges, side) {
    if (!edges.length)
        return '<div class="text-xs text-base-content/50">None</div>';
    return `
    <div class="grid gap-1">
      ${edges
        .slice(0, 10)
        .map((edge) => {
        const related = graphEndpointID(edge[side]);
        return `<div class="graph-ref-row">
            <span class="badge badge-ghost badge-xs">${escapeHTML(edge.type || "references")}</span>
            <span class="min-w-0 truncate">${escapeHTML(related)}</span>
          </div>`;
    })
        .join("")}
    </div>
  `;
}
function graphEndpointID(endpoint) {
    return typeof endpoint === "string" ? endpoint : endpoint?.id || "";
}
function searchNodeColor(node) {
    switch (node.type) {
        case "code":
            return "#2563eb";
        case "file":
            return "#64748b";
        case "doc":
        case "doc-file":
            return "#0f766e";
        case "flow":
            return "#9333ea";
        default:
            return "#94a3b8";
    }
}
function searchEdgeColor(type) {
    switch (type) {
        case "defines":
        case "documents":
            return "#64748b";
        case "depends":
        case "blocked-by":
            return "#ef4444";
        case "implements":
        case "calls":
            return "#6366f1";
        case "references":
            return "#14b8a6";
        default:
            return "#64748b";
    }
}
function stopSearchGraph(name) {
    const renderer = state.searchGraphRenderers.get(name);
    if (renderer) {
        renderer.kill();
        state.searchGraphRenderers.delete(name);
    }
}
function renderSearchLoading() {
    return `
    <div class="search-loading" aria-busy="true">
      <div class="skeleton h-4 w-2/3"></div>
      <div class="skeleton h-3 w-full"></div>
      <div class="skeleton h-3 w-5/6"></div>
    </div>
  `;
}
function renderSearchResult(result, panelName) {
    const path = result.path ? `<div class="search-path">${escapeHTML(formatResultPath(result))}</div>` : "";
    const excerpt = result.excerpt ? `<p class="search-excerpt">${escapeHTML(result.excerpt)}</p>` : "";
    const tags = [
        result.kind,
        ...(result.matchedBy || []),
        result.community ? `community ${result.community}` : "",
        result.relation,
        result.confidence && result.confidence !== "graphify" ? result.confidence : "",
    ].filter(Boolean);
    const neighbors = renderSearchNeighbors(result.neighbors || []);
    const actions = renderSearchResultActions(result, panelName);
    return `
    <article class="search-result">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <h3>${escapeHTML(result.title || result.id || "Untitled")}</h3>
          ${path}
        </div>
        <span class="badge badge-outline badge-sm shrink-0">${Math.round((result.score || 0) * 100)}%</span>
      </div>
      ${excerpt}
      ${tags.length
        ? `<div class="search-tags">${tags
            .slice(0, 5)
            .map((tag) => `<span class="badge badge-ghost badge-xs">${escapeHTML(tag)}</span>`)
            .join("")}</div>`
        : ""}
      ${neighbors}
      ${actions}
    </article>
  `;
}
function renderSearchResultActions(result, panelName) {
    const buttons = [];
    if (result.specId) {
        buttons.push(`<button class="btn btn-primary btn-xs" type="button" data-preview-spec="${escapeHTML(result.specId)}"><i data-lucide="file-text" class="h-3.5 w-3.5"></i>Preview doc</button>`);
    }
    if (result.path && (panelName !== "docsSemantic" || !result.specId)) {
        buttons.push(`<button class="btn btn-ghost btn-xs" type="button" data-preview-file="${escapeHTML(result.path)}" data-preview-line="${escapeHTML(String(result.line || 0))}"><i data-lucide="file-code" class="h-3.5 w-3.5"></i>Preview file</button>`);
    }
    if (!buttons.length)
        return "";
    return `<div class="mt-2 flex flex-wrap gap-2">${buttons.join("")}</div>`;
}
function formatResultPath(result) {
    if (result.line)
        return `${result.path}:${result.line}`;
    return result.path;
}
function renderSearchNeighbors(neighbors) {
    if (!neighbors.length)
        return "";
    return `
    <div class="search-neighbors">
      ${neighbors
        .slice(0, 5)
        .map((neighbor) => `<span class="badge badge-neutral badge-xs max-w-full truncate">${escapeHTML(neighbor.relation ? `${neighbor.relation}: ${neighbor.label || neighbor.id}` : neighbor.label || neighbor.id)}</span>`)
        .join("")}
    </div>
  `;
}
function renderSearchError(error) {
    if (error.name === "AbortError")
        return;
    state.searchLoading = false;
    state.codeGraphLoading = false;
    renderSearchPanels();
    els.searchSummary.innerHTML = `<div class="alert alert-error py-2 text-sm">${escapeHTML(error.message || String(error))}</div>`;
}
function renderSpecList() {
    const query = els.search.value.toLowerCase().trim();
    const specs = state.specs.filter((spec) => {
        const haystack = `${spec.title} ${spec.path} ${spec.status} ${spec.compliance}`.toLowerCase();
        return !query || haystack.includes(query);
    });
    const tree = buildSpecTree(specs);
    autoExpandForSelection();
    if (query) {
        expandAllVisibleFolders(tree);
    }
    els.specList.innerHTML = "";
    renderTreeNodes(tree.children, els.specList, 0);
    refreshIcons();
}
function buildSpecTree(specs) {
    const root = { name: "", path: "", type: "folder", children: new Map() };
    specs.forEach((spec) => {
        const parts = spec.path.split("/");
        let cursor = root;
        parts.forEach((part, index) => {
            const isFile = index === parts.length - 1;
            const path = parts.slice(0, index + 1).join("/");
            if (!cursor.children.has(part)) {
                cursor.children.set(part, isFile ? { name: part, path, type: "file", spec } : { name: part, path, type: "folder", children: new Map() });
            }
            cursor = cursor.children.get(part);
            if (isFile) {
                cursor.spec = spec;
            }
        });
    });
    sortTree(root);
    return root;
}
function sortTree(node) {
    if (!node.children)
        return;
    node.children = new Map([...node.children.entries()].sort(([, a], [, b]) => {
        if (a.type !== b.type)
            return a.type === "folder" ? -1 : 1;
        return a.name.localeCompare(b.name);
    }));
    node.children.forEach(sortTree);
}
function renderTreeNodes(children, parent, depth) {
    children.forEach((node) => {
        if (node.type === "folder") {
            renderFolderNode(node, parent, depth);
            if (state.expandedPaths.has(node.path)) {
                renderTreeNodes(node.children, parent, depth + 1);
            }
            return;
        }
        renderFileNode(node.spec, parent, depth);
    });
}
function renderFolderNode(node, parent, depth) {
    const expanded = state.expandedPaths.has(node.path);
    const button = document.createElement("button");
    button.className = "tree-row btn btn-ghost btn-sm min-h-8 w-full justify-start gap-1 px-2 text-left font-medium";
    button.style.paddingLeft = `${8 + depth * 16}px`;
    button.innerHTML = `
    <i data-lucide="chevron-right" class="tree-chevron h-4 w-4 shrink-0 transition-transform ${expanded ? "rotate-90" : ""}"></i>
    <i data-lucide="${expanded ? "folder-open" : "folder"}" class="h-4 w-4 shrink-0 text-base-content/60"></i>
    <span class="truncate">${escapeHTML(node.name)}</span>
  `;
    button.addEventListener("click", () => {
        if (expanded) {
            state.expandedPaths.delete(node.path);
        }
        else {
            state.expandedPaths.add(node.path);
        }
        renderSpecList();
    });
    parent.append(button);
}
function renderFileNode(spec, parent, depth) {
    const routeSpecId = activeRouteSpecId();
    const button = document.createElement("button");
    button.className = [
        "tree-row btn btn-ghost btn-sm grid h-auto min-h-8 w-full grid-cols-[auto_minmax(0,1fr)_auto] justify-start gap-2 px-2 text-left font-normal",
        spec.id === routeSpecId ? "btn-active" : "",
    ].join(" ");
    button.style.paddingLeft = `${24 + depth * 16}px`;
    button.innerHTML = `
    <i data-lucide="file-text" class="h-4 w-4 shrink-0 text-base-content/55"></i>
    <span class="truncate">${escapeHTML(displaySpecName(spec))}</span>
    ${spec.status ? `<span class="badge badge-ghost badge-sm max-w-24 truncate">${escapeHTML(spec.status)}</span>` : ""}
  `;
    button.addEventListener("click", () => selectSpec(spec.id, true));
    parent.append(button);
}
function displaySpecName(spec) {
    const base = spec.path.split("/").pop() || spec.title;
    if (base === "_overview.md")
        return spec.title;
    return spec.title || base;
}
function autoExpandForSelection() {
    const activeSpecId = activeRouteSpecId() || state.selectedId;
    if (!activeSpecId)
        return;
    const parts = activeSpecId.split("/");
    for (let index = 1; index < parts.length; index++) {
        state.expandedPaths.add(parts.slice(0, index).join("/"));
    }
}
function activeRouteSpecId() {
    return validSpecId(state.routeSpecId) ? state.routeSpecId : "";
}
function expandAllVisibleFolders(node) {
    if (!node.children)
        return;
    node.children.forEach((child) => {
        if (child.type === "folder") {
            state.expandedPaths.add(child.path);
            expandAllVisibleFolders(child);
        }
    });
}
async function selectSpec(id, showSpecTab, options = {}) {
    const updateURL = options.updateURL !== false;
    const spec = await fetchJSON(`/api/docs/${encodeURIComponent(id)}`);
    state.selectedId = id;
    state.currentSpec = spec;
    if (state.tab === "spec" || showSpecTab) {
        setPageChromeForTab("spec");
    }
    destroyDiagramsIn(els.specContent);
    await renderCurrentSpecContent();
    renderSpecList();
    if (showSpecTab) {
        switchTab("spec", { updateURL });
    }
    else if (updateURL && state.tab === "spec") {
        updateRouteURL("spec");
    }
}
async function renderCurrentSpecContent() {
    if (!state.currentSpec)
        return;
    updateRawMarkdownToggle(state.currentSpec);
    await renderSpecDocumentContent(els.specContent, state.currentSpec, "markdown card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6", {
        rawMarkdown: state.showRawMarkdown,
    });
}
async function openSpecPreview(id, options = {}) {
    if (!id)
        return;
    openPreviewLoading("Loading doc", id);
    state.previewRoute = { type: "doc", path: id, line: 0 };
    if (options.updateURL !== false) {
        updateSearchRouteURL({ replace: false });
    }
    try {
        const spec = await fetchJSON(`/api/docs/${encodeURIComponent(id)}`);
        els.previewDialogTitle.textContent = spec.title || "Doc preview";
        els.previewDialogPath.textContent = spec.path || id;
        state.previewTitle = `Doc preview: ${spec.title || id}`;
        updateDocumentTitle();
        destroyDiagramsIn(els.previewDialogBody);
        await renderSpecDocumentContent(els.previewDialogBody, spec, "preview-modal-body");
        refreshIcons();
    }
    catch (error) {
        renderPreviewError(error);
    }
}
async function renderSpecDocumentContent(root, spec, baseClass = "markdown", options = {}) {
    const language = spec.language || languageFromPath(spec.path || "");
    root.dataset.sourcePath = spec.path || spec.id || "";
    if (language === "markdown") {
        if (options.rawMarkdown) {
            root.className = baseClass
                .replace(/\bmarkdown\b/g, "")
                .replace(/\s+/g, " ")
                .trim();
            root.innerHTML = renderCodePreview(spec.raw || "", "markdown");
            highlightRenderedCode(root);
            decorateCodePreviewLines(root);
            return;
        }
        root.className = baseClass.includes("markdown") ? baseClass : `${baseClass} markdown`;
        root.innerHTML = renderMarkdown(spec.raw || "");
        decorateMarkdownSourceLines(root, spec.raw || "");
        decorateInternalDocNavigation(root, spec);
        await renderMermaidBlocks(root);
        highlightRenderedCode(root);
        return;
    }
    root.className = baseClass
        .replace(/\bmarkdown\b/g, "")
        .replace(/\s+/g, " ")
        .trim();
    root.innerHTML = renderCodePreview(spec.raw || "", language);
    highlightRenderedCode(root);
    decorateCodePreviewLines(root);
}
async function openFilePreview(path, line, options = {}) {
    if (!path)
        return;
    openPreviewLoading("Loading file", path);
    state.previewRoute = { type: "file", path, line: Number(line || 0) };
    if (options.updateURL !== false) {
        updateSearchRouteURL({ replace: false });
    }
    try {
        const file = await fetchJSON(`/api/files?${new URLSearchParams({ path }).toString()}`);
        els.previewDialogTitle.textContent = file.title || "File preview";
        els.previewDialogPath.textContent = line ? `${file.path}:${line}` : file.path;
        state.previewTitle = `File preview: ${line ? `${file.title}:${line}` : file.title}`;
        updateDocumentTitle();
        destroyDiagramsIn(els.previewDialogBody);
        els.previewDialogBody.dataset.sourcePath = file.path || path;
        els.previewDialogBody.className = "preview-modal-body";
        els.previewDialogBody.innerHTML = renderCodePreview(file.raw || "", file.language || languageFromPath(file.path));
        highlightRenderedCode(els.previewDialogBody);
        decorateCodePreviewLines(els.previewDialogBody);
        scrollPreviewToLine(line);
        refreshIcons();
    }
    catch (error) {
        renderPreviewError(error);
    }
}
function openPreviewLoading(title, path) {
    destroyDiagramsIn(els.previewDialogBody);
    els.previewDialogTitle.textContent = title;
    els.previewDialogPath.textContent = path || "";
    state.previewTitle = title;
    updateDocumentTitle();
    els.previewDialogBody.className = "preview-modal-body";
    els.previewDialogBody.innerHTML = `
    <div class="preview-loading">
      <span class="loading loading-spinner loading-sm text-primary"></span>
      <span>Opening preview...</span>
    </div>
  `;
    els.previewDialog.classList.add("modal-open");
    refreshIcons();
}
function closePreviewDialog(options = {}) {
    hideSelectionContextMenu();
    destroyDiagramsIn(els.previewDialogBody);
    els.previewDialog.classList.remove("modal-open");
    state.previewRoute = null;
    state.previewTitle = "";
    updateDocumentTitle();
    if (options.updateURL !== false) {
        updateSearchRouteURL({ replace: false });
    }
}
function renderPreviewError(error) {
    els.previewDialogBody.className = "preview-modal-body";
    els.previewDialogBody.innerHTML = `<div class="alert alert-error m-4 text-sm">${escapeHTML(error.message || String(error))}</div>`;
}
function renderCodePreview(raw, language) {
    const normalized = supportedHighlightLanguage(normalizeHighlightLanguage(language));
    return `<pre class="code-preview hljs"><code class="language-${escapeHTML(normalized || "plaintext")}">${escapeHTML(raw)}</code></pre>`;
}
function scrollPreviewToLine(line) {
    if (!line)
        return;
    requestAnimationFrame(() => {
        const target = els.previewDialogBody.querySelector(`[data-line="${line}"]`);
        if (!target)
            return;
        target.classList.add("code-line-target");
        target.scrollIntoView({ block: "center" });
    });
}
function decorateCodePreviewLines(root) {
    root.querySelectorAll(".code-preview code").forEach((code) => {
        if (code.dataset.lines === "yes")
            return;
        code.innerHTML = code.innerHTML
            .split("\n")
            .map((line, index) => `<span class="code-line" data-line="${index + 1}"><span class="code-line-number">${index + 1}</span><span class="code-line-content">${line || " "}</span></span>`)
            .join("\n");
        code.dataset.lines = "yes";
    });
}
function decorateMarkdownSourceLines(root, raw) {
    const ranges = markdownSourceRanges(raw);
    const blocks = [...root.children].filter((node) => !node.classList.contains("diagram-surface"));
    blocks.forEach((node, index) => {
        const range = ranges[index];
        if (!range)
            return;
        node.dataset.sourceLineStart = String(range.start);
        node.dataset.sourceLineEnd = String(range.end);
    });
}
function markdownSourceRanges(raw) {
    const lines = String(raw || "").split("\n");
    const ranges = [];
    let index = 0;
    while (index < lines.length) {
        while (index < lines.length && !lines[index].trim())
            index++;
        if (index >= lines.length)
            break;
        const start = index + 1;
        const trimmed = lines[index].trim();
        if (trimmed.startsWith("```") || trimmed.startsWith("~~~")) {
            const fence = trimmed.slice(0, 3);
            index++;
            while (index < lines.length && !lines[index].trim().startsWith(fence))
                index++;
            if (index < lines.length)
                index++;
            ranges.push({ start, end: index });
            continue;
        }
        if (trimmed.startsWith("|")) {
            index++;
            while (index < lines.length && lines[index].trim().startsWith("|"))
                index++;
            ranges.push({ start, end: index });
            continue;
        }
        index++;
        while (index < lines.length && lines[index].trim())
            index++;
        ranges.push({ start, end: index });
    }
    return ranges;
}
function updateRawMarkdownToggle(spec = state.currentSpec) {
    if (!els.rawMarkdownToggle)
        return;
    const language = spec ? spec.language || languageFromPath(spec.path || "") : "";
    const available = language === "markdown";
    els.rawMarkdownToggle.hidden = !available;
    els.rawMarkdownToggle.classList.toggle("btn-active", available && state.showRawMarkdown);
    els.rawMarkdownToggle.setAttribute("aria-pressed", available && state.showRawMarkdown ? "true" : "false");
    els.rawMarkdownToggle.setAttribute("aria-label", state.showRawMarkdown ? "View rendered Markdown" : "View raw Markdown");
    els.rawMarkdownToggle.setAttribute("title", state.showRawMarkdown ? "View rendered Markdown" : "View raw Markdown");
    els.rawMarkdownToggle.innerHTML = state.showRawMarkdown
        ? '<i data-lucide="file-text" class="h-4 w-4"></i>'
        : '<i data-lucide="file-code" class="h-4 w-4"></i>';
    refreshIcons();
}
function renderMarkdown(raw) {
    if (raw) {
        const metadata = renderableMarkdownMetadata(raw);
        return DOMPurify.sanitize(`${metadata.html}${markdownRenderer.render(metadata.body)}`);
    }
    return "<p>No content.</p>";
}
function renderableMarkdownMetadata(raw) {
    const lines = String(raw || "").split("\n");
    if (lines[0]?.trim() !== "---") {
        return { html: "", body: raw };
    }
    const end = lines.slice(1).findIndex((line) => line.trim() === "---");
    if (end < 0) {
        return { html: "", body: raw };
    }
    const metadataLines = lines.slice(1, end + 1);
    const rows = markdownMetadataRows(metadataLines);
    if (!rows.length) {
        return { html: "", body: raw };
    }
    return {
        html: renderMetadataTable(rows),
        body: lines.slice(end + 2).join("\n"),
    };
}
function markdownMetadataRows(lines) {
    const rows = [];
    let current = null;
    lines.forEach((line) => {
        const keyValue = line.match(/^([A-Za-z0-9_.-]+):\s*(.*)$/);
        if (keyValue) {
            current = { key: keyValue[1], value: keyValue[2].trim() };
            rows.push(current);
            return;
        }
        const listItem = line.match(/^\s*-\s+(.*)$/);
        if (listItem && current) {
            current.value = appendMetadataValue(current.value, listItem[1].trim());
            return;
        }
        const continuation = line.trim();
        if (continuation && current) {
            current.value = appendMetadataValue(current.value, continuation);
        }
    });
    return rows.filter((row) => row.key);
}
function renderMetadataTable(rows) {
    const body = rows.map((row) => `<tr><th>${escapeHTML(row.key)}</th><td>${renderMetadataValue(row.value)}</td></tr>`).join("");
    return `<table class="metadata-table"><thead><tr><th>Metadata</th><th>Value</th></tr></thead><tbody>${body}</tbody></table>\n`;
}
function renderMetadataValue(raw) {
    const values = metadataArrayValues(raw);
    if (values.length) {
        return `<span class="metadata-badges">${values.map((value) => `<span class="badge badge-ghost badge-sm">${escapeHTML(value)}</span>`).join("")}</span>`;
    }
    return escapeHTML(cleanMetadataScalar(raw));
}
function metadataArrayValues(raw) {
    const value = String(raw || "").trim();
    if (!value)
        return [];
    if (value.startsWith("[") && value.endsWith("]")) {
        try {
            const parsed = JSON.parse(value);
            if (Array.isArray(parsed)) {
                return parsed.map((item) => cleanMetadataScalar(String(item))).filter(Boolean);
            }
        }
        catch {
            return value.slice(1, -1).split(",").map(cleanMetadataScalar).filter(Boolean);
        }
    }
    return [];
}
function cleanMetadataScalar(value) {
    const trimmed = String(value || "").trim();
    if (trimmed.length >= 2 && ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'")))) {
        return trimmed.slice(1, -1);
    }
    return trimmed;
}
function appendMetadataValue(value, next) {
    if (!next)
        return value;
    return value ? `${value}, ${next}` : next;
}
function decorateInternalDocNavigation(root, spec) {
    // Internal docs navigation is attached after sanitization so generated anchors
    // use trusted DOM nodes while still routing through the SPA.
    decorateInternalDocLinks(root, spec);
    decorateInternalDocMentions(root, spec);
}
function decorateInternalDocLinks(root, spec) {
    root.querySelectorAll("a[href]").forEach((link) => {
        const target = resolveSpecNavigationTarget(link.getAttribute("href") || "", spec.path);
        if (!target)
            return;
        configureInternalSpecLink(link, target);
    });
}
function decorateInternalDocMentions(root, spec) {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
        acceptNode: (node) => {
            const parent = node.parentElement;
            if (!parent || parent.closest("a, pre, code, script, style")) {
                return NodeFilter.FILTER_REJECT;
            }
            return internalDocMentionPattern().test(node.textContent || "") ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
        },
    });
    const nodes = [];
    while (walker.nextNode()) {
        nodes.push(walker.currentNode);
    }
    nodes.forEach((node) => replaceInternalDocMentions(node, spec));
}
function replaceInternalDocMentions(node, spec) {
    const text = node.textContent || "";
    const pattern = internalDocMentionPattern();
    const fragment = document.createDocumentFragment();
    let cursor = 0;
    let changed = false;
    for (const match of text.matchAll(pattern)) {
        const raw = match[0];
        const index = match.index || 0;
        const target = resolveSpecNavigationTarget(raw, spec.path);
        if (!target)
            continue;
        fragment.append(document.createTextNode(text.slice(cursor, index)));
        fragment.append(createInternalSpecAnchor(raw, target));
        cursor = index + raw.length;
        changed = true;
    }
    if (!changed)
        return;
    fragment.append(document.createTextNode(text.slice(cursor)));
    node.replaceWith(fragment);
}
function internalDocMentionPattern() {
    return /@(?:doc|spec)\/[A-Za-z0-9_./-]+(?:#[A-Za-z0-9_-]+)?|(?:\.{1,2}\/|docs\/|specs\/)?[A-Za-z0-9_./-]+\.md(?:#[A-Za-z0-9_-]+)?/g;
}
function createInternalSpecAnchor(label, target) {
    const link = document.createElement("a");
    link.textContent = label;
    configureInternalSpecLink(link, target);
    return link;
}
function configureInternalSpecLink(link, target) {
    link.href = specRoutePath(target.specId, target.fragment);
    link.dataset.internalSpecLink = target.specId;
    if (target.fragment) {
        link.dataset.internalSpecFragment = target.fragment;
    }
    link.addEventListener("click", (event) => {
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0)
            return;
        event.preventDefault();
        navigateToSpecTarget(target).catch((error) => renderPreviewError(error));
    });
}
function resolveSpecNavigationTarget(value, sourcePath) {
    const parsed = parseInternalSpecTargetValue(value);
    if (!parsed)
        return null;
    const lookup = buildSpecLookup();
    for (const candidate of specPathCandidates(parsed.path, sourcePath)) {
        const spec = lookup.get(candidate);
        if (spec) {
            return { specId: spec.id, fragment: parsed.fragment };
        }
    }
    return null;
}
function parseInternalSpecTargetValue(value) {
    let target = String(value || "").trim();
    if (!target || target.startsWith("#") || isExternalHref(target))
        return null;
    if (target.startsWith("@doc/") || target.startsWith("@spec/")) {
        target = target.replace(/^@(doc|spec)\//, "");
    }
    if (target.startsWith("/spec/")) {
        const [routePath, fragment = ""] = target.slice("/spec/".length).split("#", 2);
        return { path: decodeURIComponent(routePath), fragment };
    }
    const hashIndex = target.indexOf("#");
    const fragment = hashIndex >= 0 ? target.slice(hashIndex + 1) : "";
    const path = hashIndex >= 0 ? target.slice(0, hashIndex) : target;
    if (!path || path.includes("://"))
        return null;
    return { path: decodeURIComponent(path), fragment };
}
function isExternalHref(value) {
    return /^[a-z][a-z0-9+.-]*:/i.test(value) || value.startsWith("//");
}
function buildSpecLookup() {
    const lookup = new Map();
    state.specs.forEach((spec) => {
        for (const alias of specAliases(spec)) {
            const key = normalizeSpecLookupKey(alias);
            if (key && !lookup.has(key)) {
                lookup.set(key, spec);
            }
        }
    });
    return lookup;
}
function specAliases(spec) {
    const pathNoExt = spec.path.replace(/\.md$/i, "");
    const basename = spec.path.split("/").pop() || spec.path;
    const title = (spec.title || "").trim().toLowerCase();
    return [
        spec.id,
        spec.path,
        `docs/${spec.path}`,
        `specs/${spec.path}`,
        pathNoExt,
        pathNoExt.replace(/-/g, "."),
        pathNoExt.replace(/\./g, "-"),
        basename,
        basename.replace(/\.md$/i, ""),
        basename.replace(/\.md$/i, "").replace(/-/g, "."),
        title,
        title.replace(/\s+/g, "-"),
        title.replace(/\s+/g, "."),
        slugifySpecText(spec.title || ""),
        slugifySpecText((spec.title || "").replace(/\s+/g, ".")),
    ];
}
function specPathCandidates(path, sourcePath) {
    const candidates = new Set();
    const add = (candidate) => {
        const key = normalizeSpecLookupKey(candidate);
        if (!key)
            return;
        candidates.add(key);
        if (!key.endsWith(".md") && !key.includes(".")) {
            candidates.add(`${key}.md`);
            candidates.add(`${key}/_overview.md`);
        }
    };
    add(path);
    if (!path.startsWith("/")) {
        add(joinSpecPath(sourcePath.split("/").slice(0, -1).join("/"), path));
    }
    return [...candidates];
}
function normalizeSpecLookupKey(value) {
    let key = String(value || "").trim();
    if (!key)
        return "";
    key = key.replace(/^@(doc|spec)\//, "");
    key = key.split(/[?#]/, 1)[0] || "";
    key = key.replace(/^\/+/, "");
    key = key.replace(/^\.\//, "");
    key = key.replace(/^docs\//, "");
    key = key.replace(/^specs\//, "");
    key = normalizePathSegments(key);
    return key === "." ? "" : key.toLowerCase();
}
function joinSpecPath(base, target) {
    if (!base || target.startsWith("/") || target.startsWith("docs/") || target.startsWith("specs/")) {
        return target;
    }
    return `${base}/${target}`;
}
function normalizePathSegments(path) {
    const segments = [];
    path.split("/").forEach((segment) => {
        if (!segment || segment === ".")
            return;
        if (segment === "..") {
            segments.pop();
            return;
        }
        segments.push(segment);
    });
    return segments.join("/");
}
function slugifySpecText(value) {
    return value
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "");
}
async function navigateToSpecTarget(target) {
    if (els.previewDialog?.classList.contains("modal-open")) {
        closePreviewDialog({ updateURL: false });
    }
    await selectSpec(target.specId, true, { updateURL: false });
    pushSpecRoute(target.specId, target.fragment);
    scrollToSpecFragment(target.fragment);
}
function highlightRenderedCode(root) {
    if (!window.hljs)
        return;
    root.querySelectorAll("pre code").forEach((block) => {
        if (block.classList.contains("mermaid") || block.classList.contains("language-mermaid"))
            return;
        if (block.dataset.highlighted === "yes" || block.classList.contains("is-highlighted"))
            return;
        try {
            window.hljs.highlightElement(block);
        }
        catch {
            block.dataset.highlighted = "yes";
        }
    });
}
function normalizeHighlightLanguage(lang) {
    const value = String(lang || "")
        .trim()
        .toLowerCase();
    const aliases = {
        cjs: "javascript",
        mjs: "javascript",
        js: "javascript",
        jsx: "javascript",
        ts: "typescript",
        tsx: "typescript",
        yml: "yaml",
        shell: "bash",
        sh: "bash",
        zsh: "bash",
        fish: "bash",
        docker: "dockerfile",
        dockerfile: "dockerfile",
        md: "markdown",
        vue: "xml",
        svelte: "xml",
        gql: "graphql",
        plaintext: "plaintext",
        text: "plaintext",
    };
    return aliases[value] || value;
}
function supportedHighlightLanguage(language) {
    if (!window.hljs || !language || window.hljs.getLanguage(language)) {
        return language || "plaintext";
    }
    return "plaintext";
}
function languageFromPath(path) {
    const lower = String(path || "").toLowerCase();
    if (lower.endsWith("dockerfile") || lower.endsWith("/dockerfile"))
        return "dockerfile";
    const ext = lower.split(".").pop();
    const map = {
        go: "go",
        js: "javascript",
        jsx: "jsx",
        ts: "typescript",
        tsx: "tsx",
        css: "css",
        scss: "scss",
        sass: "sass",
        html: "xml",
        json: "json",
        yaml: "yaml",
        yml: "yaml",
        toml: "toml",
        md: "markdown",
        py: "python",
        rb: "ruby",
        rs: "rust",
        java: "java",
        kt: "kotlin",
        swift: "swift",
        c: "c",
        h: "c",
        cpp: "cpp",
        hpp: "cpp",
        cs: "csharp",
        php: "php",
        sh: "bash",
        sql: "sql",
        xml: "xml",
        graphql: "graphql",
        gql: "graphql",
    };
    return map[ext] || "plaintext";
}
async function renderMermaidBlocks(root) {
    const blocks = [...root.querySelectorAll("pre > code.language-mermaid, pre > code.mermaid")];
    await Promise.all(blocks.map(async (block, index) => {
        const source = block.textContent.trim();
        if (!source)
            return;
        const host = document.createElement("div");
        const id = `mermaid-${state.selectedId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}-${++state.diagramSerial}`;
        await renderMermaidDiagram(host, id, source, "Mermaid", "Mermaid diagram", true);
        block.closest("pre").replaceWith(host);
    }));
}
async function renderMermaidDiagram(host, id, source, label, title, framed) {
    host.className = framed ? "mermaid diagram-surface my-5 rounded-lg border border-base-300 bg-base-100" : "mermaid diagram-surface";
    host.dataset.diagramId = id;
    host.dataset.diagramTitle = title;
    host.textContent = "Rendering diagram...";
    try {
        if (!window.mermaid) {
            throw new Error("Mermaid library is not loaded");
        }
        window.mermaid.initialize({
            startOnLoad: false,
            theme: state.theme === "dark" ? "dark" : "default",
            securityLevel: "strict",
        });
        const result = await window.mermaid.render(id, source);
        host.innerHTML = DOMPurify.sanitize(result.svg || "", diagramSanitizeConfig);
        decorateDiagram(host, id, title);
    }
    catch (error) {
        host.className = "alert alert-error my-2 text-sm";
        host.textContent = `${label} render failed: ${error.message || error}`;
    }
}
function decorateDiagram(host, id, title) {
    const svg = host.querySelector("svg");
    if (!svg)
        return;
    destroyDiagramPanZoom(id);
    const size = svgDiagramSize(svg);
    svg.classList.add("diagram-svg");
    svg.setAttribute("width", String(size.width));
    svg.setAttribute("height", String(size.height));
    svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
    svg.style.width = "100%";
    svg.style.height = "100%";
    svg.style.maxWidth = "none";
    prepareSvgPanZoomViewport(svg);
    const toolbar = document.createElement("div");
    toolbar.className = "diagram-toolbar";
    toolbar.innerHTML = `
    <div class="min-w-0 truncate text-xs font-medium text-base-content/70">${escapeHTML(title)}</div>
    <div class="flex shrink-0 items-center gap-1">
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="zoom-out" aria-label="Zoom out">
        <i data-lucide="zoom-out" class="h-4 w-4"></i>
      </button>
      <span class="diagram-zoom-level text-base-content/60 w-12 text-center text-xs tabular-nums">100%</span>
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="zoom-in" aria-label="Zoom in">
        <i data-lucide="zoom-in" class="h-4 w-4"></i>
      </button>
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="fit" aria-label="Fit diagram">
        <i data-lucide="scan" class="h-4 w-4"></i>
      </button>
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="reset" aria-label="Reset zoom">
        <i data-lucide="rotate-ccw" class="h-4 w-4"></i>
      </button>
    </div>
  `;
    const viewport = document.createElement("div");
    viewport.className = "diagram-viewport";
    viewport.tabIndex = 0;
    viewport.setAttribute("aria-label", `${title}. Scroll to zoom, drag to pan.`);
    viewport.append(svg);
    host.innerHTML = "";
    host.append(toolbar, viewport);
    registerDiagramPanZoom(id, svg, toolbar);
    refreshIcons();
}
function registerDiagramPanZoom(id, svg, toolbar) {
    state.diagramPanZoomTargets.set(id, { svg, toolbar });
    initDiagramPanZoom(id);
}
function initDiagramPanZoom(id) {
    if (state.diagramPanZoomInstances.has(id))
        return;
    const target = state.diagramPanZoomTargets.get(id);
    if (!target)
        return;
    const { svg, toolbar } = target;
    const zoomLevel = toolbar.querySelector(".diagram-zoom-level");
    const setZoomLevel = (instance) => {
        zoomLevel.textContent = `${Math.round(instance.getZoom() * 100)}%`;
    };
    requestAnimationFrame(() => {
        if (!window.svgPanZoom) {
            zoomLevel.textContent = "No zoom";
            return;
        }
        if (!diagramIsVisible(svg)) {
            zoomLevel.textContent = "Ready";
            return;
        }
        let instance;
        try {
            instance = window.svgPanZoom(svg, {
                viewportSelector: ".svg-pan-zoom_viewport",
                panEnabled: true,
                zoomEnabled: true,
                controlIconsEnabled: false,
                dblClickZoomEnabled: true,
                mouseWheelZoomEnabled: true,
                preventMouseEventsDefault: true,
                zoomScaleSensitivity: 0.4,
                minZoom: 0.2,
                maxZoom: 10,
                fit: true,
                contain: false,
                center: true,
                refreshRate: "auto",
                onZoom: () => instance && setZoomLevel(instance),
                onPan: () => instance && setZoomLevel(instance),
            });
        }
        catch {
            zoomLevel.textContent = "Static";
            return;
        }
        state.diagramPanZoomInstances.set(id, instance);
        if (!resetDiagramPanZoomView(instance, zoomLevel)) {
            destroyDiagramPanZoom(id);
            return;
        }
        toolbar.querySelector('[data-diagram-action="zoom-in"]').addEventListener("click", () => {
            runDiagramPanZoomAction(instance, zoomLevel, () => instance.zoomIn());
        });
        toolbar.querySelector('[data-diagram-action="zoom-out"]').addEventListener("click", () => {
            runDiagramPanZoomAction(instance, zoomLevel, () => instance.zoomOut());
        });
        toolbar.querySelector('[data-diagram-action="fit"]').addEventListener("click", () => {
            resetDiagramPanZoomView(instance, zoomLevel);
        });
        toolbar.querySelector('[data-diagram-action="reset"]').addEventListener("click", () => {
            runDiagramPanZoomAction(instance, zoomLevel, () => {
                instance.resetZoom();
                instance.resetPan();
            });
        });
    });
}
function resetDiagramPanZoomView(instance, zoomLevel) {
    return runDiagramPanZoomAction(instance, zoomLevel, () => {
        instance.resize();
        instance.fit();
        instance.center();
    });
}
function runDiagramPanZoomAction(instance, zoomLevel, action) {
    try {
        action();
        zoomLevel.textContent = `${Math.round(instance.getZoom() * 100)}%`;
        return true;
    }
    catch {
        zoomLevel.textContent = "Static";
        return false;
    }
}
function initVisibleDiagramPanZooms() {
    state.diagramPanZoomTargets.forEach((_, id) => initDiagramPanZoom(id));
}
function diagramIsVisible(svg) {
    const rect = svg.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0 && svg.getClientRects().length > 0;
}
function prepareSvgPanZoomViewport(svg) {
    if (svg.querySelector(":scope > .svg-pan-zoom_viewport"))
        return;
    const viewport = document.createElementNS("http://www.w3.org/2000/svg", "g");
    viewport.classList.add("svg-pan-zoom_viewport");
    const preservedTags = new Set(["defs", "style", "title", "desc"]);
    [...svg.childNodes].forEach((child) => {
        if (child.nodeType === Node.ELEMENT_NODE && preservedTags.has(child.tagName.toLowerCase())) {
            return;
        }
        viewport.append(child);
    });
    svg.append(viewport);
}
function svgDiagramSize(svg) {
    const viewBox = svg.viewBox?.baseVal;
    const attrWidth = parseFloat(svg.getAttribute("width") || "");
    const attrHeight = parseFloat(svg.getAttribute("height") || "");
    return {
        width: Math.max(1, viewBox?.width || attrWidth || 1000),
        height: Math.max(1, viewBox?.height || attrHeight || 700),
    };
}
function destroyDiagramPanZoom(id) {
    const instance = state.diagramPanZoomInstances.get(id);
    if (instance) {
        instance.destroy();
        state.diagramPanZoomInstances.delete(id);
    }
    state.diagramPanZoomTargets.delete(id);
}
function destroyDiagramsIn(root) {
    root.querySelectorAll?.(".diagram-surface[data-diagram-id]").forEach((node) => {
        if (node.dataset.diagramId) {
            destroyDiagramPanZoom(node.dataset.diagramId);
        }
    });
}
function switchTab(name, options = {}) {
    const updateURL = options.updateURL !== false;
    state.tab = name;
    setPageChromeForTab(name);
    document.querySelectorAll(".tab[data-tab]").forEach((tab) => tab.classList.toggle("tab-active", tab.dataset.tab === name));
    document.querySelectorAll(".panel").forEach((panel) => panel.classList.remove("active"));
    document.querySelector(`#${name}Tab`).classList.add("active");
    requestAnimationFrame(initVisibleDiagramPanZooms);
    if (name === "graph") {
        requestAnimationFrame(graphView.render);
    }
    if (name === "search") {
        renderSearchPanels();
        if (els.globalSearch?.value.trim() && !state.searchData) {
            scheduleSearch();
        }
    }
    if (updateURL) {
        updateRouteURL(name);
    }
}
function routeFromLocation() {
    const routePath = decodeURIComponent(window.location.pathname).replace(/^\/+/, "");
    const params = new URLSearchParams(window.location.search);
    const queryRoute = {
        fragment: decodeURIComponent(window.location.hash.replace(/^#/, "")),
        searchQuery: params.get("q") || "",
        previewType: normalizePreviewType(params.get("preview")),
        previewPath: params.get("path") || "",
        previewLine: Number(params.get("line") || 0),
    };
    const [tab = "", ...rest] = routePath.split("/");
    if (tab === "graph") {
        return { tab, ...queryRoute };
    }
    if (tab === "search") {
        return { tab, ...queryRoute };
    }
    if (tab === "spec") {
        return { tab: "spec", spec: rest.join("/"), ...queryRoute };
    }
    const queryTab = params.get("tab") || "";
    const querySpec = params.get("spec") || "";
    if (["graph", "search", "spec"].includes(queryTab)) {
        return { tab: queryTab, spec: querySpec, ...queryRoute };
    }
    return queryRoute;
}
function updateRouteURL(tab) {
    if (state.applyingRoute)
        return;
    const route = tab === "spec" ? specRoutePath(state.selectedId || defaultSpecId()) : `/${tab}`;
    const query = buildRouteQuery(tab);
    const next = `${route}${query}`;
    const current = `${window.location.pathname}${window.location.search}`;
    if (next !== current) {
        window.history.pushState({ tab, spec: state.selectedId }, "", next);
    }
    syncRouteSpecFromURL();
    renderSpecList();
}
function updateSearchRouteURL(options = {}) {
    if (state.applyingRoute || state.tab !== "search")
        return;
    const route = `/search${buildRouteQuery("search")}`;
    const current = `${window.location.pathname}${window.location.search}`;
    if (route === current)
        return;
    const method = options.replace ? "replaceState" : "pushState";
    window.history[method]({ tab: "search", spec: state.selectedId }, "", route);
    syncRouteSpecFromURL();
    renderSpecList();
}
function pushSpecRoute(specId, fragment = "") {
    const next = specRoutePath(specId, fragment);
    const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    if (next !== current) {
        window.history.pushState({ tab: "spec", spec: specId }, "", next);
    }
    syncRouteSpecFromURL();
    renderSpecList();
}
function replaceSpecRoute(specId, fragment = "") {
    const next = specRoutePath(specId, fragment);
    const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    if (next !== current) {
        window.history.replaceState({ tab: "spec", spec: specId }, "", next);
    }
    syncRouteSpecFromURL();
    renderSpecList();
}
function syncRouteSpecFromURL(route = routeFromLocation()) {
    state.routeSpecId = validSpecId(route.spec) || "";
}
function specRoutePath(specId, fragment = "") {
    const hash = fragment ? `#${encodeURIComponent(fragment)}` : "";
    return `/spec/${encodeSpecPath(specId)}${hash}`;
}
function buildRouteQuery(tab) {
    const params = new URLSearchParams();
    if (tab === "search") {
        const query = els.globalSearch?.value.trim() || "";
        if (query) {
            params.set("q", query);
        }
    }
    if (tab === "search" && state.previewRoute?.type && state.previewRoute?.path) {
        params.set("preview", state.previewRoute.type);
        params.set("path", state.previewRoute.path);
        if (state.previewRoute.line) {
            params.set("line", String(state.previewRoute.line));
        }
    }
    const query = params.toString();
    return query ? `?${query}` : "";
}
function applySearchRoute(route) {
    if (typeof route.searchQuery === "string" && els.globalSearch) {
        els.globalSearch.value = route.searchQuery;
    }
}
async function applyPreviewRoute(route) {
    if (route.previewType === "doc" && route.previewPath) {
        await openSpecPreview(route.previewPath, { updateURL: false });
        return;
    }
    if (route.previewType === "file" && route.previewPath) {
        await openFilePreview(route.previewPath, route.previewLine, { updateURL: false });
        return;
    }
    if (els.previewDialog?.classList.contains("modal-open")) {
        closePreviewDialog({ updateURL: false });
    }
}
function normalizePreviewType(type) {
    const value = String(type || "")
        .trim()
        .toLowerCase();
    return value === "doc" || value === "file" ? value : "";
}
function setPageChromeForTab(name) {
    const tabTitle = pageTitleForTab(name);
    els.pageTitle.textContent = tabTitle.title;
    els.pagePath.textContent = tabTitle.path;
    updateDocumentTitle();
}
function pageTitleForTab(name) {
    if (name === "graph") {
        return { title: "Docs Graph", path: state.project?.docsRoot || "" };
    }
    if (name === "search") {
        const query = els.globalSearch?.value.trim() || state.searchData?.query || "";
        return {
            title: query ? `Search: ${query}` : "Search",
            path: "Docs, code, and graph context",
        };
    }
    if (name === "spec") {
        const spec = state.currentSpec || state.specs.find((item) => item.id === state.selectedId);
        return { title: spec?.title || "Doc", path: spec?.path || state.selectedId || "" };
    }
    const spec = state.currentSpec || state.specs.find((item) => item.id === state.selectedId);
    return { title: spec?.title || "Doc", path: spec?.path || state.selectedId || "" };
}
function updateDocumentTitle() {
    const page = pageTitleForTab(state.tab || "spec").title;
    const parts = [state.previewTitle, page, state.project?.name, state.project?.generatedTitle || "Docs Preview"].filter(Boolean);
    document.title = dedupeTitleParts(parts).join(" | ");
}
function dedupeTitleParts(parts) {
    const seen = new Set();
    return parts.filter((part) => {
        if (seen.has(part))
            return false;
        seen.add(part);
        return true;
    });
}
function encodeSpecPath(path) {
    return path.split("/").map(encodeURIComponent).join("/");
}
function scrollToSpecFragment(fragment) {
    if (!fragment)
        return;
    requestAnimationFrame(() => {
        const target = findFragmentTarget(els.specContent, fragment) || findFragmentTarget(els.previewDialogBody, fragment);
        target?.scrollIntoView({ block: "start" });
    });
}
function findFragmentTarget(root, fragment) {
    if (!root)
        return null;
    const decoded = decodeURIComponent(fragment);
    const escaped = cssEscape(decoded);
    const direct = root.querySelector(`#${escaped}, a[name="${escaped}"]`);
    if (direct)
        return direct;
    const wanted = slugifySpecText(decoded);
    return [...root.querySelectorAll("h1, h2, h3, h4, h5, h6")].find((heading) => slugifySpecText(heading.textContent || "") === wanted);
}
function cssEscape(value) {
    return window.CSS?.escape ? window.CSS.escape(value) : value.replace(/["\\#.;?+*~':!^$[\]()=>|/@]/g, "\\$&");
}
function updateSelectionContextMenu() {
    const selection = window.getSelection();
    if (!selection || selection.isCollapsed || selection.rangeCount === 0) {
        hideSelectionContextMenu();
        return;
    }
    const target = resolveSelectionCopyTarget(selection);
    if (!target) {
        hideSelectionContextMenu();
        return;
    }
    state.selectionCopyTarget = target;
    const rect = selection.getRangeAt(0).getBoundingClientRect();
    if (!rect.width && !rect.height) {
        hideSelectionContextMenu();
        return;
    }
    const menu = els.selectionContextMenu;
    menu.hidden = false;
    const left = Math.max(8, Math.min(window.innerWidth - menu.offsetWidth - 8, rect.left + rect.width / 2 - menu.offsetWidth / 2));
    const top = Math.max(8, Math.min(window.innerHeight - menu.offsetHeight - 8, rect.bottom + 8));
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
    els.selectionCopyButton.innerHTML = '<i data-lucide="copy" class="h-3.5 w-3.5"></i><span>Copy filepath and line index</span>';
    refreshIcons();
}
function resolveSelectionCopyTarget(selection) {
    const range = selection.getRangeAt(0);
    const root = selectionPreviewRoot(range.commonAncestorContainer);
    if (!root)
        return null;
    const path = root.dataset.sourcePath || state.currentSpec?.path || "";
    if (!path)
        return null;
    const lines = selectedSourceLines(root, range);
    if (!lines.length)
        return null;
    return { path, start: Math.min(...lines), end: Math.max(...lines) };
}
function selectionPreviewRoot(node) {
    const element = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
    return element?.closest?.("#specContent, #previewDialogBody") || null;
}
function selectedSourceLines(root, range) {
    const lines = new Set();
    [range.startContainer, range.endContainer].forEach((node) => addSourceLinesFromNode(lines, node));
    root.querySelectorAll("[data-line], [data-source-line-start]").forEach((node) => {
        try {
            if (range.intersectsNode(node))
                addSourceLinesFromNode(lines, node);
        }
        catch {
            // Ignore browser edge cases where intersectsNode rejects detached nodes.
        }
    });
    return [...lines].filter((line) => Number.isFinite(line) && line > 0);
}
function addSourceLinesFromNode(lines, node) {
    const element = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
    const line = element?.closest?.("[data-line]");
    if (line?.dataset.line) {
        lines.add(Number(line.dataset.line));
        return;
    }
    const block = element?.closest?.("[data-source-line-start]");
    if (!block)
        return;
    const start = Number(block.dataset.sourceLineStart || 0);
    const end = Number(block.dataset.sourceLineEnd || start);
    for (let current = start; current <= end; current++) {
        lines.add(current);
    }
}
function hideSelectionContextMenu() {
    state.selectionCopyTarget = null;
    if (els.selectionContextMenu) {
        els.selectionContextMenu.hidden = true;
    }
}
async function copySelectionReference() {
    const target = state.selectionCopyTarget;
    if (!target)
        return;
    const text = `${target.path}:${target.start}-${target.end}`;
    await navigator.clipboard.writeText(text);
    els.selectionCopyButton.innerHTML = '<i data-lucide="check" class="h-3.5 w-3.5"></i><span>Copied</span>';
    refreshIcons();
    window.setTimeout(hideSelectionContextMenu, 650);
}
async function applyRouteFromLocation() {
    const route = routeFromLocation();
    const tab = route.tab || "spec";
    state.applyingRoute = true;
    try {
        syncRouteSpecFromURL(route);
        renderSpecList();
        applySearchRoute(route);
        if (route.spec && validSpecId(route.spec)) {
            await selectSpec(route.spec, false, { updateURL: false });
            scrollToSpecFragment(route.fragment || "");
        }
        else if (!route.tab && !route.spec && state.selectedId) {
            replaceSpecRoute(state.selectedId, route.fragment || "");
        }
        switchTab(tab, { updateURL: false });
        await applyPreviewRoute(route);
    }
    finally {
        state.applyingRoute = false;
    }
}
function escapeHTML(value) {
    return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}
document.querySelectorAll(".tab[data-tab]").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
window.addEventListener("popstate", () => {
    applyRouteFromLocation().catch(() => { });
});
els.search?.addEventListener("input", renderSpecList);
els.graphSearch?.addEventListener("input", graphView.render);
els.graphFit?.addEventListener("click", graphView.fit);
els.globalSearch?.addEventListener("input", scheduleSearch);
els.codeGraphReload?.addEventListener("click", () => {
    reloadCodeGraph().catch((error) => {
        state.codeGraphLoading = false;
        updateCodeGraphReloadControl();
        renderSearchError(error);
    });
});
els.themeToggle.addEventListener("click", () => {
    applyTheme(state.theme === "dark" ? "light" : "dark", { persist: true, rerender: true });
});
els.rawMarkdownToggle?.addEventListener("click", () => {
    if (!state.currentSpec)
        return;
    state.showRawMarkdown = !state.showRawMarkdown;
    destroyDiagramsIn(els.specContent);
    renderCurrentSpecContent().catch(() => { });
});
document.addEventListener("mouseup", () => window.setTimeout(updateSelectionContextMenu, 0));
document.addEventListener("keyup", (event) => {
    if (event.key === "Escape") {
        hideSelectionContextMenu();
        return;
    }
    window.setTimeout(updateSelectionContextMenu, 0);
});
document.addEventListener("scroll", hideSelectionContextMenu, true);
els.selectionContextMenu?.addEventListener("mousedown", (event) => event.preventDefault());
els.selectionCopyButton?.addEventListener("click", () => {
    copySelectionReference().catch(() => hideSelectionContextMenu());
});
els.previewDialog?.querySelectorAll("[data-close-preview]").forEach((button) => {
    button.addEventListener("click", () => closePreviewDialog({ updateURL: true }));
});
function getInitialTheme() {
    const stored = localStorage.getItem("spec-preview-theme");
    if (stored === "dark" || stored === "light")
        return stored;
    return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}
function applyTheme(theme, options) {
    state.theme = theme;
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    if (options.persist) {
        localStorage.setItem("spec-preview-theme", theme);
    }
    updateThemeControl();
    if (options.rerender) {
        rerenderForTheme();
    }
}
function updateThemeControl() {
    const dark = state.theme === "dark";
    els.themeToggle.innerHTML = `
    <i data-lucide="${dark ? "sun" : "moon"}" class="h-4 w-4"></i>
  `;
    els.themeToggle.setAttribute("aria-label", dark ? "Switch to light mode" : "Switch to dark mode");
    els.themeToggle.setAttribute("title", dark ? "Switch to light mode" : "Switch to dark mode");
    refreshIcons();
}
function rerenderForTheme() {
    if (state.selectedId) {
        selectSpec(state.selectedId, false);
    }
    graphView.render();
}
function refreshIcons() {
    if (window.lucide) {
        lucide.createIcons();
    }
}
refreshIcons();
function connectHotReload() {
    if (!window.EventSource)
        return;
    const events = new EventSource("/api/events");
    events.addEventListener("ready", (event) => {
        const token = parseEventToken(event.data);
        if (state.hotReloadToken && token && token !== state.hotReloadToken) {
            window.location.reload();
            return;
        }
        state.hotReloadToken = token || state.hotReloadToken;
    });
    events.addEventListener("change", (event) => {
        const token = parseEventToken(event.data);
        state.hotReloadToken = token || state.hotReloadToken;
        reloadPreviewData().catch(() => window.location.reload());
    });
    events.addEventListener("error", () => {
        events.close();
        setTimeout(connectHotReload, 1500);
    });
}
function parseEventToken(value) {
    try {
        return JSON.parse(value);
    }
    catch {
        return value;
    }
}
connectHotReload();
load().catch((err) => {
    els.pageTitle.textContent = "Failed to load docs";
    els.pagePath.textContent = err.message;
    document.title = "Failed to load docs | Docs Preview";
});
