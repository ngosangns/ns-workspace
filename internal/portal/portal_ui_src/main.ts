import { createApp } from "vue";
import { Quasar, Dark } from "quasar";
import iconSet from "quasar/icon-set/material-symbols-outlined";

import "@quasar/extras/material-symbols-outlined/material-symbols-outlined.css";
import "quasar/src/css/index.sass";
import "./public/style.css";

import App from "./App.vue";
import { router } from "./router";

const app = createApp(App);

app.use(Quasar, {
  plugins: {
    Dark,
  },
  config: {
    dark: "auto",
  },
  iconSet,
});

app.use(router);
app.mount("#app");

// Subtle scroll-entry animation for elements with .fade-in-up
const observed = new WeakSet<Element>();
const observer = new IntersectionObserver(
  (entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        entry.target.classList.add("is-visible");
        observer.unobserve(entry.target);
        observed.delete(entry.target);
      }
    });
  },
  { threshold: 0.1, rootMargin: "0px 0px -40px 0px" },
);

function observeAnimations() {
  document.querySelectorAll(".fade-in-up").forEach((el) => {
    if (!observed.has(el)) {
      observed.add(el);
      observer.observe(el);
    }
  });
}

observeAnimations();

const mutationObserver = new MutationObserver(() => {
  observeAnimations();
});

mutationObserver.observe(document.body, { childList: true, subtree: true });
