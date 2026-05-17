import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { resolve } from "path";

export default defineConfig({
  plugins: [vue()],
  base: "/",
  root: "internal/preview/preview_ui_src",
  publicDir: "public",
  build: {
    outDir: "../preview_ui",
    emptyOutDir: true,
    rollupOptions: {
      input: "internal/preview/preview_ui_src/index.html",
    },
  },
  resolve: {
    alias: {
      "@": resolve(__dirname, "internal/preview/preview_ui_src"),
    },
  },
  server: {
    port: 5173,
  },
});
