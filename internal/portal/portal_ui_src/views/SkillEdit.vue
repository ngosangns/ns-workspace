<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, type Skill } from "../api";

const route = useRoute();
const router = useRouter();
const id = ref<string>((route.query.id as string) || "");
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
  success.value = "";
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
  try {
    await api.resetSkill(id.value);
    await load();
  } catch (e: any) {
    error.value = e.message || String(e);
  }
}

watch(
  () => route.query.id,
  (newId) => {
    id.value = (newId as string) || "";
    load();
  },
);

onMounted(load);
</script>

<template>
  <div>
    <div class="row q-gutter-sm q-mb-md items-center">
      <q-btn flat icon="sym_o_arrow_back" label="Back" @click="router.push('/skills')" />
      <q-space />
      <q-btn v-if="skill?.overridden" flat color="negative" icon="sym_o_restore" label="Reset to default" @click="reset" />
      <q-btn color="primary" icon="sym_o_save" :loading="saving" label="Save" @click="save" />
    </div>

    <h2 class="text-h5 q-mb-md">Edit Skill: {{ id }}</h2>

    <q-banner v-if="error" class="bg-negative text-white q-mb-md" rounded>{{ error }}</q-banner>
    <q-banner v-if="success" class="bg-positive text-white q-mb-md" rounded>{{ success }}</q-banner>

    <div v-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <q-input
      v-else
      v-model="content"
      type="textarea"
      filled
      bg-color="grey-10"
      input-class="text-mono"
      :input-style="{ minHeight: '400px' }"
      label="Skill content"
    />
  </div>
</template>
