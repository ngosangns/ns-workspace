<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api, type Skill } from "../api";
import { useRouter } from "../router";

const skills = ref<Skill[]>([]);
const loading = ref(true);
const error = ref("");
const { navigate } = useRouter();

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
  if (!confirm(`Reset skill "${id}" to default?`)) return;
  try {
    await api.resetSkill(id);
    await load();
  } catch (e: any) {
    error.value = e.message || String(e);
  }
}

function edit(id: string) {
  navigate(`#skills/edit?id=${encodeURIComponent(id)}`);
}

onMounted(load);
</script>

<template>
  <div>
    <h2 class="page-title">Skills</h2>
    <p v-if="loading" class="empty">Loading...</p>
    <p v-else-if="error" class="empty" style="color: var(--danger)">{{ error }}</p>
    <div v-else class="list">
      <div v-for="skill in skills" :key="skill.id" class="list-item">
        <div>
          <div class="title">{{ skill.name }}</div>
          <div class="meta">
            <span :class="['badge', skill.overridden ? 'overlay' : 'embedded']">
              {{ skill.overridden ? "Overridden" : "Embedded" }}
            </span>
          </div>
        </div>
        <div class="toolbar" style="margin-bottom: 0">
          <button class="btn" @click="edit(skill.id)">Edit</button>
          <button v-if="skill.overridden" class="btn danger" @click="reset(skill.id)">Reset</button>
        </div>
      </div>
    </div>
  </div>
</template>
