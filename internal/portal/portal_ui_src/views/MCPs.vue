<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { PhFloppyDisk, PhArrowCounterClockwise } from "@phosphor-icons/vue";
import { api, type MCPManifest, type MCPServerItem } from "../api";
import AppAlert from "../components/AppAlert.vue";
import CodeEditor from "../components/CodeEditor.vue";
import UiButton from "../components/UiButton.vue";
import { useFlashMessage } from "../composables/useFlashMessage";

const manifest = ref<MCPManifest | null>(null);
/** Single catalog source: all servers + disabled name list. */
const fileRaw = ref("");
const tab = ref<"list" | "edit">("list");
const loading = ref(true);
const saving = ref(false);
const resetting = ref(false);
const error = ref("");
const toggling = ref<Record<string, boolean>>({});
const { message: success, flash, clear: clearSuccess } = useFlashMessage();

const isOverridden = computed(() => manifest.value?.overridden ?? false);

const items = computed<MCPServerItem[]>(() => {
  if (manifest.value?.items?.length) {
    return manifest.value.items;
  }
  const enabled = manifest.value?.mcpServers ?? {};
  const disabled = manifest.value?.disabledServers ?? {};
  const names = new Set([...Object.keys(enabled), ...Object.keys(disabled)]);
  return [...names]
    .sort()
    .map((name) => (name in enabled ? { name, enabled: true, config: enabled[name] } : { name, enabled: false, config: disabled[name] }));
});

const enabledCount = computed(() => items.value.filter((i) => i.enabled).length);
const disabledCount = computed(() => items.value.filter((i) => !i.enabled).length);

const isValidJSON = computed(() => {
  try {
    JSON.parse(fileRaw.value);
    return true;
  } catch {
    return false;
  }
});

function applyManifest(m: MCPManifest) {
  manifest.value = m;
  fileRaw.value =
    m.content ||
    JSON.stringify(
      {
        mcpServers: {
          ...(m.mcpServers ?? {}),
          ...(m.disabledServers ?? {}),
        },
        disabled: Object.keys(m.disabledServers ?? {}).sort(),
      },
      null,
      2,
    );
}

async function load() {
  loading.value = true;
  error.value = "";
  clearSuccess();
  try {
    applyManifest(await api.getMCPs());
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!isValidJSON.value) {
    error.value = "Invalid JSON";
    return;
  }
  saving.value = true;
  error.value = "";
  clearSuccess();
  try {
    applyManifest(await api.updateMCPsContent(fileRaw.value));
    flash("Saved catalog");
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    saving.value = false;
  }
}

async function reset() {
  resetting.value = true;
  error.value = "";
  clearSuccess();
  try {
    applyManifest(await api.resetMCPs());
    flash("Reset to embedded default");
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    resetting.value = false;
  }
}

async function toggleEnabled(item: MCPServerItem, next: boolean) {
  toggling.value = { ...toggling.value, [item.name]: true };
  error.value = "";
  clearSuccess();
  try {
    applyManifest(await api.setMCPEnabled(item.name, next));
    flash(next ? `Enabled ${item.name}` : `Disabled ${item.name}`);
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    toggling.value = { ...toggling.value, [item.name]: false };
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

onMounted(load);
</script>

<template>
  <div>
    <header class="page-header fade-in-up">
      <h1 class="page-title">MCP Servers</h1>
      <p class="page-subtitle">
        {{
          loading
            ? "Loading MCP catalog..."
            : `${items.length} servers · ${enabledCount} enabled · ${disabledCount} disabled. One catalog for edit; disable keeps config (sync only ships enabled).`
        }}
      </p>
    </header>

    <div class="surface overflow-hidden fade-in-up">
      <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
        <div class="flex gap-0.5 rounded-md border border-border bg-app-muted p-0.5">
          <button
            type="button"
            class="rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)]"
            :class="tab === 'list' ? 'bg-surface text-fg shadow-sm' : 'text-fg-secondary hover:text-fg'"
            @click="tab = 'list'"
          >
            List
          </button>
          <button
            type="button"
            class="rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)]"
            :class="tab === 'edit' ? 'bg-surface text-fg shadow-sm' : 'text-fg-secondary hover:text-fg'"
            @click="tab = 'edit'"
          >
            Edit JSON
          </button>
        </div>
        <div class="flex-1" />
        <span v-if="!loading" class="status-pill status-pill--ok">{{ enabledCount }} enabled</span>
        <span v-if="!loading && disabledCount" class="status-pill status-pill--muted">{{ disabledCount }} disabled</span>
        <span v-if="isOverridden" class="status-pill status-pill--warn">Custom</span>
        <span v-else class="status-pill status-pill--ok">Default</span>
        <UiButton variant="warning" size="sm" :disabled="!isOverridden || loading" :loading="resetting" @click="reset">
          <PhArrowCounterClockwise :size="14" weight="bold" />
          Reset
        </UiButton>
      </div>

      <div v-if="error || success" class="space-y-2 px-4 pt-3">
        <AppAlert v-if="error" kind="error" class="!mb-0">{{ error }}</AppAlert>
        <AppAlert v-if="success" kind="success" class="!mb-0">{{ success }}</AppAlert>
      </div>

      <div v-if="loading" class="min-h-[200px]" aria-busy="true">
        <div class="skeleton m-4 h-[480px] rounded-[10px]" />
      </div>

      <template v-else>
        <div v-if="tab === 'list'">
          <div v-if="items.length === 0" class="px-5 py-10 text-center">
            <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No MCP servers</p>
            <p class="m-0 text-[13px] text-fg-muted">Add servers in the Edit JSON tab, then save.</p>
          </div>
          <ul v-else class="m-0 list-none divide-y divide-border p-0">
            <li
              v-for="item in items"
              :key="item.name"
              class="flex flex-wrap items-center gap-3 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated"
              :class="item.enabled ? '' : 'opacity-60'"
            >
              <div class="min-w-0 flex-1">
                <div class="font-mono text-[14px] font-semibold text-fg">{{ item.name }}</div>
                <div class="mt-0.5 truncate font-mono text-[11.5px] text-fg-muted">
                  {{ summarize(item.config) }}
                </div>
              </div>
              <span :class="['status-pill', item.enabled ? 'status-pill--ok' : 'status-pill--muted']">
                {{ item.enabled ? "Enabled" : "Disabled" }}
              </span>
              <label class="flex items-center gap-2">
                <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">
                  {{ item.enabled ? "On" : "Off" }}
                </span>
                <input
                  type="checkbox"
                  class="h-4 w-4 accent-[var(--color-accent)]"
                  :checked="item.enabled"
                  :disabled="toggling[item.name]"
                  :aria-label="`Enable MCP ${item.name}`"
                  @change="toggleEnabled(item, ($event.target as HTMLInputElement).checked)"
                />
              </label>
            </li>
          </ul>
        </div>

        <div v-else>
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <UiButton variant="primary" :loading="saving" :disabled="!isValidJSON" @click="save">
              <PhFloppyDisk :size="16" weight="bold" />
              Save catalog
            </UiButton>
            <span v-if="!isValidJSON" class="text-[12.5px] text-negative">Invalid JSON</span>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">
              Shape:
              <code class="font-mono text-[12px]">mcpServers</code>
              + optional
              <code class="font-mono text-[12px]">disabled[]</code>
            </span>
          </div>
          <CodeEditor v-model="fileRaw" lang="json" />
        </div>
      </template>
    </div>
  </div>
</template>
