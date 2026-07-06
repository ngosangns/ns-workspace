import { createRouter, createWebHashHistory, type RouteRecordRaw } from "vue-router";
import Dashboard from "./views/Dashboard.vue";
import Skills from "./views/Skills.vue";
import MCPs from "./views/MCPs.vue";
import Registry from "./views/Registry.vue";
import Adapters from "./views/Adapters.vue";
import Claude from "./views/Claude.vue";

const routes: RouteRecordRaw[] = [
  { path: "/", component: Dashboard },
  { path: "/skills", component: Skills },
  { path: "/mcps", component: MCPs },
  { path: "/registry", component: Registry },
  { path: "/adapters", component: Adapters },
  { path: "/claude", component: Claude },
];

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
});
