<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { api, type RegistrySkills } from "../api";

const registry = ref<RegistrySkills | null>(null);
const raw = ref("");
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const success = ref("");

const isValid = computed(() => {
  try {
    JSON.parse(raw.value);
    return true;
  } catch {
    return false;
  }
});

async function load() {
  loading.value = true;
  error.value = "";
  success.value = "";
  try {
    registry.value = await api.getRegistry();
    raw.value = JSON.stringify(registry.value, null, 2);
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
    const parsed = JSON.parse(raw.value);
    registry.value = await api.updateRegistry(parsed);
    raw.value = JSON.stringify(registry.value, null, 2);
    success.value = "Saved successfully";
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <header class="editor-header fade-in-up">
      <div>
        <h1 class="editor-title">Registry Skills</h1>
        <p class="editor-subtitle">Skills installed via <code>npx skills add</code> during sync.</p>
      </div>
    </header>

    <div class="editor-surface fade-in-up">
      <div class="editor-toolbar">
        <q-btn color="primary" icon="sym_o_save" :disable="!isValid" :loading="saving" label="Save" @click="save" />
        <q-chip v-if="isValid" icon="sym_o_check" color="positive" text-color="white" class="editor-chip">Valid JSON</q-chip>
        <q-chip v-else icon="sym_o_error" color="negative" text-color="white" class="editor-chip">Invalid JSON</q-chip>
        <q-space />
        <span class="editor-hint">Registry entries are merged on the next sync.</span>
      </div>

      <q-banner v-if="error" class="bg-negative text-white q-mx-md q-mt-md rounded-borders" rounded>{{ error }}</q-banner>
      <q-banner v-if="success" class="bg-positive text-white q-mx-md q-mt-md rounded-borders" rounded>{{ success }}</q-banner>

      <div v-if="loading" class="flex flex-center q-pa-xl">
        <q-spinner color="primary" size="3em" />
      </div>
      <q-input
        v-else
        v-model="raw"
        type="textarea"
        filled
        bg-color="grey-10"
        input-class="text-mono editor-input"
        :input-style="{ minHeight: '500px', fontFamily: 'var(--font-mono)' }"
        label="Registry skills JSON"
        hide-bottom-space
      />
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

.editor-subtitle code {
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
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

.editor-input :deep(textarea) {
  padding: 18px;
  font-size: 13px;
  line-height: 1.6;
}
</style>
