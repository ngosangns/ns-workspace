<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { api, type Skill } from "../api";

const skills = ref<Skill[]>([]);
const loading = ref(true);
const error = ref("");
const router = useRouter();

async function load() {
  loading.value = true;
  error.value = "";
  try {
    skills.value = await api.getSkills();
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function reset(id: string) {
  try {
    await api.resetSkill(id);
    await load();
  } catch (e: any) {
    error.value = e.message || String(e);
  }
}

function edit(id: string) {
  router.push(`/skills/edit?id=${encodeURIComponent(id)}`);
}

onMounted(load);
</script>

<template>
  <div>
    <h2 class="text-h5 q-mb-md">Skills</h2>

    <q-banner v-if="error" class="bg-negative text-white q-mb-md" rounded>{{ error }}</q-banner>
    <div v-else-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <q-list v-else bordered separator class="bg-secondary rounded-borders">
      <q-item v-for="skill in skills" :key="skill.id">
        <q-item-section>
          <q-item-label class="text-weight-medium">{{ skill.name }}</q-item-label>
          <q-item-label caption>
            <q-badge :color="skill.overridden ? 'primary' : 'grey-7'" text-color="dark">
              {{ skill.overridden ? "Overridden" : "Embedded" }}
            </q-badge>
          </q-item-label>
        </q-item-section>
        <q-item-section side>
          <div class="row q-gutter-sm">
            <q-btn flat dense color="primary" icon="sym_o_edit" label="Edit" @click="edit(skill.id)" />
            <q-btn v-if="skill.overridden" flat dense color="negative" icon="sym_o_restore" label="Reset" @click="reset(skill.id)" />
          </div>
        </q-item-section>
      </q-item>
    </q-list>
  </div>
</template>
