/**
 * OKF Bundle Viewer — SolidJS port of the knowledge-catalog viewer.
 * Licensed under Apache License 2.0; see COPYRIGHT.md.
 * Consumes window.BUNDLE / window.BUNDLE_NAME; expects global cytoscape + marked.
 */
import { createSignal, For, Show, onMount, onCleanup } from "solid-js";
import { render } from "solid-js/web";
import "./viz.css";

declare global {
  interface Window {
    BUNDLE: OkfBundle;
    BUNDLE_NAME: string;
  }
  // Vendor scripts inject these globals.
  const cytoscape: any;
  const marked: { parse: (src: string, opts?: object) => string };
}

interface OkfNodeData {
  id: string;
  label: string;
  type: string;
  color: string;
  size?: number;
  description?: string;
  resource?: string;
  tags?: string[];
}

interface OkfBundle {
  nodes: { data: OkfNodeData }[];
  edges: { data: { source: string; target: string; id?: string } }[];
  bodies: Record<string, string>;
  types: string[];
  palette: Record<string, string>;
}

function App() {
  const bundle = window.BUNDLE;
  const bundleName = window.BUNDLE_NAME || "Knowledge";
  document.title = `${bundleName} — OKF Viewer`;

  const [selectedId, setSelectedId] = createSignal<string | null>(null);
  const [search, setSearch] = createSignal("");
  const [typeFilter, setTypeFilter] = createSignal("");
  const [layoutName, setLayoutName] = createSignal("cose");

  let graphEl!: HTMLDivElement;
  let cy: any = null;

  const nodeIndex: Record<string, OkfNodeData> = {};
  for (const n of bundle.nodes) nodeIndex[n.data.id] = n.data;

  const backlinks: Record<string, string[]> = {};
  for (const edge of bundle.edges) {
    const { source, target } = edge.data;
    if (!backlinks[target]) backlinks[target] = [];
    backlinks[target].push(source);
  }

  onMount(() => {
    cy = cytoscape({
      container: graphEl,
      elements: [...bundle.nodes, ...bundle.edges],
      style: [
        {
          selector: "node",
          style: {
            "background-color": "data(color)",
            label: "data(label)",
            color: "#0f172a",
            "font-size": 11,
            "text-valign": "bottom",
            "text-margin-y": 4,
            "text-wrap": "wrap",
            "text-max-width": 120,
            width: "data(size)",
            height: "data(size)",
            "border-width": 1,
            "border-color": "#0f172a",
          },
        },
        { selector: "node:selected", style: { "border-width": 3, "border-color": "#f59e0b" } },
        {
          selector: "edge",
          style: {
            width: 1.5,
            "line-color": "#cbd5e1",
            "target-arrow-color": "#cbd5e1",
            "target-arrow-shape": "triangle",
            "curve-style": "bezier",
            "arrow-scale": 0.9,
          },
        },
        {
          selector: "edge:selected",
          style: { "line-color": "#f59e0b", "target-arrow-color": "#f59e0b", width: 2.5 },
        },
        { selector: ".dim", style: { opacity: 0.15 } },
      ],
      layout: { name: "cose", animate: false, padding: 30 },
      wheelSensitivity: 0.2,
    });

    cy.on("tap", "node", (evt: any) => setSelectedId(evt.target.id()));
    cy.on("tap", (evt: any) => {
      if (evt.target === cy) setSelectedId(null);
    });

    const initial = bundle.nodes.find((n) => n.data.type === "BigQuery Dataset") || bundle.nodes[0];
    if (initial) setSelectedId(initial.data.id);
  });

  onCleanup(() => {
    cy?.destroy();
    cy = null;
  });

  function applyFilters() {
    if (!cy) return;
    const q = search().trim().toLowerCase();
    const t = typeFilter();
    cy.nodes().forEach((n: any) => {
      const d = n.data();
      let dim = false;
      if (t && d.type !== t) dim = true;
      if (q) {
        const hay = `${d.label || ""} ${d.id} ${(d.tags || []).join(" ")}`.toLowerCase();
        if (!hay.includes(q)) dim = true;
      }
      n.toggleClass("dim", dim);
    });
    cy.edges().forEach((edge: any) => {
      edge.toggleClass("dim", edge.source().hasClass("dim") || edge.target().hasClass("dim"));
    });
  }

  function changeLayout(name: string) {
    setLayoutName(name);
    cy?.layout({ name, animate: false, padding: 30 }).run();
  }

  function showDetail(id: string) {
    setSelectedId(id);
    if (!cy) return;
    cy.elements().unselect();
    const node = cy.getElementById(id);
    if (node) {
      node.select();
      cy.animate({ center: { eles: node }, zoom: Math.max(cy.zoom(), 1.0) }, { duration: 200 });
    }
  }

  const selected = () => {
    const id = selectedId();
    return id ? nodeIndex[id] : null;
  };
  const bodyHtml = () => {
    const id = selectedId();
    if (!id) return "";
    const body = bundle.bodies[id] || "";
    return marked.parse(body, { breaks: false, gfm: true }) as string;
  };

  return (
    <>
      <header>
        <div class="title">
          <strong id="bundle-name">{bundleName}</strong>
          <span class="muted">OKF bundle</span>
        </div>
        <div class="controls">
          <input
            id="search"
            type="search"
            placeholder="Search title / id / tag"
            value={search()}
            onInput={(e) => {
              setSearch(e.currentTarget.value);
              applyFilters();
            }}
          />
          <select
            id="filter-type"
            value={typeFilter()}
            onChange={(e) => {
              setTypeFilter(e.currentTarget.value);
              applyFilters();
            }}
          >
            <option value="">All types</option>
            <For each={bundle.types}>{(t) => <option value={t}>{t}</option>}</For>
          </select>
          <select id="layout" value={layoutName()} onChange={(e) => changeLayout(e.currentTarget.value)}>
            <option value="cose">cose (force)</option>
            <option value="concentric">concentric</option>
            <option value="breadthfirst">breadth-first</option>
            <option value="circle">circle</option>
            <option value="grid">grid</option>
          </select>
          <button
            id="reset"
            type="button"
            onClick={() => {
              cy?.fit(null, 30);
              setSelectedId(null);
            }}
          >
            Reset view
          </button>
        </div>
      </header>
      <main>
        <section id="graph" ref={graphEl} />
        <section id="detail">
          <Show
            when={selected()}
            fallback={
              <div id="detail-empty" class="muted">
                Click a node to see its details.
              </div>
            }
          >
            {(data) => (
              <article id="detail-content">
                <header class="detail-header">
                  <span class="type-chip" id="detail-type" style={{ background: data().color }}>
                    {data().type}
                  </span>
                  <h1 id="detail-title">{data().label}</h1>
                  <div class="muted" id="detail-id">
                    {data().id}
                  </div>
                </header>
                <dl class="frontmatter">
                  <dt>Description</dt>
                  <dd id="detail-description">{data().description || "—"}</dd>
                  <dt>Resource</dt>
                  <dd id="detail-resource">
                    <Show when={data().resource} fallback="—">
                      <a class="external" href={data().resource} target="_blank" rel="noopener">
                        {data().resource}
                      </a>
                    </Show>
                  </dd>
                  <dt>Tags</dt>
                  <dd id="detail-tags">
                    <Show when={data().tags?.length} fallback="—">
                      <For each={data().tags || []}>{(t) => <span class="tag">{t}</span>}</For>
                    </Show>
                  </dd>
                </dl>
                <hr />
                <div id="detail-body" innerHTML={bodyHtml()} />
                <Show when={(backlinks[data().id] || []).length}>
                  <section id="detail-backlinks">
                    <h2>Cited by</h2>
                    <ul id="backlinks-list">
                      <For each={backlinks[data().id] || []}>
                        {(src) => (
                          <li>
                            <a
                              href={`#node-${src}`}
                              onClick={(e) => {
                                e.preventDefault();
                                showDetail(src);
                              }}
                            >
                              {nodeIndex[src]?.label || src}
                            </a>
                            <span class="muted"> ({src})</span>
                          </li>
                        )}
                      </For>
                    </ul>
                  </section>
                </Show>
              </article>
            )}
          </Show>
        </section>
      </main>
    </>
  );
}

const mount = document.getElementById("export-root") || document.body;
render(() => <App />, mount);
