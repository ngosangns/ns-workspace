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
      input: {
        index: resolve(__dirname, "internal/preview/preview_ui_src/index.html"),
        search: resolve(__dirname, "internal/preview/preview_ui_src/search.html"),
      },
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
