import { createSignal, For, Show, onMount } from "solid-js";
import { A } from "@solidjs/router";
import { PhFolder, PhFile } from "../components/Icons";
import { api, type Skill, type MCPManifest, type RegistrySkills, type Adapter, type StatusSummary } from "../api";
import AppAlert from "../components/AppAlert";
import SyncPanel from "../components/SyncPanel";
import PageHeader from "../components/PageHeader";
import StatusPill from "../components/StatusPill";
import { errMessage } from "../lib/errors";

export default function Dashboard() {
  const [skills, setSkills] = createSignal<Skill[]>([]);
  const [mcps, setMcps] = createSignal<MCPManifest | null>(null);
  const [registry, setRegistry] = createSignal<RegistrySkills | null>(null);
  const [adapters, setAdapters] = createSignal<Adapter[]>([]);
  const [status, setStatus] = createSignal<StatusSummary | null>(null);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal("");

  async function load(showLoading = true) {
    if (showLoading) setLoading(true);
    setError("");
    try {
      const [s, m, r, a, st] = await Promise.all([api.getSkills(), api.getMCPs(), api.getRegistry(), api.getAdapters(), api.getStatus()]);
      setSkills(s);
      setMcps(m);
      setRegistry(r);
      setAdapters(a);
      setStatus(st);
    } catch (e) {
      setError(errMessage(e));
    } finally {
      if (showLoading) setLoading(false);
    }
  }

  onMount(() => load());

  const mcpItems = () => {
    const m = mcps();
    if (m?.items?.length) return m.items;
    const enabled = Object.keys(m?.mcpServers ?? {}).length;
    const disabled = Object.keys(m?.disabledServers ?? {}).length;
    return { total: enabled + disabled, enabled, disabled };
  };

  const mcpStats = () => {
    const items = mcpItems();
    if (Array.isArray(items)) {
      const enabled = items.filter((i) => i.enabled).length;
      return { total: items.length, enabled, disabled: items.length - enabled };
    }
    return items;
  };

  const registryStats = () => {
    const reg = registry();
    if (reg?.items?.length) {
      const enabled = reg.items.filter((i) => i.enabled).length;
      return { total: reg.items.length, enabled, disabled: reg.items.length - enabled };
    }
    const enabled = reg?.skills?.length ?? 0;
    const disabled = reg?.disabledSkills?.length ?? 0;
    return { total: enabled + disabled, enabled, disabled };
  };

  const adapterStats = () => {
    const list = adapters();
    const enabled = list.filter((a) => a.enabled).length;
    return { total: list.length, enabled, disabled: list.length - enabled };
  };

  return (
    <div>
      <PageHeader title="Dashboard" subtitle="Overview of agents, skills, and integrations on this machine." />

      <Show when={error()}>
        <AppAlert kind="error">{error()}</AppAlert>
      </Show>

      <Show when={!error() && loading()}>
        <div class="fade-in-up is-visible" aria-busy="true" aria-label="Loading dashboard">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-[1.2fr_1fr]">
            <div class="skeleton min-h-[220px] md:row-span-2" />
            <div class="skeleton h-[120px]" />
            <div class="skeleton h-[120px]" />
            <div class="skeleton h-[120px]" />
            <div class="skeleton h-[120px]" />
          </div>
        </div>
      </Show>

      <Show when={!error() && !loading()}>
        <section class="mb-7 fade-in-up is-visible" aria-label="Summary metrics">
          <div class="grid grid-cols-1 gap-3 md:grid-cols-[1.35fr_1fr_1fr] md:grid-rows-[auto_auto]">
            <A
              href="/skills"
              class="surface flex min-h-[220px] flex-col justify-end p-[18px] no-underline transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong md:row-span-2"
              style="background: linear-gradient(165deg, var(--color-accent-soft), transparent 55%), var(--color-surface)"
            >
              <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Skills</div>
              <div class="mb-2.5 text-[2.75rem] font-semibold leading-none tracking-tighter text-fg tabular-nums">{skills().length}</div>
              <div class="mb-2.5 text-[13px] font-medium text-fg-secondary">Installed on this host</div>
              <div class="mt-auto flex flex-wrap gap-1.5">
                <StatusPill kind="accent">{skills().filter((s) => s.overridden).length} overridden</StatusPill>
                <StatusPill kind="muted">{skills().filter((s) => !s.enabled).length} disabled</StatusPill>
                <StatusPill kind="ok">{skills().filter((s) => s.enabled).length} enabled</StatusPill>
              </div>
            </A>

            <A
              href="/mcps"
              class="surface block p-[18px] no-underline transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated"
            >
              <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">MCP</div>
              <div class="mb-1.5 text-[1.625rem] font-semibold leading-none tracking-tight text-fg tabular-nums">{mcpStats().total}</div>
              <div class="text-[13px] font-medium text-fg-secondary">
                Servers · {mcpStats().enabled} on · {mcpStats().disabled} off
              </div>
            </A>

            <A
              href="/registry"
              class="surface block p-[18px] no-underline transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated"
            >
              <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Registry</div>
              <div class="mb-1.5 text-[1.625rem] font-semibold leading-none tracking-tight text-fg tabular-nums">
                {registryStats().total}
              </div>
              <div class="text-[13px] font-medium text-fg-secondary">
                Skills · {registryStats().enabled} on · {registryStats().disabled} off
              </div>
            </A>

            <A
              href="/adapters"
              class="surface block p-[18px] no-underline transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated"
            >
              <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Adapters</div>
              <div class="mb-1.5 text-[1.625rem] font-semibold leading-none tracking-tight text-fg tabular-nums">
                {adapterStats().total}
              </div>
              <div class="text-[13px] font-medium text-fg-secondary">
                Providers · {adapterStats().enabled} on · {adapterStats().disabled} off
              </div>
            </A>

            <div class="surface p-[18px] transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated md:col-span-2">
              <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Agents home</div>
              <div
                class="overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[13px] text-fg"
                title={status()?.agentsDir || undefined}
              >
                {status()?.agentsDir || "not set"}
              </div>
            </div>
          </div>
        </section>

        <section class="page-section fade-in-up is-visible">
          <h2 class="page-section-title">Path status</h2>
          <div class="bezel">
            <div class="bezel-inner">
              <Show when={!status()?.paths?.length}>
                <div class="px-[18px] py-[18px] text-[13px] text-fg-muted">No path status reported.</div>
              </Show>
              <For each={status()?.paths}>
                {(p) => (
                  <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3.5 transition duration-160 ease-[var(--ease-out-soft)] last:border-b-0 hover:bg-hover">
                    {p.isDir ? <PhFolder size={18} class="shrink-0 text-fg-muted" /> : <PhFile size={18} class="shrink-0 text-fg-muted" />}
                    <span class="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[12.5px] text-fg max-md:w-full max-md:whitespace-normal max-md:break-words">
                      {p.path}
                    </span>
                    <StatusPill kind={p.exists ? "ok" : "err"}>{p.exists ? "Exists" : "Missing"}</StatusPill>
                    <span class="w-[70px] shrink-0 text-right text-xs text-fg-muted max-md:ml-auto max-md:w-auto max-md:text-left">
                      {p.isDir ? "Directory" : "File"}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </div>
        </section>

        <section class="page-section fade-in-up is-visible">
          <SyncPanel onDone={() => load(false)} />
        </section>
      </Show>
    </div>
  );
}
