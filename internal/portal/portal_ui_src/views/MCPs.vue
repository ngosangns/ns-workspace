<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { api, type MCPManifest, type MCPServers } from "../api";
import CodeEditor from "../components/CodeEditor.vue";

const manifest = ref<MCPManifest | null>(null);
const preset = ref<MCPServers | null>(null);
const effectiveRaw = ref("");
const tab = ref<"effective" | "preset">("effective");
const loading = ref(true);
const saving = ref(false);
const resetting = ref(false);
const error = ref("");
const success = ref("");

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
  success.value = "";
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
  success.value = "";
  try {
    const parsed = JSON.parse(effectiveRaw.value);
    manifest.value = await api.updateMCPs({ mcpServers: parsed });
    effectiveRaw.value = JSON.stringify(manifest.value?.mcpServers ?? {}, null, 2);
    success.value = "Saved successfully";
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    saving.value = false;
  }
}

async function reset() {
  resetting.value = true;
  error.value = "";
  success.value = "";
  try {
    manifest.value = await api.resetMCPs();
    effectiveRaw.value = JSON.stringify(manifest.value?.mcpServers ?? {}, null, 2);
    success.value = "Reset to preset successfully";
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
    <header class="editor-header fade-in-up">
      <div>
        <h1 class="editor-title">MCP Servers</h1>
        <p class="editor-subtitle">Configure MCP server definitions used during sync.</p>
      </div>
    </header>

    <div class="editor-surface fade-in-up">
      <div class="editor-toolbar">
        <q-tabs v-model="tab" dense class="text-grey-5" active-color="primary" indicator-color="primary" align="left" narrow-indicator>
          <q-tab name="effective" label="Effective" />
          <q-tab name="preset" label="Preset" />
        </q-tabs>
        <q-space />
        <q-chip v-if="isOverridden" icon="sym_o_edit" color="warning" text-color="dark" class="editor-chip">Overridden</q-chip>
        <q-chip v-else icon="sym_o_database" color="positive" text-color="white" class="editor-chip">Embedded preset</q-chip>
      </div>

      <q-banner v-if="error" class="bg-negative text-white q-mx-md q-mt-md rounded-borders" rounded>{{ error }}</q-banner>
      <q-banner v-if="success" class="bg-positive text-white q-mx-md q-mt-md rounded-borders" rounded>{{ success }}</q-banner>

      <div v-if="loading" class="flex flex-center q-pa-xl">
        <q-spinner color="primary" size="3em" />
      </div>
      <q-tab-panels v-else v-model="tab" animated class="bg-transparent">
        <q-tab-panel name="effective" class="q-pa-none">
          <div class="editor-toolbar">
            <q-btn color="primary" icon="sym_o_save" :disable="!isValid" :loading="saving" label="Save" @click="save" />
            <q-btn
              color="warning"
              icon="sym_o_restart_alt"
              :disable="!isOverridden"
              :loading="resetting"
              label="Reset to preset"
              outline
              @click="reset"
            />
            <q-chip v-if="isValid" icon="sym_o_check" color="positive" text-color="white" class="editor-chip">Valid JSON</q-chip>
            <q-chip v-else icon="sym_o_error" color="negative" text-color="white" class="editor-chip">Invalid JSON</q-chip>
            <q-space />
            <span class="editor-hint">Edits are written to the MCP overlay config.</span>
          </div>
          <CodeEditor v-model="effectiveRaw" lang="json" />
        </q-tab-panel>
        <q-tab-panel name="preset" class="q-pa-none">
          <div class="editor-toolbar">
            <q-chip icon="sym_o_lock" color="info" text-color="dark" class="editor-chip">Read-only preset</q-chip>
            <q-space />
            <span class="editor-hint">This is the embedded preset. Override it from the Effective tab.</span>
          </div>
          <CodeEditor :model-value="presetRaw" lang="json" readonly />
        </q-tab-panel>
      </q-tab-panels>
    </div>
  </div>
</template>

<style scoped>
.editor-header {
  margin-bottom: 24px;
}

.editor-title {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin: 0 0 6px;
  color: var(--color-text);
}

.editor-subtitle {
  font-size: 15px;
  color: var(--color-text-secondary);
  margin: 0;
}

.editor-surface {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.editor-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-border);
}

.editor-chip {
  font-size: 12px;
  font-weight: 600;
}

.editor-hint {
  font-size: 13px;
  color: var(--color-text-muted);
}
</style>
