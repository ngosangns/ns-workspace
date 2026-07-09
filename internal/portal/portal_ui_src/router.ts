import { createRouter, createWebHashHistory, type RouteRecordRaw } from "vue-router";

const routes: RouteRecordRaw[] = [
  {
    path: "/",
    name: "dashboard",
    component: () => import("./views/Dashboard.vue"),
  },
  {
    path: "/skills",
    name: "skills",
    component: () => import("./views/Skills.vue"),
  },
  {
    path: "/mcps",
    name: "mcps",
    component: () => import("./views/MCPs.vue"),
  },
  {
    path: "/registry",
    name: "registry",
    component: () => import("./views/Registry.vue"),
  },
  {
    path: "/adapters",
    name: "adapters",
    component: () => import("./views/Adapters.vue"),
  },
];

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 };
  },
});
