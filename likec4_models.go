package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

func discoverLikeC4ModelProjects(projectRoot string) []likeC4ModelProject {
	type discoveredFile struct {
		abs string
		rel string
	}
	var sources []discoveredFile
	configs := map[string]string{}
	_ = filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipLikeC4ScanDir(d.Name()) && path != projectRoot {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".c4" || ext == ".likec4" {
			sources = append(sources, discoveredFile{abs: path, rel: rel})
			return nil
		}
		if isLikeC4ConfigName(name) {
			configs[filepath.Dir(path)] = rel
		}
		return nil
	})
	if len(sources) == 0 {
		return nil
	}

	groups := map[string][]string{}
	for _, source := range sources {
		root := nearestLikeC4ConfigRoot(filepath.Dir(source.abs), configs)
		if root == "" {
			root = projectRoot
		}
		groups[root] = append(groups[root], source.rel)
	}

	roots := make([]string, 0, len(groups))
	for root := range groups {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	projects := make([]likeC4ModelProject, 0, len(roots))
	for _, root := range roots {
		files := groups[root]
		sort.Strings(files)
		relRoot, err := filepath.Rel(projectRoot, root)
		if err != nil || relRoot == "." {
			relRoot = ""
		}
		relRoot = filepath.ToSlash(relRoot)
		name := filepath.Base(root)
		if relRoot == "" {
			name = filepath.Base(projectRoot)
		}
		projects = append(projects, likeC4ModelProject{
			ID:          "workspace:" + likeC4StableID(firstNonEmpty(relRoot, name)),
			Name:        firstNonEmpty(relRoot, name),
			Root:        root,
			SourceFiles: files,
		})
	}
	return projects
}

func shouldSkipLikeC4ScanDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".next", ".nuxt", "dist", "build", "coverage", "vendor":
		return true
	default:
		return false
	}
}

func isLikeC4ConfigName(name string) bool {
	if name == ".likec4rc" {
		return true
	}
	return strings.HasPrefix(name, "likec4.config.") || strings.HasPrefix(name, ".likec4rc.")
}

func nearestLikeC4ConfigRoot(dir string, configs map[string]string) string {
	for {
		if _, ok := configs[dir]; ok {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func buildSpecLikeC4ModelProject(project specProject) likeC4ModelProject {
	source := buildSpecLikeC4Source(project)
	if source == "" {
		return likeC4ModelProject{}
	}
	return likeC4ModelProject{
		ID:          "generated:specs",
		Name:        "Generated from specs",
		Root:        project.Summary.SpecsRoot,
		SourceFiles: []string{"specs/_index.md", "specs/**/*.md"},
		Generated:   true,
		Source:      source,
	}
}

func buildSpecLikeC4Source(project specProject) string {
	if len(project.Graph.Nodes) == 0 && len(project.Documents) == 0 {
		return ""
	}
	nodes := project.Graph.Nodes
	if len(nodes) == 0 {
		for _, doc := range project.Documents {
			if doc.ID == "_index.md" || doc.ID == "_sync.md" {
				continue
			}
			nodes = append(nodes, graphNode{
				ID:       moduleIDFromPath(doc.Path),
				Label:    doc.Title,
				SpecID:   doc.ID,
				Category: doc.Category,
				Status:   doc.Status,
			})
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	idByNode := uniqueLikeC4IDs(nodes)
	var b strings.Builder
	b.WriteString("specification {\n")
	b.WriteString("  element spec { style { shape component } }\n")
	b.WriteString("  relationship depends { line dashed }\n")
	b.WriteString("  relationship relates { line dotted }\n")
	b.WriteString("}\n\n")
	b.WriteString("model {\n")
	b.WriteString("  specs = spec \"Specs\" {\n")
	for _, node := range nodes {
		id := idByNode[node.ID]
		if id == "" {
			continue
		}
		title := firstNonEmpty(node.Label, node.ID, node.SpecID)
		fmt.Fprintf(&b, "    %s = spec %q", id, title)
		metadata := likeC4Metadata(map[string]string{
			"category": node.Category,
			"status":   node.Status,
			"spec":     node.SpecID,
			"id":       node.ID,
		})
		if metadata == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(" {\n")
		b.WriteString(metadata)
		b.WriteString("    }\n")
	}
	b.WriteString("  }\n")
	for _, edge := range project.Graph.Edges {
		from := idByNode[edge.From]
		to := idByNode[edge.To]
		if from == "" || to == "" {
			continue
		}
		fmt.Fprintf(&b, "  specs.%s -[depends]-> specs.%s %q\n", from, to, firstNonEmpty(edge.Label, "depends"))
	}
	for _, rel := range project.Graph.Relationships {
		from := idByNode[rel.From]
		to := idByNode[rel.To]
		if from == "" || to == "" {
			continue
		}
		fmt.Fprintf(&b, "  specs.%s -[relates]-> specs.%s %q\n", from, to, firstNonEmpty(rel.Description, rel.Section, "relates"))
	}
	b.WriteString("}\n\n")
	b.WriteString("views {\n")
	b.WriteString("  view specs-overview of specs {\n")
	b.WriteString("    title \"Specs LikeC4 Model\"\n")
	b.WriteString("    include *\n")
	b.WriteString("    include * -> *\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

func uniqueLikeC4IDs(nodes []graphNode) map[string]string {
	out := map[string]string{}
	used := map[string]int{}
	for _, node := range nodes {
		key := firstNonEmpty(node.ID, node.SpecID, node.Label)
		if key == "" {
			continue
		}
		id := likeC4Identifier(key)
		count := used[id]
		used[id] = count + 1
		if count > 0 {
			id = fmt.Sprintf("%s-%d", id, count+1)
		}
		out[node.ID] = id
	}
	return out
}

func likeC4Identifier(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ".md")
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-_")
	if value == "" {
		value = "node"
	}
	first, _ := utf8FirstRune(value)
	if unicode.IsDigit(first) {
		value = "n-" + value
	}
	return value
}

func utf8FirstRune(value string) (rune, bool) {
	for _, r := range value {
		return r, true
	}
	return 0, false
}

func likeC4StableID(value string) string {
	hash := sha1.Sum([]byte(value))
	return likeC4Identifier(value) + "-" + hex.EncodeToString(hash[:4])
}

func likeC4Metadata(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("      metadata {\n")
	for _, key := range keys {
		fmt.Fprintf(&b, "        %s %q\n", likeC4Identifier(key), values[key])
	}
	b.WriteString("      }\n")
	return b.String()
}
