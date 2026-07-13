package agentsync

import (
	"encoding/json"
	"strings"
)

// mergeShallow merges overlay on top of base with one-level
// precedence: keys present in overlay overwrite keys in base; neither
// recurses.
func mergeShallow(base, overlay map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

// mergeDeep merges overlay on top of base with recursive precedence:
// nested map[string]any values are merged recursively, all other values
// are replaced.
func mergeDeep(base, overlay map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if existing, ok := out[k].(map[string]any); ok {
			if overMap, ok2 := v.(map[string]any); ok2 {
				out[k] = mergeDeep(existing, overMap)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// asMap coerces v into a map[string]any suitable for mergeShallow /
// mergeDeep. nil becomes an empty map; non-map types become empty (so
// the caller does not silently reuse a wrong-typed value).
func asMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// mergeJSONAt walks keyPath inside obj, creating intermediate maps as
// needed, then shallow-merges values into the leaf map. Existing leaf
// keys with the same name are replaced.
func mergeJSONAt(obj map[string]any, keyPath []string, values map[string]any) {
	cursor := obj
	for _, key := range keyPath {
		next, _ := cursor[key].(map[string]any)
		if next == nil {
			next = map[string]any{}
			cursor[key] = next
		}
		cursor = next
	}
	for name, value := range values {
		cursor[name] = value
	}
}

// replaceJSONAt replaces the map at keyPath inside obj with a fresh map
// containing values. An empty keyPath wipes obj's top-level keys and
// copies values directly.
func replaceJSONAt(obj map[string]any, keyPath []string, values map[string]any) {
	if len(keyPath) == 0 {
		for name := range obj {
			delete(obj, name)
		}
		for name, value := range values {
			obj[name] = value
		}
		return
	}
	cursor := obj
	for _, key := range keyPath[:len(keyPath)-1] {
		next, _ := cursor[key].(map[string]any)
		if next == nil {
			next = map[string]any{}
			cursor[key] = next
		}
		cursor = next
	}
	leaf := map[string]any{}
	for name, value := range values {
		leaf[name] = value
	}
	cursor[keyPath[len(keyPath)-1]] = leaf
}

// replaceManagedBlock returns current with the begin..end managed block
// replaced by block. If no block is present, block is appended with one
// blank line separator (or returned alone when current is empty).
//
// This is the idiomatic format used by AppendManagedBlock to write
// `codex mcp` managed blocks without clobbering user
// content outside the markers.
func replaceManagedBlock(current, begin, end, block string) string {
	start := strings.Index(current, begin)
	if start >= 0 {
		stop := strings.Index(current[start:], end)
		if stop >= 0 {
			stop = start + stop + len(end)
			next := strings.TrimRight(current[:start], "\n") + "\n" + block + strings.TrimLeft(current[stop:], "\n")
			return strings.TrimLeft(next, "\n")
		}
	}
	if strings.TrimSpace(current) == "" {
		return block
	}
	return strings.TrimRight(current, "\n") + "\n\n" + block
}

// removeTOMLTables strips every [prefix.<name>] table section (and its
// body) from a TOML document for each name in names. Names may be written
// as bare keys or quoted strings in the original document. The function
// stops removing lines at the next table/header line (`[`) or managed
// block marker (`# >>>` / `# <<<`). It is intentionally conservative and
// only removes exact table-header matches, preserving user config outside
// those tables.
func removeTOMLTables(content, prefix string, names []string) string {
	if len(names) == 0 {
		return content
	}
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	// sectionHeaderLine matches lines like:
	//   [mcp_servers.foo]
	//   [mcp_servers."foo"]
	//   [mcp_servers.'foo']
	// capturing the unquoted name.
	lines := strings.Split(content, "\n")
	var out []string
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if skip {
			// End the current skipped section when we hit another table or
			// a managed block marker.
			if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "# >>>") || strings.HasPrefix(trimmed, "# <<<") {
				skip = false
			} else {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "[") {
			name, ok := parseTOMLTableName(trimmed, prefix)
			if ok {
				if _, match := nameSet[name]; match {
					skip = true
					continue
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// parseTOMLTableName parses a TOML table header line of the form
// [prefix.<name>] or [prefix."<name>"] and returns the unquoted name.
// It returns ("", false) if the line does not match the expected prefix.
func parseTOMLTableName(line, prefix string) (string, bool) {
	// Strip surrounding brackets.
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	inner := strings.TrimSpace(line[1 : len(line)-1])
	expected := prefix + "."
	if !strings.HasPrefix(inner, expected) {
		return "", false
	}
	name := strings.TrimSpace(inner[len(expected):])
	// Unquote if necessary.
	if len(name) >= 2 {
		if (name[0] == '"' && name[len(name)-1] == '"') || (name[0] == '\'' && name[len(name)-1] == '\'') {
			return name[1 : len(name)-1], true
		}
	}
	return name, true
}

// encodeJSONInline renders v as compact JSON suitable for embedding in a
// shell single-quoted argument. Returns an error if v fails to marshal.
func encodeJSONInline(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// encodeJSONIndent renders v as 2-space indented JSON with a trailing
// newline. Used for human-readable registry manifest and the various
// preset files.
func encodeJSONIndent(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}

// decodeJSON parses raw JSON bytes into out. Thin wrapper to keep the
// adapter concrete classes from importing encoding/json directly.
func decodeJSON(data []byte, out *map[string]any) error {
	return json.Unmarshal(data, out)
}
