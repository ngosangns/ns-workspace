<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useQuasar } from "quasar";

const $q = useQuasar();
const route = useRoute();
const router = useRouter();
const drawerOpen = ref(false);

interface NavItem {
  label: string;
  icon: string;
  to: string;
}

const navItems: NavItem[] = [
  { label: "Dashboard", icon: "sym_o_dashboard", to: "/" },
  { label: "Skills", icon: "sym_o_psychology", to: "/skills" },
  { label: "MCPs", icon: "sym_o_dns", to: "/mcps" },
  { label: "Registry", icon: "sym_o_apps", to: "/registry" },
  { label: "Adapters", icon: "sym_o_extension", to: "/adapters" },
];

function navigate(to: string) {
  router.push(to);
  drawerOpen.value = false;
}

function toggleDark() {
  $q.dark.toggle();
}

function isActive(to: string) {
  return route.path === to || route.path.startsWith(`${to}/`);
}
</script>

<template>
  <q-layout view="hHh Lpr lff" class="text-grey-1">
    <q-btn flat round dense class="lt-md app-menu-fab" icon="sym_o_menu" @click="drawerOpen = !drawerOpen" />

    <q-drawer v-model="drawerOpen" bordered class="app-drawer lt-md">
      <q-list padding class="app-nav-list">
        <q-item
          v-for="item in navItems"
          :key="item.to"
          clickable
          :class="['app-nav-item', { 'app-nav-item--active': isActive(item.to) }]"
          @click="navigate(item.to)"
        >
          <q-item-section avatar>
            <q-icon :name="item.icon" />
          </q-item-section>
          <q-item-section>
            <q-item-label>{{ item.label }}</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
    </q-drawer>

    <div class="app-sidebar gt-sm">
      <div class="app-sidebar-inner">
        <div class="app-sidebar-brand">
          <div class="app-logo">
            <span class="app-logo-mark">ns</span>
          </div>
          <span class="app-sidebar-title">ns-workspace</span>
        </div>

        <nav class="app-sidebar-nav">
          <button
            v-for="item in navItems"
            :key="item.to"
            type="button"
            :class="['app-sidebar-link', { 'app-sidebar-link--active': isActive(item.to) }]"
            @click="navigate(item.to)"
          >
            <q-icon :name="item.icon" class="app-sidebar-link-icon" />
            <span>{{ item.label }}</span>
          </button>
        </nav>

        <div class="app-sidebar-footer">
          <button type="button" class="app-sidebar-link" @click="toggleDark">
            <q-icon :name="$q.dark.isActive ? 'sym_o_light_mode' : 'sym_o_dark_mode'" class="app-sidebar-link-icon" />
            <span>{{ $q.dark.isActive ? "Light mode" : "Dark mode" }}</span>
          </button>
        </div>
      </div>
    </div>

    <q-page-container class="app-page-container">
      <q-page class="app-page">
        <router-view />
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<style scoped>
.app-menu-fab {
  position: fixed;
  top: 16px;
  left: 16px;
  z-index: 2002;
  color: var(--color-text-secondary);
  background: rgba(11, 13, 16, 0.8);
  backdrop-filter: blur(12px);
  border: 1px solid var(--color-border);
}

.app-menu-fab:hover {
  color: var(--color-text);
}

.app-sidebar {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: 240px;
  z-index: 2001;
  padding: 16px;
}

.app-sidebar-inner {
  height: 100%;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  display: flex;
  flex-direction: column;
  padding: 16px 12px;
}

.app-sidebar-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px 20px;
  border-bottom: 1px solid var(--color-border);
  margin-bottom: 12px;
}

.app-sidebar-title {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--color-text);
}

.app-sidebar-nav {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.app-sidebar-link {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  text-align: left;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  transition:
    color var(--transition-fast),
    background var(--transition-fast);
}

.app-sidebar-link:hover {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.04);
}

.app-sidebar-link--active {
  color: var(--color-accent);
  background: rgba(45, 212, 191, 0.1);
}

.app-sidebar-link-icon {
  font-size: 20px;
}

.app-sidebar-footer {
  border-top: 1px solid var(--color-border);
  padding-top: 12px;
  margin-top: 12px;
}

.app-page-container {
  margin-left: 0;
}

@media (min-width: 1024px) {
  .app-page-container {
    margin-left: 240px;
  }
}

.app-page {
  max-width: 1200px;
  padding: 24px 16px 40px;
}

@media (min-width: 768px) {
  .app-page {
    padding: 32px 32px 40px;
  }
}

.app-drawer {
  background: var(--color-surface);
}

.app-nav-list {
  padding: 12px;
}

.app-nav-item {
  border-radius: var(--radius-md);
  margin-bottom: 4px;
  color: var(--color-text-secondary);
}

.app-nav-item:hover {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.04);
}

.app-nav-item--active {
  color: var(--color-accent);
  background: rgba(45, 212, 191, 0.1);
}
</style>
