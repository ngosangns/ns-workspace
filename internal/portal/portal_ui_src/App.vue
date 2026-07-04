<script setup lang="ts">
import { ref, computed } from "vue";
import { useRouter } from "./router";

const { current: currentView } = useRouter();
const route = ref(window.location.hash || "#");

window.addEventListener("hashchange", () => {
  route.value = window.location.hash || "#";
});

const isActive = (prefix: string) => computed(() => route.value.startsWith(prefix)).value;
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>ns-workspace</h1>
        <p>Skills & MCP Portal</p>
      </div>
      <nav class="nav">
        <a href="#" :class="{ active: route === '#' }">Dashboard</a>
        <a href="#skills" :class="{ active: isActive('#skills') }">Skills</a>
        <a href="#mcps" :class="{ active: route === '#mcps' }">MCPs</a>
        <a href="#registry" :class="{ active: route === '#registry' }">Registry</a>
        <a href="#adapters" :class="{ active: route === '#adapters' }">Adapters</a>
      </nav>
    </aside>
    <main class="main">
      <component :is="currentView" />
    </main>
  </div>
</template>
