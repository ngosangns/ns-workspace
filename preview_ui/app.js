const state = {
  project: null,
  specs: [],
  graph: null,
  graphInstance: null,
  graphDiagramRenderId: 0,
  likec4: null,
  likec4Loading: false,
  theme: getInitialTheme(),
  lightbox: {
    scale: 1,
    x: 0,
    y: 0,
    dragging: false,
    dragStartX: 0,
    dragStartY: 0,
    originX: 0,
    originY: 0,
  },
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
  graphDiagram: document.querySelector("#graphDiagram"),
  graphStats: document.querySelector("#graphStats"),
  graphCanvas: document.querySelector("#graphCanvas"),
  relationships: document.querySelector("#relationships"),
  likec4Models: document.querySelector("#likec4Models"),
  themeToggle: document.querySelector("#themeToggle"),
  themeLabel: document.querySelector("#themeLabel"),
  diagramLightbox: document.querySelector("#diagramLightbox"),
  diagramLightboxTitle: document.querySelector("#diagramLightboxTitle"),
  diagramLightboxContent: document.querySelector("#diagramLightboxContent"),
  diagramLightboxClose: document.querySelector("#diagramLightboxClose"),
  diagramZoomIn: document.querySelector("#diagramZoomIn"),
  diagramZoomOut: document.querySelector("#diagramZoomOut"),
  diagramZoomFit: document.querySelector("#diagramZoomFit"),
  diagramZoomReset: document.querySelector("#diagramZoomReset"),
  diagramZoomLevel: document.querySelector("#diagramZoomLevel"),
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
  state.likec4 = null;
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
  state.likec4 = null;
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
  if (state.tab === "models") {
    await loadLikeC4Models(true);
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
  await renderLikeC4Blocks(els.specContent);
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
      const id = `mermaid-${state.selectedId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}`;
      await renderMermaidDiagram(host, id, source, "Mermaid", "Mermaid diagram", true);
      block.closest("pre").replaceWith(host);
    }),
  );
}

async function renderLikeC4Blocks(root) {
  const blocks = [
    ...root.querySelectorAll(
      "pre > code.language-likec4, pre > code.language-likec4-dsl, pre > code.language-c4, pre > code.likec4, pre > code.c4",
    ),
  ];
  await Promise.all(
    blocks.map(async (block, index) => {
      const source = block.textContent.trim();
      if (!source) return;
      const host = document.createElement("div");
      host.className = "my-5 rounded-lg border border-base-300 bg-base-100 p-4 text-sm text-base-content/70";
      host.textContent = "Rendering LikeC4 diagram...";
      block.closest("pre").replaceWith(host);
      try {
        const result = await postJSON("/api/render/likec4", { source });
        const diagrams = result.diagrams || [];
        if (diagrams.length === 0) {
          throw new Error("LikeC4 did not return any diagrams");
        }
        host.className = "my-5 grid gap-4";
        host.innerHTML = "";
        await Promise.all(
          diagrams.map(async (diagram, diagramIndex) => {
            const panel = document.createElement("figure");
            const caption = document.createElement("figcaption");
            const target = document.createElement("div");
            panel.className = "overflow-auto rounded-lg border border-base-300 bg-base-100 p-4";
            caption.className = "mb-3 flex items-center gap-2 text-xs font-semibold uppercase text-base-content/55";
            caption.innerHTML = `<i data-lucide="workflow" class="h-4 w-4"></i><span>${escapeHTML(diagram.name || "LikeC4")}</span>`;
            panel.append(caption, target);
            host.append(panel);
            const id = `likec4-${state.selectedId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}-${diagramIndex}`;
            await renderMermaidDiagram(target, id, diagram.mermaid || "", "LikeC4", diagram.name || "LikeC4 diagram", false);
          }),
        );
        refreshIcons();
      } catch (error) {
        host.className = "alert alert-error my-5 block whitespace-pre-wrap text-sm";
        host.textContent = `LikeC4 render failed: ${error.message || error}`;
      }
    }),
  );
}

async function renderMermaidDiagram(host, id, source, label, title, framed) {
  host.className = framed
    ? "mermaid diagram-surface my-5 overflow-auto rounded-lg border border-base-300 bg-base-100 p-4"
    : "mermaid diagram-surface overflow-auto";
  host.dataset.diagramTitle = title;
  try {
    const { svg } = await mermaid.render(id, source);
    host.innerHTML = DOMPurify.sanitize(svg, diagramSanitizeConfig);
    decorateDiagram(host, title);
  } catch (error) {
    host.className = "alert alert-error my-2 text-sm";
    host.textContent = `${label} render failed: ${error.message || error}`;
  }
}

function decorateDiagram(host, title) {
  const svg = host.querySelector("svg");
  if (!svg) return;
  host.tabIndex = 0;
  host.setAttribute("role", "button");
  host.setAttribute("aria-label", `Open ${title}`);
  host.title = "Open diagram";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "diagram-zoom btn btn-ghost btn-xs";
  button.setAttribute("aria-label", `Open ${title}`);
  button.innerHTML = '<i data-lucide="expand" class="h-4 w-4"></i>';
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    openDiagramLightbox(title, svg);
  });
  host.append(button);
  host.addEventListener("click", () => openDiagramLightbox(title, svg));
  host.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openDiagramLightbox(title, svg);
    }
  });
  refreshIcons();
}

function openDiagramLightbox(title, svg) {
  if (!svg) return;
  els.diagramLightboxTitle.textContent = title;
  els.diagramLightboxContent.innerHTML = "";
  resetLightboxTransform();
  const stage = document.createElement("div");
  stage.className = "diagram-lightbox__stage";
  const size = svgDiagramSize(svg);
  const clone = svg.cloneNode(true);
  clone.setAttribute("width", String(size.width));
  clone.setAttribute("height", String(size.height));
  clone.dataset.baseWidth = String(size.width);
  clone.dataset.baseHeight = String(size.height);
  clone.style.width = `${size.width}px`;
  clone.style.height = `${size.height}px`;
  clone.classList.add("diagram-lightbox__svg");
  stage.append(clone);
  els.diagramLightboxContent.append(stage);
  els.diagramLightbox.showModal();
  requestAnimationFrame(fitLightboxDiagram);
}

function svgDiagramSize(svg) {
  const viewBox = svg.viewBox?.baseVal;
  const attrWidth = parseFloat(svg.getAttribute("width") || "");
  const attrHeight = parseFloat(svg.getAttribute("height") || "");
  const rect = svg.getBoundingClientRect();
  return {
    width: Math.max(1, viewBox?.width || attrWidth || rect.width || 1000),
    height: Math.max(1, viewBox?.height || attrHeight || rect.height || 700),
  };
}

function closeDiagramLightbox() {
  if (els.diagramLightbox.open) {
    els.diagramLightbox.close();
  }
  els.diagramLightboxContent.innerHTML = "";
}

function resetLightboxTransform() {
  state.lightbox.scale = 1;
  state.lightbox.x = 0;
  state.lightbox.y = 0;
  state.lightbox.dragging = false;
  updateLightboxTransform();
}

function updateLightboxTransform() {
  const stage = els.diagramLightboxContent.querySelector(".diagram-lightbox__stage");
  if (stage) {
    const svg = stage.querySelector("svg");
    const baseWidth = parseFloat(svg?.dataset.baseWidth || "");
    const baseHeight = parseFloat(svg?.dataset.baseHeight || "");
    if (svg && baseWidth > 0 && baseHeight > 0) {
      const renderWidth = Math.max(1, Math.round(baseWidth * state.lightbox.scale));
      const renderHeight = Math.max(1, Math.round(baseHeight * state.lightbox.scale));
      svg.setAttribute("width", String(renderWidth));
      svg.setAttribute("height", String(renderHeight));
      svg.style.width = `${renderWidth}px`;
      svg.style.height = `${renderHeight}px`;
    }
    stage.style.transform = `translate(${state.lightbox.x}px, ${state.lightbox.y}px)`;
  }
  els.diagramZoomLevel.textContent = `${Math.round(state.lightbox.scale * 100)}%`;
}

function zoomLightbox(delta, clientX, clientY) {
  const previous = state.lightbox.scale;
  const next = Math.min(6, Math.max(0.2, previous * delta));
  const rect = els.diagramLightboxContent.getBoundingClientRect();
  const anchorX = clientX == null ? rect.left + rect.width / 2 : clientX;
  const anchorY = clientY == null ? rect.top + rect.height / 2 : clientY;
  const localX = anchorX - rect.left - state.lightbox.x;
  const localY = anchorY - rect.top - state.lightbox.y;
  state.lightbox.x -= localX * (next / previous - 1);
  state.lightbox.y -= localY * (next / previous - 1);
  state.lightbox.scale = next;
  updateLightboxTransform();
}

function fitLightboxDiagram() {
  const stage = els.diagramLightboxContent.querySelector(".diagram-lightbox__stage");
  const svg = stage?.querySelector("svg");
  if (!stage || !svg) return;
  const diagramWidth = parseFloat(svg.dataset.baseWidth || "") || svgDiagramSize(svg).width;
  const diagramHeight = parseFloat(svg.dataset.baseHeight || "") || svgDiagramSize(svg).height;
  const viewport = els.diagramLightboxContent.getBoundingClientRect();
  const stageStyles = getComputedStyle(stage);
  const horizontalChrome = parseFloat(stageStyles.paddingLeft) + parseFloat(stageStyles.paddingRight) + 2;
  const verticalChrome = parseFloat(stageStyles.paddingTop) + parseFloat(stageStyles.paddingBottom) + 2;
  const scale = Math.min(1, (viewport.width - 32 - horizontalChrome) / diagramWidth, (viewport.height - 32 - verticalChrome) / diagramHeight);
  state.lightbox.scale = Math.max(0.2, scale);
  const stageWidth = diagramWidth * state.lightbox.scale + horizontalChrome;
  const stageHeight = diagramHeight * state.lightbox.scale + verticalChrome;
  state.lightbox.x = Math.max(16, (viewport.width - stageWidth) / 2);
  state.lightbox.y = Math.max(16, (viewport.height - stageHeight) / 2);
  updateLightboxTransform();
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

function switchTab(name) {
  state.tab = name;
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("tab-active", tab.dataset.tab === name));
  document.querySelectorAll(".panel").forEach((panel) => panel.classList.remove("active"));
  document.querySelector(`#${name}Tab`).classList.add("active");
  if (name === "graph" && state.graphInstance) {
    state.graphInstance.resize();
    state.graphInstance.fit(undefined, 36);
  }
  if (name === "models") {
    loadLikeC4Models(false);
  }
}

async function loadLikeC4Models(force) {
  if (state.likec4Loading || (state.likec4 && !force)) return;
  state.likec4Loading = true;
  els.likec4Models.innerHTML = '<div class="alert py-3 text-sm">Rendering LikeC4 models...</div>';
  try {
    state.likec4 = await fetchJSON("/api/likec4");
    await renderLikeC4Models();
  } catch (error) {
    els.likec4Models.innerHTML = `<div class="alert alert-error py-3 text-sm">${escapeHTML(error.message || String(error))}</div>`;
  } finally {
    state.likec4Loading = false;
  }
}

async function renderLikeC4Models() {
  const projects = state.likec4?.projects || [];
  els.likec4Models.innerHTML = "";
  if (projects.length === 0) {
    els.likec4Models.innerHTML = '<div class="alert py-3 text-sm">No LikeC4 models or spec graph found.</div>';
    return;
  }
  for (const project of projects) {
    const card = document.createElement("article");
    card.className = "card border-base-300 bg-base-100 border";
    const body = document.createElement("div");
    body.className = "card-body gap-4";
    body.innerHTML = `
      <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
        <div class="min-w-0">
          <h2 class="card-title text-base">
            <i data-lucide="${project.generated ? "sparkles" : "boxes"}" class="h-5 w-5"></i>
            <span class="truncate">${escapeHTML(project.name)}</span>
          </h2>
          <p class="text-base-content/60 mt-1 break-all text-sm">${escapeHTML(project.root || "")}</p>
        </div>
        <span class="badge ${project.generated ? "badge-info" : "badge-ghost"}">${project.generated ? "Generated from specs" : "LikeC4 workspace"}</span>
      </div>
      <div class="text-base-content/60 text-xs">${escapeHTML((project.sourceFiles || []).slice(0, 8).join(", ") || "No source files")}</div>
    `;
    card.append(body);
    els.likec4Models.append(card);

    if (project.error) {
      const error = document.createElement("div");
      error.className = "alert alert-error block whitespace-pre-wrap text-sm";
      error.textContent = project.error;
      body.append(error);
      continue;
    }

    const diagrams = project.diagrams || [];
    if (diagrams.length === 0) {
      body.insertAdjacentHTML("beforeend", '<div class="alert py-3 text-sm">No diagrams generated.</div>');
      continue;
    }
    for (const [index, diagram] of diagrams.entries()) {
      const figure = document.createElement("figure");
      const caption = document.createElement("figcaption");
      const target = document.createElement("div");
      figure.className = "overflow-auto rounded-lg border border-base-300 bg-base-100 p-4";
      caption.className = "mb-3 flex items-center gap-2 text-xs font-semibold uppercase text-base-content/55";
      caption.innerHTML = `<i data-lucide="workflow" class="h-4 w-4"></i><span>${escapeHTML(diagram.name || "diagram")}</span>`;
      figure.append(caption, target);
      body.append(figure);
      const id = `likec4-model-${project.id.replace(/[^a-zA-Z0-9_-]/g, "-")}-${index}`;
      await renderMermaidDiagram(target, id, diagram.mermaid || "", "LikeC4", diagram.name || "diagram", false);
    }
    if (project.source) {
      const details = document.createElement("details");
      details.className = "collapse collapse-arrow border border-base-300";
      details.innerHTML = `
        <summary class="collapse-title min-h-0 py-3 text-sm font-medium">Generated LikeC4 source</summary>
        <pre class="collapse-content overflow-auto text-xs"><code>${escapeHTML(project.source)}</code></pre>
      `;
      body.append(details);
    }
  }
  refreshIcons();
}

function renderGraph() {
  const { nodes, edges, relationships } = state.graph;
  const palette = graphPalette();
  renderDependencyDiagram();
  els.graphStats.textContent = `${nodes.length} nodes, ${edges.length} dependencies, ${relationships.length} relationships`;
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
}

async function renderDependencyDiagram() {
  const source = state.graph?.dependencyDiagram?.trim();
  const renderId = ++state.graphDiagramRenderId;
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
els.themeToggle.addEventListener("click", () => {
  applyTheme(state.theme === "dark" ? "light" : "dark", { persist: true, rerender: true });
});
els.diagramLightboxClose.addEventListener("click", closeDiagramLightbox);
els.diagramLightbox.addEventListener("click", (event) => {
  if (event.target === els.diagramLightbox) {
    closeDiagramLightbox();
  }
});
els.diagramLightbox.addEventListener("close", () => {
  els.diagramLightboxContent.innerHTML = "";
});
els.diagramZoomIn.addEventListener("click", () => zoomLightbox(1.18));
els.diagramZoomOut.addEventListener("click", () => zoomLightbox(1 / 1.18));
els.diagramZoomReset.addEventListener("click", resetLightboxTransform);
els.diagramZoomFit.addEventListener("click", fitLightboxDiagram);
els.diagramLightboxContent.addEventListener(
  "wheel",
  (event) => {
    event.preventDefault();
    zoomLightbox(event.deltaY < 0 ? 1.12 : 1 / 1.12, event.clientX, event.clientY);
  },
  { passive: false },
);
els.diagramLightboxContent.addEventListener("pointerdown", (event) => {
  if (!els.diagramLightboxContent.querySelector(".diagram-lightbox__stage")) return;
  state.lightbox.dragging = true;
  state.lightbox.dragStartX = event.clientX;
  state.lightbox.dragStartY = event.clientY;
  state.lightbox.originX = state.lightbox.x;
  state.lightbox.originY = state.lightbox.y;
  els.diagramLightboxContent.classList.add("is-panning");
  els.diagramLightboxContent.setPointerCapture(event.pointerId);
});
els.diagramLightboxContent.addEventListener("pointermove", (event) => {
  if (!state.lightbox.dragging) return;
  state.lightbox.x = state.lightbox.originX + event.clientX - state.lightbox.dragStartX;
  state.lightbox.y = state.lightbox.originY + event.clientY - state.lightbox.dragStartY;
  updateLightboxTransform();
});
els.diagramLightboxContent.addEventListener("pointerup", (event) => {
  state.lightbox.dragging = false;
  els.diagramLightboxContent.classList.remove("is-panning");
  if (els.diagramLightboxContent.hasPointerCapture(event.pointerId)) {
    els.diagramLightboxContent.releasePointerCapture(event.pointerId);
  }
});
els.diagramLightboxContent.addEventListener("pointercancel", () => {
  state.lightbox.dragging = false;
  els.diagramLightboxContent.classList.remove("is-panning");
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
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: theme === "dark" ? "dark" : "default",
  });
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
  state.likec4 = null;
  if (state.tab === "models") {
    loadLikeC4Models(true);
  }
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
