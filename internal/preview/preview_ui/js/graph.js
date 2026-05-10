import { renderNetworkGraph } from "./network_graph.js";
export function createDocsGraph({ state, els, escapeHTML, refreshIcons, openSpecPreview, openFilePreview }) {
    let renderedGraph = null;
    return {
        fit,
        render,
        stop,
    };
    function render() {
        if (!els.graphCanvas || !state.graph)
            return;
        stop();
        const query = (els.graphSearch?.value || "").trim().toLowerCase();
        const graph = normalizedGraphData(state.graph, query);
        renderedGraph = graph;
        els.graphStats.textContent = `${graph.nodes.length} nodes, ${graph.links.length} edges`;
        renderDetails(graph);
        els.graphCanvas.innerHTML = "";
        state.graphRenderer = renderNetworkGraph({
            container: els.graphCanvas,
            graph,
            selectedId: state.graphSelectedId,
            nodeColor,
            edgeColor: edgeColorForTheme(state.theme),
            labelColor: state.theme === "dark" ? "#f8fafc" : "#0f172a",
            onSelectNode: openGraphNode,
            onClearSelection: clearGraphSelection,
        });
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
            if (!query || haystack.includes(query))
                visible.add(node.id);
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
        if (!els.graphDetails)
            return;
        const selected = graph.nodes.find((item) => item.id === state.graphSelectedId);
        const node = selected || graph.nodes[0];
        if (!node) {
            els.graphDetails.innerHTML = '<div class="p-4 text-sm text-base-content/60">No graph nodes.</div>';
            return;
        }
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
        ${!node.specId && node.path ? `<button class="btn btn-outline btn-sm" type="button" data-preview-file="${escapeHTML(node.path)}"><i data-lucide="file-code" class="h-4 w-4"></i>Preview file</button>` : ""}
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
        if (!edges.length)
            return '<div class="text-sm text-base-content/50">None</div>';
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
    function openGraphNode(node) {
        state.graphSelectedId = node.id;
        state.graphRenderer?.setSelected?.(node.id);
        if (renderedGraph) {
            renderDetails(renderedGraph);
        }
    }
    function clearGraphSelection() {
        state.graphSelectedId = "";
        state.graphRenderer?.setSelected?.("");
        if (renderedGraph) {
            renderDetails(renderedGraph);
        }
    }
    function stop() {
        if (state.graphRenderer) {
            state.graphRenderer.kill();
            state.graphRenderer = null;
        }
        renderedGraph = null;
    }
    function fit() {
        if (state.graphRenderer) {
            state.graphRenderer.fit();
        }
        else {
            render();
        }
    }
}
function nodeColor(node) {
    if (node.type === "external")
        return "#94a3b8";
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
function edgeColorForTheme(theme) {
    return theme === "dark" ? edgeColor : darkEdgeColor;
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
function darkEdgeColor(type) {
    switch (type) {
        case "depends":
            return "#991b1b";
        case "implements":
            return "#3730a3";
        case "blocked-by":
            return "#9a3412";
        case "verifies":
            return "#115e59";
        case "provides":
            return "#166534";
        case "consumes":
            return "#854d0e";
        default:
            return "#334155";
    }
}
