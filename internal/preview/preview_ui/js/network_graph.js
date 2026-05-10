import Graph from "graphology";
import forceAtlas2 from "graphology-layout-forceatlas2";
import Sigma from "sigma";
export function renderNetworkGraph(options) {
    const graph = new Graph({ multi: true, type: "directed" });
    const nodes = normalizeNodes(options.graph.nodes);
    seedCircularLayout(nodes);
    for (const node of nodes) {
        graph.mergeNode(node.id, {
            ...node,
            color: options.nodeColor(node),
            label: node.label || node.id,
            labelColor: options.labelColor || "#0f172a",
            size: nodeSize(node),
        });
    }
    addEdges(graph, options.graph.links, options.edgeColor);
    applyReadableLayout(graph);
    let selectedId = options.selectedId && graph.hasNode(options.selectedId) ? options.selectedId : "";
    const renderer = new Sigma(graph, options.container, {
        allowInvalidContainer: true,
        autoCenter: true,
        autoRescale: true,
        defaultEdgeType: "arrow",
        defaultNodeColor: "#94a3b8",
        defaultDrawNodeHover: drawNodeHoverLabelOnly,
        hideEdgesOnMove: false,
        hideLabelsOnMove: false,
        itemSizesReference: "screen",
        labelColor: { attribute: "labelColor", color: options.labelColor || "#0f172a" },
        labelDensity: 1,
        labelGridCellSize: 90,
        labelRenderedSizeThreshold: 0,
        labelSize: 12,
        labelWeight: "650",
        maxCameraRatio: 10,
        minCameraRatio: 0.05,
        minEdgeThickness: 1.3,
        renderEdgeLabels: false,
        renderLabels: true,
        zIndex: true,
        nodeReducer: (node, data) => {
            const selected = Boolean(selectedId);
            const related = selected && (node === selectedId || graph.areNeighbors(node, selectedId));
            const dimmed = selected && !related;
            return {
                ...data,
                color: dimmed ? colorWithOpacity(data.color, 0.18) : data.color,
                forceLabel: true,
                label: data.label,
                labelColor: dimmed ? colorWithOpacity(data.labelColor, 0.22) : data.labelColor,
                type: "circle",
                zIndex: node === selectedId ? 4 : related ? 3 : 1,
            };
        },
        edgeReducer: (edge, data) => {
            const [source, target] = graph.extremities(edge);
            const selected = Boolean(selectedId);
            const related = selected && (source === selectedId || target === selectedId);
            return {
                ...data,
                color: selected && !related ? colorWithOpacity(data.color, 0.14) : data.color,
                hidden: false,
            };
        },
    });
    const disposeWheelGuard = installModifierWheelZoomGuard(options.container);
    renderer.on("clickNode", ({ node }) => {
        const attrs = graph.getNodeAttributes(node);
        selectedId = node;
        renderer.refresh();
        options.onSelectNode(attrs);
    });
    renderer.on("clickStage", () => {
        if (!selectedId)
            return;
        selectedId = "";
        renderer.refresh();
        options.onClearSelection?.();
    });
    renderer.on("enterNode", () => {
        options.container.classList.add("is-node-hover");
    });
    renderer.on("leaveNode", () => {
        options.container.classList.remove("is-node-hover");
    });
    requestAnimationFrame(() => fitRenderer(renderer));
    return {
        fit: () => fitRenderer(renderer),
        kill: () => {
            disposeWheelGuard();
            renderer.kill();
        },
        setSelected: (id) => {
            selectedId = graph.hasNode(id) ? id : "";
            renderer.refresh();
        },
    };
}
function installModifierWheelZoomGuard(container) {
    const options = { capture: true };
    const handleWheel = (event) => {
        if (event.metaKey || event.ctrlKey)
            return;
        event.stopPropagation();
        event.stopImmediatePropagation();
    };
    container.addEventListener("wheel", handleWheel, options);
    return () => container.removeEventListener("wheel", handleWheel, options);
}
function drawNodeHoverLabelOnly(context, data, settings) {
    if (!data.label)
        return;
    const size = settings.labelSize;
    const font = settings.labelFont;
    const weight = settings.labelWeight;
    const color = settings.labelColor.attribute
        ? data[settings.labelColor.attribute] || settings.labelColor.color || "#000"
        : settings.labelColor.color;
    context.fillStyle = color;
    context.font = `${weight} ${size}px ${font}`;
    context.fillText(data.label, data.x + data.size + 3, data.y + size / 3);
}
function normalizeNodes(nodes) {
    const out = [];
    const seen = new Set();
    for (const node of nodes || []) {
        const id = String(node.id || "").trim();
        if (!id || seen.has(id))
            continue;
        seen.add(id);
        out.push({
            ...node,
            id,
            label: node.label || id,
            labelColor: "#0f172a",
            x: 0,
            y: 0,
            size: nodeSize(node),
            color: "#94a3b8",
        });
    }
    return out;
}
function seedCircularLayout(nodes) {
    const radius = Math.max(8, Math.sqrt(Math.max(nodes.length, 1)) * 9);
    nodes.forEach((node, index) => {
        const angle = (index / Math.max(nodes.length, 1)) * Math.PI * 2;
        const jitter = (stableHash(node.id) % 17) / 17;
        node.x = Math.cos(angle) * (radius + jitter * 2);
        node.y = Math.sin(angle) * (radius + jitter * 2);
    });
}
function addEdges(graph, links, edgeColor) {
    let serial = 0;
    for (const link of links || []) {
        const source = endpointID(link.source);
        const target = endpointID(link.target);
        if (!source || !target || source === target || !graph.hasNode(source) || !graph.hasNode(target))
            continue;
        const relation = link.type || link.label || "references";
        const key = `edge:${source}:${target}:${relation}:${serial++}`;
        graph.addDirectedEdgeWithKey(key, source, target, {
            color: edgeColor(relation),
            confidence: link.confidence || link.origin || "",
            label: relation,
            relation,
            size: relation === "defines" || relation === "documents" ? 1.2 : 1.5,
            type: "arrow",
        });
    }
}
function applyReadableLayout(graph) {
    if (graph.order < 2)
        return;
    const iterations = graph.order > 500 ? 80 : graph.order > 180 ? 110 : 160;
    const settings = forceAtlas2.inferSettings(graph);
    forceAtlas2.assign(graph, {
        iterations,
        settings: {
            ...settings,
            barnesHutOptimize: graph.order > 120,
            edgeWeightInfluence: 0.35,
            gravity: graph.order > 160 ? 1.2 : 0.8,
            scalingRatio: graph.order > 160 ? 12 : 8,
            slowDown: 8,
        },
    });
}
function fitRenderer(renderer) {
    renderer.resize(true);
    renderer.refresh();
    renderer
        .getCamera()
        .animatedReset({ duration: 260 })
        .then(() => {
        renderer.resize(true);
        renderer.refresh();
    })
        .catch(() => { });
    requestAnimationFrame(() => {
        renderer.refresh();
        requestAnimationFrame(() => renderer.refresh());
    });
}
function endpointID(endpoint) {
    if (typeof endpoint === "string")
        return endpoint;
    return endpoint?.id || "";
}
function nodeSize(node) {
    switch (node.type) {
        case "external":
        case "file":
        case "doc-file":
            return 6;
        case "flow":
            return 7;
        default:
            return 8;
    }
}
function colorWithOpacity(color, opacity) {
    const hex = color.startsWith("#") ? color.slice(1) : "";
    if (hex.length === 3) {
        const r = Number.parseInt(hex[0] + hex[0], 16);
        const g = Number.parseInt(hex[1] + hex[1], 16);
        const b = Number.parseInt(hex[2] + hex[2], 16);
        return `rgba(${r}, ${g}, ${b}, ${opacity})`;
    }
    if (hex.length === 6) {
        const r = Number.parseInt(hex.slice(0, 2), 16);
        const g = Number.parseInt(hex.slice(2, 4), 16);
        const b = Number.parseInt(hex.slice(4, 6), 16);
        return `rgba(${r}, ${g}, ${b}, ${opacity})`;
    }
    const rgb = color.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/);
    if (rgb)
        return `rgba(${rgb[1]}, ${rgb[2]}, ${rgb[3]}, ${opacity})`;
    const rgba = color.match(/^rgba\((\d+),\s*(\d+),\s*(\d+),\s*[\d.]+\)$/);
    if (rgba)
        return `rgba(${rgba[1]}, ${rgba[2]}, ${rgba[3]}, ${opacity})`;
    return color;
}
function stableHash(value) {
    let hash = 0;
    for (let index = 0; index < value.length; index += 1) {
        hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
    }
    return hash;
}
