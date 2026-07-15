import js from "@eslint/js";
import globals from "globals";

// TypeScript 7's public API no longer exposes ts.Extension.* the way
// typescript-eslint 8 expects, so TS/TSX lint is handled by Biome instead.
// ESLint here covers plain JS tooling scripts only.
export default [
  {
    ignores: [
      "internal/portal/portal_ui/**",
      "internal/preview/preview_ui/**",
      "internal/preview/export_ui/**",
      "internal/portal/portal_ui_src/**",
      "internal/preview/preview_ui_src/**",
      "internal/preview/export_ui_src/**",
      "node_modules/**",
    ],
  },
  {
    files: ["scripts/**/*.cjs"],
    ...js.configs.recommended,
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "commonjs",
      globals: {
        ...globals.node,
      },
    },
  },
  {
    files: ["scripts/**/*.{js,mjs}", "*.config.{js,mjs,cjs}"],
    ...js.configs.recommended,
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      globals: {
        ...globals.node,
      },
    },
  },
];
