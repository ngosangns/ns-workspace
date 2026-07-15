<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { PhFloppyDisk } from "@phosphor-icons/vue";
import { api, type RegistrySkills, type RegistrySkillItem } from "../api";
import AppAlert from "../components/AppAlert.vue";
import CodeEditor from "../components/CodeEditor.vue";
import UiButton from "../components/UiButton.vue";
import { useFlashMessage } from "../composables/useFlashMessage";

const registry = ref<RegistrySkills | null>(null);
const raw = ref("");
const tab = ref<"list" | "file">("list");
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const toggling = ref<Record<string, boolean>>({});
const { message: success, flash, clear: clearSuccess } = useFlashMessage();

const isValid = computed(() => {
  try {
    JSON.parse(raw.value);
    return true;
  } catch {
    return false;
  }
});

const items = computed<RegistrySkillItem[]>(() => {
  if (registry.value?.items?.length) {
    return registry.value.items;
  }
  const enabled = (registry.value?.skills ?? []).map((s) => ({ ...s, enabled: true }));
  const disabled = (registry.value?.disabledSkills ?? []).map((s) => ({ ...s, enabled: false }));
  return [...enabled, ...disabled].sort((a, b) => a.name.localeCompare(b.name));
});

const enabledCount = computed(() => items.value.filter((i) => i.enabled).length);
const disabledCount = computed(() => items.value.filter((i) => !i.enabled).length);

function applyRegistry(reg: RegistrySkills) {
  registry.value = reg;
  raw.value = JSON.stringify({ skills: reg.skills ?? [] }, null, 2);
}

async function load() {
  loading.value = true;
  error.value = "";
  clearSuccess();
  try {
    applyRegistry(await api.getRegistry());
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!isValid.value) {
    error.value = "Invalid JSON";
    return;
  }
  saving.value = true;
  error.value = "";
  clearSuccess();
  try {
    const parsed = JSON.parse(raw.value);
    applyRegistry(await api.updateRegistry(parsed));
    flash("Saved — removed skills move to skills.disabled.json");
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    saving.value = false;
  }
}

async function toggleEnabled(item: RegistrySkillItem, next: boolean) {
  toggling.value = { ...toggling.value, [item.name]: true };
  error.value = "";
  clearSuccess();
  try {
    applyRegistry(await api.setRegistrySkillEnabled(item.name, next));
    flash(next ? `Enabled ${item.name}` : `Disabled ${item.name} — moved to skills.disabled.json`);
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    toggling.value = { ...toggling.value, [item.name]: false };
  }
}

onMounted(load);
</script>

<template>
  <div>
    <header class="page-header fade-in-up">
      <h1 class="page-title">Registry Skills</h1>
      <p class="page-subtitle">
        Skills installed via
        <code class="rounded border border-border bg-app-muted px-1.5 py-px font-mono text-xs text-fg-secondary">npx skills add</code>
        during sync. Disable moves an entry into
        <code class="rounded border border-border bg-app-muted px-1 font-mono text-xs">skills.disabled.json</code>
        (not delete).
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
            :class="tab === 'file' ? 'bg-surface text-fg shadow-sm' : 'text-fg-secondary hover:text-fg'"
            @click="tab = 'file'"
          >
            File (enabled)
          </button>
        </div>
        <div class="flex-1" />
        <span v-if="!loading" class="status-pill status-pill--ok">{{ enabledCount }} enabled</span>
        <span v-if="!loading && disabledCount" class="status-pill status-pill--muted">{{ disabledCount }} disabled</span>
      </div>

      <div v-if="error || success" class="space-y-2 px-4 pt-3">
        <AppAlert v-if="error" kind="error" class="!mb-0">{{ error }}</AppAlert>
        <AppAlert v-if="success" kind="success" class="!mb-0">{{ success }}</AppAlert>
      </div>

      <div v-if="loading" class="min-h-[200px]" aria-busy="true">
        <div class="skeleton m-4 h-[480px] rounded-[10px]" />
      </div>

      <template v-else>
        <div v-if="tab === 'list'" class="p-4">
          <div v-if="items.length === 0" class="rounded-lg border border-dashed border-border-strong px-5 py-10 text-center">
            <p class="m-0 text-[14px] text-fg-muted">No registry skills defined.</p>
          </div>
          <ul v-else class="m-0 list-none space-y-2 p-0">
            <li
              v-for="item in items"
              :key="item.name"
              class="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-elevated px-4 py-3"
              :class="item.enabled ? '' : 'opacity-75'"
            >
              <div class="min-w-0 flex-1">
                <div class="font-mono text-[14px] font-semibold text-fg">{{ item.name }}</div>
                <div class="mt-0.5 truncate font-mono text-[11.5px] text-fg-muted">
                  {{ item.source || item.installer || item.skill }}
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
                  :aria-label="`Enable registry skill ${item.name}`"
                  @change="toggleEnabled(item, ($event.target as HTMLInputElement).checked)"
                />
              </label>
            </li>
          </ul>
        </div>

        <div v-else>
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <UiButton variant="primary" :disabled="!isValid" :loading="saving" @click="save">
              <PhFloppyDisk :size="16" weight="bold" />
              Save
            </UiButton>
            <span :class="['status-pill', isValid ? 'status-pill--ok' : 'status-pill--err']">
              {{ isValid ? "Valid JSON" : "Invalid JSON" }}
            </span>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">
              Disabled skills live in
              <code class="font-mono text-[12px]">skills.disabled.json</code>.
            </span>
          </div>
          <CodeEditor v-model="raw" lang="json" />
        </div>
      </template>
    </div>
  </div>
</template>
