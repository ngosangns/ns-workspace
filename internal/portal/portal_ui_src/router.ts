import { createRouter, createWebHashHistory, type RouteRecordRaw } from "vue-router";
import Dashboard from "./views/Dashboard.vue";
import Skills from "./views/Skills.vue";
import SkillEdit from "./views/SkillEdit.vue";
import MCPs from "./views/MCPs.vue";
import Registry from "./views/Registry.vue";
import Adapters from "./views/Adapters.vue";

const routes: RouteRecordRaw[] = [
  { path: "/", component: Dashboard },
  { path: "/skills", component: Skills },
  { path: "/skills/edit", component: SkillEdit },
  { path: "/mcps", component: MCPs },
  { path: "/registry", component: Registry },
  { path: "/adapters", component: Adapters },
];

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
});
