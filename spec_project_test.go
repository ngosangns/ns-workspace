package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanSpecProjectParsesViclassStyleIndex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "AGENTS.md", "# Agents\n")
	writeTestFile(t, root, "specs/_sync.md", `# Spec Sync State

## Current Sync

- **Last Synced Commit**: abc123
- **Branch**: main
- **Sync Date**: 2026-05-08 10:00 +0700
`)
	writeTestFile(t, root, "specs/_index.md", `# Spec Index & Dependency Graph

## Modules

| Module | Spec File | Status | Version | Compliance | Priority |
| ------ | --------- | ------ | ------- | ---------- | -------- |
| Editor Core | [editor-core.md](./modules/editor-core.md) | Finalized | v1.5 | Compliant | P0 |
| Portal | [portal/](./modules/portal/_overview.md) | Active | v1.0 | Unchecked | P1 |

## Dependency Graph

`+"```"+`
editor.core (leaf - no dependencies)
├── editor.word → editor.core
portal.common → editor.core, common.libs.captcha
`+"```"+`

## Relationship Map

### Data Flows

- Portal Common → Editor Core: consumes errors
`)
	writeTestFile(t, root, "specs/modules/editor-core.md", `# Editor Core

## Meta

- **Status**: Draft
- **Version**: v0.1
`)
	writeTestFile(t, root, "specs/modules/portal/_overview.md", "# Portal\n")

	project, err := scanSpecProject(root, "specs")
	if err != nil {
		t.Fatal(err)
	}
	if !project.Summary.AgentsFound || !project.Summary.IndexFound || !project.Summary.SyncFound {
		t.Fatalf("expected project markers to be found: %+v", project.Summary)
	}
	if project.Summary.Sync["Last Synced Commit"] != "abc123" {
		t.Fatalf("sync state not parsed: %+v", project.Summary.Sync)
	}
	core := findDoc(t, project.Documents, "modules/editor-core.md")
	if core.Title != "Editor Core" || core.Status != "Finalized" || core.Version != "v1.5" || core.Priority != "P0" {
		t.Fatalf("module table metadata not applied: %+v", core)
	}
	if !hasEdge(project.Graph.Edges, "editor.word", "editor.core") {
		t.Fatalf("dependency edge not parsed: %+v", project.Graph.Edges)
	}
	if !hasEdge(project.Graph.Edges, "portal.common", "common.libs.captcha") {
		t.Fatalf("comma dependency edge not parsed: %+v", project.Graph.Edges)
	}
	if len(project.Graph.Relationships) != 1 || project.Graph.Relationships[0].Description != "consumes errors" {
		t.Fatalf("relationship map not parsed: %+v", project.Graph.Relationships)
	}
}

func TestScanSpecProjectParsesMermaidDependencyGraph(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "specs/_index.md", `# Spec Index & Dependency Graph

## Modules

| Module | Spec File | Status | Version | Compliance | Priority |
| ------ | --------- | ------ | ------- | ---------- | -------- |
| Editor Core | [editor-core.md](./modules/editor-core.md) | Finalized | v1.5 | Unchecked | P0 |
| MFE & WW | [mfe.md](./modules/mfe.md) | Finalized | v1.0 | Unchecked | P0 |
| Editor UI | [editorui/](./modules/editorui/_overview.md) | Finalized | v1.0 | Unchecked | P0 |
| Turnstile Captcha | [turnstile-captcha.md](./modules/turnstile-captcha.md) | Finalized | v1.0 | Unchecked | P1 |
| Portal Homepage | [portal-homepage.md](./modules/portal/portal-homepage.md) | Draft | v1.0 | Unchecked | P1 |
| Data Models | [data-models.md](./shared/data-models.md) | Finalized | v1.0 | Unchecked | P1 |

## Dependency Graph

`+"```mermaid"+`
flowchart LR
    %% Arrows point from consumer to dependency.
    subgraph Editors
        editor_core["editor-core"]
        editor_word["editor-word"]
    end
    subgraph Portal
        portal_homepage["portal.homepage"]
    end
    subgraph Tooling
        dev_tooling["dev-tooling"]
    end
    subgraph Runtime
        mfe["mfe"]
        ww["ww"]
    end
    subgraph EditorUI
        editorui_common["editorui.common"]
    end
    subgraph Shared
        turnstile["turnstile-captcha"]
    end
    subgraph Shared
        shared_models["shared/data-models"]
    end
    editor_word --> editor_core
    editor_core --> shared_models
    editorui_common --> editor_core
    ww --> mfe
    portal_homepage --> turnstile
    portal_homepage --> editor_core
    dev_tooling --> specs_root["root specs"]
`+"```"+`

FORBIDDEN:
- editor.core -> editor.* (core cannot depend on specific editor)
- portal.* -> editor.* directly for normal document loading (portal must go through MFE/config)
`)
	writeTestFile(t, root, "specs/modules/editor-core.md", "# Editor Core\n")
	writeTestFile(t, root, "specs/modules/mfe.md", "# MFE & Web Wrappers\n")
	writeTestFile(t, root, "specs/modules/editorui/_overview.md", "# EditorUI Module\n")
	writeTestFile(t, root, "specs/modules/turnstile-captcha.md", "# Turnstile Captcha\n")
	writeTestFile(t, root, "specs/modules/portal/portal-homepage.md", "# Portal Homepage\n")
	writeTestFile(t, root, "specs/shared/data-models.md", "# Data Models\n")

	project, err := scanSpecProject(root, "specs")
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range [][2]string{
		{"editor-word", "editor-core"},
		{"editor-core", "shared/data-models"},
		{"editorui.common", "editor-core"},
		{"ww", "mfe"},
		{"portal.homepage", "turnstile-captcha"},
		{"portal.homepage", "editor-core"},
		{"dev-tooling", "root specs"},
	} {
		if !hasEdge(project.Graph.Edges, edge[0], edge[1]) {
			t.Fatalf("missing mermaid edge %s -> %s in %+v", edge[0], edge[1], project.Graph.Edges)
		}
	}
	if !hasGraphNode(project.Graph.Nodes, "root specs") {
		t.Fatalf("missing inline mermaid node: %+v", project.Graph.Nodes)
	}
	if !hasGraphNode(project.Graph.Nodes, "portal.homepage") || hasGraphNode(project.Graph.Nodes, "portal-homepage") {
		t.Fatalf("expected Mermaid label to be the canonical graph node: %+v", project.Graph.Nodes)
	}
	if !hasGraphNode(project.Graph.Nodes, "shared/data-models") || hasGraphNode(project.Graph.Nodes, "data-models") {
		t.Fatalf("expected shared path label to be the canonical graph node: %+v", project.Graph.Nodes)
	}
	for _, node := range []struct {
		id     string
		specID string
	}{
		{"ww", "modules/mfe.md"},
		{"editorui.common", "modules/editorui/_overview.md"},
		{"turnstile-captcha", "modules/turnstile-captcha.md"},
	} {
		if !hasGraphNodeSpec(project.Graph.Nodes, node.id, node.specID) {
			t.Fatalf("expected graph node %s to link to %s: %+v", node.id, node.specID, project.Graph.Nodes)
		}
	}
	if !strings.Contains(project.Graph.DependencyDiagram, `subgraph Editors`) || !strings.Contains(project.Graph.DependencyDiagram, `editor_word --> editor_core`) {
		t.Fatalf("dependency diagram source not preserved: %s", project.Graph.DependencyDiagram)
	}
	if len(project.Graph.Constraints) != 2 || project.Graph.Constraints[0].From != "editor.core" || project.Graph.Constraints[0].To != "editor.*" {
		t.Fatalf("forbidden dependency rules not parsed: %+v", project.Graph.Constraints)
	}
	if project.Graph.Constraints[1].From != "portal.*" || project.Graph.Constraints[1].To != "editor.*" {
		t.Fatalf("forbidden dependency target was not cleaned: %+v", project.Graph.Constraints[1])
	}
}

func TestScanSpecProjectSplitsRelationshipMapLists(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "specs/_index.md", `# Spec Index

## Relationship Map

### API Dependencies

- portal.homepage, portal.classrooms, ww → config.server: runtime editor lookup
`)
	writeTestFile(t, root, "specs/overview.md", "# Overview\n")

	project, err := scanSpecProject(root, "specs")
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range [][2]string{
		{"portal.homepage", "config.server"},
		{"portal.classrooms", "config.server"},
		{"ww", "config.server"},
	} {
		if !hasRelationship(project.Graph.Relationships, edge[0], edge[1]) {
			t.Fatalf("missing relationship %s -> %s in %+v", edge[0], edge[1], project.Graph.Relationships)
		}
	}
}

func TestScanSpecProjectParsesSpecMetadataAndContentConnections(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "specs/modules/editor-core.md", `# Editor Core

## Meta

- **Status**: Finalized
- **Links**: [Data Models](../shared/data-models.md), [Portal](./portal/homepage.md)

## Notes

Runtime behavior also mentions turnstile-captcha.md in prose.
`)
	writeTestFile(t, root, "specs/shared/data-models.md", "# Data Models\n")
	writeTestFile(t, root, "specs/modules/portal/homepage.md", "# Portal Homepage\n\nSee [Editor Core](../editor-core.md).\n")
	writeTestFile(t, root, "specs/modules/turnstile-captcha.md", "# Turnstile Captcha\n")

	project, err := scanSpecProject(root, "specs")
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range [][2]string{
		{"editor-core", "data-models"},
		{"editor-core", "homepage"},
		{"editor-core", "turnstile-captcha"},
		{"homepage", "editor-core"},
	} {
		if !hasRelationship(project.Graph.Relationships, edge[0], edge[1]) {
			t.Fatalf("missing document connection %s -> %s in %+v", edge[0], edge[1], project.Graph.Relationships)
		}
	}
	if !hasGraphNodeSpec(project.Graph.Nodes, "data-models", "shared/data-models.md") {
		t.Fatalf("metadata-linked shared spec should remain clickable in graph: %+v", project.Graph.Nodes)
	}
}

func TestScanSpecProjectFallsBackWithoutIndex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "specs/modules/dev-tooling.md", `# Dev Tooling

## Meta

- **Status**: Draft
- **Version**: v1.14
- **Compliance**: Unchecked
`)
	project, err := scanSpecProject(root, "specs")
	if err != nil {
		t.Fatal(err)
	}
	if project.Summary.IndexFound {
		t.Fatalf("index should be missing")
	}
	if len(project.Documents) != 1 {
		t.Fatalf("expected fallback scan to find one document, got %d", len(project.Documents))
	}
	doc := project.Documents[0]
	if doc.Title != "Dev Tooling" || doc.Status != "Draft" || doc.Version != "v1.14" {
		t.Fatalf("fallback metadata not parsed: %+v", doc)
	}
}

func TestRenderMarkdownSupportsGFMTables(t *testing.T) {
	html, err := renderMarkdown([]byte(`| File | Description |
| ---- | ----------- |
| ` + "`a/b.ts`" + ` | table cell |
`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<td><code>a/b.ts</code></td>") {
		t.Fatalf("expected GFM table HTML, got: %s", html)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findDoc(t *testing.T, docs []specDocument, id string) specDocument {
	t.Helper()
	for _, doc := range docs {
		if doc.ID == id {
			return doc
		}
	}
	t.Fatalf("missing doc %s in %+v", id, docs)
	return specDocument{}
}

func hasEdge(edges []graphEdge, from, to string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to {
			return true
		}
	}
	return false
}

func hasGraphNode(nodes []graphNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func hasGraphNodeSpec(nodes []graphNode, id, specID string) bool {
	for _, node := range nodes {
		if node.ID == id && node.SpecID == specID {
			return true
		}
	}
	return false
}

func hasRelationship(relationships []graphRelation, from, to string) bool {
	for _, rel := range relationships {
		if rel.From == from && rel.To == to {
			return true
		}
	}
	return false
}
