/**
 * Drives the shipped preview markdown helper (renderDocumentBody) on the real path:
 * empty html + raw markdown must become HTML (not left as source).
 *
 * Bundle the TS module with esbuild (devDependency), then assert.
 */
import { createRequire } from "node:module";
import { mkdtempSync, readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const require = createRequire(import.meta.url);
const esbuild = require("esbuild");
const entry = join(root, "internal/preview/preview_ui_src/lib/markdown.ts");
const outDir = mkdtempSync(join(tmpdir(), "preview-md-"));
const outFile = join(outDir, "markdown.mjs");

await esbuild.build({
  entryPoints: [entry],
  bundle: true,
  platform: "node",
  format: "esm",
  outfile: outFile,
  logLevel: "silent",
});

const mod = await import(outFile + `?t=${Date.now()}`);
if (typeof mod.renderDocumentBody !== "function") {
  console.error("FAIL: renderDocumentBody not exported from shipped markdown module");
  process.exit(1);
}

const rawOnly = mod.renderDocumentBody({
  html: "",
  raw: "# Hello **docs**\n\nParagraph with `code`.\n",
});
if (!rawOnly.includes("<strong>") && !rawOnly.includes("<em>")) {
  // marked uses <strong> for **
  if (!/<strong>docs<\/strong>/.test(rawOnly) && !rawOnly.includes("<p>")) {
    console.error("FAIL: raw markdown was not rendered to HTML:", rawOnly);
    process.exit(1);
  }
}
if (!rawOnly.includes("Hello") || rawOnly.includes("# Hello")) {
  // heading should become <h1>, not leave markdown heading syntax as sole content
  if (!/<h1[\s>]/.test(rawOnly)) {
    console.error("FAIL: expected heading HTML from raw markdown:", rawOnly);
    process.exit(1);
  }
}
if (rawOnly.includes("**docs**")) {
  console.error("FAIL: bold markers left unrendered:", rawOnly);
  process.exit(1);
}

const preferHtml = mod.renderDocumentBody({
  html: "<p>pre-rendered</p>",
  raw: "# should not appear",
});
if (preferHtml !== "<p>pre-rendered</p>") {
  console.error("FAIL: non-empty html should be preferred:", preferHtml);
  process.exit(1);
}

const empty = mod.renderDocumentBody({ html: "", raw: "" });
if (empty !== "") {
  console.error("FAIL: empty input should return empty string");
  process.exit(1);
}

// Structural: Docs.tsx must call renderDocumentBody (not raw <pre> fallback only).
const docsView = readFileSync(join(root, "internal/preview/preview_ui_src/views/Docs.tsx"), "utf8");
if (!docsView.includes("renderDocumentBody")) {
  console.error("FAIL: Docs.tsx does not call renderDocumentBody");
  process.exit(1);
}
if (docsView.includes("fallback={<pre") && !docsView.includes("renderDocumentBody")) {
  console.error("FAIL: Docs still only uses raw pre fallback");
  process.exit(1);
}

console.log("PASS renderDocumentBody client markdown path");
console.log("sample:", rawOnly.slice(0, 120).replace(/\n/g, " "));
