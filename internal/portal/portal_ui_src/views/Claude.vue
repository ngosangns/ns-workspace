<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { api, type ClaudeSettings, type ClaudeEnv } from "../api";
import CodeEditor from "../components/CodeEditor.vue";

const manifest = ref<ClaudeSettings | null>(null);
const preset = ref<ClaudeSettings | null>(null);
const tab = ref<"effective" | "preset">("effective");
const loading = ref(true);
const saving = ref(false);
const resetting = ref(false);
const error = ref("");
const success = ref("");

const env = computed<ClaudeEnv>({
  get: () => manifest.value?.env ?? {},
  set: (value) => {
    if (manifest.value) {
      manifest.value.env = value;
    }
  },
});

const isOverridden = computed(() => manifest.value?.overridden ?? false);

const presetRaw = computed(() => JSON.stringify(preset.value ?? {}, null, 2));

async function load() {
  loading.value = true;
  error.value = "";
  success.value = "";
  try {
    const [m, p] = await Promise.all([api.getClaudeSettings(), api.getClaudeSettingsPreset()]);
    manifest.value = m;
    preset.value = p;
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!manifest.value) return;
  saving.value = true;
  error.value = "";
  success.value = "";
  try {
    manifest.value = await api.updateClaudeSettings(manifest.value);
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
    manifest.value = await api.resetClaudeSettings();
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
        <h1 class="editor-title">Claude Code</h1>
        <p class="editor-subtitle">Configure custom models and endpoint for Claude Code.</p>
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
            <q-btn color="primary" icon="sym_o_save" :loading="saving" label="Save" @click="save" />
            <q-btn
              color="warning"
              icon="sym_o_restart_alt"
              :disable="!isOverridden"
              :loading="resetting"
              label="Reset to preset"
              outline
              @click="reset"
            />
            <q-space />
            <span class="editor-hint">Edits are written to the Claude Code settings overlay.</span>
          </div>
          <div class="q-pa-lg">
            <div class="row q-col-gutter-md">
              <div class="col-12 col-md-6">
                <q-input
                  v-model="env.ANTHROPIC_BASE_URL"
                  filled
                  bg-color="grey-10"
                  label="Base URL"
                  hint="Custom Anthropic-compatible API endpoint"
                  clearable
                />
              </div>
              <div class="col-12 col-md-6">
                <q-input
                  v-model="env.ANTHROPIC_AUTH_TOKEN"
                  filled
                  bg-color="grey-10"
                  label="Auth token"
                  type="password"
                  hint="Bearer token sent to the custom endpoint"
                  clearable
                />
              </div>
              <div class="col-12 col-md-6">
                <q-input v-model="env.ANTHROPIC_MODEL" filled bg-color="grey-10" label="Model" hint="Main model identifier" clearable />
              </div>
              <div class="col-12 col-md-6">
                <q-input
                  v-model="env.ANTHROPIC_SMALL_FAST_MODEL"
                  filled
                  bg-color="grey-10"
                  label="Small / fast model"
                  hint="Lightweight model for quick tasks"
                  clearable
                />
              </div>
            </div>
          </div>
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
