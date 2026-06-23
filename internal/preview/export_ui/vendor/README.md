# Vendored libraries (offline embed)

These files are committed to the repo on purpose so that the static HTML export
(`export --inline-assets=true`, the default) is fully self-contained and opens
over `file://` with **no network requests**. This is a deliberate tradeoff: the
export must not depend on the Vite build pipeline used by `preview_ui_src/`.

| File | Library | Version | License |
| --- | --- | --- | --- |
| `cytoscape.min.js` | [Cytoscape.js](https://js.cytoscape.org/) | 3.30.2 | MIT |
| `marked.min.js` | [marked](https://marked.js.org/) | 12.0.2 | MIT |

Both libraries are MIT licensed; their copyright headers are preserved inline in
the minified files. To refresh:

```bash
curl -fsSL -o cytoscape.min.js https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js
curl -fsSL -o marked.min.js     https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js
```

When `--inline-assets=false`, the template references these same versions from a
CDN instead of inlining them.
