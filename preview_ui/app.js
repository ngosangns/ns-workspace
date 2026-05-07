const state = {
  project: null,
  specs: [],
  graph: null,
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
  graphSvg: document.querySelector("#graphSvg"),
  relationships: document.querySelector("#relationships"),
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
    card.className = "card";
    card.innerHTML = `<div class="card-value">${escapeHTML(String(value))}</div><div class="card-label">${label}</div>`;
    els.summaryCards.append(card);
  });

  els.syncState.innerHTML = "";
  const syncEntries = Object.entries(project.sync || {});
  if (syncEntries.length === 0) {
    els.syncState.innerHTML = "<dt>State</dt><dd>Unavailable</dd>";
  } else {
    syncEntries.forEach(([key, value]) => {
      const dt = document.createElement("dt");
      const dd = document.createElement("dd");
      dt.textContent = key;
      dd.textContent = value;
      els.syncState.append(dt, dd);
    });
  }

  els.warnings.innerHTML = "";
  const warnings = project.warnings || [];
  if (warnings.length === 0) {
    els.warnings.innerHTML = '<div class="warning" style="color: var(--ok); background: #eaf8f0;">No structural warnings.</div>';
  } else {
    warnings.forEach((warning) => {
      const item = document.createElement("div");
      item.className = "warning";
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
      label.className = "group-label";
      label.textContent = lastCategory;
      els.specList.append(label);
    }
    const button = document.createElement("button");
    button.className = `spec-item ${spec.id === state.selectedId ? "active" : ""}`;
    button.innerHTML = `
      <span class="spec-title">${escapeHTML(spec.title)}</span>
      ${spec.status ? `<span class="pill">${escapeHTML(spec.status)}</span>` : ""}
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
  els.specContent.innerHTML = spec.html || "<p>No content.</p>";
  renderSpecList();
  highlightGraphNode(id);
  if (showSpecTab) switchTab("spec");
}

function switchTab(name) {
  state.tab = name;
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("active", tab.dataset.tab === name));
  document.querySelectorAll(".panel").forEach((panel) => panel.classList.remove("active"));
  document.querySelector(`#${name}Tab`).classList.add("active");
}

function renderGraph() {
  const { nodes, edges, relationships } = state.graph;
  const width = 1200;
  const height = 720;
  const cx = width / 2;
  const cy = height / 2;
  const radius = Math.min(width, height) * 0.38;
  const positions = new Map();
  nodes.forEach((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(nodes.length, 1) - Math.PI / 2;
    positions.set(node.id, {
      x: cx + Math.cos(angle) * radius,
      y: cy + Math.sin(angle) * radius,
    });
  });

  els.graphSvg.innerHTML = `
    <defs>
      <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
        <path d="M 0 0 L 10 5 L 0 10 z" fill="#aab1ba"></path>
      </marker>
    </defs>
  `;
  edges.forEach((edge) => {
    const from = positions.get(edge.from);
    const to = positions.get(edge.to);
    if (!from || !to) return;
    const line = svg("line", {
      class: "edge",
      x1: from.x,
      y1: from.y,
      x2: to.x,
      y2: to.y,
      "marker-end": "url(#arrow)",
    });
    els.graphSvg.append(line);
  });
  nodes.forEach((node) => {
    const pos = positions.get(node.id);
    const group = svg("g", { class: "node", "data-spec-id": node.specId || "" });
    group.append(svg("circle", { cx: pos.x, cy: pos.y, r: 34, fill: colorFor(node.category), stroke: "#fff", "stroke-width": 3 }));
    const text = svg("text", {
      x: pos.x,
      y: pos.y + 52,
      "text-anchor": "middle",
      "font-size": 13,
      fill: "#232529",
    });
    text.textContent = node.id;
    group.append(text);
    if (node.specId) {
      group.addEventListener("click", () => selectSpec(node.specId, true));
    }
    els.graphSvg.append(group);
  });
  highlightGraphNode(state.selectedId);

  els.relationships.innerHTML = "";
  relationships.slice(0, 120).forEach((rel) => {
    const item = document.createElement("div");
    item.className = "relation";
    item.innerHTML = `<strong>${escapeHTML(rel.from)} → ${escapeHTML(rel.to)}</strong><span>${escapeHTML(rel.description || rel.section || "")}</span>`;
    els.relationships.append(item);
  });
  if (relationships.length === 0) {
    els.relationships.innerHTML = "<p>No relationship map entries found.</p>";
  }
}

function highlightGraphNode(specId) {
  document.querySelectorAll(".node circle").forEach((circle) => circle.setAttribute("stroke", "#fff"));
  document.querySelectorAll(`.node[data-spec-id="${cssEscape(specId)}"] circle`).forEach((circle) => {
    circle.setAttribute("stroke", "#2563eb");
    circle.setAttribute("stroke-width", "5");
  });
}

function svg(tag, attrs) {
  const el = document.createElementNS("http://www.w3.org/2000/svg", tag);
  Object.entries(attrs).forEach(([key, value]) => el.setAttribute(key, value));
  return el;
}

function colorFor(category) {
  const palette = {
    modules: "#dbeafe",
    shared: "#dcfce7",
    compliance: "#fef3c7",
    planning: "#ede9fe",
    docs: "#fce7f3",
    versions: "#e0f2fe",
    root: "#e5e7eb",
  };
  return palette[category] || "#e5e7eb";
}

function escapeHTML(value) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

function cssEscape(value) {
  if (window.CSS && CSS.escape) return CSS.escape(value);
  return value.replace(/"/g, '\\"');
}

document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
[els.search, els.categoryFilter, els.statusFilter, els.complianceFilter].forEach((el) => el.addEventListener("input", renderSpecList));

load().catch((err) => {
  els.pageTitle.textContent = "Failed to load specs";
  els.pagePath.textContent = err.message;
});
