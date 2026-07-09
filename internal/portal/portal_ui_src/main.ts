import { createApp } from "vue";
import "./style.css";
import App from "./App.vue";
import { router } from "./router";

const app = createApp(App);
app.use(router);
app.mount("#app");

// Subtle scroll-entry animation for elements with .fade-in-up
const prefersReduced = typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

if (!prefersReduced) {
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
    { threshold: 0.08, rootMargin: "0px 0px -24px 0px" },
  );

  function observeAnimations() {
    document.querySelectorAll(".fade-in-up:not(.is-visible)").forEach((el) => {
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
} else {
  document.querySelectorAll(".fade-in-up").forEach((el) => el.classList.add("is-visible"));
  const mutationObserver = new MutationObserver(() => {
    document.querySelectorAll(".fade-in-up").forEach((el) => el.classList.add("is-visible"));
  });
  mutationObserver.observe(document.body, { childList: true, subtree: true });
}
