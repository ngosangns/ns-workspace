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
    <h2 class="text-h5 q-mb-md">Registry Skills</h2>

    <div class="row q-gutter-sm q-mb-md">
      <q-btn color="primary" icon="sym_o_save" :disable="!isValid" :loading="saving" label="Save" @click="save" />
      <q-chip v-if="isValid" icon="sym_o_check" color="positive" text-color="white">Valid JSON</q-chip>
      <q-chip v-else icon="sym_o_error" color="negative" text-color="white">Invalid JSON</q-chip>
    </div>

    <q-banner v-if="error" class="bg-negative text-white q-mb-md" rounded>{{ error }}</q-banner>
    <q-banner v-if="success" class="bg-positive text-white q-mb-md" rounded>{{ success }}</q-banner>

    <p class="text-caption text-grey-5 q-mb-md">These skills are installed via <code>npx skills add</code> when running sync.</p>

    <div v-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <q-input
      v-else
      v-model="raw"
      type="textarea"
      filled
      bg-color="grey-10"
      input-class="text-mono"
      :input-style="{ minHeight: '400px', fontFamily: 'monospace' }"
      label="Registry skills JSON"
    />
  </div>
</template>
