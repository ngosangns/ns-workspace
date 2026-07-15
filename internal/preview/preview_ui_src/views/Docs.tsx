import { createMemo, createResource, createSignal, For, Show, Suspense } from "solid-js";
import { A, useParams } from "@solidjs/router";
import { api, type SpecDocument } from "../lib/api";
import { renderDocumentBody } from "../lib/markdown";

export default function Docs() {
  const params = useParams();
  const [docs] = createResource(() => api.getDocs());
  const [project] = createResource(() => api.getProject());
  const [filter, setFilter] = createSignal("");

  const selectedId = () => params.id || "";

  const [selected] = createResource(selectedId, async (id) => {
    if (!id) return null;
    return api.getDoc(id);
  });

  const filtered = () => {
    const list = docs() || [];
    const q = filter().trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (d) =>
        d.title.toLowerCase().includes(q) ||
        d.path.toLowerCase().includes(q) ||
        d.id.toLowerCase().includes(q) ||
        (d.category || "").toLowerCase().includes(q),
    );
  };

  return (
    <div class="grid gap-4 lg:grid-cols-[280px_1fr]">
      <aside class="rounded-xl border border-border bg-surface p-3">
        <div class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">
          {project()?.name || "Project"} · {project()?.totalSpecs ?? "…"} docs
        </div>
        <input
          type="search"
          class="mb-3 w-full rounded-md border border-border bg-app-muted px-2.5 py-2 text-[13px] text-fg outline-none focus:border-accent"
          placeholder="Filter docs…"
          value={filter()}
          onInput={(e) => setFilter(e.currentTarget.value)}
        />
        <Show when={docs.loading}>
          <div class="text-[13px] text-fg-muted">Loading…</div>
        </Show>
        <Show when={docs.error}>
          <div class="text-[13px] text-negative">{String(docs.error)}</div>
        </Show>
        <ul class="m-0 max-h-[70vh] list-none space-y-0.5 overflow-auto p-0">
          <For each={filtered()}>
            {(doc: SpecDocument) => (
              <li>
                <A
                  href={`/docs/${encodeURIComponent(doc.id)}`}
                  class={`block rounded-md px-2.5 py-2 text-[13px] transition hover:bg-hover ${
                    selectedId() === doc.id ? "bg-accent-soft text-fg" : "text-fg-secondary"
                  }`}
                >
                  <div class="font-medium text-fg">{doc.title}</div>
                  <div class="truncate font-mono text-[11px] text-fg-muted">{doc.path}</div>
                </A>
              </li>
            )}
          </For>
        </ul>
      </aside>

      <section class="min-h-[60vh] rounded-xl border border-border bg-surface p-5">
        <Show when={!selectedId()}>
          <div class="text-fg-secondary">
            <h1 class="m-0 mb-2 text-xl font-semibold text-fg">{project()?.generatedTitle || "Docs"}</h1>
            <p class="m-0 text-[14px]">Select a document from the list to read it.</p>
          </div>
        </Show>
        <Suspense fallback={<div class="text-fg-muted">Loading document…</div>}>
          <Show when={selected()}>
            {(doc) => {
              const bodyHtml = createMemo(() => renderDocumentBody(doc()));
              return (
                <article>
                  <header class="mb-4 border-b border-border pb-3">
                    <h1 class="m-0 text-2xl font-semibold tracking-tight text-fg">{doc().title}</h1>
                    <div class="mt-1 font-mono text-xs text-fg-muted">{doc().path}</div>
                    <Show when={doc().description}>
                      <p class="mt-2 text-[14px] text-fg-secondary">{doc().description}</p>
                    </Show>
                  </header>
                  <div class="prose-doc" data-testid="doc-body" innerHTML={bodyHtml()} />
                </article>
              );
            }}
          </Show>
        </Suspense>
      </section>
    </div>
  );
}
