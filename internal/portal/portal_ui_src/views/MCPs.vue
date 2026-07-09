<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { PhFloppyDisk, PhArrowCounterClockwise } from "@phosphor-icons/vue";
import { api, type MCPManifest, type MCPServers } from "../api";
import AppAlert from "../components/AppAlert.vue";
import CodeEditor from "../components/CodeEditor.vue";
import UiButton from "../components/UiButton.vue";
import { useFlashMessage } from "../composables/useFlashMessage";

const manifest = ref<MCPManifest | null>(null);
const preset = ref<MCPServers | null>(null);
const effectiveRaw = ref("");
const tab = ref<"effective" | "preset">("effective");
const loading = ref(true);
const saving = ref(false);
const resetting = ref(false);
const error = ref("");
const { message: success, flash, clear: clearSuccess } = useFlashMessage();

const isValid = computed(() => {
  try {
    JSON.parse(effectiveRaw.value);
    return true;
  } catch {
    return false;
  }
});

const isOverridden = computed(() => manifest.value?.overridden ?? false);

const presetRaw = computed(() => JSON.stringify(preset.value?.mcpServers ?? {}, null, 2));

async function load() {
  loading.value = true;
  error.value = "";
  clearSuccess();
  try {
    const [m, p] = await Promise.all([api.getMCPs(), api.getMCPPreset()]);
    manifest.value = m;
    preset.value = p;
    effectiveRaw.value = JSON.stringify(m.mcpServers ?? {}, null, 2);
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
    const parsed = JSON.parse(effectiveRaw.value);
    manifest.value = await api.updateMCPs({ mcpServers: parsed });
    effectiveRaw.value = JSON.stringify(manifest.value?.mcpServers ?? {}, null, 2);
    flash("Saved");
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
    manifest.value = await api.resetMCPs();
    effectiveRaw.value = JSON.stringify(manifest.value?.mcpServers ?? {}, null, 2);
    flash("Reset to preset");
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    resetting.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <header class="page-header fade-in-up">
      <h1 class="page-title">MCP Servers</h1>
      <p class="page-subtitle">Configure MCP server definitions used during sync.</p>
    </header>

    <div class="surface overflow-hidden fade-in-up">
      <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
        <div class="flex gap-0.5 rounded-md border border-border bg-app-muted p-0.5">
          <button
            type="button"
            class="rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)]"
            :class="tab === 'effective' ? 'bg-surface text-fg shadow-sm' : 'text-fg-secondary hover:text-fg'"
            @click="tab = 'effective'"
          >
            Effective
          </button>
          <button
            type="button"
            class="rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)]"
            :class="tab === 'preset' ? 'bg-surface text-fg shadow-sm' : 'text-fg-secondary hover:text-fg'"
            @click="tab = 'preset'"
          >
            Preset
          </button>
        </div>
        <div class="flex-1" />
        <span v-if="isOverridden" class="status-pill status-pill--warn">Overridden</span>
        <span v-else class="status-pill status-pill--ok">Embedded preset</span>
      </div>

      <div v-if="error || success" class="space-y-2 px-4 pt-3">
        <AppAlert v-if="error" kind="error" class="!mb-0">{{ error }}</AppAlert>
        <AppAlert v-if="success" kind="success" class="!mb-0">{{ success }}</AppAlert>
      </div>

      <div v-if="loading" class="min-h-[200px]" aria-busy="true">
        <div class="skeleton m-4 h-[480px] rounded-[10px]" />
      </div>

      <template v-else>
        <div v-if="tab === 'effective'">
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <UiButton variant="primary" :disabled="!isValid" :loading="saving" @click="save">
              <PhFloppyDisk :size="16" weight="bold" />
              Save
            </UiButton>
            <UiButton variant="warning" :disabled="!isOverridden" :loading="resetting" @click="reset">
              <PhArrowCounterClockwise :size="16" weight="bold" />
              Reset to preset
            </UiButton>
            <span :class="['status-pill', isValid ? 'status-pill--ok' : 'status-pill--err']">
              {{ isValid ? "Valid JSON" : "Invalid JSON" }}
            </span>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">Edits are written to the MCP overlay config.</span>
          </div>
          <CodeEditor v-model="effectiveRaw" lang="json" />
        </div>
        <div v-else>
          <div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <span class="status-pill status-pill--muted">Read-only preset</span>
            <div class="flex-1" />
            <span class="text-[12.5px] text-fg-muted">Override from the Effective tab.</span>
          </div>
          <CodeEditor :model-value="presetRaw" lang="json" readonly />
        </div>
      </template>
    </div>
  </div>
</template>
