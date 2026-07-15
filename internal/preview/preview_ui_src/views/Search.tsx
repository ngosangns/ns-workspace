import { createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api, type SearchResult } from "../lib/api";

function ResultList(props: { title: string; items: SearchResult[] }) {
  return (
    <section class="rounded-xl border border-border bg-surface p-4">
      <h2 class="m-0 mb-3 text-[13px] font-semibold uppercase tracking-wider text-fg-muted">
        {props.title} <span class="text-fg-secondary">({props.items.length})</span>
      </h2>
      <Show when={props.items.length === 0}>
        <p class="m-0 text-[13px] text-fg-muted">No results</p>
      </Show>
      <ul class="m-0 list-none space-y-2 p-0">
        <For each={props.items}>
          {(item) => (
            <li class="rounded-md border border-border bg-elevated px-3 py-2">
              <div class="flex items-baseline justify-between gap-2">
                <A
                  href={item.path?.endsWith(".md") ? `/docs/${encodeURIComponent(item.id)}` : "/"}
                  class="font-medium text-fg hover:text-accent"
                >
                  {item.title}
                </A>
                <span class="font-mono text-[11px] text-fg-muted">{item.score.toFixed(2)}</span>
              </div>
              <Show when={item.path}>
                <div class="font-mono text-[11px] text-fg-muted">{item.path}</div>
              </Show>
              <Show when={item.excerpt || item.description}>
                <p class="m-0 mt-1 line-clamp-3 text-[12.5px] text-fg-secondary">{item.excerpt || item.description}</p>
              </Show>
            </li>
          )}
        </For>
      </ul>
    </section>
  );
}

export default function Search() {
  const initial = typeof window !== "undefined" ? new URLSearchParams(window.location.hash.split("?")[1] || "").get("q") || "" : "";
  const [query, setQuery] = createSignal(initial);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal("");
  const [panels, setPanels] = createSignal<{
    docsSemantic: SearchResult[];
    docsGraph: SearchResult[];
    codeSemantic: SearchResult[];
    codeGraph: SearchResult[];
  } | null>(null);
  const [warnings, setWarnings] = createSignal<string[]>([]);

  async function runSearch(e?: Event) {
    e?.preventDefault();
    const q = query().trim();
    if (!q) return;
    setLoading(true);
    setError("");
    try {
      const res = await api.search(q);
      setPanels(res.panels);
      setWarnings(res.warnings || []);
    } catch (err: any) {
      setError(err.message || String(err));
      setPanels(null);
    } finally {
      setLoading(false);
    }
  }

  // Auto-run if initial query present
  if (initial) void runSearch();

  return (
    <div class="space-y-4">
      <header>
        <h1 class="m-0 mb-1 text-xl font-semibold tracking-tight">Search</h1>
        <p class="m-0 text-[14px] text-fg-secondary">Hybrid docs + code search via PreviewHandler.</p>
      </header>

      <form class="flex flex-wrap gap-2" onSubmit={runSearch}>
        <input
          type="search"
          class="min-w-[240px] flex-1 rounded-md border border-border bg-surface px-3 py-2 text-[14px] outline-none focus:border-accent"
          placeholder="Search docs and code…"
          value={query()}
          onInput={(e) => setQuery(e.currentTarget.value)}
        />
        <button
          type="submit"
          class="rounded-md bg-accent px-4 py-2 text-[13px] font-semibold text-ink disabled:opacity-50"
          disabled={loading() || !query().trim()}
        >
          {loading() ? "Searching…" : "Search"}
        </button>
      </form>

      <Show when={error()}>
        <div class="rounded-md border border-negative/35 bg-negative/10 px-3 py-2 text-[13px] text-negative">{error()}</div>
      </Show>
      <Show when={warnings().length}>
        <div class="rounded-md border border-border bg-elevated px-3 py-2 text-[12px] text-fg-muted">{warnings().join(" · ")}</div>
      </Show>

      <Show when={panels()}>
        {(p) => (
          <div class="grid gap-3 md:grid-cols-2">
            <ResultList title="Docs semantic" items={p().docsSemantic || []} />
            <ResultList title="Docs graph" items={p().docsGraph || []} />
            <ResultList title="Code semantic" items={p().codeSemantic || []} />
            <ResultList title="Code graph" items={p().codeGraph || []} />
          </div>
        )}
      </Show>
    </div>
  );
}
