import { createSignal, createMemo, For, Show, onMount } from "solid-js";
import { PhFloppyDisk, PhArrowCounterClockwise, PhPlus, PhTrash } from "../components/Icons";
import { api, type MCPManifest, type MCPServerItem, type MCPServerConfig } from "../api";
import AppAlert from "../components/AppAlert";
import CodeEditor from "../components/CodeEditor";
import UiButton from "../components/UiButton";
import UiDialog from "../components/UiDialog";
import UiSelect from "../components/UiSelect";
import PageHeader from "../components/PageHeader";
import PageFeedback from "../components/PageFeedback";
import EmptyState from "../components/EmptyState";
import StatusPill from "../components/StatusPill";
import EnableSwitch from "../components/EnableSwitch";
import UiSegmented from "../components/UiSegmented";
import SearchInput from "../components/SearchInput";
import { usePageFeedback } from "../lib/usePageFeedback";
import {
  configToForm,
  emptyForm,
  formToConfig,
  inferTransport,
  summarizeConfig,
  transportLabel,
  validateForm,
  type MCPFormState,
} from "../lib/mcpConfig";

type Filter = "all" | "enabled" | "disabled";
type AdvancedTab = "file" | "preset";
type DialogMode = "form" | "raw";

export default function MCPs() {
  const [manifest, setManifest] = createSignal<MCPManifest | null>(null);
  const [fileRaw, setFileRaw] = createSignal("");
  const [presetRaw, setPresetRaw] = createSignal("");
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [resetting, setResetting] = createSignal(false);
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});
  const [query, setQuery] = createSignal("");
  const [filter, setFilter] = createSignal<Filter>("all");
  const [showAdvanced, setShowAdvanced] = createSignal(false);
  const [advancedTab, setAdvancedTab] = createSignal<AdvancedTab>("file");
  const [presetLoaded, setPresetLoaded] = createSignal(false);
  const fb = usePageFeedback();

  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [dialogMode, setDialogMode] = createSignal<DialogMode>("form");
  const [isCreate, setIsCreate] = createSignal(false);
  const [editingName, setEditingName] = createSignal("");
  const [editingEnabled, setEditingEnabled] = createSignal(true);
  const [form, setForm] = createSignal<MCPFormState>(emptyForm());
  const [rawOne, setRawOne] = createSignal("{}");
  const [dialogError, setDialogError] = createSignal("");
  const [dialogSaving, setDialogSaving] = createSignal(false);
  const [removing, setRemoving] = createSignal(false);

  const isOverridden = createMemo(() => manifest()?.overridden ?? false);

  const items = createMemo<MCPServerItem[]>(() => {
    const m = manifest();
    if (m?.items?.length) return m.items;
    const enabled = m?.mcpServers ?? {};
    const disabled = m?.disabledServers ?? {};
    const names = new Set([...Object.keys(enabled), ...Object.keys(disabled)]);
    return [...names]
      .sort()
      .map((name) =>
        name in enabled
          ? { name, enabled: true, config: enabled[name] as MCPServerConfig }
          : { name, enabled: false, config: disabled[name] as MCPServerConfig },
      );
  });

  const enabledCount = createMemo(() => items().filter((i) => i.enabled).length);
  const disabledCount = createMemo(() => items().filter((i) => !i.enabled).length);

  const filtered = createMemo(() => {
    let list = items();
    const f = filter();
    if (f === "enabled") list = list.filter((i) => i.enabled);
    if (f === "disabled") list = list.filter((i) => !i.enabled);
    const q = query().trim().toLowerCase();
    if (!q) return list;
    return list.filter((i) => {
      const summary = summarizeConfig(i.config).toLowerCase();
      return i.name.toLowerCase().includes(q) || summary.includes(q);
    });
  });

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
    fb.clear();
    try {
      applyManifest(await api.getMCPs());
    } catch (e) {
      fb.fail(e);
    } finally {
      setLoading(false);
    }
  }

  async function loadPreset() {
    try {
      const preset = await api.getMCPPreset();
      setPresetRaw(JSON.stringify(preset, null, 2));
      setPresetLoaded(true);
    } catch (e) {
      fb.fail(e);
    }
  }

  async function saveCatalog() {
    if (!isValidJSON()) {
      fb.setError("Invalid JSON");
      return;
    }
    setSaving(true);
    fb.clear();
    try {
      applyManifest(await api.updateMCPsContent(fileRaw()));
      fb.flash("Saved catalog");
    } catch (e) {
      fb.fail(e);
    } finally {
      setSaving(false);
    }
  }

  async function reset() {
    setResetting(true);
    fb.clear();
    try {
      applyManifest(await api.resetMCPs());
      fb.flash("Reset to embedded default");
    } catch (e) {
      fb.fail(e);
    } finally {
      setResetting(false);
    }
  }

  async function toggleEnabled(item: MCPServerItem, next: boolean) {
    setToggling((t) => ({ ...t, [item.name]: true }));
    fb.clearError();
    try {
      applyManifest(await api.setMCPEnabled(item.name, next));
      fb.flash(next ? `Enabled ${item.name}` : `Disabled ${item.name}`);
    } catch (e) {
      fb.fail(e);
    } finally {
      setToggling((t) => ({ ...t, [item.name]: false }));
    }
  }

  function openCreate() {
    setIsCreate(true);
    setEditingName("");
    setEditingEnabled(true);
    setForm(emptyForm("stdio"));
    setRawOne("{}");
    setDialogMode("form");
    setDialogError("");
    setDialogOpen(true);
  }

  function openEdit(item: MCPServerItem) {
    setIsCreate(false);
    setEditingName(item.name);
    setEditingEnabled(item.enabled);
    setForm(configToForm(item.name, item.config));
    setRawOne(JSON.stringify(item.config ?? {}, null, 2));
    setDialogMode("form");
    setDialogError("");
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    setDialogError("");
  }

  function patchForm(partial: Partial<MCPFormState>) {
    setForm((f) => ({ ...f, ...partial }));
  }

  /** Build enabled map + disabled name list from current manifest. */
  function catalogMaps(): { all: Record<string, MCPServerConfig>; disabled: string[] } {
    const all: Record<string, MCPServerConfig> = {};
    const disabled: string[] = [];
    for (const item of items()) {
      all[item.name] = (item.config ?? {}) as MCPServerConfig;
      if (!item.enabled) disabled.push(item.name);
    }
    return { all, disabled };
  }

  async function saveDialog() {
    setDialogSaving(true);
    setDialogError("");
    try {
      let name = isCreate() ? form().name.trim() : editingName();
      let config: MCPServerConfig;

      if (dialogMode() === "raw") {
        try {
          config = JSON.parse(rawOne()) as MCPServerConfig;
        } catch {
          setDialogError("Invalid JSON for server config");
          return;
        }
        if (isCreate()) {
          const err = validateForm({ ...form(), name }, { requireName: true });
          if (err) {
            setDialogError(err);
            return;
          }
        }
      } else {
        const f = form();
        const err = validateForm(f, { requireName: isCreate() });
        if (err) {
          setDialogError(err);
          return;
        }
        name = isCreate() ? f.name.trim() : editingName();
        config = formToConfig(f);
      }

      const { all, disabled } = catalogMaps();
      if (isCreate() && name in all) {
        setDialogError(`Server "${name}" already exists`);
        return;
      }

      // Rename not supported on edit — name is locked.
      all[name] = config;
      // New servers are enabled by default.
      let nextDisabled = disabled.filter((n) => n !== name);
      if (!isCreate() && !editingEnabled()) {
        if (!nextDisabled.includes(name)) nextDisabled = [...nextDisabled, name];
      }

      applyManifest(await api.updateMCPCatalog(all, nextDisabled));
      fb.flash(isCreate() ? `Added ${name}` : `Saved ${name}`);
      closeDialog();
    } catch (e) {
      setDialogError(e instanceof Error ? e.message : String(e));
    } finally {
      setDialogSaving(false);
    }
  }

  async function removeServer() {
    const name = editingName();
    if (!name || isCreate()) return;
    if (!confirm(`Remove "${name}" permanently from the catalog? Disable keeps the config; Remove deletes it.`)) {
      return;
    }
    setRemoving(true);
    setDialogError("");
    try {
      applyManifest(await api.deleteMCP(name));
      fb.flash(`Removed ${name}`);
      closeDialog();
    } catch (e) {
      setDialogError(e instanceof Error ? e.message : String(e));
    } finally {
      setRemoving(false);
    }
  }

  async function openAdvanced() {
    setShowAdvanced(true);
    setAdvancedTab("file");
  }

  async function showPresetTab() {
    setAdvancedTab("preset");
    if (!presetLoaded()) await loadPreset();
  }

  onMount(load);

  const fieldClass =
    "w-full rounded-md border border-border bg-surface px-3 py-2 text-[13px] text-fg outline-none focus:border-accent-ring font-mono";
  const labelClass = "text-[11px] font-medium uppercase tracking-wide text-fg-muted";

  return (
    <div>
      <PageHeader
        title="MCP Servers"
        subtitle={
          loading()
            ? "Loading MCP catalog..."
            : `${items().length} servers · ${enabledCount()} enabled · ${disabledCount()} disabled. Disable keeps config (sync only ships enabled).`
        }
      />

      <div class="mb-4 flex flex-wrap items-center gap-3 fade-in-up is-visible">
        <SearchInput value={query()} placeholder="Search servers…" aria-label="Search MCP servers" onInput={setQuery} />
        <UiSegmented
          value={filter()}
          options={[
            { value: "all" as const, label: "All" },
            { value: "enabled" as const, label: "Enabled" },
            { value: "disabled" as const, label: "Disabled" },
          ]}
          aria-label="Filter by status"
          onChange={setFilter}
        />
        <div class="flex-1" />
        <Show when={!loading()}>
          <StatusPill kind="ok">{enabledCount()} on</StatusPill>
          <Show when={disabledCount()}>
            <StatusPill kind="muted">{disabledCount()} off</StatusPill>
          </Show>
        </Show>
        <StatusPill kind={isOverridden() ? "warn" : "ok"}>{isOverridden() ? "Custom" : "Default"}</StatusPill>
        <UiButton variant="primary" size="sm" disabled={loading()} onClick={openCreate}>
          <PhPlus size={14} weight="bold" />
          Add server
        </UiButton>
        <UiButton variant="secondary" size="sm" disabled={loading()} onClick={() => void openAdvanced()}>
          Advanced
        </UiButton>
        <UiButton variant="warning" size="sm" disabled={!isOverridden() || loading()} loading={resetting()} onClick={() => void reset()}>
          <PhArrowCounterClockwise size={14} weight="bold" />
          Reset
        </UiButton>
      </div>

      <PageFeedback error={fb.error()} success={fb.success()} class="!px-0 mb-3" />

      <Show when={loading()}>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-busy="true">
          <For each={[1, 2, 3, 4, 5, 6]}>{() => <div class="skeleton h-28 rounded-lg" />}</For>
        </div>
      </Show>

      <Show when={!loading() && filtered().length === 0}>
        <div class="surface">
          <EmptyState
            title={items().length === 0 ? "No MCP servers" : "No matches"}
            description={items().length === 0 ? "Add a server or edit the catalog in Advanced." : "Try a different filter."}
          >
            <Show when={items().length === 0}>
              <UiButton variant="primary" onClick={openCreate}>
                <PhPlus size={14} weight="bold" />
                Add server
              </UiButton>
            </Show>
          </EmptyState>
        </div>
      </Show>

      <Show when={!loading() && filtered().length > 0}>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 fade-in-up is-visible">
          <For each={filtered()}>
            {(item) => {
              const transport = () => inferTransport(item.config);
              return (
                <div
                  class={`surface flex flex-col gap-2 p-4 transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated ${
                    item.enabled ? "" : "opacity-70"
                  }`}
                >
                  <button type="button" class="min-w-0 flex-1 text-left" onClick={() => openEdit(item)}>
                    <div class="mb-1.5 flex flex-wrap items-center gap-2">
                      <span class="font-mono text-[14px] font-semibold text-fg">{item.name}</span>
                      <StatusPill kind={transport() === "unknown" ? "muted" : "accent"}>{transportLabel(transport())}</StatusPill>
                    </div>
                    <div class="line-clamp-2 font-mono text-[11.5px] leading-snug text-fg-muted">{summarizeConfig(item.config) || "—"}</div>
                  </button>
                  <div class="mt-auto flex items-center justify-end border-t border-border pt-2" onClick={(e) => e.stopPropagation()}>
                    <EnableSwitch
                      checked={item.enabled}
                      disabled={toggling()[item.name]}
                      aria-label={`Enable MCP ${item.name}`}
                      onChange={(next) => void toggleEnabled(item, next)}
                    />
                  </div>
                </div>
              );
            }}
          </For>
        </div>
      </Show>

      {/* Advanced panel */}
      <Show when={showAdvanced()}>
        <div class="surface mt-6 overflow-hidden fade-in-up is-visible">
          <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
            <span class="text-[13px] font-semibold text-fg">Advanced</span>
            <UiSegmented
              value={advancedTab()}
              options={[
                { value: "file" as const, label: "Catalog JSON" },
                { value: "preset" as const, label: "Embedded preset" },
              ]}
              aria-label="Advanced views"
              onChange={(v) => {
                if (v === "preset") void showPresetTab();
                else setAdvancedTab("file");
              }}
            />
            <div class="flex-1" />
            <UiButton size="sm" variant="ghost" onClick={() => setShowAdvanced(false)}>
              Close
            </UiButton>
          </div>
          <Show when={advancedTab() === "file"}>
            <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
              <UiButton variant="primary" loading={saving()} disabled={!isValidJSON()} onClick={() => void saveCatalog()}>
                <PhFloppyDisk size={16} weight="bold" />
                Save catalog
              </UiButton>
              <Show when={!isValidJSON()}>
                <span class="text-[12.5px] text-negative">Invalid JSON</span>
              </Show>
              <div class="flex-1" />
              <span class="text-[12.5px] text-fg-muted">
                Shape: <code class="font-mono text-[12px]">mcpServers</code> + optional{" "}
                <code class="font-mono text-[12px]">disabled[]</code>
              </span>
            </div>
            <CodeEditor value={fileRaw()} onChange={setFileRaw} lang="json" />
          </Show>
          <Show when={advancedTab() === "preset"}>
            <div class="border-b border-border px-4 py-2 text-[12.5px] text-fg-muted">Read-only embedded default (not your overlay).</div>
            <CodeEditor value={presetRaw() || "Loading…"} lang="json" readonly />
          </Show>
        </div>
      </Show>

      {/* Server dialog */}
      <UiDialog
        open={dialogOpen()}
        title={isCreate() ? "Add MCP server" : editingName()}
        subtitle={
          isCreate()
            ? "New server is enabled by default"
            : `${transportLabel(inferTransport(formToConfig(form())))} · ${editingEnabled() ? "enabled" : "disabled"}`
        }
        onClose={closeDialog}
      >
        <div class="mb-3 flex flex-wrap items-center gap-2">
          <UiSegmented
            value={dialogMode()}
            options={[
              { value: "form" as const, label: "Form" },
              { value: "raw" as const, label: "Raw JSON" },
            ]}
            aria-label="Edit mode"
            onChange={(mode) => {
              if (mode === "raw") {
                setRawOne(JSON.stringify(formToConfig(form()), null, 2));
              } else {
                try {
                  const cfg = JSON.parse(rawOne()) as MCPServerConfig;
                  setForm(configToForm(isCreate() ? form().name : editingName(), cfg));
                } catch {
                  /* keep form */
                }
              }
              setDialogMode(mode);
            }}
          />
        </div>

        <Show when={dialogError()}>
          <AppAlert kind="error">{dialogError()}</AppAlert>
        </Show>

        <Show when={dialogMode() === "form"}>
          <div class="space-y-3">
            <Show when={isCreate()}>
              <label class="flex flex-col gap-1">
                <span class={labelClass}>Name</span>
                <input
                  class={fieldClass}
                  value={form().name}
                  placeholder="my-server"
                  onInput={(e) => patchForm({ name: e.currentTarget.value })}
                />
              </label>
            </Show>
            <div class="flex flex-col gap-1">
              <label class={labelClass} for="mcp-transport">
                Transport
              </label>
              <UiSelect
                id="mcp-transport"
                value={form().transport}
                options={[
                  { value: "stdio", label: "stdio (command)" },
                  { value: "http", label: "HTTP" },
                  { value: "sse", label: "SSE" },
                ]}
                onChange={(v) => patchForm({ transport: v as MCPFormState["transport"] })}
              />
            </div>
            <Show when={form().transport === "stdio"}>
              <label class="flex flex-col gap-1">
                <span class={labelClass}>Command</span>
                <input
                  class={fieldClass}
                  value={form().command}
                  placeholder="npx"
                  onInput={(e) => patchForm({ command: e.currentTarget.value })}
                />
              </label>
              <label class="flex flex-col gap-1">
                <span class={labelClass}>Args (one per line)</span>
                <textarea
                  class={`${fieldClass} min-h-[88px] resize-y`}
                  value={form().argsText}
                  placeholder={"-y\nsome-package"}
                  onInput={(e) => patchForm({ argsText: e.currentTarget.value })}
                />
              </label>
              <label class="flex flex-col gap-1">
                <span class={labelClass}>Env (KEY=value per line)</span>
                <textarea
                  class={`${fieldClass} min-h-[72px] resize-y`}
                  value={form().envText}
                  onInput={(e) => patchForm({ envText: e.currentTarget.value })}
                />
              </label>
            </Show>
            <Show when={form().transport === "http" || form().transport === "sse"}>
              <label class="flex flex-col gap-1">
                <span class={labelClass}>URL</span>
                <input
                  class={fieldClass}
                  value={form().url}
                  placeholder="http://127.0.0.1:3000/mcp"
                  onInput={(e) => patchForm({ url: e.currentTarget.value })}
                />
              </label>
              <label class="flex flex-col gap-1">
                <span class={labelClass}>Headers (KEY=value per line)</span>
                <textarea
                  class={`${fieldClass} min-h-[72px] resize-y`}
                  value={form().headersText}
                  onInput={(e) => patchForm({ headersText: e.currentTarget.value })}
                />
              </label>
            </Show>
          </div>
        </Show>

        <Show when={dialogMode() === "raw"}>
          <CodeEditor value={rawOne()} onChange={setRawOne} lang="json" />
        </Show>

        <div class="mt-4 flex flex-wrap items-center gap-2 border-t border-border pt-4">
          <UiButton variant="primary" loading={dialogSaving()} onClick={() => void saveDialog()}>
            <PhFloppyDisk size={16} weight="bold" />
            {isCreate() ? "Add" : "Save"}
          </UiButton>
          <UiButton variant="secondary" disabled={dialogSaving() || removing()} onClick={closeDialog}>
            Cancel
          </UiButton>
          <div class="flex-1" />
          <Show when={!isCreate()}>
            <UiButton variant="danger" size="sm" loading={removing()} disabled={dialogSaving()} onClick={() => void removeServer()}>
              <PhTrash size={14} weight="bold" />
              Remove
            </UiButton>
          </Show>
        </div>
      </UiDialog>
    </div>
  );
}
