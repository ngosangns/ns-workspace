import { createSignal, For, Show, createEffect, onMount } from "solid-js";
import { PhCircleNotch } from "./Icons";
import { api, type Adapter, type SyncJob } from "../api";
import AppAlert from "./AppAlert";
import UiButton from "./UiButton";
import UiSelect from "./UiSelect";

export default function SyncPanel(props: { onDone?: () => void }) {
  const [logs, setLogs] = createSignal<string[]>([]);
  const [running, setRunning] = createSignal(false);
  const [error, setError] = createSignal("");
  const [currentJob, setCurrentJob] = createSignal<SyncJob | null>(null);
  let logEnd: HTMLDivElement | undefined;

  const [adapters, setAdapters] = createSignal<Adapter[]>([]);
  const [adaptersLoading, setAdaptersLoading] = createSignal(false);
  const [selectedProvider, setSelectedProvider] = createSignal("");

  const providerOptions = () => [{ label: "All providers", value: "" }, ...adapters().map((a) => ({ label: a.name, value: a.id }))];

  const selectedProviderLabel = () => {
    if (!selectedProvider()) return "all providers";
    const found = adapters().find((a) => a.id === selectedProvider());
    return found ? found.name : selectedProvider();
  };

  async function loadAdapters() {
    setAdaptersLoading(true);
    try {
      setAdapters(await api.getAdapters());
    } catch {
      setAdapters([]);
    } finally {
      setAdaptersLoading(false);
    }
  }

  createEffect(() => {
    logs().length;
    queueMicrotask(() => logEnd?.scrollIntoView({ block: "end", behavior: "smooth" }));
  });

  async function run(command: string) {
    if (running()) return;
    setRunning(true);
    setError("");
    setLogs([]);
    setCurrentJob(null);
    try {
      const tools = selectedProvider() || undefined;
      const job = await api.startSync(command, tools);
      setCurrentJob(job);
      stream(job.id);
    } catch (e) {
      setRunning(false);
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function parseLogLine(raw: string): string {
    try {
      const parsed = JSON.parse(raw);
      if (typeof parsed === "string") return parsed;
    } catch {
      // keep raw
    }
    return raw;
  }

  function stream(jobId: string) {
    const es = api.streamSync(jobId);
    es.onmessage = (event) => {
      if (event.data) setLogs((prev) => [...prev, parseLogLine(event.data)]);
    };
    es.addEventListener("end", () => {
      es.close();
      setRunning(false);
      props.onDone?.();
    });
    es.onerror = () => {
      if (logs().length === 0 && running()) {
        setError("Sync stream failed before any output arrived.");
      }
      es.close();
      setRunning(false);
    };
  }

  onMount(loadAdapters);

  return (
    <div class="surface p-[18px]">
      <div class="mb-3.5">
        <h2 class="m-0 mb-1 text-[0.9375rem] font-semibold tracking-tight text-fg">Sync</h2>
        <p class="m-0 text-[13px] text-fg-muted">Run agentsync commands against local providers.</p>
      </div>

      <div class="mb-3.5 flex flex-wrap items-center gap-x-3.5 gap-y-2.5">
        <div class="flex min-w-[180px] flex-wrap items-center gap-1.5">
          <label class="mr-0.5 text-[11px] font-semibold tracking-wide text-fg-muted" for="sync-provider">
            Provider
          </label>
          <UiSelect
            id="sync-provider"
            value={selectedProvider()}
            options={providerOptions()}
            disabled={running() || adaptersLoading()}
            onChange={setSelectedProvider}
          />
        </div>

        <div class="hidden h-[22px] w-px shrink-0 bg-border sm:block" aria-hidden="true" />

        <div class="flex flex-wrap items-center gap-1.5">
          <span class="mr-0.5 text-[11px] font-semibold tracking-wide text-fg-muted">Inspect</span>
          <UiButton disabled={running()} onClick={() => run("status")}>
            Status
          </UiButton>
          <UiButton disabled={running()} onClick={() => run("doctor")}>
            Doctor
          </UiButton>
        </div>

        <div class="hidden h-[22px] w-px shrink-0 bg-border sm:block" aria-hidden="true" />

        <div class="flex flex-wrap items-center gap-1.5">
          <span class="mr-0.5 text-[11px] font-semibold tracking-wide text-fg-muted">Modify</span>
          <UiButton disabled={running()} onClick={() => run("init")}>
            Init
          </UiButton>
          <UiButton variant="primary" disabled={running()} onClick={() => run("update")}>
            Update
          </UiButton>
        </div>

        <div class="hidden h-[22px] w-px shrink-0 bg-border sm:block" aria-hidden="true" />

        <div class="flex flex-wrap items-center gap-1.5">
          <span class="mr-0.5 text-[11px] font-semibold tracking-wide text-fg-muted">Registry</span>
          <UiButton disabled={running()} onClick={() => run("registry")}>
            Install
          </UiButton>
        </div>
      </div>

      <Show when={error()}>
        <AppAlert kind="error">{error()}</AppAlert>
      </Show>

      <Show when={logs().length > 0 || running()}>
        <div class="overflow-hidden rounded-md border border-border bg-app" role="log" aria-live="polite">
          <div class="flex items-center gap-2.5 border-b border-border bg-elevated px-3.5 py-2.5">
            <span
              class={`h-1.5 w-1.5 shrink-0 rounded-full ${running() ? "bg-accent shadow-[0_0_0_3px_var(--color-accent-soft)]" : "bg-fg-muted"}`}
              aria-hidden="true"
            />
            <span class="inline-flex min-w-0 flex-1 items-center overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-fg-secondary">
              <Show when={running()}>
                <PhCircleNotch size={14} class="mr-2 shrink-0 animate-spin text-accent" />
              </Show>
              {currentJob()?.command || "sync"} · {selectedProviderLabel()} · {running() ? "running" : "done"}
            </span>
            <span class="shrink-0 font-mono text-[11px] text-fg-muted">{logs().length} lines</span>
          </div>
          <div class="h-[300px] overflow-auto">
            <div class="px-3.5 py-3">
              <For each={logs()}>
                {(line) => <div class="whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-fg-secondary">{line}</div>}
              </For>
              <Show when={running() && logs().length === 0}>
                <div class="font-mono text-xs text-fg-muted">Starting...</div>
              </Show>
              <div ref={logEnd} />
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}
