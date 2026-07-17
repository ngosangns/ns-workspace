import { createSignal, createMemo, For, Show, onMount } from "solid-js";
import { PhFloppyDisk, PhArrowCounterClockwise } from "../components/Icons";
import { api, type RegistrySkills, type RegistrySkillItem } from "../api";
import CodeEditor from "../components/CodeEditor";
import UiButton from "../components/UiButton";
import PageHeader from "../components/PageHeader";
import PageFeedback from "../components/PageFeedback";
import ListSkeleton from "../components/ListSkeleton";
import EmptyState from "../components/EmptyState";
import ResourceRow from "../components/ResourceRow";
import StatusPill from "../components/StatusPill";
import EnableSwitch from "../components/EnableSwitch";
import UiSegmented from "../components/UiSegmented";
import SearchInput from "../components/SearchInput";
import { usePageFeedback } from "../lib/usePageFeedback";

type Tab = "list" | "file";

export default function Registry() {
  const [registry, setRegistry] = createSignal<RegistrySkills | null>(null);
  const [raw, setRaw] = createSignal("");
  const [tab, setTab] = createSignal<Tab>("list");
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [resetting, setResetting] = createSignal(false);
  const [query, setQuery] = createSignal("");
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});
  const fb = usePageFeedback();

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

  const filtered = createMemo(() => {
    const q = query().trim().toLowerCase();
    if (!q) return items();
    return items().filter((i) => {
      const hay = `${i.name} ${i.source ?? ""} ${i.skill} ${i.installer ?? ""}`.toLowerCase();
      return hay.includes(q);
    });
  });

  const enabledCount = createMemo(() => items().filter((i) => i.enabled).length);
  const disabledCount = createMemo(() => items().filter((i) => !i.enabled).length);
  const isOverridden = createMemo(() => registry()?.overridden ?? false);

  function applyRegistry(reg: RegistrySkills) {
    setRegistry(reg);
    setRaw(JSON.stringify({ skills: reg.skills ?? [] }, null, 2));
  }

  async function load() {
    setLoading(true);
    fb.clear();
    try {
      applyRegistry(await api.getRegistry());
    } catch (e) {
      fb.fail(e);
    } finally {
      setLoading(false);
    }
  }

  async function save() {
    if (!isValid()) {
      fb.setError("Invalid JSON");
      return;
    }
    setSaving(true);
    fb.clear();
    try {
      const parsed = JSON.parse(raw());
      applyRegistry(await api.updateRegistry(parsed));
      fb.flash("Saved — removed skills move to disabled list");
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
      applyRegistry(await api.resetRegistry());
      fb.flash("Reset registry to embedded default");
    } catch (e) {
      fb.fail(e);
    } finally {
      setResetting(false);
    }
  }

  async function toggleEnabled(item: RegistrySkillItem, next: boolean) {
    setToggling((t) => ({ ...t, [item.name]: true }));
    fb.clearError();
    try {
      applyRegistry(await api.setRegistrySkillEnabled(item.name, next));
      fb.flash(next ? `Enabled ${item.name}` : `Disabled ${item.name}`);
    } catch (e) {
      fb.fail(e);
    } finally {
      setToggling((t) => ({ ...t, [item.name]: false }));
    }
  }

  onMount(load);

  return (
    <div>
      <PageHeader
        title="Registry Skills"
        subtitle={
          loading()
            ? "Loading registry..."
            : `${items().length} skills · ${enabledCount()} enabled · ${disabledCount()} disabled. Disable keeps the entry (sync skips it); it is not deleted.`
        }
      />

      <div class="surface overflow-hidden fade-in-up is-visible">
        <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
          <UiSegmented
            value={tab()}
            options={[
              { value: "list" as const, label: "List" },
              { value: "file" as const, label: "Edit JSON" },
            ]}
            aria-label="Registry views"
            onChange={setTab}
          />
          <Show when={tab() === "list"}>
            <SearchInput value={query()} placeholder="Filter by name or source…" aria-label="Filter registry skills" onInput={setQuery} />
          </Show>
          <div class="flex-1" />
          <Show when={!loading()}>
            <StatusPill kind="ok">{enabledCount()} enabled</StatusPill>
          </Show>
          <Show when={!loading() && disabledCount()}>
            <StatusPill kind="muted">{disabledCount()} disabled</StatusPill>
          </Show>
          <StatusPill kind={isOverridden() ? "warn" : "ok"}>{isOverridden() ? "Custom" : "Default"}</StatusPill>
          <UiButton variant="warning" size="sm" disabled={!isOverridden() || loading()} loading={resetting()} onClick={() => void reset()}>
            <PhArrowCounterClockwise size={14} weight="bold" />
            Reset
          </UiButton>
        </div>

        <PageFeedback error={fb.error()} success={fb.success()} />

        <Show when={loading()}>
          <ListSkeleton aria-label="Loading registry" />
        </Show>

        <Show when={!loading() && tab() === "list"}>
          <Show when={filtered().length === 0}>
            <EmptyState
              title={items().length === 0 ? "No registry skills" : "No matches"}
              description={
                items().length === 0 ? "Skills installed via npx skills add during sync appear here." : "Try a different filter."
              }
            />
          </Show>
          <Show when={filtered().length > 0}>
            <ul class="m-0 list-none divide-y divide-border p-0">
              <For each={filtered()}>
                {(item) => (
                  <ResourceRow enabled={item.enabled}>
                    <div class="min-w-0 flex-1">
                      <div class="font-mono text-[14px] font-semibold text-fg">{item.name}</div>
                      <div class="mt-0.5 truncate font-mono text-[11.5px] text-fg-muted">{item.source || item.installer || item.skill}</div>
                    </div>
                    <div class="flex shrink-0 items-center gap-3 self-center">
                      <EnableSwitch
                        checked={item.enabled}
                        disabled={toggling()[item.name]}
                        aria-label={`Enable registry skill ${item.name}`}
                        onChange={(next) => void toggleEnabled(item, next)}
                      />
                    </div>
                  </ResourceRow>
                )}
              </For>
            </ul>
          </Show>
        </Show>

        <Show when={!loading() && tab() === "file"}>
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <UiButton variant="primary" disabled={!isValid()} loading={saving()} onClick={() => void save()}>
              <PhFloppyDisk size={16} weight="bold" />
              Save
            </UiButton>
            <StatusPill kind={isValid() ? "ok" : "err"}>{isValid() ? "Valid JSON" : "Invalid JSON"}</StatusPill>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">Enabled list only — disabled entries stay in the disabled overlay.</span>
          </div>
          <CodeEditor value={raw()} onChange={setRaw} lang="json" />
        </Show>
      </div>
    </div>
  );
}
