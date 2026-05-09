import { createDocsGraph } from "./js/graph.js";

const state = {
  project: null,
  specs: [],
  graph: null,
  graphSimulation: null,
  searchGraphSimulations: new Map(),
  searchGraphSelections: new Map(),
  graphSelectedId: "",
  searchData: null,
  searchController: null,
  searchLoading: false,
  codeGraphController: null,
  codeGraphLoading: false,
  searchTimer: null,
  hotReloadToken: "",
  theme: getInitialTheme(),
  diagramPanZoomInstances: new Map(),
  diagramPanZoomTargets: new Map(),
  expandedPaths: new Set(),
  selectedId: "",
  tab: "overview",
  applyingRoute: false,
  previewRoute: null,
  diagramSerial: 0,
};

const els = {
  projectName: document.querySelector("#projectName"),
  search: document.querySelector("#search"),
  specList: document.querySelector("#specList"),
  pageTitle: document.querySelector("#pageTitle"),
  pagePath: document.querySelector("#pagePath"),
  summaryCards: document.querySelector("#summaryCards"),
  syncState: document.querySelector("#syncState"),
  warnings: document.querySelector("#warnings"),
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
  themeLabel: document.querySelector("#themeLabel"),
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

applyTheme(state.theme, { persist: false, rerender: false });

const diagramSanitizeConfig = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  ADD_TAGS: ["foreignObject", "marker", "defs", "text", "tspan", "div", "span", "p", "br"],
  ADD_ATTR: ["viewBox", "xmlns", "d", "x", "y", "x1", "x2", "y1", "y2", "cx", "cy", "rx", "ry", "r", "points", "marker-end", "marker-start", "text-anchor", "dominant-baseline", "transform", "width", "height", "fill", "stroke", "stroke-width", "class", "id", "style", "dominant-baseline", "alignment-baseline"],
};

const graphView = createDocsGraph({ state, els, escapeHTML, refreshIcons, selectSpec });

async function load() {
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/docs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  const route = routeFromLocation();
  state.selectedId = validSpecId(route.spec) || defaultSpecId();
  applySearchRoute(route);
  renderOverview();
  graphView.render();
  renderSpecList();
  if (state.selectedId) {
    await selectSpec(state.selectedId, false);
  }
  switchTab(route.tab || "overview", { updateURL: false });
  await applyPreviewRoute(route);
}

async function reloadPreviewData() {
  const previousSelection = state.selectedId;
  const route = routeFromLocation();
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/docs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  state.selectedId = validSpecId(route.spec) || validSpecId(previousSelection) || defaultSpecId();
  applySearchRoute(route);
  renderOverview();
  graphView.render();
  renderSpecList();
  if (state.selectedId) {
    await selectSpec(state.selectedId, false);
  }
  switchTab(route.tab || state.tab || "overview", { updateURL: false });
  await applyPreviewRoute(route);
}

function defaultSpecId() {
  return state.specs.find((spec) => spec.path === "overview.md")?.id || state.specs[0]?.id || "";
}

function validSpecId(id) {
  if (!id) return "";
  return state.specs.some((spec) => spec.id === id) ? id : "";
}

async function fetchJSON(path) {
  const res = await fetch(path);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function renderOverview() {
  const project = state.project;
  els.projectName.textContent = project.name;
  els.summaryCards.innerHTML = "";
  [
    ["Specs", project.totalSpecs],
    ["Categories", Object.keys(project.categories || {}).length],
    ["Statuses", Object.keys(project.statusCounts || {}).length],
    ["Compliance", Object.keys(project.compliance || {}).length],
    ["Warnings", (project.warnings || []).length],
  ].forEach(([label, value]) => {
    const card = document.createElement("div");
    card.className = "card border border-base-300 bg-base-100 shadow-sm";
    card.innerHTML = `
      <div class="card-body gap-1 p-4">
        <div class="text-3xl font-bold">${escapeHTML(String(value))}</div>
        <div class="text-sm text-base-content/60">${label}</div>
      </div>
    `;
    els.summaryCards.append(card);
  });

  els.syncState.innerHTML = "";
  const syncEntries = Object.entries(project.sync || {});
  if (syncEntries.length === 0) {
    els.syncState.innerHTML = '<dt class="text-base-content/60">State</dt><dd>Unavailable</dd>';
  } else {
    syncEntries.forEach(([key, value]) => {
      const dt = document.createElement("dt");
      const dd = document.createElement("dd");
      dt.className = "text-base-content/60";
      dd.className = "break-words";
      dt.textContent = key;
      dd.textContent = value;
      els.syncState.append(dt, dd);
    });
  }

  els.warnings.innerHTML = "";
  const warnings = project.warnings || [];
  if (warnings.length === 0) {
    els.warnings.innerHTML = '<div class="alert alert-success py-3 text-sm">No structural warnings.</div>';
  } else {
    warnings.forEach((warning) => {
      const item = document.createElement("div");
      item.className = "alert alert-warning py-3 text-sm";
      item.textContent = warning;
      els.warnings.append(item);
    });
  }
}

function scheduleSearch() {
  window.clearTimeout(state.searchTimer);
  const query = els.globalSearch?.value.trim() || "";
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
  if (!els.globalSearch) return;
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
  if (!res.ok) throw new Error(await res.text());
  state.searchData = await res.json();
  state.searchLoading = false;
  renderSearchPanels();
}

function renderSearchPanels() {
  if (!els.searchSummary) return;
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
  if (!query || state.codeGraphLoading) return;
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
    if (!res.ok) throw new Error(await res.text());
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
  } catch (error) {
    if (error.name !== "AbortError") {
      state.codeGraphLoading = false;
      updateCodeGraphReloadControl();
      renderSearchPanel("codeGraph", panels.codeGraph || [], "No code graph matches.", false);
      els.searchSummary.innerHTML = `<div class="alert alert-error py-2 text-sm">${escapeHTML(error.message || String(error))}</div>`;
    }
  }
}

function updateCodeGraphReloadControl() {
  if (!els.codeGraphReload) return;
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
      ${warnings.slice(0, 2).map((warning) => `<span class="badge badge-warning badge-sm max-w-full truncate">${escapeHTML(warning)}</span>`).join("")}
    </div>
  `;
}

function renderSearchPanel(name, results, emptyText, loading) {
  const list = els[`${name}Results`];
  const count = els[`${name}Count`];
  if (!list || !count) return;
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
    if (!source || !target || source === target) return;
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
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function renderSearchResultGraph(name, graph, canvas, details) {
  if (!canvas || !details) return;
  if (!window.d3) {
    canvas.innerHTML = '<div class="alert alert-error m-4 text-sm">D3 library is not loaded.</div>';
    return;
  }
  stopSearchGraph(name);
  const selected = state.searchGraphSelections.get(name);
  const selectedNode = graph.nodes.find((node) => node.id === selected) || graph.nodes[0];
  if (selectedNode) {
    state.searchGraphSelections.set(name, selectedNode.id);
  }
  renderSearchGraphDetails(name, graph, details);

  canvas.innerHTML = "";
  const rect = canvas.getBoundingClientRect();
  const width = Math.max(480, rect.width || 720);
  const height = Math.max(320, rect.height || 420);
  const svg = d3
    .select(canvas)
    .append("svg")
    .attr("class", "docs-graph-svg search-graph-svg")
    .attr("viewBox", [-width / 2, -height / 2, width, height])
    .attr("role", "img");
  const viewport = svg.append("g");
  const zoom = d3.zoom().scaleExtent([0.35, 4]).on("zoom", (event) => viewport.attr("transform", event.transform));
  svg.call(zoom);

  const markerID = `search-graph-arrow-${name}`;
  svg
    .append("defs")
    .append("marker")
    .attr("id", markerID)
    .attr("viewBox", "0 -5 10 10")
    .attr("refX", 18)
    .attr("refY", 0)
    .attr("markerWidth", 6)
    .attr("markerHeight", 6)
    .attr("orient", "auto")
    .append("path")
    .attr("d", "M0,-5L10,0L0,5")
    .attr("fill", "currentColor")
    .attr("opacity", 0.45);

  const link = viewport
    .append("g")
    .attr("class", "graph-links")
    .selectAll("line")
    .data(graph.links)
    .join("line")
    .attr("stroke", (edge) => searchEdgeColor(edge.type))
    .attr("stroke-width", 1.5)
    .attr("stroke-dasharray", (edge) => (edge.confidence === "INFERRED" ? "4 4" : ""))
    .attr("marker-end", `url(#${markerID})`);

  const simulation = d3
    .forceSimulation(graph.nodes)
    .force("link", d3.forceLink(graph.links).id((node) => node.id).distance((edge) => (edge.type === "defines" ? 74 : 112)))
    .force("charge", d3.forceManyBody().strength(-360))
    .force("center", d3.forceCenter(0, 0))
    .force("collision", d3.forceCollide().radius(34));

  const node = viewport
    .append("g")
    .attr("class", "graph-nodes")
    .selectAll("g")
    .data(graph.nodes)
    .join("g")
    .attr("class", (item) => `graph-node ${item.id === selectedNode?.id ? "selected" : ""}`)
    .call(d3.drag().on("start", dragStarted).on("drag", dragged).on("end", dragEnded))
    .on("click", (_, item) => {
      state.searchGraphSelections.set(name, item.id);
      renderSearchGraphDetails(name, graph, details);
      node.classed("selected", (candidate) => candidate.id === item.id);
      if (name === "codeGraph") {
        const preview = codeGraphNodePreview(item);
        if (preview.path) {
          openFilePreview(preview.path, preview.line, { updateURL: true });
        }
      }
    });

  node
    .append("circle")
    .attr("r", (item) => (item.type === "file" || item.type === "doc-file" ? 8 : 9))
    .attr("fill", searchNodeColor)
    .attr("stroke", "hsl(var(--b1))")
    .attr("stroke-width", 2);

  node
    .append("text")
    .attr("x", 12)
    .attr("y", 4)
    .text((item) => item.label || item.id);

  node.append("title").text((item) => `${item.label || item.id}\n${item.path || item.id}`);

  simulation.on("tick", () => {
    link
      .attr("x1", (edge) => edge.source.x)
      .attr("y1", (edge) => edge.source.y)
      .attr("x2", (edge) => edge.target.x)
      .attr("y2", (edge) => edge.target.y);
    node.attr("transform", (item) => `translate(${item.x},${item.y})`);
  });
  state.searchGraphSimulations.set(name, simulation);

  requestAnimationFrame(() => {
    const bounds = viewport.node().getBBox?.();
    if (bounds && Number.isFinite(bounds.width) && bounds.width > 0) {
      fitSearchGraph(svg, zoom, bounds, width, height);
    }
  });

  function dragStarted(event) {
    if (!event.active) simulation.alphaTarget(0.25).restart();
    event.subject.fx = event.subject.x;
    event.subject.fy = event.subject.y;
  }

  function dragged(event) {
    event.subject.fx = event.x;
    event.subject.fy = event.y;
  }

  function dragEnded(event) {
    if (!event.active) simulation.alphaTarget(0);
    event.subject.fx = null;
    event.subject.fy = null;
  }
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

function codeGraphNodePreview(node) {
  return {
    path: node.previewPath || node.path || "",
    line: Number(node.previewLine || node.line || 0),
  };
}

function renderSearchGraphEdgeList(edges, side) {
  if (!edges.length) return '<div class="text-xs text-base-content/50">None</div>';
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

function fitSearchGraph(svg, zoom, bounds, width, height) {
  const fullWidth = bounds.width || width;
  const fullHeight = bounds.height || height;
  const scale = Math.max(0.35, Math.min(2.5, 0.82 / Math.max(fullWidth / width, fullHeight / height)));
  const tx = -scale * (bounds.x + fullWidth / 2);
  const ty = -scale * (bounds.y + fullHeight / 2);
  svg.transition().duration(300).call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
}

function stopSearchGraph(name) {
  const simulation = state.searchGraphSimulations.get(name);
  if (simulation) {
    simulation.stop();
    state.searchGraphSimulations.delete(name);
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
      ${tags.length ? `<div class="search-tags">${tags.slice(0, 5).map((tag) => `<span class="badge badge-ghost badge-xs">${escapeHTML(tag)}</span>`).join("")}</div>` : ""}
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
  if (result.path && panelName !== "docsSemantic") {
    buttons.push(`<button class="btn btn-ghost btn-xs" type="button" data-preview-file="${escapeHTML(result.path)}" data-preview-line="${escapeHTML(String(result.line || 0))}"><i data-lucide="file-code" class="h-3.5 w-3.5"></i>Preview file</button>`);
  }
  if (!buttons.length) return "";
  return `<div class="mt-2 flex flex-wrap gap-2">${buttons.join("")}</div>`;
}

function formatResultPath(result) {
  if (result.line) return `${result.path}:${result.line}`;
  return result.path;
}

function renderSearchNeighbors(neighbors) {
  if (!neighbors.length) return "";
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
  if (error.name === "AbortError") return;
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
  if (!node.children) return;
  node.children = new Map(
    [...node.children.entries()].sort(([, a], [, b]) => {
      if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
      return a.name.localeCompare(b.name);
    }),
  );
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
    } else {
      state.expandedPaths.add(node.path);
    }
    renderSpecList();
  });
  parent.append(button);
}

function renderFileNode(spec, parent, depth) {
  const button = document.createElement("button");
  button.className = [
    "tree-row btn btn-ghost btn-sm grid h-auto min-h-8 w-full grid-cols-[auto_minmax(0,1fr)_auto] justify-start gap-2 px-2 text-left font-normal",
    spec.id === state.selectedId ? "btn-active" : "",
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
  if (base === "_overview.md") return spec.title;
  return spec.title || base;
}

function autoExpandForSelection() {
  if (!state.selectedId) return;
  const parts = state.selectedId.split("/");
  for (let index = 1; index < parts.length; index++) {
    state.expandedPaths.add(parts.slice(0, index).join("/"));
  }
}

function expandAllVisibleFolders(node) {
  if (!node.children) return;
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
  els.pageTitle.textContent = spec.title;
  els.pagePath.textContent = spec.path;
  destroyDiagramsIn(els.specContent);
  els.specContent.innerHTML = renderMarkdown(spec.raw);
  await renderMermaidBlocks(els.specContent);
  highlightRenderedCode(els.specContent);
  renderSpecList();
  if (showSpecTab) {
    switchTab("spec", { updateURL });
  } else if (updateURL && state.tab === "spec") {
    updateRouteURL("spec");
  }
}

async function openSpecPreview(id, options = {}) {
  if (!id) return;
  openPreviewLoading("Loading doc", id);
  state.previewRoute = { type: "doc", path: id, line: 0 };
  if (options.updateURL !== false) {
    updateSearchRouteURL({ replace: false });
  }
  try {
    const spec = await fetchJSON(`/api/docs/${encodeURIComponent(id)}`);
    els.previewDialogTitle.textContent = spec.title || "Doc preview";
    els.previewDialogPath.textContent = spec.path || id;
    destroyDiagramsIn(els.previewDialogBody);
    els.previewDialogBody.className = "preview-modal-body markdown";
    els.previewDialogBody.innerHTML = renderMarkdown(spec.raw);
    await renderMermaidBlocks(els.previewDialogBody);
    highlightRenderedCode(els.previewDialogBody);
    refreshIcons();
  } catch (error) {
    renderPreviewError(error);
  }
}

async function openFilePreview(path, line, options = {}) {
  if (!path) return;
  openPreviewLoading("Loading file", path);
  state.previewRoute = { type: "file", path, line: Number(line || 0) };
  if (options.updateURL !== false) {
    updateSearchRouteURL({ replace: false });
  }
  try {
    const file = await fetchJSON(`/api/files?${new URLSearchParams({ path }).toString()}`);
    els.previewDialogTitle.textContent = file.title || "File preview";
    els.previewDialogPath.textContent = line ? `${file.path}:${line}` : file.path;
    destroyDiagramsIn(els.previewDialogBody);
    els.previewDialogBody.className = "preview-modal-body";
    els.previewDialogBody.innerHTML = renderCodePreview(file.raw || "", file.language || languageFromPath(file.path));
    highlightRenderedCode(els.previewDialogBody);
    decorateCodePreviewLines(els.previewDialogBody);
    scrollPreviewToLine(line);
    refreshIcons();
  } catch (error) {
    renderPreviewError(error);
  }
}

function openPreviewLoading(title, path) {
  destroyDiagramsIn(els.previewDialogBody);
  els.previewDialogTitle.textContent = title;
  els.previewDialogPath.textContent = path || "";
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
  destroyDiagramsIn(els.previewDialogBody);
  els.previewDialog.classList.remove("modal-open");
  state.previewRoute = null;
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
  if (!line) return;
  requestAnimationFrame(() => {
    const target = els.previewDialogBody.querySelector(`[data-line="${line}"]`);
    if (!target) return;
    target.classList.add("code-line-target");
    target.scrollIntoView({ block: "center" });
  });
}

function decorateCodePreviewLines(root) {
  root.querySelectorAll(".code-preview code").forEach((code) => {
    if (code.dataset.lines === "yes") return;
    code.innerHTML = code.innerHTML
      .split("\n")
      .map((line, index) => `<span class="code-line" data-line="${index + 1}"><span class="code-line-number">${index + 1}</span><span class="code-line-content">${line || " "}</span></span>`)
      .join("\n");
    code.dataset.lines = "yes";
  });
}

function renderMarkdown(raw) {
  if (raw) {
    return DOMPurify.sanitize(markdownRenderer.render(raw));
  }
  return "<p>No content.</p>";
}

function highlightRenderedCode(root) {
  if (!window.hljs) return;
  root.querySelectorAll("pre code").forEach((block) => {
    if (block.classList.contains("mermaid") || block.classList.contains("language-mermaid")) return;
    if (block.dataset.highlighted === "yes" || block.classList.contains("is-highlighted")) return;
    try {
      window.hljs.highlightElement(block);
    } catch {
      block.dataset.highlighted = "yes";
    }
  });
}

function normalizeHighlightLanguage(lang) {
  const value = String(lang || "").trim().toLowerCase();
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
  if (lower.endsWith("dockerfile") || lower.endsWith("/dockerfile")) return "dockerfile";
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
  await Promise.all(
    blocks.map(async (block, index) => {
      const source = block.textContent.trim();
      if (!source) return;
      const host = document.createElement("div");
      const id = `mermaid-${state.selectedId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}-${++state.diagramSerial}`;
      await renderMermaidDiagram(host, id, source, "Mermaid", "Mermaid diagram", true);
      block.closest("pre").replaceWith(host);
    }),
  );
}

async function renderMermaidDiagram(host, id, source, label, title, framed) {
  host.className = framed
    ? "mermaid diagram-surface my-5 rounded-lg border border-base-300 bg-base-100"
    : "mermaid diagram-surface";
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
  } catch (error) {
    host.className = "alert alert-error my-2 text-sm";
    host.textContent = `${label} render failed: ${error.message || error}`;
  }
}

function decorateDiagram(host, id, title) {
  const svg = host.querySelector("svg");
  if (!svg) return;
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
  if (state.diagramPanZoomInstances.has(id)) return;
  const target = state.diagramPanZoomTargets.get(id);
  if (!target) return;
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
    } catch (error) {
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
  } catch (error) {
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
  if (svg.querySelector(":scope > .svg-pan-zoom_viewport")) return;
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
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("tab-active", tab.dataset.tab === name));
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
    searchQuery: params.get("q") || "",
    previewType: normalizePreviewType(params.get("preview")),
    previewPath: params.get("path") || "",
    previewLine: Number(params.get("line") || 0),
  };
  const [tab = "", ...rest] = routePath.split("/");
  if (tab === "overview") {
    return { tab, ...queryRoute };
  }
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
  if (["overview", "graph", "search", "spec"].includes(queryTab)) {
    return { tab: queryTab, spec: querySpec, ...queryRoute };
  }
  return queryRoute;
}

function updateRouteURL(tab) {
  if (state.applyingRoute) return;
  const route = tab === "spec" ? `/spec/${encodeSpecPath(state.selectedId || defaultSpecId())}` : `/${tab}`;
  const query = buildRouteQuery(tab);
  const next = `${route}${query}`;
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.pushState({ tab, spec: state.selectedId }, "", next);
  }
}

function updateSearchRouteURL(options = {}) {
  if (state.applyingRoute || state.tab !== "search") return;
  const route = `/search${buildRouteQuery("search")}`;
  const current = `${window.location.pathname}${window.location.search}`;
  if (route === current) return;
  const method = options.replace ? "replaceState" : "pushState";
  window.history[method]({ tab: "search", spec: state.selectedId }, "", route);
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
  const value = String(type || "").trim().toLowerCase();
  return value === "doc" || value === "file" ? value : "";
}

function encodeSpecPath(path) {
  return path.split("/").map(encodeURIComponent).join("/");
}

async function applyRouteFromLocation() {
  const route = routeFromLocation();
  const tab = route.tab || "overview";
  state.applyingRoute = true;
  try {
    applySearchRoute(route);
    if (route.spec && validSpecId(route.spec)) {
      await selectSpec(route.spec, false, { updateURL: false });
    }
    switchTab(tab, { updateURL: false });
    await applyPreviewRoute(route);
  } finally {
    state.applyingRoute = false;
  }
}

function escapeHTML(value) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
window.addEventListener("popstate", () => {
  applyRouteFromLocation().catch(() => {});
});
els.search?.addEventListener("input", renderSpecList);
els.graphSearch?.addEventListener("input", graphView.render);
els.graphFit?.addEventListener("click", graphView.render);
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
els.previewDialog?.querySelectorAll("[data-close-preview]").forEach((button) => {
  button.addEventListener("click", () => closePreviewDialog({ updateURL: true }));
});
function getInitialTheme() {
  const stored = localStorage.getItem("spec-preview-theme");
  if (stored === "dark" || stored === "light") return stored;
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
    <span id="themeLabel" class="hidden sm:inline">${dark ? "Light" : "Dark"}</span>
  `;
  els.themeLabel = document.querySelector("#themeLabel");
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
  if (!window.EventSource) return;
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
  } catch {
    return value;
  }
}

connectHotReload();

load().catch((err) => {
  els.pageTitle.textContent = "Failed to load docs";
  els.pagePath.textContent = err.message;
});
