<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useQuasar } from "quasar";

const $q = useQuasar();
const route = useRoute();
const router = useRouter();
const drawerOpen = ref(true);

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
}

function toggleDark() {
  $q.dark.toggle();
}
</script>

<template>
  <q-layout view="hHh Lpr lff" class="bg-dark text-grey-1">
    <q-header elevated class="bg-secondary text-grey-1">
      <q-toolbar>
        <q-btn flat round dense icon="sym_o_menu" @click="drawerOpen = !drawerOpen" />
        <q-toolbar-title>ns-workspace</q-toolbar-title>
        <q-btn flat round dense :icon="$q.dark.isActive ? 'sym_o_light_mode' : 'sym_o_dark_mode'" @click="toggleDark" />
      </q-toolbar>
    </q-header>

    <q-drawer v-model="drawerOpen" show-if-above bordered class="bg-secondary">
      <q-list padding>
        <q-item
          v-for="item in navItems"
          :key="item.to"
          clickable
          :active="route.path === item.to || route.path.startsWith(item.to + '/')"
          active-class="bg-primary text-dark"
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

    <q-page-container>
      <q-page padding>
        <router-view />
      </q-page>
    </q-page-container>
  </q-layout>
</template>
