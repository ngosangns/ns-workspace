import { createResource, createSignal, onCleanup, Show } from "solid-js";
import { api, type SpecGraph } from "../lib/api";
// graphology / sigma ship loose types across package versions; use any for draw path.
import Graph from "graphology";
import forceAtlas2 from "graphology-layout-forceatlas2";
import Sigma from "sigma";

export default function GraphView() {
  const [graphData] = createResource(() => api.getGraph());
  const [error, setError] = createSignal("");
  let container!: HTMLDivElement;
  let renderer: { kill: () => void } | null = null;

  const dispose = () => {
    if (renderer) {
      renderer.kill();
      renderer = null;
    }
  };
  onCleanup(dispose);

  const untrackInterval = setInterval(() => {
    const data = graphData();
    if (!data || !container || renderer) return;
    try {
      draw(data);
    } catch (e: any) {
      setError(e.message || String(e));
    }
  }, 100);
  onCleanup(() => clearInterval(untrackInterval));

  function draw(data: SpecGraph) {
    dispose();
    const g: any = new (Graph as any)();
    for (const n of data.nodes || []) {
      if (!g.hasNode(n.id)) {
        g.addNode(n.id, {
          label: n.label || n.id,
          x: Math.random(),
          y: Math.random(),
          size: 6,
          color: "#2bb8a8",
        });
      }
    }
    for (const e of data.edges || []) {
      if (g.hasNode(e.from) && g.hasNode(e.to) && !g.hasEdge(e.from, e.to)) {
        g.addEdge(e.from, e.to, { size: 1, color: "#3a4455" });
      }
    }
    if (g.order > 0) {
      (forceAtlas2 as any).assign(g, { iterations: 80, settings: { gravity: 1, scalingRatio: 8 } });
    }
    renderer = new (Sigma as any)(g, container, {
      allowInvalidContainer: true,
      labelColor: { color: "#9aa3b2" },
      defaultEdgeColor: "#3a4455",
    });
  }

  return (
    <div class="space-y-3">
      <header>
        <h1 class="m-0 mb-1 text-xl font-semibold tracking-tight">Graph</h1>
        <p class="m-0 text-[14px] text-fg-secondary">
          Relationship graph from docs metadata ({graphData()?.nodes?.length ?? "…"} nodes, {graphData()?.edges?.length ?? "…"} edges).
        </p>
      </header>
      <Show when={graphData.error || error()}>
        <div class="rounded-md border border-negative/35 bg-negative/10 px-3 py-2 text-[13px] text-negative">
          {error() || String(graphData.error)}
        </div>
      </Show>
      <Show when={graphData.loading}>
        <div class="text-[13px] text-fg-muted">Loading graph…</div>
      </Show>
      <div ref={container} class="h-[min(70vh,640px)] w-full overflow-hidden rounded-xl border border-border bg-surface" />
    </div>
  );
}
