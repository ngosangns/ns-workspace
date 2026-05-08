const state = {
  project: null,
  specs: [],
  graph: null,
  graphInstance: null,
  expandedPaths: new Set(),
  selectedId: "",
  tab: "overview",
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
  relationships: document.querySelector("#relationships"),
};

const markdownRenderer = window.markdownit({
  html: false,
  linkify: true,
  typographer: false,
});

mermaid.initialize({
  startOnLoad: false,
  securityLevel: "strict",
  theme: "default",
});

async function load() {
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/specs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  state.selectedId = specs.find((spec) => spec.path === "overview.md")?.id || specs[0]?.id || "";
  renderFilters();
  renderOverview();
  renderSpecList();
  renderGraph();
  if (state.selectedId) {
    await selectSpec(state.selectedId, false);
  }
}

async function reloadPreviewData() {
  const previousSelection = state.selectedId;
  const [project, specs, graph] = await Promise.all([
    fetchJSON("/api/project"),
    fetchJSON("/api/specs"),
    fetchJSON("/api/graph"),
  ]);
  state.project = project;
  state.specs = specs;
  state.graph = graph;
  state.selectedId = specs.some((spec) => spec.id === previousSelection)
    ? previousSelection
    : specs.find((spec) => spec.path === "overview.md")?.id || specs[0]?.id || "";
  renderFilters();
  renderOverview();
  renderSpecList();
  renderGraph();
  if (state.selectedId) {
    await selectSpec(state.selectedId, false);
  }
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

async function selectSpec(id, showSpecTab) {
  const spec = await fetchJSON(`/api/specs/${encodeURIComponent(id)}`);
  state.selectedId = id;
  els.pageTitle.textContent = spec.title;
  els.pagePath.textContent = spec.path;
  els.specContent.innerHTML = renderMarkdown(spec.raw, spec.html);
  await renderMermaidBlocks(els.specContent);
  renderSpecList();
  highlightGraphNode(id);
  if (showSpecTab) switchTab("spec");
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
      host.className = "mermaid my-5 overflow-auto rounded-lg border border-base-300 bg-base-100 p-4";
      const id = `mermaid-${state.selectedId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}`;
      try {
        const { svg } = await mermaid.render(id, source);
        host.innerHTML = DOMPurify.sanitize(svg, { ADD_TAGS: ["svg", "g", "path", "rect", "circle", "ellipse", "line", "polyline", "polygon", "marker", "defs", "text", "tspan"], ADD_ATTR: ["viewBox", "d", "x", "y", "x1", "x2", "y1", "y2", "cx", "cy", "rx", "ry", "r", "points", "marker-end", "marker-start", "text-anchor", "dominant-baseline", "transform", "width", "height", "fill", "stroke", "stroke-width", "class", "id"] });
      } catch (error) {
        host.className = "alert alert-error my-5 text-sm";
        host.textContent = `Mermaid render failed: ${error.message || error}`;
      }
      block.closest("pre").replaceWith(host);
    }),
  );
}

function switchTab(name) {
  state.tab = name;
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("tab-active", tab.dataset.tab === name));
  document.querySelectorAll(".panel").forEach((panel) => panel.classList.remove("active"));
  document.querySelector(`#${name}Tab`).classList.add("active");
  if (name === "graph" && state.graphInstance) {
    state.graphInstance.resize();
    state.graphInstance.fit(undefined, 36);
  }
}

function renderGraph() {
  const { nodes, edges, relationships } = state.graph;
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
      },
    })),
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
          "border-color": "#ffffff",
          "border-width": 3,
          color: "#232529",
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
        style: { "background-color": "#dbeafe" },
      },
      {
        selector: "node[category = 'shared']",
        style: { "background-color": "#dcfce7" },
      },
      {
        selector: "node[category = 'compliance']",
        style: { "background-color": "#fef3c7" },
      },
      {
        selector: "node[category = 'docs']",
        style: { "background-color": "#fce7f3" },
      },
      {
        selector: "edge",
        style: {
          "curve-style": "bezier",
          "line-color": "#aab1ba",
          "target-arrow-color": "#aab1ba",
          "target-arrow-shape": "triangle",
          width: 1.4,
        },
      },
      {
        selector: ".selected",
        style: {
          "border-color": "#2563eb",
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
[els.search, els.categoryFilter, els.statusFilter, els.complianceFilter].forEach((el) => el.addEventListener("input", renderSpecList));

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
