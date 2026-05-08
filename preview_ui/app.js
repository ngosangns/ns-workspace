import { createDocsGraph } from "./js/graph.js";

const state = {
  project: null,
  specs: [],
  graph: null,
  graphSimulation: null,
  graphSelectedId: "",
  theme: getInitialTheme(),
  diagramPanZoomInstances: new Map(),
  diagramPanZoomTargets: new Map(),
  expandedPaths: new Set(),
  selectedId: "",
  tab: "overview",
  applyingRoute: false,
  diagramSerial: 0,
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
  graphCanvas: document.querySelector("#graphCanvas"),
  graphDetails: document.querySelector("#graphDetails"),
  graphSearch: document.querySelector("#graphSearch"),
  graphStats: document.querySelector("#graphStats"),
  graphFit: document.querySelector("#graphFit"),
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

const graphView = createDocsGraph({ state, els, escapeHTML, refreshIcons, selectSpec });

async function load() {
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/specs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  const route = routeFromLocation();
  state.selectedId = validSpecId(route.spec) || defaultSpecId();
  renderFilters();
  renderOverview();
  graphView.render();
  renderSpecList();
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
  state.selectedId = validSpecId(route.spec) || validSpecId(previousSelection) || defaultSpecId();
  renderFilters();
  renderOverview();
  graphView.render();
  renderSpecList();
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
}

function fillSelect(select, label, values) {
  select.innerHTML = "";
  select.append(new Option(label, ""));
  values.forEach((value) => select.append(new Option(value, value)));
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

function renderSpecList() {
  const query = els.search.value.toLowerCase().trim();
  const category = els.categoryFilter.value;
  const status = els.statusFilter.value;
  const compliance = els.complianceFilter.value;
  const specs = state.specs.filter((spec) => {
    const haystack = `${spec.title} ${spec.path} ${spec.status} ${spec.compliance}`.toLowerCase();
    return (
      (!query || haystack.includes(query)) &&
      (!category || spec.category === category) &&
      (!status || spec.status === status) &&
      (!compliance || spec.compliance === compliance)
    );
  });

  const tree = buildSpecTree(specs);
  autoExpandForSelection();
  if (query || category || status || compliance) {
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
  const spec = await fetchJSON(`/api/specs/${encodeURIComponent(id)}`);
  state.selectedId = id;
  els.pageTitle.textContent = spec.title;
  els.pagePath.textContent = spec.path;
  destroyDiagramsIn(els.specContent);
  els.specContent.innerHTML = renderMarkdown(spec.raw);
  await renderMermaidBlocks(els.specContent);
  renderSpecList();
  if (showSpecTab) {
    switchTab("spec", { updateURL });
  } else if (updateURL && state.tab === "spec") {
    updateRouteURL("spec");
  }
}

function renderMarkdown(raw) {
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
  if (updateURL) {
    updateRouteURL(name);
  }
}

function routeFromLocation() {
  const routePath = decodeURIComponent(window.location.pathname).replace(/^\/+/, "");
  const [tab = "", ...rest] = routePath.split("/");
  if (tab === "overview") {
    return { tab };
  }
  if (tab === "graph") {
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

function escapeHTML(value) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
window.addEventListener("popstate", () => {
  applyRouteFromLocation().catch(() => {});
});
[els.search, els.categoryFilter, els.statusFilter, els.complianceFilter].forEach((el) => el.addEventListener("input", renderSpecList));
els.graphSearch?.addEventListener("input", graphView.render);
els.graphFit?.addEventListener("click", graphView.render);
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
