# Graph Report - /Users/ngosangns/Github/ns-workspace  (2026-05-08)

## Corpus Check
- 7 files · ~17,037 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 230 nodes · 509 edges · 11 communities detected
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 23 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]

## God Nodes (most connected - your core abstractions)
1. `parseSpecGraph()` - 16 edges
2. `run()` - 15 edges
3. `scanSpecProject()` - 14 edges
4. `selectSpec()` - 13 edges
5. `apply()` - 12 edges
6. `load()` - 11 edges
7. `reloadPreviewData()` - 11 edges
8. `previewServer` - 10 edges
9. `writeFileManaged()` - 10 edges
10. `renderSpecList()` - 9 edges

## Surprising Connections (you probably didn't know these)
- `specsRoot()` --calls--> `expandPath()`  [INFERRED]
  /Users/ngosangns/Github/ns-workspace/spec_project.go → /Users/ngosangns/Github/ns-workspace/main.go
- `runPreview()` --calls--> `expandPath()`  [INFERRED]
  /Users/ngosangns/Github/ns-workspace/preview.go → /Users/ngosangns/Github/ns-workspace/main.go
- `run()` --calls--> `runPreview()`  [INFERRED]
  /Users/ngosangns/Github/ns-workspace/main.go → /Users/ngosangns/Github/ns-workspace/preview.go
- `TestPreviewHTTPHandlers()` --calls--> `newPreviewServer()`  [INFERRED]
  /Users/ngosangns/Github/ns-workspace/preview_test.go → /Users/ngosangns/Github/ns-workspace/preview.go
- `renderMermaidSVGWithMMDC()` --calls--> `run()`  [INFERRED]
  /Users/ngosangns/Github/ns-workspace/preview.go → /Users/ngosangns/Github/ns-workspace/main.go

## Communities

### Community 0 - "Community 0"
Cohesion: 0.09
Nodes (48): applyRouteFromLocation(), applyTheme(), autoExpandForSelection(), buildSpecTree(), decorateDiagram(), defaultSpecId(), destroyDiagramPanZoom(), destroyDiagramsIn() (+40 more)

### Community 1 - "Community 1"
Cohesion: 0.1
Nodes (47): adapters(), apply(), backupAndRemove(), backupPath(), checkJSON(), compact(), copyAny(), copyDir() (+39 more)

### Community 2 - "Community 2"
Cohesion: 0.13
Nodes (20): mermaidRenderer, mermaidRenderRequest, mermaidRenderResponse, previewOptions, previewServer, mermaidTheme(), newestEmbeddedModToken(), newestModToken() (+12 more)

### Community 3 - "Community 3"
Cohesion: 0.15
Nodes (19): graphConstraint, graphEdge, graphNode, graphRelation, moduleMeta, projectSummary, specDocument, specGraph (+11 more)

### Community 4 - "Community 4"
Cohesion: 0.28
Nodes (15): TestPreviewHTTPHandlers(), readOptional(), scanSpecProject(), findDoc(), hasEdge(), hasGraphNode(), hasGraphNodeSpec(), hasRelationship() (+7 more)

### Community 5 - "Community 5"
Cohesion: 0.12
Nodes (0): 

### Community 6 - "Community 6"
Cohesion: 0.23
Nodes (12): contentWithoutMetadata(), headingLevel(), isSpecControlFile(), isSpecLinkMetadataKey(), mentionsSpecPath(), mentionsSpecTitle(), metadataBlock(), parseDocumentConnections() (+4 more)

### Community 7 - "Community 7"
Cohesion: 0.27
Nodes (12): addKnownSpecAliases(), canonicalGraphEndpoint(), canonicalSpecNodeID(), dedupeEdges(), dedupeRelationships(), documentGraphID(), fencedBlockAfterHeading(), moduleIDFromPath() (+4 more)

### Community 8 - "Community 8"
Cohesion: 0.24
Nodes (10): extractMarkdownLinkTarget(), isMarkdownSeparatorRow(), markdownTableCells(), normalizeHeaders(), normalizeSpecPath(), parseModuleTable(), resolveSpecReference(), splitMetadataLine() (+2 more)

### Community 9 - "Community 9"
Cohesion: 0.4
Nodes (6): cleanConstraintNode(), cleanGraphLine(), firstNonEmpty(), parseForbiddenDependencies(), parseForbiddenLine(), splitConstraintTarget()

### Community 10 - "Community 10"
Cohesion: 0.4
Nodes (6): cleanNodeName(), extractMermaidInlineLabel(), parseMermaidEdge(), parseRelationships(), resolveMermaidEndpoint(), splitNodeList()

## Knowledge Gaps
- **17 isolated node(s):** `specProject`, `projectSummary`, `specDocument`, `specGraph`, `graphNode` (+12 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `scanSpecProject()` connect `Community 4` to `Community 8`, `Community 2`, `Community 3`, `Community 7`?**
  _High betweenness centrality (0.386) - this node is a cross-community bridge._
- **Why does `TestRenderMarkdownSupportsGFMTables()` connect `Community 4` to `Community 0`?**
  _High betweenness centrality (0.357) - this node is a cross-community bridge._
- **Why does `renderMarkdown()` connect `Community 0` to `Community 4`?**
  _High betweenness centrality (0.353) - this node is a cross-community bridge._
- **Are the 6 inferred relationships involving `run()` (e.g. with `renderMermaidSVGWithMMDC()` and `TestPreviewHelpIsAccepted()`) actually correct?**
  _`run()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `scanSpecProject()` (e.g. with `.load()` and `TestScanSpecProjectParsesViclassStyleIndex()`) actually correct?**
  _`scanSpecProject()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **What connects `specProject`, `projectSummary`, `specDocument` to the rest of the system?**
  _17 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._