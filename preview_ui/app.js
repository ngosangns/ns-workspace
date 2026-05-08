const state = {
  project: null,
  specs: [],
  graph: null,
  graphInstance: null,
  graphDiagramRenderId: 0,
  theme: getInitialTheme(),
  diagramPanZoomInstances: new Map(),
  diagramPanZoomTargets: new Map(),
  searchIndex: null,
  layoutSplit: null,
  renderingTree: false,
  expandedPaths: new Set(),
  selectedId: "",
  tab: "overview",
  applyingRoute: false,
};

const els = {
  projectName: document.querySelector("#projectName"),
  search: document.querySelector("#search"),
  categoryFilter: document.querySelector("#categoryFilter"),
  statusFilter: document.querySelector("#statusFilter"),
  complianceFilter: document.querySelector("#complianceFilter"),
  specList: document.querySelector("#specList"),
  pageTitle: document.querySelector("#pageTitle"),
  pagePath: document.querySelector("#pagePath"),
  summaryCards: document.querySelector("#summaryCards"),
  syncState: document.querySelector("#syncState"),
  warnings: document.querySelector("#warnings"),
  specContent: document.querySelector("#specContent"),
  graphDiagram: document.querySelector("#graphDiagram"),
  graphStats: document.querySelector("#graphStats"),
  graphCanvas: document.querySelector("#graphCanvas"),
  relationships: document.querySelector("#relationships"),
  constraints: document.querySelector("#constraints"),
  themeToggle: document.querySelector("#themeToggle"),
  themeLabel: document.querySelector("#themeLabel"),
};

const markdownRenderer = window.markdownit({
  html: false,
  linkify: true,
  typographer: false,
});

applyTheme(state.theme, { persist: false, rerender: false });

const diagramSanitizeConfig = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  ADD_TAGS: ["foreignObject", "marker", "defs", "text", "tspan", "div", "span", "p", "br"],
  ADD_ATTR: ["viewBox", "xmlns", "d", "x", "y", "x1", "x2", "y1", "y2", "cx", "cy", "rx", "ry", "r", "points", "marker-end", "marker-start", "text-anchor", "dominant-baseline", "transform", "width", "height", "fill", "stroke", "stroke-width", "class", "id", "style", "dominant-baseline", "alignment-baseline"],
};

async function load() {
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/specs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  buildSearchIndex();
  const route = routeFromLocation();
  state.selectedId = validSpecId(route.spec) || defaultSpecId();
  renderFilters();
  renderOverview();
  renderSpecList();
  renderGraph();
  if (state.selectedId) {
    await selectSpec(state.selectedId, false);
  }
  switchTab(route.tab || "overview", { updateURL: false });
}

async function reloadPreviewData() {
  const previousSelection = state.selectedId;
  const route = routeFromLocation();
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/specs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  buildSearchIndex();
  state.selectedId = validSpecId(route.spec) || validSpecId(previousSelection) || defaultSpecId();
  renderFilters();
  renderOverview();
  renderSpecList();
  renderGraph();
  if (state.selectedId) {
    await selectSpec(state.selectedId, false);
  }
  switchTab(route.tab || state.tab || "overview", { updateURL: false });
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

function renderFilters() {
  fillSelect(els.categoryFilter, "All categories", unique(state.specs.map((spec) => spec.category)));
  fillSelect(els.statusFilter, "All statuses", unique(state.specs.map((spec) => spec.status).filter(Boolean)));
  fillSelect(
    els.complianceFilter,
    "All compliance",
    unique(state.specs.map((spec) => spec.compliance).filter(Boolean)),
  );
  initFilterControls();
}

function buildSearchIndex() {
  if (!window.FlexSearch) {
    state.searchIndex = null;
    return;
  }
  const index = new FlexSearch.Document({
    tokenize: "forward",
    document: {
      id: "id",
      index: ["title", "path", "category", "status", "compliance", "priority"],
    },
  });
  state.specs.forEach((spec) => index.add({
    id: spec.id,
    title: spec.title || "",
    path: spec.path || "",
    category: spec.category || "",
    status: spec.status || "",
    compliance: spec.compliance || "",
    priority: spec.priority || "",
  }));
  state.searchIndex = index;
}

function fillSelect(select, label, values) {
  if (select.tomselect) {
    select.tomselect.destroy();
  }
  select.innerHTML = "";
  select.append(new Option(label, ""));
  values.forEach((value) => select.append(new Option(value, value)));
}

function initFilterControls() {
  if (!window.TomSelect) return;
  [els.categoryFilter, els.statusFilter, els.complianceFilter].forEach((select) => {
    new TomSelect(select, {
      allowEmptyOption: true,
      create: false,
      controlInput: null,
      hideSelected: false,
      plugins: ["clear_button"],
    });
  });
}

function unique(values) {
  return [...new Set(values)].sort((a, b) => a.localeCompare(b));
}

function renderOverview() {
  const project = state.project;
  els.projectName.textContent = project.name;
  els.summaryCards.innerHTML = "";
  [
    ["Specs", project.totalSpecs],
    ["Categories", Object.keys(project.categories || {}).length],
    ["Statuses", Object.keys(project.statusCounts || {}).length],
    ["Graph nodes", state.graph.nodes.length],
    ["Graph edges", state.graph.edges.length],
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

function renderSpecList() {
  const specs = filteredSpecs();
  const treeData = buildSpecTreeData(specs);
  state.renderingTree = true;
  const tree = $(els.specList);
  tree.off(".specTree");
  if (tree.jstree(true)) {
    tree.jstree("destroy");
  }
  tree
    .on("ready.jstree.specTree", () => {
      const instance = tree.jstree(true);
      openSelectedSpecParents(instance);
      if (els.search.value || els.categoryFilter.value || els.statusFilter.value || els.complianceFilter.value) {
        instance.open_all();
      }
      if (state.selectedId) {
        instance.deselect_all(true);
        instance.select_node(specTreeNodeId(state.selectedId), false, true);
      }
      state.renderingTree = false;
    })
    .on("select_node.jstree.specTree", (_event, data) => {
      if (state.renderingTree) return;
      const specId = data.node?.data?.specId;
      if (specId) {
        selectSpec(specId, true);
      }
    })
    .on("open_node.jstree.specTree", (_event, data) => {
      const path = String(data.node.id || "").replace(/^folder:/, "");
      if (path && path !== data.node.id) {
        state.expandedPaths.add(path);
      }
    })
    .on("close_node.jstree.specTree", (_event, data) => {
      const path = String(data.node.id || "").replace(/^folder:/, "");
      if (path && path !== data.node.id) {
        state.expandedPaths.delete(path);
      }
    })
    .jstree({
      core: {
        data: treeData,
        multiple: false,
        themes: {
          dots: false,
          icons: true,
          responsive: true,
        },
      },
      plugins: ["wholerow"],
    });
}

function filteredSpecs() {
  const query = els.search.value.trim();
  const category = els.categoryFilter.value;
  const status = els.statusFilter.value;
  const compliance = els.complianceFilter.value;
  const ids = query ? searchSpecIds(query) : null;
  return state.specs.filter((spec) => (
    (!ids || ids.has(spec.id)) &&
    (!category || spec.category === category) &&
    (!status || spec.status === status) &&
    (!compliance || spec.compliance === compliance)
  ));
}

function searchSpecIds(query) {
  if (!state.searchIndex) {
    const needle = query.toLowerCase();
    return new Set(state.specs.filter((spec) => `${spec.title} ${spec.path} ${spec.status} ${spec.compliance} ${spec.category} ${spec.priority}`.toLowerCase().includes(needle)).map((spec) => spec.id));
  }
  const ids = new Set();
  state.searchIndex.search(query, { enrich: true, suggest: true }).forEach((field) => {
    field.result.forEach((hit) => ids.add(hit.id));
  });
  return ids;
}

function buildSpecTreeData(specs) {
  const root = { children: new Map() };
  specs.forEach((spec) => {
    const parts = spec.path.split("/");
    let cursor = root;
    parts.forEach((part, index) => {
      const isFile = index === parts.length - 1;
      const path = parts.slice(0, index + 1).join("/");
      if (!cursor.children.has(part)) {
        cursor.children.set(part, isFile ? { text: displaySpecName(spec), path, type: "file", spec } : { text: part, path, type: "folder", children: new Map() });
      }
      cursor = cursor.children.get(part);
      if (isFile) {
        cursor.spec = spec;
      }
    });
  });
  return sortSpecTreeNodes(root.children).map(specTreeNode);
}

function sortSpecTreeNodes(children) {
  return [...children.values()].sort((a, b) => {
    if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
    return a.text.localeCompare(b.text);
  });
}

function specTreeNode(node) {
  if (node.type === "file") {
    return {
      id: specTreeNodeId(node.spec.id),
      text: node.spec.status ? `${node.text} · ${node.spec.status}` : node.text,
      icon: "jstree-file",
      data: { specId: node.spec.id },
    };
  }
  return {
    id: `folder:${node.path}`,
    text: node.text,
    state: { opened: state.expandedPaths.has(node.path) },
    children: sortSpecTreeNodes(node.children).map(specTreeNode),
  };
}

function specTreeNodeId(specId) {
  return `spec:${specId}`;
}

function openSelectedSpecParents(instance) {
  const spec = state.specs.find((item) => item.id === state.selectedId);
  if (!spec) return;
  const parts = spec.path.split("/");
  for (let index = 1; index < parts.length; index++) {
    const folderPath = parts.slice(0, index).join("/");
    state.expandedPaths.add(folderPath);
    instance.open_node(`folder:${folderPath}`);
  }
}

function displaySpecName(spec) {
  const base = spec.path.split("/").pop() || spec.title;
  if (base === "_overview.md") return spec.title;
  return spec.title || base;
}

async function selectSpec(id, showSpecTab, options = {}) {
  const updateURL = options.updateURL !== false;
  const spec = await fetchJSON(`/api/specs/${encodeURIComponent(id)}`);
  state.selectedId = id;
  els.pageTitle.textContent = spec.title;
  els.pagePath.textContent = spec.path;
  destroyDiagramsIn(els.specContent);
  els.specContent.innerHTML = renderMarkdown(spec.raw, spec.html);
  await renderMermaidBlocks(els.specContent);
  renderSpecList();
  highlightGraphNode(id);
  if (showSpecTab) {
    switchTab("spec", { updateURL });
  } else if (updateURL && state.tab === "spec") {
    updateRouteURL("spec");
  }
}

function renderMarkdown(raw, fallbackHTML) {
  if (fallbackHTML) {
    return DOMPurify.sanitize(fallbackHTML);
  }
  if (raw) {
    return DOMPurify.sanitize(markdownRenderer.render(raw));
  }
  return "<p>No content.</p>";
}

async function renderMermaidBlocks(root) {
  const blocks = [...root.querySelectorAll("pre > code.language-mermaid, pre > code.mermaid")];
  await Promise.all(
    blocks.map(async (block, index) => {
      const source = block.textContent.trim();
      if (!source) return;
      const host = document.createElement("div");
      const id = `mermaid-${state.selectedId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}`;
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
    const result = await postJSON("/api/render/mermaid", { source, theme: state.theme });
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
        zoomScaleSensitivity: 0.2,
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

async function postJSON(path, payload) {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function switchTab(name, options = {}) {
  const updateURL = options.updateURL !== false;
  state.tab = name;
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("tab-active", tab.dataset.tab === name));
  document.querySelectorAll(".panel").forEach((panel) => panel.classList.remove("active"));
  document.querySelector(`#${name}Tab`).classList.add("active");
  if (name === "graph" && state.graphInstance) {
    state.graphInstance.resize();
    state.graphInstance.fit(undefined, 36);
  }
  requestAnimationFrame(initVisibleDiagramPanZooms);
  if (updateURL) {
    updateRouteURL(name);
  }
}

function routeFromLocation() {
  const routePath = decodeURIComponent(window.location.pathname).replace(/^\/+/, "");
  const [tab = "", ...rest] = routePath.split("/");
  if (["overview", "graph"].includes(tab)) {
    return { tab };
  }
  if (tab === "spec") {
    return { tab: "spec", spec: rest.join("/") };
  }
  const params = new URLSearchParams(window.location.search);
  const queryTab = params.get("tab") || "";
  const querySpec = params.get("spec") || "";
  if (["overview", "graph", "spec"].includes(queryTab)) {
    return { tab: queryTab, spec: querySpec };
  }
  return {};
}

function updateRouteURL(tab) {
  if (state.applyingRoute) return;
  const route = tab === "spec" ? `/spec/${encodeSpecPath(state.selectedId || defaultSpecId())}` : `/${tab}`;
  const next = `${route}${window.location.search}`;
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.pushState({ tab, spec: state.selectedId }, "", next);
  }
}

function encodeSpecPath(path) {
  return path.split("/").map(encodeURIComponent).join("/");
}

async function applyRouteFromLocation() {
  const route = routeFromLocation();
  const tab = route.tab || "overview";
  state.applyingRoute = true;
  try {
    if (route.spec && validSpecId(route.spec)) {
      await selectSpec(route.spec, false, { updateURL: false });
    }
    switchTab(tab, { updateURL: false });
  } finally {
    state.applyingRoute = false;
  }
}

function renderGraph() {
  const { nodes, edges, relationships, constraints = [] } = state.graph;
  const palette = graphPalette();
  renderDependencyDiagram();
  els.graphStats.textContent = `${nodes.length} nodes, ${edges.length} dependencies, ${relationships.length} relationships, ${constraints.length} rules`;
  if (state.graphInstance) {
    state.graphInstance.destroy();
    state.graphInstance = null;
  }
  const relationshipEdges = relationships.map((rel, index) => ({
    data: {
      id: `relationship-${index}-${rel.from}-${rel.to}`,
      source: rel.from,
      target: rel.to,
      label: rel.section || "relationship",
      kind: "relationship",
    },
  }));
  const elements = [
    ...nodes.map((node) => ({
      data: {
        id: node.id,
        label: node.id,
        title: node.label,
        specId: node.specId || "",
        category: node.category || "unknown",
        status: node.status || "",
      },
    })),
    ...edges.map((edge, index) => ({
      data: {
        id: `edge-${index}-${edge.from}-${edge.to}`,
        source: edge.from,
        target: edge.to,
        label: edge.label || "",
        kind: "dependency",
      },
    })),
    ...relationshipEdges,
  ];

  state.graphInstance = cytoscape({
    container: els.graphCanvas,
    elements,
    wheelSensitivity: 0.18,
    minZoom: 0.22,
    maxZoom: 2.2,
    style: [
      {
        selector: "node",
        style: {
          "background-color": "mapData(category, root, versions, #e5e7eb, #e0f2fe)",
          "border-color": palette.nodeBorder,
          "border-width": 3,
          color: palette.text,
          label: "data(label)",
          "font-size": 12,
          "text-wrap": "wrap",
          "text-max-width": 120,
          "text-valign": "bottom",
          "text-margin-y": 10,
          height: 44,
          width: 44,
        },
      },
      {
        selector: "node[category = 'modules']",
        style: { "background-color": palette.modules },
      },
      {
        selector: "node[category = 'shared']",
        style: { "background-color": palette.shared },
      },
      {
        selector: "node[category = 'compliance']",
        style: { "background-color": palette.compliance },
      },
      {
        selector: "node[category = 'docs']",
        style: { "background-color": palette.docs },
      },
      {
        selector: "edge",
        style: {
          "curve-style": "bezier",
          "line-color": palette.edge,
          "target-arrow-color": palette.edge,
          "target-arrow-shape": "triangle",
          width: 1.4,
        },
      },
      {
        selector: "edge[kind = 'relationship']",
        style: {
          "line-style": "dashed",
          "line-color": palette.relationshipEdge,
          "target-arrow-color": palette.relationshipEdge,
          opacity: 0.72,
        },
      },
      {
        selector: ".selected",
        style: {
          "border-color": palette.selected,
          "border-width": 6,
        },
      },
    ],
    layout: {
      name: "cose",
      animate: false,
      fit: true,
      padding: 48,
      nodeRepulsion: 100000,
      idealEdgeLength: 96,
    },
  });

  state.graphInstance.on("tap", "node", (event) => {
    const specId = event.target.data("specId");
    if (specId) {
      selectSpec(specId, true);
    }
  });
  highlightGraphNode(state.selectedId);
  renderRelationships(relationships);
  renderConstraints(constraints);
}

async function renderDependencyDiagram() {
  const source = state.graph?.dependencyDiagram?.trim();
  const renderId = ++state.graphDiagramRenderId;
  destroyDiagramsIn(els.graphDiagram);
  els.graphDiagram.innerHTML = "";
  if (!source) {
    els.graphDiagram.innerHTML = '<div class="alert py-3 text-sm">No dependency diagram source found.</div>';
    return;
  }
  const host = document.createElement("div");
  els.graphDiagram.append(host);
  await renderMermaidDiagram(host, `dependency-graph-${renderId}`, source, "Dependency graph", "Dependency graph", false);
}

function graphPalette() {
  if (state.theme === "dark") {
    return {
      text: "#e5e7eb",
      edge: "#7b8494",
      relationshipEdge: "#a78bfa",
      nodeBorder: "#111827",
      selected: "#60a5fa",
      modules: "#1d4ed8",
      shared: "#15803d",
      compliance: "#b45309",
      docs: "#be185d",
    };
  }
  return {
    text: "#232529",
    edge: "#aab1ba",
    relationshipEdge: "#8b5cf6",
    nodeBorder: "#ffffff",
    selected: "#2563eb",
    modules: "#dbeafe",
    shared: "#dcfce7",
    compliance: "#fef3c7",
    docs: "#fce7f3",
  };
}

function renderRelationships(relationships) {
  els.relationships.innerHTML = "";
  relationships.slice(0, 160).forEach((rel) => {
    const item = document.createElement("div");
    item.className = "py-3 text-sm";
    item.innerHTML = `
      <div class="flex items-center gap-2 font-semibold">
        <span>${escapeHTML(rel.from)}</span>
        <i data-lucide="arrow-right" class="h-4 w-4 shrink-0 text-base-content/45"></i>
        <span>${escapeHTML(rel.to)}</span>
      </div>
      <div class="text-base-content/60">${escapeHTML(rel.description || rel.section || "")}</div>
    `;
    els.relationships.append(item);
  });
  if (relationships.length === 0) {
    els.relationships.innerHTML = '<p class="text-sm text-base-content/60">No relationship map entries found.</p>';
  }
  refreshIcons();
}

function renderConstraints(constraints) {
  els.constraints.innerHTML = "";
  constraints.forEach((rule) => {
    const item = document.createElement("div");
    item.className = "rounded-lg border border-error/25 bg-error/5 p-3 text-sm";
    item.innerHTML = `
      <div class="flex items-center gap-2 font-semibold text-error">
        <i data-lucide="ban" class="h-4 w-4 shrink-0"></i>
        <span>${escapeHTML(rule.from || "Forbidden")}</span>
        <i data-lucide="arrow-right" class="h-4 w-4 shrink-0"></i>
        <span>${escapeHTML(rule.to || "")}</span>
      </div>
      <div class="mt-1 text-base-content/65">${escapeHTML(rule.description || rule.raw || "")}</div>
    `;
    els.constraints.append(item);
  });
  if (constraints.length === 0) {
    els.constraints.innerHTML = '<p class="text-sm text-base-content/60">No forbidden dependency rules found.</p>';
  }
  refreshIcons();
}

function highlightGraphNode(specId) {
  if (!state.graphInstance) return;
  state.graphInstance.nodes().removeClass("selected");
  if (!specId) return;
  state.graphInstance.nodes().filter((node) => node.data("specId") === specId).addClass("selected");
}

function escapeHTML(value) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
window.addEventListener("popstate", () => {
  applyRouteFromLocation().catch(() => {});
});
[els.search, els.categoryFilter, els.statusFilter, els.complianceFilter].forEach((el) => {
  el.addEventListener("input", renderSpecList);
  el.addEventListener("change", renderSpecList);
});
els.themeToggle.addEventListener("click", () => {
  applyTheme(state.theme === "dark" ? "light" : "dark", { persist: true, rerender: true });
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
  if (state.graph) {
    renderGraph();
  }
  if (state.selectedId) {
    selectSpec(state.selectedId, false);
  }
}

function refreshIcons() {
  if (window.lucide) {
    lucide.createIcons();
  }
}

function initDocumentLayout() {
  if (!window.Split || state.layoutSplit || !window.matchMedia("(min-width: 1024px)").matches) return;
  state.layoutSplit = Split(["#sidebarPane", "#contentPane"], {
    sizes: [24, 76],
    minSize: [280, 520],
    gutterSize: 1,
    snapOffset: 0,
    onDragEnd: () => {
      if (state.graphInstance) {
        state.graphInstance.resize();
        state.graphInstance.fit(undefined, 36);
      }
      initVisibleDiagramPanZooms();
    },
  });
}

initDocumentLayout();
refreshIcons();

function connectHotReload() {
  if (!window.EventSource) return;
  const events = new EventSource("/api/events");
  events.addEventListener("change", () => {
    reloadPreviewData().catch(() => window.location.reload());
  });
  events.addEventListener("error", () => {
    events.close();
    setTimeout(connectHotReload, 1500);
  });
}

connectHotReload();

load().catch((err) => {
  els.pageTitle.textContent = "Failed to load specs";
  els.pagePath.textContent = err.message;
});
