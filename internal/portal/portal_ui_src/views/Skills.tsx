import { createSignal, For, Show, onMount } from "solid-js";
import { PhArrowCounterClockwise } from "../components/Icons";
import { api, type Skill } from "../api";
import AppAlert from "../components/AppAlert";
import CodeEditor from "../components/CodeEditor";
import UiButton from "../components/UiButton";
import UiDialog from "../components/UiDialog";

export default function Skills() {
  const [skills, setSkills] = createSignal<Skill[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal("");
  const [toggling, setToggling] = createSignal<Record<string, boolean>>({});

  const [dialog, setDialog] = createSignal(false);
  const [selected, setSelected] = createSignal<Skill | null>(null);
  const [content, setContent] = createSignal("");
  const [dialogLoading, setDialogLoading] = createSignal(false);
  const [dialogError, setDialogError] = createSignal("");

  async function load() {
    setLoading(true);
    setError("");
    try {
      setSkills(await api.getSkills());
    } catch (e: any) {
      setError(e.message || String(e));
    } finally {
      setLoading(false);
    }
  }

  async function reset(id: string) {
    try {
      await api.resetSkill(id);
      await load();
    } catch (e: any) {
      setError(e.message || String(e));
    }
  }

  async function toggleEnabled(skill: Skill, next: boolean) {
    setToggling((t) => ({ ...t, [skill.id]: true }));
    setError("");
    try {
      const updated = await api.setSkillEnabled(skill.id, next);
      setSkills((list) =>
        list.map((s) => (s.id === skill.id ? { ...s, enabled: updated.enabled, description: updated.description ?? s.description } : s)),
      );
    } catch (e: any) {
      setError(e.message || String(e));
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
    } catch (e: any) {
      setDialogError(e.message || String(e));
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

  onMount(load);

  return (
    <div>
      <header class="page-header fade-in-up is-visible">
        <h1 class="page-title">Skills</h1>
        <p class="page-subtitle">
          {loading()
            ? "Loading skills..."
            : `${skills().length} skills · ${skills().filter((s) => s.enabled).length} enabled · ${skills().filter((s) => !s.enabled).length} disabled. Disable keeps skills listed; they are skipped during sync.`}
        </p>
      </header>

      <Show when={error()}>
        <AppAlert kind="error">{error()}</AppAlert>
      </Show>

      <Show when={!error() && loading()}>
        <div class="surface overflow-hidden fade-in-up is-visible" aria-busy="true" aria-label="Loading skills">
          <div class="space-y-0 divide-y divide-border p-0">
            <For each={[1, 2, 3, 4, 5, 6]}>
              {() => (
                <div class="px-4 py-3">
                  <div class="skeleton h-14 rounded-md" />
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      <Show when={!error() && !loading() && skills().length === 0}>
        <div class="fade-in-up is-visible rounded-lg border border-dashed border-border-strong bg-surface px-5 py-12 text-center">
          <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No skills found</p>
          <p class="m-0 text-[13px] text-fg-muted">Sync or install skills to see them listed here.</p>
        </div>
      </Show>

      <Show when={!error() && !loading() && skills().length > 0}>
        <div class="surface overflow-hidden fade-in-up is-visible">
          <ul class="m-0 list-none divide-y divide-border p-0">
            <For each={skills()}>
              {(skill) => (
                <li
                  class={`flex flex-wrap items-start gap-x-4 gap-y-2 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated ${skill.enabled ? "" : "opacity-60"}`}
                >
                  <button type="button" class="min-w-0 flex-1 text-left" onClick={() => open(skill)}>
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="text-[14px] font-semibold tracking-tight text-fg">{skill.name}</span>
                      <Show when={skill.name !== skill.id}>
                        <span class="font-mono text-[11.5px] text-fg-muted">{skill.id}</span>
                      </Show>
                    </div>
                    <p class="m-0 mt-1 line-clamp-2 text-[13px] leading-normal text-fg-secondary">{description(skill)}</p>
                    <div class="mt-1.5 font-mono text-[11.5px] text-fg-muted">{skill.source}</div>
                  </button>
                  <div class="flex shrink-0 flex-wrap items-center gap-2 self-center">
                    <span class={`status-pill ${skill.enabled ? "status-pill--ok" : "status-pill--muted"}`}>
                      {skill.enabled ? "Enabled" : "Disabled"}
                    </span>
                    <span class={`status-pill ${skill.overridden ? "status-pill--accent" : "status-pill--muted"}`}>
                      {skill.overridden ? "Custom" : "Default"}
                    </span>
                    <Show when={skill.overridden}>
                      <UiButton size="sm" variant="danger" onClick={() => reset(skill.id)}>
                        <PhArrowCounterClockwise size={14} weight="bold" />
                        Reset
                      </UiButton>
                    </Show>
                    <label class="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                      <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">{skill.enabled ? "On" : "Off"}</span>
                      <input
                        type="checkbox"
                        class="h-4 w-4 accent-[var(--color-accent)]"
                        checked={skill.enabled}
                        disabled={toggling()[skill.id]}
                        aria-label={`Enable skill ${skill.name}`}
                        onChange={(e) => toggleEnabled(skill, e.currentTarget.checked)}
                      />
                    </label>
                  </div>
                </li>
              )}
            </For>
          </ul>
        </div>
      </Show>

      <UiDialog
        open={dialog()}
        title={selected()?.name || "Skill"}
        subtitle={selected()?.description || selected()?.source}
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
