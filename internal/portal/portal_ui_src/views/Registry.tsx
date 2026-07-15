import { createSignal, createMemo, For, Show, onMount } from "solid-js";
import { PhFloppyDisk } from "../components/Icons";
import { api, type RegistrySkills, type RegistrySkillItem } from "../api";
import AppAlert from "../components/AppAlert";
import CodeEditor from "../components/CodeEditor";
import UiButton from "../components/UiButton";
import { useFlashMessage } from "../composables/useFlashMessage";

export default function Registry() {
  const [registry, setRegistry] = createSignal<RegistrySkills | null>(null);
  const [raw, setRaw] = createSignal("");
  const [tab, setTab] = createSignal<"list" | "file">("list");
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal("");
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});
  const { message: success, flash, clear: clearSuccess } = useFlashMessage();

  const isValid = createMemo(() => {
    try {
      JSON.parse(raw());
      return true;
    } catch {
      return false;
    }
  });

  const items = createMemo<RegistrySkillItem[]>(() => {
    const reg = registry();
    if (reg?.items?.length) return reg.items;
    const enabled = (reg?.skills ?? []).map((s) => ({ ...s, enabled: true }));
    const disabled = (reg?.disabledSkills ?? []).map((s) => ({ ...s, enabled: false }));
    return [...enabled, ...disabled].sort((a, b) => a.name.localeCompare(b.name));
  });

  const enabledCount = createMemo(() => items().filter((i) => i.enabled).length);
  const disabledCount = createMemo(() => items().filter((i) => !i.enabled).length);

  function applyRegistry(reg: RegistrySkills) {
    setRegistry(reg);
    setRaw(JSON.stringify({ skills: reg.skills ?? [] }, null, 2));
  }

  async function load() {
    setLoading(true);
    setError("");
    clearSuccess();
    try {
      applyRegistry(await api.getRegistry());
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setLoading(false);
    }
  }

  async function save() {
    if (!isValid()) {
      setError("Invalid JSON");
      return;
    }
    setSaving(true);
    setError("");
    clearSuccess();
    try {
      const parsed = JSON.parse(raw());
      applyRegistry(await api.updateRegistry(parsed));
      flash("Saved — removed skills move to skills.disabled.json");
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setSaving(false);
    }
  }

  async function toggleEnabled(item: RegistrySkillItem, next: boolean) {
    setToggling((t) => ({ ...t, [item.name]: true }));
    setError("");
    clearSuccess();
    try {
      applyRegistry(await api.setRegistrySkillEnabled(item.name, next));
      flash(next ? `Enabled ${item.name}` : `Disabled ${item.name} — moved to skills.disabled.json`);
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setToggling((t) => ({ ...t, [item.name]: false }));
    }
  }

  onMount(load);

  return (
    <div>
      <header class="page-header fade-in-up is-visible">
        <h1 class="page-title">Registry Skills</h1>
        <p class="page-subtitle">
          Skills installed via{" "}
          <code class="rounded border border-border bg-app-muted px-1.5 py-px font-mono text-xs text-fg-secondary">npx skills add</code>{" "}
          during sync. Disable moves an entry into{" "}
          <code class="rounded border border-border bg-app-muted px-1 font-mono text-xs">skills.disabled.json</code> (not delete).
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
              class={`rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)] ${tab() === "file" ? "bg-surface text-fg shadow-sm" : "text-fg-secondary hover:text-fg"}`}
              onClick={() => setTab("file")}
            >
              File (enabled)
            </button>
          </div>
          <div class="flex-1" />
          <Show when={!loading()}>
            <span class="status-pill status-pill--ok">{enabledCount()} enabled</span>
          </Show>
          <Show when={!loading() && disabledCount()}>
            <span class="status-pill status-pill--muted">{disabledCount()} disabled</span>
          </Show>
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
          <div class="p-4">
            <Show when={items().length === 0}>
              <div class="rounded-lg border border-dashed border-border-strong px-5 py-10 text-center">
                <p class="m-0 text-[14px] text-fg-muted">No registry skills defined.</p>
              </div>
            </Show>
            <Show when={items().length > 0}>
              <ul class="m-0 list-none space-y-2 p-0">
                <For each={items()}>
                  {(item) => (
                    <li
                      class={`flex flex-wrap items-center gap-3 rounded-lg border border-border bg-elevated px-4 py-3 ${item.enabled ? "" : "opacity-75"}`}
                    >
                      <div class="min-w-0 flex-1">
                        <div class="font-mono text-[14px] font-semibold text-fg">{item.name}</div>
                        <div class="mt-0.5 truncate font-mono text-[11.5px] text-fg-muted">
                          {item.source || item.installer || item.skill}
                        </div>
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
                          aria-label={`Enable registry skill ${item.name}`}
                          onChange={(e) => toggleEnabled(item, e.currentTarget.checked)}
                        />
                      </label>
                    </li>
                  )}
                </For>
              </ul>
            </Show>
          </div>
        </Show>

        <Show when={!loading() && tab() === "file"}>
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <UiButton variant="primary" disabled={!isValid()} loading={saving()} onClick={save}>
              <PhFloppyDisk size={16} weight="bold" />
              Save
            </UiButton>
            <span class={`status-pill ${isValid() ? "status-pill--ok" : "status-pill--err"}`}>
              {isValid() ? "Valid JSON" : "Invalid JSON"}
            </span>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">
              Disabled skills live in <code class="font-mono text-[12px]">skills.disabled.json</code>.
            </span>
          </div>
          <CodeEditor value={raw()} onChange={setRaw} lang="json" />
        </Show>
      </div>
    </div>
  );
}
