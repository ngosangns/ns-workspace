import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { quasar, transformAssetUrls } from "@quasar/vite-plugin";
import { resolve } from "path";

export default defineConfig({
  plugins: [
    vue({
      template: { transformAssetUrls },
    }),
    quasar({
      sassVariables: resolve(__dirname, "internal/portal/portal_ui_src/quasar-variables.sass"),
      autoImportComponentCase: "kebab",
    }),
  ],
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
