import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";

export default defineConfig({
  plugins: [solid(), tailwindcss()],
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
            if (id.includes("solid-js") || id.includes("@solidjs")) {
              return "solid";
            }
            if (id.includes("@fontsource")) {
              return "fonts";
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
