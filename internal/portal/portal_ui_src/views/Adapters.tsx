import { createSignal, For, Show, onMount } from "solid-js";
import { PhArrowSquareOut } from "../components/Icons";
import { api, type Adapter } from "../api";
import PageHeader from "../components/PageHeader";
import PageFeedback from "../components/PageFeedback";
import ListSkeleton from "../components/ListSkeleton";
import EmptyState from "../components/EmptyState";
import ResourceRow from "../components/ResourceRow";
import StatusPill from "../components/StatusPill";
import EnableSwitch from "../components/EnableSwitch";
import { usePageFeedback } from "../lib/usePageFeedback";

export default function Adapters() {
  const [adapters, setAdapters] = createSignal<Adapter[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});
  const fb = usePageFeedback();

  function tierKind(tier: string): "ok" | "warn" | "muted" {
    switch (tier) {
      case "stable":
        return "ok";
      case "manual":
        return "warn";
      default:
        return "muted";
    }
  }

  async function load() {
    setLoading(true);
    fb.clear();
    try {
      setAdapters(await api.getAdapters());
    } catch (e) {
      fb.fail(e);
    } finally {
      setLoading(false);
    }
  }

  async function toggleEnabled(adapter: Adapter, next: boolean) {
    setToggling((t) => ({ ...t, [adapter.id]: true }));
    fb.clearError();
    try {
      const updated = await api.setAdapterEnabled(adapter.id, next);
      setAdapters((list) => list.map((a) => (a.id === adapter.id ? { ...a, enabled: updated.enabled } : a)));
      fb.flash(next ? `Enabled ${adapter.name}` : `Disabled ${adapter.name}`);
    } catch (e) {
      fb.fail(e);
    } finally {
      setToggling((t) => ({ ...t, [adapter.id]: false }));
    }
  }

  onMount(load);

  const enabledCount = () => adapters().filter((a) => a.enabled).length;
  const disabledCount = () => adapters().filter((a) => !a.enabled).length;

  return (
    <div>
      <PageHeader
        title="Adapters"
        subtitle={
          loading()
            ? "Loading adapters..."
            : `${adapters().length} providers · ${enabledCount()} enabled · ${disabledCount()} disabled. Disable keeps providers listed; they are skipped during sync.`
        }
      />

      <PageFeedback error={fb.error()} success={fb.success()} class="!px-0" />

      <Show when={loading()}>
        <div class="surface overflow-hidden fade-in-up is-visible">
          <ListSkeleton aria-label="Loading adapters" />
        </div>
      </Show>

      <Show when={!loading() && adapters().length === 0 && !fb.error()}>
        <div class="fade-in-up is-visible rounded-lg border border-dashed border-border-strong bg-surface">
          <EmptyState title="No adapters configured" description="Provider adapters appear here once presets are available." />
        </div>
      </Show>

      <Show when={!loading() && adapters().length > 0}>
        <div class="surface overflow-hidden fade-in-up is-visible">
          <ul class="m-0 list-none divide-y divide-border p-0">
            <For each={adapters()}>
              {(adapter) => (
                <ResourceRow enabled={adapter.enabled}>
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="text-[14px] font-semibold tracking-tight text-fg">{adapter.name}</span>
                      <span class="font-mono text-[11.5px] text-fg-muted">{adapter.id}</span>
                      <StatusPill kind={tierKind(adapter.tier)}>{adapter.tier}</StatusPill>
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
                    <EnableSwitch
                      checked={adapter.enabled}
                      disabled={toggling()[adapter.id]}
                      aria-label={`Enable provider ${adapter.name}`}
                      onChange={(next) => void toggleEnabled(adapter, next)}
                    />
                  </div>
                </ResourceRow>
              )}
            </For>
          </ul>
        </div>
      </Show>
    </div>
  );
}
