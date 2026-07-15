import { createSignal, createMemo, For, Show, onMount } from "solid-js";
import { PhFloppyDisk, PhArrowCounterClockwise } from "../components/Icons";
import { api, type MCPManifest, type MCPServerItem } from "../api";
import AppAlert from "../components/AppAlert";
import CodeEditor from "../components/CodeEditor";
import UiButton from "../components/UiButton";
import { useFlashMessage } from "../composables/useFlashMessage";

export default function MCPs() {
  const [manifest, setManifest] = createSignal<MCPManifest | null>(null);
  const [fileRaw, setFileRaw] = createSignal("");
  const [tab, setTab] = createSignal<"list" | "edit">("list");
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [resetting, setResetting] = createSignal(false);
  const [error, setError] = createSignal("");
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});
  const { message: success, flash, clear: clearSuccess } = useFlashMessage();

  const isOverridden = createMemo(() => manifest()?.overridden ?? false);

  const items = createMemo<MCPServerItem[]>(() => {
    const m = manifest();
    if (m?.items?.length) return m.items;
    const enabled = m?.mcpServers ?? {};
    const disabled = m?.disabledServers ?? {};
    const names = new Set([...Object.keys(enabled), ...Object.keys(disabled)]);
    return [...names]
      .sort()
      .map((name) => (name in enabled ? { name, enabled: true, config: enabled[name] } : { name, enabled: false, config: disabled[name] }));
  });

  const enabledCount = createMemo(() => items().filter((i) => i.enabled).length);
  const disabledCount = createMemo(() => items().filter((i) => !i.enabled).length);

  const isValidJSON = createMemo(() => {
    try {
      JSON.parse(fileRaw());
      return true;
    } catch {
      return false;
    }
  });

  function applyManifest(m: MCPManifest) {
    setManifest(m);
    setFileRaw(
      m.content ||
        JSON.stringify(
          {
            mcpServers: { ...(m.mcpServers ?? {}), ...(m.disabledServers ?? {}) },
            disabled: Object.keys(m.disabledServers ?? {}).sort(),
          },
          null,
          2,
        ),
    );
  }

  async function load() {
    setLoading(true);
    setError("");
    clearSuccess();
    try {
      applyManifest(await api.getMCPs());
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setLoading(false);
    }
  }

  async function save() {
    if (!isValidJSON()) {
      setError("Invalid JSON");
      return;
    }
    setSaving(true);
    setError("");
    clearSuccess();
    try {
      applyManifest(await api.updateMCPsContent(fileRaw()));
      flash("Saved catalog");
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setSaving(false);
    }
  }

  async function reset() {
    setResetting(true);
    setError("");
    clearSuccess();
    try {
      applyManifest(await api.resetMCPs());
      flash("Reset to embedded default");
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setResetting(false);
    }
  }

  async function toggleEnabled(item: MCPServerItem, next: boolean) {
    setToggling((t) => ({ ...t, [item.name]: true }));
    setError("");
    clearSuccess();
    try {
      applyManifest(await api.setMCPEnabled(item.name, next));
      flash(next ? `Enabled ${item.name}` : `Disabled ${item.name}`);
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setToggling((t) => ({ ...t, [item.name]: false }));
    }
  }

  function summarize(config: unknown): string {
    if (!config || typeof config !== "object") return "";
    const c = config as Record<string, unknown>;
    if (typeof c.url === "string") return c.url;
    if (typeof c.command === "string") {
      const args = Array.isArray(c.args) ? c.args.map(String).join(" ") : "";
      return args ? `${c.command} ${args}` : c.command;
    }
    return JSON.stringify(config);
  }

  onMount(load);

  return (
    <div>
      <header class="page-header fade-in-up is-visible">
        <h1 class="page-title">MCP Servers</h1>
        <p class="page-subtitle">
          {loading()
            ? "Loading MCP catalog..."
            : `${items().length} servers · ${enabledCount()} enabled · ${disabledCount()} disabled. One catalog for edit; disable keeps config (sync only ships enabled).`}
        </p>
      </header>

      <div class="surface overflow-hidden fade-in-up is-visible">
        <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
          <div class="flex gap-0.5 rounded-md border border-border bg-app-muted p-0.5">
            <button
              type="button"
              class={`rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)] ${tab() === "list" ? "bg-surface text-fg shadow-sm" : "text-fg-secondary hover:text-fg"}`}
              onClick={() => setTab("list")}
            >
              List
            </button>
            <button
              type="button"
              class={`rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)] ${tab() === "edit" ? "bg-surface text-fg shadow-sm" : "text-fg-secondary hover:text-fg"}`}
              onClick={() => setTab("edit")}
            >
              Edit JSON
            </button>
          </div>
          <div class="flex-1" />
          <Show when={!loading()}>
            <span class="status-pill status-pill--ok">{enabledCount()} enabled</span>
          </Show>
          <Show when={!loading() && disabledCount()}>
            <span class="status-pill status-pill--muted">{disabledCount()} disabled</span>
          </Show>
          <span class={`status-pill ${isOverridden() ? "status-pill--warn" : "status-pill--ok"}`}>
            {isOverridden() ? "Custom" : "Default"}
          </span>
          <UiButton variant="warning" size="sm" disabled={!isOverridden() || loading()} loading={resetting()} onClick={reset}>
            <PhArrowCounterClockwise size={14} weight="bold" />
            Reset
          </UiButton>
        </div>

        <Show when={error() || success()}>
          <div class="space-y-2 px-4 pt-3">
            <Show when={error()}>
              <AppAlert kind="error" class="!mb-0">
                {error()}
              </AppAlert>
            </Show>
            <Show when={success()}>
              <AppAlert kind="success" class="!mb-0">
                {success()}
              </AppAlert>
            </Show>
          </div>
        </Show>

        <Show when={loading()}>
          <div class="min-h-[200px]" aria-busy="true">
            <div class="skeleton m-4 h-[480px] rounded-[10px]" />
          </div>
        </Show>

        <Show when={!loading() && tab() === "list"}>
          <Show when={items().length === 0}>
            <div class="px-5 py-10 text-center">
              <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No MCP servers</p>
              <p class="m-0 text-[13px] text-fg-muted">Add servers in the Edit JSON tab, then save.</p>
            </div>
          </Show>
          <Show when={items().length > 0}>
            <ul class="m-0 list-none divide-y divide-border p-0">
              <For each={items()}>
                {(item) => (
                  <li
                    class={`flex flex-wrap items-center gap-3 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated ${item.enabled ? "" : "opacity-60"}`}
                  >
                    <div class="min-w-0 flex-1">
                      <div class="font-mono text-[14px] font-semibold text-fg">{item.name}</div>
                      <div class="mt-0.5 truncate font-mono text-[11.5px] text-fg-muted">{summarize(item.config)}</div>
                    </div>
                    <span class={`status-pill ${item.enabled ? "status-pill--ok" : "status-pill--muted"}`}>
                      {item.enabled ? "Enabled" : "Disabled"}
                    </span>
                    <label class="flex items-center gap-2">
                      <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">{item.enabled ? "On" : "Off"}</span>
                      <input
                        type="checkbox"
                        class="h-4 w-4 accent-[var(--color-accent)]"
                        checked={item.enabled}
                        disabled={toggling()[item.name]}
                        aria-label={`Enable MCP ${item.name}`}
                        onChange={(e) => toggleEnabled(item, e.currentTarget.checked)}
                      />
                    </label>
                  </li>
                )}
              </For>
            </ul>
          </Show>
        </Show>

        <Show when={!loading() && tab() === "edit"}>
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <UiButton variant="primary" loading={saving()} disabled={!isValidJSON()} onClick={save}>
              <PhFloppyDisk size={16} weight="bold" />
              Save catalog
            </UiButton>
            <Show when={!isValidJSON()}>
              <span class="text-[12.5px] text-negative">Invalid JSON</span>
            </Show>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">
              Shape: <code class="font-mono text-[12px]">mcpServers</code> + optional <code class="font-mono text-[12px]">disabled[]</code>
            </span>
          </div>
          <CodeEditor value={fileRaw()} onChange={setFileRaw} lang="json" />
        </Show>
      </div>
    </div>
  );
}
