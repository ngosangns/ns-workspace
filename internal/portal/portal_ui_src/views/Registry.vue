<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { PhFloppyDisk } from "@phosphor-icons/vue";
import { api, type RegistrySkills } from "../api";
import AppAlert from "../components/AppAlert.vue";
import CodeEditor from "../components/CodeEditor.vue";
import UiButton from "../components/UiButton.vue";
import { useFlashMessage } from "../composables/useFlashMessage";

const registry = ref<RegistrySkills | null>(null);
const raw = ref("");
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const { message: success, flash, clear: clearSuccess } = useFlashMessage();

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
  clearSuccess();
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
  clearSuccess();
  try {
    const parsed = JSON.parse(raw.value);
    registry.value = await api.updateRegistry(parsed);
    raw.value = JSON.stringify(registry.value, null, 2);
    flash("Saved");
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
    <header class="page-header fade-in-up">
      <h1 class="page-title">Registry Skills</h1>
      <p class="page-subtitle">
        Skills installed via
        <code class="rounded border border-border bg-app-muted px-1.5 py-px font-mono text-xs text-fg-secondary">npx skills add</code>
        during sync.
      </p>
    </header>

    <div class="surface overflow-hidden fade-in-up">
      <div class="flex flex-wrap items-center gap-3 border-b border-border bg-elevated px-4 py-3">
        <UiButton variant="primary" :disabled="!isValid" :loading="saving" @click="save">
          <PhFloppyDisk :size="16" weight="bold" />
          Save
        </UiButton>
        <span :class="['status-pill', isValid ? 'status-pill--ok' : 'status-pill--err']">
          {{ isValid ? "Valid JSON" : "Invalid JSON" }}
        </span>
        <div class="flex-1" />
        <span class="text-[12.5px] text-fg-muted">Registry entries are merged on the next sync.</span>
      </div>

      <div v-if="error || success" class="space-y-2 px-4 pt-3">
        <AppAlert v-if="error" kind="error" class="!mb-0">{{ error }}</AppAlert>
        <AppAlert v-if="success" kind="success" class="!mb-0">{{ success }}</AppAlert>
      </div>

      <div v-if="loading" class="min-h-[200px]" aria-busy="true">
        <div class="skeleton m-4 h-[480px] rounded-[10px]" />
      </div>
      <CodeEditor v-else v-model="raw" lang="json" />
    </div>
  </div>
</template>
