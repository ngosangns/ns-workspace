import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { resolve } from "path";

export default defineConfig({
  plugins: [vue()],
  base: "/",
  root: "internal/portal/portal_ui_src",
  publicDir: "public",
  build: {
    outDir: "../portal_ui",
    emptyOutDir: true,
    rollupOptions: {
      input: resolve(__dirname, "internal/portal/portal_ui_src/index.html"),
    },
  },
  resolve: {
    alias: {
      "@": resolve(__dirname, "internal/portal/portal_ui_src"),
    },
  },
  server: {
    port: 5174,
  },
});
