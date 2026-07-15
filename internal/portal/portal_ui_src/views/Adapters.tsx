import { createSignal, For, Show, onMount } from "solid-js";
import { PhArrowSquareOut } from "../components/Icons";
import { api, type Adapter } from "../api";
import AppAlert from "../components/AppAlert";

export default function Adapters() {
  const [adapters, setAdapters] = createSignal<Adapter[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal("");
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});

  function tierClass(tier: string): string {
    switch (tier) {
      case "stable":
        return "status-pill--ok";
      case "manual":
        return "status-pill--warn";
      default:
        return "status-pill--muted";
    }
  }

  async function load() {
    setLoading(true);
    setError("");
    try {
      setAdapters(await api.getAdapters());
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setLoading(false);
    }
  }

  async function toggleEnabled(adapter: Adapter, next: boolean) {
    setToggling((t) => ({ ...t, [adapter.id]: true }));
    setError("");
    try {
      const updated = await api.setAdapterEnabled(adapter.id, next);
      setAdapters((list) => list.map((a) => (a.id === adapter.id ? { ...a, enabled: updated.enabled } : a)));
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setToggling((t) => ({ ...t, [adapter.id]: false }));
    }
  }

  onMount(load);

  return (
    <div>
      <header class="page-header fade-in-up is-visible">
        <h1 class="page-title">Adapters</h1>
        <p class="page-subtitle">
          {loading()
            ? "Loading adapters..."
            : `${adapters().length} providers · ${adapters().filter((a) => a.enabled).length} enabled · ${adapters().filter((a) => !a.enabled).length} disabled. Disable keeps providers listed; they are skipped during sync.`}
        </p>
      </header>

      <Show when={error()}>
        <AppAlert kind="error">{error()}</AppAlert>
      </Show>

      <Show when={!error() && loading()}>
        <div class="surface overflow-hidden fade-in-up is-visible" aria-busy="true" aria-label="Loading adapters">
          <div class="space-y-0 divide-y divide-border p-0">
            <For each={[1, 2, 3, 4, 5, 6]}>
              {() => (
                <div class="px-4 py-3">
                  <div class="skeleton h-12 rounded-md" />
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      <Show when={!error() && !loading() && adapters().length === 0}>
        <div class="fade-in-up is-visible rounded-lg border border-dashed border-border-strong bg-surface px-5 py-12 text-center">
          <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No adapters configured</p>
          <p class="m-0 text-[13px] text-fg-muted">Provider adapters appear here once presets are available.</p>
        </div>
      </Show>

      <Show when={!error() && !loading() && adapters().length > 0}>
        <div class="surface overflow-hidden fade-in-up is-visible">
          <ul class="m-0 list-none divide-y divide-border p-0">
            <For each={adapters()}>
              {(adapter) => (
                <li
                  class={`flex flex-wrap items-start gap-x-4 gap-y-2 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated ${adapter.enabled ? "" : "opacity-60"}`}
                >
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="text-[14px] font-semibold tracking-tight text-fg">{adapter.name}</span>
                      <span class="font-mono text-[11.5px] text-fg-muted">{adapter.id}</span>
                      <span class={`status-pill ${tierClass(adapter.tier)}`}>{adapter.tier}</span>
                    </div>
                    <Show when={adapter.notes}>
                      <p class="m-0 mt-1 text-[13px] leading-normal text-fg-secondary">{adapter.notes}</p>
                    </Show>
                    <Show when={adapter.artifacts?.length || adapter.docs?.length}>
                      <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1.5">
                        <Show when={adapter.artifacts?.length}>
                          <div class="flex flex-wrap gap-1.5">
                            <For each={adapter.artifacts}>
                              {(artifact) => (
                                <span class="rounded-sm border border-border bg-app-muted px-2 py-[2px] font-mono text-[11px] text-fg-secondary">
                                  {artifact}
                                </span>
                              )}
                            </For>
                          </div>
                        </Show>
                        <Show when={adapter.docs?.length}>
                          <div class="flex flex-wrap gap-2.5">
                            <For each={adapter.docs}>
                              {(doc) => (
                                <a
                                  href={doc}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  class="inline-flex items-center gap-1 text-xs font-medium text-accent transition duration-160 ease-[var(--ease-out-soft)] hover:text-accent-hover hover:underline hover:underline-offset-2"
                                >
                                  <PhArrowSquareOut size={14} weight="bold" />
                                  Docs
                                </a>
                              )}
                            </For>
                          </div>
                        </Show>
                      </div>
                    </Show>
                  </div>
                  <div class="flex shrink-0 items-center gap-3 self-center">
                    <span class={`status-pill ${adapter.enabled ? "status-pill--ok" : "status-pill--muted"}`}>
                      {adapter.enabled ? "Enabled" : "Disabled"}
                    </span>
                    <label class="flex items-center gap-2">
                      <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">{adapter.enabled ? "On" : "Off"}</span>
                      <input
                        type="checkbox"
                        class="h-4 w-4 accent-[var(--color-accent)]"
                        checked={adapter.enabled}
                        disabled={toggling()[adapter.id]}
                        aria-label={`Enable provider ${adapter.name}`}
                        onChange={(e) => toggleEnabled(adapter, e.currentTarget.checked)}
                      />
                    </label>
                  </div>
                </li>
              )}
            </For>
          </ul>
        </div>
      </Show>
    </div>
  );
}
