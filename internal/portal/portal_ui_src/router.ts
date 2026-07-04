import { ref, computed, type Component } from "vue";
import Dashboard from "./views/Dashboard.vue";
import Skills from "./views/Skills.vue";
import SkillEdit from "./views/SkillEdit.vue";
import MCPs from "./views/MCPs.vue";
import Registry from "./views/Registry.vue";
import Adapters from "./views/Adapters.vue";

const routes: Record<string, Component> = {
  "#": Dashboard,
  "#skills": Skills,
  "#skills/edit": SkillEdit,
  "#mcps": MCPs,
  "#registry": Registry,
  "#adapters": Adapters,
};

const hash = ref(window.location.hash || "#");

window.addEventListener("hashchange", () => {
  hash.value = window.location.hash || "#";
});

export function useRouter() {
  const current = computed(() => {
    const base = hash.value.split("?")[0];
    return routes[base] || Dashboard;
  });

  const params = computed(() => {
    const parts = hash.value.split("?");
    if (parts.length < 2) return {};
    const search = new URLSearchParams(parts[1]);
    const out: Record<string, string> = {};
    search.forEach((value, key) => {
      out[key] = value;
    });
    return out;
  });

  function navigate(to: string) {
    window.location.hash = to;
  }

  return { current, params, navigate };
}
