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
    <h2 class="page-title">Registry Skills</h2>
    <div class="toolbar">
      <button class="btn primary" :disabled="saving || !isValid" @click="save">{{ saving ? "Saving..." : "Save" }}</button>
    </div>
    <p v-if="loading" class="empty">Loading...</p>
    <p v-else-if="error" class="empty" style="color: var(--danger)">{{ error }}</p>
    <template v-else>
      <p v-if="success" class="empty" style="color: var(--accent)">{{ success }}</p>
      <p class="meta">These skills are installed via <code>npx skills add</code> when running sync.</p>
      <textarea v-model="raw" class="editor json-editor" />
    </template>
  </div>
</template>
