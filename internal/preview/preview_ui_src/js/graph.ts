export function createDocsGraph({ state, els, escapeHTML, refreshIcons, openSpecPreview, openFilePreview }) {
  return {
    render,
    stop,
  };

  function render() {
    if (!els.graphCanvas || !state.graph) return;
    if (!window.d3) {
      els.graphCanvas.innerHTML = '<div class="alert alert-error m-4 text-sm">D3 library is not loaded.</div>';
      return;
    }
    stop();

    const query = (els.graphSearch?.value || "").trim().toLowerCase();
    const graph = normalizedGraphData(state.graph, query);
    els.graphStats.textContent = `${graph.nodes.length} nodes, ${graph.links.length} edges`;
    renderDetails(graph);

    els.graphCanvas.innerHTML = "";
    const rect = els.graphCanvas.getBoundingClientRect();
    const width = Math.max(640, rect.width || 960);
    const height = Math.max(420, rect.height || 620);
    const svg = d3
      .select(els.graphCanvas)
      .append("svg")
      .attr("class", "docs-graph-svg")
      .attr("viewBox", [-width / 2, -height / 2, width, height])
      .attr("role", "img");

    const viewport = svg.append("g");
    const zoom = d3
      .zoom()
      .scaleExtent([0.25, 5])
      .on("zoom", (event) => viewport.attr("transform", event.transform));
    svg.call(zoom);

    const defs = svg.append("defs");
    defs
      .append("marker")
      .attr("id", "graph-arrow")
      .attr("viewBox", "0 -5 10 10")
      .attr("refX", 17)
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
      .attr("stroke", (edge) => edgeColor(edge.type))
      .attr("stroke-width", 1.6)
      .attr("stroke-dasharray", (edge) => (edge.origin === "index" ? "" : "4 4"))
      .attr("marker-end", "url(#graph-arrow)");

    const simulation = d3
      .forceSimulation(graph.nodes)
      .force(
        "link",
        d3
          .forceLink(graph.links)
          .id((n) => n.id)
          .distance((edge) => (edge.type === "depends" ? 120 : 96)),
      )
      .force("charge", d3.forceManyBody().strength(-420))
      .force("center", d3.forceCenter(0, 0))
      .force("collision", d3.forceCollide().radius(38));

    const node = viewport
      .append("g")
      .attr("class", "graph-nodes")
      .selectAll("g")
      .data(graph.nodes)
      .join("g")
      .attr("class", (n) => `graph-node ${n.id === state.graphSelectedId ? "selected" : ""}`)
      .call(d3.drag().on("start", graphDragStarted).on("drag", graphDragged).on("end", graphDragEnded))
      .on("click", (_, n) => openGraphNode(n));

    node
      .append("circle")
      .attr("r", (n) => (n.type === "external" ? 7 : 9))
      .attr("fill", (n) => nodeColor(n))
      .attr("stroke", "hsl(var(--b1))")
      .attr("stroke-width", 2);

    node
      .append("text")
      .attr("x", 12)
      .attr("y", 4)
      .text((n) => n.label || n.id);

    node.append("title").text((n) => `${n.label || n.id}\n${n.path || n.id}`);

    simulation.on("tick", () => {
      link
        .attr("x1", (d) => d.source.x)
        .attr("y1", (d) => d.source.y)
        .attr("x2", (d) => d.target.x)
        .attr("y2", (d) => d.target.y);
      node.attr("transform", (d) => `translate(${d.x},${d.y})`);
    });
    state.graphSimulation = simulation;

    requestAnimationFrame(() => {
      const bounds = viewport.node().getBBox?.();
      if (bounds && Number.isFinite(bounds.width) && bounds.width > 0) {
        fitGraph(svg, zoom, bounds, width, height);
      }
    });

    function graphDragStarted(event) {
      if (!event.active) simulation.alphaTarget(0.25).restart();
      event.subject.fx = event.subject.x;
      event.subject.fy = event.subject.y;
    }

    function graphDragged(event) {
      event.subject.fx = event.x;
      event.subject.fy = event.y;
    }

    function graphDragEnded(event) {
      if (!event.active) simulation.alphaTarget(0);
      event.subject.fx = null;
      event.subject.fy = null;
    }
  }

  function normalizedGraphData(raw, query) {
    const visible = new Set();
    const allNodes = (raw.nodes || []).map((node) => ({
      ...node,
      type: node.type || (node.specId ? "doc" : "external"),
      label: node.label || node.id,
    }));
    for (const node of allNodes) {
      const haystack = `${node.id} ${node.label} ${node.path || ""} ${node.category || ""} ${node.status || ""}`.toLowerCase();
      if (!query || haystack.includes(query)) visible.add(node.id);
    }
    const links = (raw.edges || [])
      .map((edge) => ({ ...edge, source: edge.from, target: edge.to, type: edge.type || edge.label || "references" }))
      .filter((edge) => !query || visible.has(edge.source) || visible.has(edge.target));
    links.forEach((edge) => {
      visible.add(edge.source);
      visible.add(edge.target);
    });
    return {
      nodes: allNodes.filter((node) => visible.has(node.id)),
      links,
    };
  }

  function renderDetails(graph) {
    if (!els.graphDetails) return;
    const node = graph.nodes.find((item) => item.id === state.graphSelectedId) || graph.nodes[0];
    if (!node) {
      els.graphDetails.innerHTML = '<div class="p-4 text-sm text-base-content/60">No graph nodes.</div>';
      return;
    }
    state.graphSelectedId = node.id;
    const incoming = graph.links.filter((edge) => edge.target === node.id || edge.target?.id === node.id);
    const outgoing = graph.links.filter((edge) => edge.source === node.id || edge.source?.id === node.id);
    els.graphDetails.innerHTML = `
      <div class="grid gap-4 p-4">
        <div>
          <div class="text-xs uppercase tracking-wide text-base-content/50">${escapeHTML(node.type || "node")}</div>
          <h3 class="mt-1 text-lg font-semibold">${escapeHTML(node.label || node.id)}</h3>
          <p class="break-words text-sm text-base-content/60">${escapeHTML(node.path || node.id)}</p>
        </div>
        ${node.specId ? `<button class="btn btn-primary btn-sm" type="button" data-preview-spec="${escapeHTML(node.specId)}"><i data-lucide="file-text" class="h-4 w-4"></i>Preview doc</button>` : ""}
        ${!node.specId && node.path ? `<button class="btn btn-ghost btn-sm" type="button" data-preview-file="${escapeHTML(node.path)}"><i data-lucide="file-code" class="h-4 w-4"></i>Preview file</button>` : ""}
        <div>
          <h4 class="mb-2 text-sm font-semibold">Outgoing refs (${outgoing.length})</h4>
          ${renderEdgeList(outgoing, "target")}
        </div>
        <div>
          <h4 class="mb-2 text-sm font-semibold">Incoming refs (${incoming.length})</h4>
          ${renderEdgeList(incoming, "source")}
        </div>
      </div>
    `;
    els.graphDetails.querySelector("[data-preview-spec]")?.addEventListener("click", (event) => {
      openSpecPreview(event.currentTarget.dataset.previewSpec, { updateURL: true });
    });
    els.graphDetails.querySelector("[data-preview-file]")?.addEventListener("click", (event) => {
      openFilePreview(event.currentTarget.dataset.previewFile, 0, { updateURL: true });
    });
    els.graphDetails.querySelectorAll("[data-select-node]").forEach((button) => {
      button.addEventListener("click", () => {
        const target = graph.nodes.find((item) => item.id === button.dataset.selectNode);
        if (target) {
          openGraphNode(target);
        }
      });
    });
    refreshIcons();
  }

  function renderEdgeList(edges, side) {
    if (!edges.length) return '<div class="text-sm text-base-content/50">None</div>';
    return `
      <div class="grid gap-1">
        ${edges
          .slice(0, 12)
          .map((edge) => {
            const related = typeof edge[side] === "string" ? edge[side] : edge[side].id;
            return `<button class="graph-ref-row" type="button" data-select-node="${escapeHTML(related)}">
              <span class="badge badge-ghost badge-sm">${escapeHTML(edge.type || "references")}</span>
              <span class="min-w-0 truncate">${escapeHTML(related)}</span>
            </button>`;
          })
          .join("")}
      </div>
    `;
  }

  function selectGraphNode(id) {
    state.graphSelectedId = id;
    render();
  }

  function openGraphNode(node) {
    state.graphSelectedId = node.id;
    renderDetails(normalizedGraphData(state.graph, (els.graphSearch?.value || "").trim().toLowerCase()));
    // Graph interactions should preview docs/files without changing the main tab selection.
    if (node.specId) {
      openSpecPreview(node.specId, { updateURL: true });
      return;
    }
    if (node.path) {
      openFilePreview(node.path, 0, { updateURL: true });
      return;
    }
    selectGraphNode(node.id);
  }

  function stop() {
    if (state.graphSimulation) {
      state.graphSimulation.stop();
      state.graphSimulation = null;
    }
  }
}

function fitGraph(svg, zoom, bounds, width, height) {
  const fullWidth = bounds.width || width;
  const fullHeight = bounds.height || height;
  const scale = Math.max(0.25, Math.min(2.5, 0.86 / Math.max(fullWidth / width, fullHeight / height)));
  const tx = -scale * (bounds.x + fullWidth / 2);
  const ty = -scale * (bounds.y + fullHeight / 2);
  svg.transition().duration(350).call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
}

function nodeColor(node) {
  if (node.type === "external") return "#94a3b8";
  switch (node.category) {
    case "modules":
      return "#2563eb";
    case "shared":
      return "#16a34a";
    case "decisions":
      return "#9333ea";
    case "planning":
      return "#f59e0b";
    case "compliance":
      return "#dc2626";
    default:
      return "#0f766e";
  }
}

function edgeColor(type) {
  switch (type) {
    case "depends":
      return "#ef4444";
    case "implements":
      return "#6366f1";
    case "blocked-by":
      return "#f97316";
    case "verifies":
      return "#14b8a6";
    case "provides":
      return "#22c55e";
    case "consumes":
      return "#eab308";
    default:
      return "#64748b";
  }
}
