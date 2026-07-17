import { createSignal, createMemo, createEffect, For, Show, onMount } from "solid-js";
import { useSearchParams } from "@solidjs/router";
import { PhArrowCounterClockwise, PhDownloadSimple, PhTrash } from "../components/Icons";
import { api, type Skill, type CatalogSkill, type RegistrySource } from "../api";
import AppAlert from "../components/AppAlert";
import CodeEditor from "../components/CodeEditor";
import UiButton from "../components/UiButton";
import UiDialog from "../components/UiDialog";
import UiSelect from "../components/UiSelect";
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
import { catalogKey, formatInstalls, sourceKind, sourceLabel } from "./skills/format";

type Tab = "installed" | "discover";

const REGISTRY_ALL = "all";

/** Normalize search param values (string | string[] | undefined). */
function paramStr(v: string | string[] | undefined, fallback = ""): string {
  if (Array.isArray(v)) return (v[0] ?? fallback).toString();
  if (v == null || v === "") return fallback;
  return String(v);
}

function RegistryFilterSelect(props: {
  id: string;
  value: string;
  options: { value: string; label: string }[];
  loading?: boolean;
  onChange: (value: string) => void;
}) {
  const options = () =>
    props.options.length > 1
      ? props.options
      : [
          { value: REGISTRY_ALL, label: "All registries" },
          ...(props.loading ? [{ value: "__loading", label: "Loading registries…" }] : []),
        ];
  return (
    <div class="min-w-[200px]">
      <label class="sr-only" for={props.id}>
        Registry
      </label>
      <UiSelect
        id={props.id}
        value={props.value}
        options={options()}
        disabled={props.loading}
        onChange={(v) => {
          if (v === "__loading") return;
          props.onChange(v);
        }}
      />
    </div>
  );
}

export default function Skills() {
  /** URL: #/skills?tab=discover&registry=owner/repo&q=name */
  const [searchParams, setSearchParams] = useSearchParams();

  const tab = createMemo((): Tab => (paramStr(searchParams.tab) === "discover" ? "discover" : "installed"));
  const registryFilter = createMemo(() => {
    const r = paramStr(searchParams.registry, REGISTRY_ALL).trim();
    return r || REGISTRY_ALL;
  });
  const discoverQuery = createMemo(() => paramStr(searchParams.q));

  const [skills, setSkills] = createSignal<Skill[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});
  const [localQuery, setLocalQuery] = createSignal("");
  const fb = usePageFeedback();

  const [registries, setRegistries] = createSignal<RegistrySource[]>([]);
  const [registriesLoading, setRegistriesLoading] = createSignal(false);
  const [catalog, setCatalog] = createSignal<CatalogSkill[]>([]);
  /** Package SKILL.md counts from /catalog — kept separate so select options stay stable. */
  const [catalogCounts, setCatalogCounts] = createSignal<Record<string, number>>({});
  const [selectedKeys, setSelectedKeys] = createSignal<Record<string, boolean>>({});
  const [searching, setSearching] = createSignal(false);
  const [searchError, setSearchError] = createSignal("");
  const [installing, setInstalling] = createSignal<Record<string, boolean>>({});
  const [uninstalling, setUninstalling] = createSignal<Record<string, boolean>>({});
  const [batchInstalling, setBatchInstalling] = createSignal(false);
  let catalogRequestId = 0;
  /** Avoid re-fetch when effect re-runs for unrelated reasons with same key. */
  let lastCatalogKey = "";

  const [dialog, setDialog] = createSignal(false);
  const [selected, setSelected] = createSignal<Skill | null>(null);
  const [content, setContent] = createSignal("");
  const [dialogLoading, setDialogLoading] = createSignal(false);
  const [dialogError, setDialogError] = createSignal("");

  const listableRegistries = createMemo(() => registries().filter((r) => r.listable));

  /** Options for both tabs: All + each listable GitHub registry. */
  const registryOptions = createMemo(() => {
    const counts = catalogCounts();
    const opts = [{ value: REGISTRY_ALL, label: "All registries" }];
    for (const reg of listableRegistries()) {
      let label = reg.source;
      const packageCount = counts[reg.source] ?? 0;
      if (packageCount > 0) {
        label += ` (${packageCount} skill${packageCount === 1 ? "" : "s"})`;
      } else {
        const configured = reg.enabledEntries + reg.disabledEntries;
        if (configured > 0) {
          label += ` (${configured} configured)`;
        }
      }
      opts.push({ value: reg.source, label });
    }
    return opts;
  });

  const filteredInstalled = createMemo(() => {
    const q = localQuery().trim().toLowerCase();
    const reg = registryFilter();
    return skills().filter((s) => {
      if (reg !== REGISTRY_ALL) {
        const src = (s.registrySource ?? "").trim();
        if (src !== reg) return false;
      }
      if (!q) return true;
      const hay = `${s.name} ${s.id} ${s.description ?? ""} ${s.source} ${s.registrySource ?? ""}`.toLowerCase();
      return hay.includes(q);
    });
  });

  const filteredCatalog = createMemo(() => {
    const q = discoverQuery().trim().toLowerCase();
    const list = catalog();
    if (!q) return list;
    return list.filter((s) => {
      const hay = `${s.name} ${s.skillId} ${s.source} ${s.path ?? ""}`.toLowerCase();
      return hay.includes(q);
    });
  });

  /** Selected rows that are still visible (respects name filter). */
  const selectedCatalogItems = createMemo(() => {
    const keys = selectedKeys();
    return filteredCatalog().filter((s) => keys[catalogKey(s)]);
  });

  const selectedCount = createMemo(() => selectedCatalogItems().length);

  async function load() {
    setLoading(true);
    fb.clearError();
    try {
      setSkills(await api.getSkills());
    } catch (e) {
      fb.fail(e);
    } finally {
      setLoading(false);
    }
  }

  async function loadRegistries() {
    setRegistriesLoading(true);
    try {
      const res = await api.listSkillRegistries();
      setRegistries(res.registries ?? []);
    } catch (e) {
      setSearchError(e instanceof Error ? e.message : String(e));
      setRegistries([]);
    } finally {
      setRegistriesLoading(false);
    }
  }

  async function loadCatalog(opts?: { registry?: string; refresh?: boolean }) {
    const requestId = ++catalogRequestId;
    const registry = (opts?.registry ?? registryFilter() ?? REGISTRY_ALL).trim() || REGISTRY_ALL;
    const refresh = opts?.refresh === true;

    setSearching(true);
    setSearchError("");
    fb.clearSuccess();
    setSelectedKeys({});
    try {
      if (listableRegistries().length === 0 && registry !== REGISTRY_ALL) {
        if (requestId !== catalogRequestId) return;
        setCatalog([]);
        setSearchError("No listable GitHub registries available. Check Registry page sources.");
        return;
      }
      if (listableRegistries().length === 0) {
        if (requestId !== catalogRequestId) return;
        setCatalog([]);
        setSearchError("No listable GitHub registries available. Check Registry page sources.");
        return;
      }

      const res = await api.listSkillsCatalog({
        registry: registry === REGISTRY_ALL ? REGISTRY_ALL : registry,
        refresh,
      });
      if (requestId !== catalogRequestId) return;

      const skillsList = res.skills ?? [];
      setCatalog(skillsList);

      // Refresh per-source package counts from the loaded list.
      if (registry === REGISTRY_ALL) {
        const bySource: Record<string, number> = {};
        for (const sk of skillsList) {
          if (!sk.source) continue;
          bySource[sk.source] = (bySource[sk.source] ?? 0) + 1;
        }
        setCatalogCounts((prev) => ({ ...prev, ...bySource }));
      } else {
        const packageCount = typeof res.count === "number" ? res.count : skillsList.length;
        setCatalogCounts((m) => (m[registry] === packageCount ? m : { ...m, [registry]: packageCount }));
      }

      if (!skillsList.length) {
        setSearchError(
          registry === REGISTRY_ALL ? "No skills found across listable registries" : `No skills found in registry ${registry}`,
        );
      }
    } catch (err) {
      if (requestId !== catalogRequestId) return;
      setCatalog([]);
      setSearchError(err instanceof Error ? err.message : String(err));
    } finally {
      if (requestId === catalogRequestId) {
        setSearching(false);
      }
    }
  }

  async function reset(id: string) {
    try {
      await api.resetSkill(id);
      await load();
      fb.flash(`Reset ${id}`);
    } catch (e) {
      fb.fail(e);
    }
  }

  async function toggleEnabled(skill: Skill, next: boolean) {
    setToggling((t) => ({ ...t, [skill.id]: true }));
    fb.clearError();
    try {
      const updated = await api.setSkillEnabled(skill.id, next);
      setSkills((list) =>
        list.map((s) => (s.id === skill.id ? { ...s, enabled: updated.enabled, description: updated.description ?? s.description } : s)),
      );
      fb.flash(next ? `Enabled ${skill.name}` : `Disabled ${skill.name}`);
    } catch (e) {
      fb.fail(e);
    } finally {
      setToggling((t) => ({ ...t, [skill.id]: false }));
    }
  }

  async function open(skill: Skill) {
    setSelected(skill);
    setContent("");
    setDialogError("");
    setDialog(true);
    setDialogLoading(true);
    try {
      const s = await api.getSkill(skill.id);
      setContent(s.content || "");
      setSelected({ ...skill, ...s });
    } catch (e) {
      setDialogError(e instanceof Error ? e.message : String(e));
    } finally {
      setDialogLoading(false);
    }
  }

  function closeDialog() {
    setDialog(false);
    setSelected(null);
    setContent("");
    setDialogError("");
  }

  function description(skill: Skill): string {
    const d = skill.description?.trim();
    if (d) return d;
    return "No description in skill frontmatter.";
  }

  function toggleSelect(hit: CatalogSkill, next: boolean) {
    const k = catalogKey(hit);
    setSelectedKeys((m) => ({ ...m, [k]: next }));
  }

  function selectAllVisible(next: boolean) {
    const updates: Record<string, boolean> = { ...selectedKeys() };
    for (const hit of filteredCatalog()) {
      updates[catalogKey(hit)] = next;
    }
    setSelectedKeys(updates);
  }

  async function installFromCatalog(hit: CatalogSkill) {
    const key = catalogKey(hit);
    setInstalling((m) => ({ ...m, [key]: true }));
    setSearchError("");
    fb.clearSuccess();
    try {
      await api.installSkill({
        source: hit.source,
        skill: hit.skillId || hit.name,
        name: hit.skillId || hit.name,
        path: hit.path,
      });
      fb.flash(`Installed ${hit.skillId || hit.name}`);
      setCatalog((list) => list.map((s) => (s.id === hit.id ? { ...s, installed: true } : s)));
      await load();
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : String(err));
    } finally {
      setInstalling((m) => ({ ...m, [key]: false }));
    }
  }

  async function uninstallFromCatalog(hit: CatalogSkill) {
    const skillId = hit.skillId || hit.name;
    if (!skillId) return;
    if (!confirm(`Uninstall "${skillId}" from agents home? This removes ~/.agents/skills/${skillId} and its registry entry.`)) {
      return;
    }
    const key = catalogKey(hit);
    setUninstalling((m) => ({ ...m, [key]: true }));
    setSearchError("");
    fb.clearSuccess();
    try {
      await api.uninstallSkill(skillId);
      fb.flash(`Uninstalled ${skillId}`);
      setCatalog((list) => list.map((s) => (s.id === hit.id || s.skillId === skillId ? { ...s, installed: false } : s)));
      await load();
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : String(err));
    } finally {
      setUninstalling((m) => ({ ...m, [key]: false }));
    }
  }

  async function installSelected() {
    const items = selectedCatalogItems();
    if (!items.length) {
      setSearchError("Select at least one skill");
      return;
    }
    if (items.length > 50) {
      setSearchError("Batch install is limited to 50 skills at a time — deselect some and retry");
      return;
    }
    setBatchInstalling(true);
    setSearchError("");
    fb.clearSuccess();
    try {
      const res = await api.installSkillsBatch(
        items.map((s) => ({
          source: s.source,
          skill: s.skillId || s.name,
          name: s.skillId || s.name,
          path: s.path,
        })),
      );
      const ok = res.installed?.length ?? 0;
      const fail = res.failed?.length ?? 0;
      fb.flash(`Installed ${ok}${fail ? `, ${fail} failed` : ""}`);
      if (fail && res.failed?.length) {
        setSearchError(res.failed.map((f) => `${f.skill}: ${f.error}`).join(" · "));
      }
      const installedIds = new Set((res.installed ?? []).map((i) => i.skill.id));
      setCatalog((list) => list.map((s) => (installedIds.has(s.skillId) || installedIds.has(s.name) ? { ...s, installed: true } : s)));
      setSelectedKeys({});
      await load();
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : String(err));
    } finally {
      setBatchInstalling(false);
    }
  }

  function setTabParam(next: Tab) {
    setSearchParams({ tab: next === "installed" ? undefined : next });
  }

  function setRegistryParam(value: string) {
    const next = value.trim() || REGISTRY_ALL;
    setSearchParams({
      registry: next === REGISTRY_ALL ? undefined : next,
    });
  }

  function setDiscoverQueryParam(value: string) {
    const q = value.trim();
    setSearchParams({ q: q ? q : undefined });
  }

  function onRegistryChange(value: string) {
    const next = value.trim() || REGISTRY_ALL;
    if (next === registryFilter()) return;
    // Force catalog reload when registry changes on Discover (even if effect key races).
    lastCatalogKey = "";
    setRegistryParam(next);
  }

  function openDiscover() {
    setSearchError("");
    setTabParam("discover");
  }

  function openInstalled() {
    setTabParam("installed");
  }

  onMount(async () => {
    await Promise.all([load(), loadRegistries()]);
  });

  // Discover: load catalog when tab/registry are ready (incl. deep-link URL params).
  createEffect(() => {
    if (tab() !== "discover") return;
    if (registriesLoading()) return;
    const reg = registryFilter();
    // Track listable set so first registries load triggers fetch.
    const n = listableRegistries().length;
    const key = `${reg}::${n}`;
    if (key === lastCatalogKey) return;
    lastCatalogKey = key;
    if (n === 0) {
      setCatalog([]);
      return;
    }
    void loadCatalog({ registry: reg });
  });

  return (
    <div>
      <PageHeader
        title="Skills"
        subtitle={
          loading() ? "Loading skills..." : `${skills().length} installed · filter by registry · multi-select install from Discover.`
        }
      />

      <div class="surface overflow-hidden fade-in-up is-visible mb-4">
        <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
          <UiSegmented
            value={tab()}
            options={[
              { value: "installed" as const, label: "Installed" },
              { value: "discover" as const, label: "Discover" },
            ]}
            aria-label="Skills views"
            onChange={(v) => {
              if (v === "discover") void openDiscover();
              else openInstalled();
            }}
          />
          <Show when={tab() === "installed"}>
            <RegistryFilterSelect
              id="skills-installed-registry"
              value={registryFilter()}
              options={registryOptions()}
              loading={registriesLoading()}
              onChange={(v) => void onRegistryChange(v)}
            />
            <SearchInput
              value={localQuery()}
              placeholder="Filter installed skills…"
              aria-label="Search installed skills"
              onInput={setLocalQuery}
            />
          </Show>
        </div>

        <PageFeedback error={fb.error() || searchError()} success={fb.success()} />

        <Show when={tab() === "installed"}>
          <Show when={loading()}>
            <ListSkeleton aria-label="Loading skills" />
          </Show>

          <Show when={!loading() && filteredInstalled().length === 0}>
            <EmptyState
              title={skills().length === 0 ? "No skills found" : "No matches"}
              description={
                skills().length === 0
                  ? "Use Discover to browse registries and install skills."
                  : registryFilter() !== REGISTRY_ALL
                    ? "No installed skills for this registry. Try All registries or Discover."
                    : "Try a different name filter."
              }
            />
          </Show>

          <Show when={!loading() && filteredInstalled().length > 0}>
            <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-2 text-[12.5px] text-fg-muted">
              <span>
                {filteredInstalled().length} shown
                {registryFilter() !== REGISTRY_ALL || localQuery() ? ` · ${skills().length} total` : ""}
              </span>
            </div>
            <ul class="m-0 list-none divide-y divide-border p-0">
              <For each={filteredInstalled()}>
                {(skill) => (
                  <ResourceRow enabled={skill.enabled}>
                    <button type="button" class="min-w-0 flex-1 text-left" onClick={() => void open(skill)}>
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="text-[14px] font-semibold tracking-tight text-fg">{skill.name}</span>
                        <Show when={skill.name !== skill.id}>
                          <span class="font-mono text-[11.5px] text-fg-muted">{skill.id}</span>
                        </Show>
                      </div>
                      <p class="m-0 mt-1 line-clamp-2 text-[13px] leading-normal text-fg-secondary">{description(skill)}</p>
                      <div class="mt-1.5 flex flex-wrap items-center gap-2 font-mono text-[11.5px] text-fg-muted">
                        <span>{skill.source}</span>
                        <Show when={skill.registrySource}>
                          <span>·</span>
                          <span>{skill.registrySource}</span>
                        </Show>
                      </div>
                    </button>
                    <div class="flex shrink-0 flex-wrap items-center gap-2 self-center" onClick={(e) => e.stopPropagation()}>
                      <StatusPill kind={sourceKind(skill.source)}>{sourceLabel(skill.source)}</StatusPill>
                      <Show when={skill.overridden}>
                        <UiButton size="sm" variant="danger" onClick={() => void reset(skill.id)}>
                          <PhArrowCounterClockwise size={14} weight="bold" />
                          Reset
                        </UiButton>
                      </Show>
                      <EnableSwitch
                        checked={skill.enabled}
                        disabled={toggling()[skill.id]}
                        aria-label={`Enable skill ${skill.name}`}
                        onChange={(next) => void toggleEnabled(skill, next)}
                      />
                    </div>
                  </ResourceRow>
                )}
              </For>
            </ul>
          </Show>
        </Show>

        <Show when={tab() === "discover"}>
          <div class="space-y-3 border-b border-border px-4 py-3">
            <div class="flex flex-wrap items-end gap-2">
              <RegistryFilterSelect
                id="skills-discover-registry"
                value={registryFilter()}
                options={registryOptions()}
                loading={registriesLoading()}
                onChange={(v) => void onRegistryChange(v)}
              />
              <SearchInput
                value={discoverQuery()}
                placeholder="Filter loaded skills by name…"
                aria-label="Filter skills"
                class="max-w-none"
                onInput={setDiscoverQueryParam}
              />
              <UiButton
                variant="secondary"
                loading={searching()}
                disabled={searching()}
                onClick={() => {
                  const reg = registryFilter();
                  lastCatalogKey = `${reg}::${listableRegistries().length}`;
                  void loadCatalog({ registry: reg, refresh: true });
                }}
              >
                Reload catalog
              </UiButton>
              <UiButton
                variant="primary"
                loading={batchInstalling()}
                disabled={batchInstalling() || selectedCount() === 0}
                onClick={() => void installSelected()}
              >
                <PhDownloadSimple size={16} weight="bold" />
                Install selected ({selectedCount()})
              </UiButton>
            </div>
            <p class="m-0 text-[12px] text-fg-muted">
              Lists every <code class="font-mono text-[11px]">SKILL.md</code> in configured GitHub registries. Choose{" "}
              <strong>All registries</strong> to merge packages, or pick one source. Multi-select then <strong>Install selected</strong> (up
              to 50). Uses GitHub API (<code class="font-mono text-[11px]">gh auth token</code> /{" "}
              <code class="font-mono text-[11px]">GITHUB_TOKEN</code>). Install runs{" "}
              <code class="font-mono text-[11px]">npx skills add</code> into <code class="font-mono text-[11px]">~/.agents</code>.
            </p>
            <Show when={!registriesLoading() && listableRegistries().length === 0}>
              <AppAlert kind="error" class="!mb-0">
                No listable GitHub registries found. On the Registry page, enable skills with a GitHub{" "}
                <code class="font-mono text-[11px]">owner/repo</code> source (not a placeholder like{" "}
                <code class="font-mono text-[11px]">org/repo</code>), or Reset registry to defaults. Then reload Discover.
              </AppAlert>
            </Show>
          </div>

          <Show when={searching()}>
            <ListSkeleton aria-label="Loading catalog" />
          </Show>

          <Show when={!searching() && filteredCatalog().length === 0 && !searchError()}>
            <EmptyState title="No skills to show" description="Pick a registry (or All), Reload catalog, or clear the name filter." />
          </Show>

          <Show when={!searching() && filteredCatalog().length > 0}>
            <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-2 text-[12.5px] text-fg-muted">
              <label class="flex items-center gap-2">
                <input
                  type="checkbox"
                  class="h-4 w-4 accent-[var(--color-accent)]"
                  checked={filteredCatalog().length > 0 && filteredCatalog().every((s) => selectedKeys()[catalogKey(s)])}
                  onChange={(e) => selectAllVisible(e.currentTarget.checked)}
                />
                Select all visible ({filteredCatalog().length})
              </label>
              <span class="flex-1" />
              <span>
                {catalog().length} in catalog
                {discoverQuery() ? ` · ${filteredCatalog().length} filtered` : ""}
                {selectedCount() > 0 ? ` · ${selectedCount()} selected` : ""}
              </span>
            </div>
            <ul class="m-0 max-h-[min(70vh,720px)] list-none divide-y divide-border overflow-y-auto p-0">
              <For each={filteredCatalog()}>
                {(hit) => {
                  const key = () => catalogKey(hit);
                  return (
                    <ResourceRow>
                      <input
                        type="checkbox"
                        class="mt-1 h-4 w-4 shrink-0 accent-[var(--color-accent)]"
                        checked={!!selectedKeys()[key()]}
                        aria-label={`Select ${hit.skillId}`}
                        onChange={(e) => toggleSelect(hit, e.currentTarget.checked)}
                      />
                      <div class="min-w-0 flex-1">
                        <div class="flex flex-wrap items-center gap-2">
                          <span class="text-[14px] font-semibold tracking-tight text-fg">{hit.name || hit.skillId}</span>
                          <span class="font-mono text-[11.5px] text-fg-muted">{hit.source}</span>
                        </div>
                        <div class="mt-0.5 flex flex-wrap items-center gap-2 text-[12px] text-fg-muted">
                          <span class="font-mono">{hit.skillId}</span>
                          <Show when={hit.path}>
                            <span>·</span>
                            <span class="truncate font-mono text-[11px]">{hit.path}</span>
                          </Show>
                          <Show when={hit.installs}>
                            <span>·</span>
                            <span>{formatInstalls(hit.installs)} installs</span>
                          </Show>
                          <Show when={hit.url}>
                            <span>·</span>
                            <a class="text-accent hover:underline" href={hit.url} target="_blank" rel="noopener noreferrer">
                              skills.sh
                            </a>
                          </Show>
                        </div>
                      </div>
                      <Show when={hit.installed}>
                        <StatusPill kind="ok">Installed</StatusPill>
                      </Show>
                      <div class="flex shrink-0 flex-wrap items-center gap-1.5">
                        <UiButton
                          size="sm"
                          variant={hit.installed ? "secondary" : "primary"}
                          loading={installing()[key()]}
                          disabled={installing()[key()] || uninstalling()[key()] || batchInstalling()}
                          onClick={() => void installFromCatalog(hit)}
                        >
                          <PhDownloadSimple size={14} weight="bold" />
                          {hit.installed ? "Reinstall" : "Install"}
                        </UiButton>
                        <Show when={hit.installed}>
                          <UiButton
                            size="sm"
                            variant="danger"
                            loading={uninstalling()[key()]}
                            disabled={installing()[key()] || uninstalling()[key()] || batchInstalling()}
                            onClick={() => void uninstallFromCatalog(hit)}
                          >
                            <PhTrash size={14} weight="bold" />
                            Uninstall
                          </UiButton>
                        </Show>
                      </div>
                    </ResourceRow>
                  );
                }}
              </For>
            </ul>
          </Show>
        </Show>
      </div>

      <UiDialog
        open={dialog()}
        title={selected()?.name || "Skill"}
        subtitle={selected()?.description || selected()?.registrySource || selected()?.source}
        onClose={closeDialog}
      >
        <Show when={dialogError()}>
          <AppAlert kind="error">{dialogError()}</AppAlert>
        </Show>
        <Show when={!dialogError() && dialogLoading()}>
          <div aria-busy="true">
            <div class="skeleton h-[420px]" />
          </div>
        </Show>
        <Show when={!dialogError() && !dialogLoading()}>
          <CodeEditor value={content()} lang="markdown" readonly />
        </Show>
      </UiDialog>
    </div>
  );
}
