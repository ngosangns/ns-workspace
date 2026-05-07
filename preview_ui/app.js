const state = {
  project: null,
  specs: [],
  graph: null,
  graphInstance: null,
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

marked.use({
  gfm: true,
  breaks: false,
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

  els.specList.innerHTML = "";
  let lastCategory = "";
  specs.forEach((spec) => {
    if (spec.category !== lastCategory) {
      lastCategory = spec.category;
      const label = document.createElement("div");
      label.className = "px-2 pt-4 pb-1 text-xs font-bold uppercase tracking-wide text-base-content/50";
      label.textContent = lastCategory;
      els.specList.append(label);
    }
    const button = document.createElement("button");
    button.className = [
      "btn btn-ghost btn-sm grid h-auto min-h-9 w-full grid-cols-[minmax(0,1fr)_auto] justify-start gap-2 px-2 text-left font-normal",
      spec.id === state.selectedId ? "btn-active" : "",
    ].join(" ");
    button.innerHTML = `
      <span class="truncate">${escapeHTML(spec.title)}</span>
      ${spec.status ? `<span class="badge badge-ghost badge-sm">${escapeHTML(spec.status)}</span>` : ""}
    `;
    button.addEventListener("click", () => selectSpec(spec.id, true));
    els.specList.append(button);
  });
}

async function selectSpec(id, showSpecTab) {
  const spec = await fetchJSON(`/api/specs/${encodeURIComponent(id)}`);
  state.selectedId = id;
  els.pageTitle.textContent = spec.title;
  els.pagePath.textContent = spec.path;
  els.specContent.innerHTML = renderMarkdown(spec.raw, spec.html);
  renderSpecList();
  highlightGraphNode(id);
  if (showSpecTab) switchTab("spec");
}

function renderMarkdown(raw, fallbackHTML) {
  if (!raw) {
    return fallbackHTML || "<p>No content.</p>";
  }
  return DOMPurify.sanitize(marked.parse(raw));
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
      <div class="font-semibold">${escapeHTML(rel.from)} → ${escapeHTML(rel.to)}</div>
      <div class="text-base-content/60">${escapeHTML(rel.description || rel.section || "")}</div>
    `;
    els.relationships.append(item);
  });
  if (relationships.length === 0) {
    els.relationships.innerHTML = '<p class="text-sm text-base-content/60">No relationship map entries found.</p>';
  }
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

load().catch((err) => {
  els.pageTitle.textContent = "Failed to load specs";
  els.pagePath.textContent = err.message;
});
