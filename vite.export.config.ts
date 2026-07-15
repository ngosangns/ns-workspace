import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import { resolve } from "path";

// Build a single IIFE viewer script for offline export HTML injection.
export default defineConfig({
  plugins: [solid()],
  build: {
    outDir: resolve(__dirname, "internal/preview/export_ui"),
    emptyOutDir: false,
    lib: {
      entry: resolve(__dirname, "internal/preview/export_ui_src/main.tsx"),
      name: "NsExportViewer",
      formats: ["iife"],
      fileName: () => "viz.js",
    },
    rollupOptions: {
      // cytoscape and marked are provided by vendor scripts in the template
      external: [],
      output: {
        assetFileNames: (assetInfo) => {
          if (assetInfo.name?.endsWith(".css")) {
            return "viz.css";
          }
          return assetInfo.name || "asset";
        },
      },
    },
    cssCodeSplit: false,
  },
});
