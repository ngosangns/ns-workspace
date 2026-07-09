import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  base: "/",
  root: "internal/portal/portal_ui_src",
  publicDir: "public",
  build: {
    outDir: "../portal_ui",
    emptyOutDir: true,
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      input: resolve(__dirname, "internal/portal/portal_ui_src/index.html"),
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            if (id.includes("@codemirror") || id.includes("/codemirror/")) {
              return "codemirror";
            }
            if (id.includes("vue") || id.includes("vue-router")) {
              return "vue";
            }
            if (id.includes("@fontsource")) {
              return "fonts";
            }
            if (id.includes("@phosphor-icons")) {
              return "icons";
            }
          }
        },
      },
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
