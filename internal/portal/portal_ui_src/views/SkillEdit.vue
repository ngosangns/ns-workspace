<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { api, type Skill } from "../api";
import { useRouter } from "../router";

const { params, navigate } = useRouter();
const id = ref(params.value.id || "");
const skill = ref<Skill | null>(null);
const content = ref("");
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const success = ref("");

async function load() {
  if (!id.value) return;
  loading.value = true;
  error.value = "";
  try {
    skill.value = await api.getSkill(id.value);
    content.value = skill.value.content || "";
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  error.value = "";
  success.value = "";
  try {
    await api.updateSkill(id.value, content.value);
    success.value = "Saved successfully";
    await load();
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    saving.value = false;
  }
}

async function reset() {
  if (!confirm(`Reset skill "${id.value}" to default?`)) return;
  try {
    await api.resetSkill(id.value);
    await load();
  } catch (e: any) {
    error.value = e.message || String(e);
  }
}

watch(
  () => params.value.id,
  (newId) => {
    id.value = newId || "";
    load();
  },
);

onMounted(load);
</script>

<template>
  <div>
    <div class="toolbar">
      <button class="btn" @click="navigate('#skills')">← Back</button>
      <button class="btn primary" :disabled="saving" @click="save">{{ saving ? "Saving..." : "Save" }}</button>
      <button v-if="skill?.overridden" class="btn danger" @click="reset">Reset to default</button>
    </div>
    <h2 class="page-title">Edit Skill: {{ id }}</h2>
    <p v-if="loading" class="empty">Loading...</p>
    <p v-else-if="error" class="empty" style="color: var(--danger)">{{ error }}</p>
    <template v-else>
      <p v-if="success" class="empty" style="color: var(--accent)">{{ success }}</p>
      <textarea v-model="content" class="editor" />
    </template>
  </div>
</template>
