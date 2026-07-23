package agentsync

import (
	"encoding/json"
	"sort"
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

// removeManagedBlock strips the begin..end managed block (and surrounding
// blank lines) from current. When no block is present the input is returned
// unchanged.
func removeManagedBlock(current, begin, end string) string {
	start := strings.Index(current, begin)
	if start < 0 {
		return current
	}
	stop := strings.Index(current[start:], end)
	if stop < 0 {
		return current
	}
	stop = start + stop + len(end)
	next := strings.TrimRight(current[:start], "\n")
	rest := strings.TrimLeft(current[stop:], "\n")
	if next == "" {
		return rest
	}
	if rest == "" {
		return next + "\n"
	}
	return next + "\n\n" + rest
}

// extractManagedBlockMCPNames returns every mcp_servers.<name> table header
// found inside the begin..end managed block. Used so a catalog shrink (portal
// disable) can purge orphan tables that previously lived only in the block
// and any duplicates outside it.
func extractManagedBlockMCPNames(current, begin, end string) []string {
	start := strings.Index(current, begin)
	if start < 0 {
		return nil
	}
	stop := strings.Index(current[start:], end)
	if stop < 0 {
		return nil
	}
	section := current[start : start+stop]
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") {
			continue
		}
		name, ok := parseTOMLTableName(trimmed, "mcp_servers")
		if !ok || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// extractNonMCPFromManagedBlock returns TOML content trapped between the
// managed markers that is not part of [mcp_servers] / [mcp_servers.<name>]
// tables. Older writes sometimes left the end marker after user tables
// (e.g. Codex [projects.*]); rewriting the block must not delete them.
func extractNonMCPFromManagedBlock(current, begin, end string) string {
	start := strings.Index(current, begin)
	if start < 0 {
		return ""
	}
	stop := strings.Index(current[start:], end)
	if stop < 0 {
		return ""
	}
	section := current[start+len(begin) : start+stop]
	var kept []string
	skipMCP := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if skipMCP {
			if strings.HasPrefix(trimmed, "[") {
				skipMCP = false
			} else {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "[") {
			if trimmed == "[mcp_servers]" || strings.HasPrefix(trimmed, "[mcp_servers.") {
				skipMCP = true
				continue
			}
		}
		kept = append(kept, line)
	}
	// Drop leading/trailing blank lines so re-injection stays tidy.
	for len(kept) > 0 && strings.TrimSpace(kept[0]) == "" {
		kept = kept[1:]
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n")
}

// injectAfterManagedBlock inserts trapped (user) content immediately after
// the managed end marker. If the marker is missing, content is appended.
func injectAfterManagedBlock(doc, end, trapped string) string {
	trapped = strings.TrimSpace(trapped)
	if trapped == "" {
		return doc
	}
	idx := strings.Index(doc, end)
	if idx < 0 {
		if strings.TrimSpace(doc) == "" {
			return trapped + "\n"
		}
		return strings.TrimRight(doc, "\n") + "\n\n" + trapped + "\n"
	}
	at := idx + len(end)
	head := strings.TrimRight(doc[:at], "\n")
	tail := strings.TrimLeft(doc[at:], "\n")
	if tail == "" {
		return head + "\n\n" + trapped + "\n"
	}
	return head + "\n\n" + trapped + "\n\n" + tail
}

// uniqueStrings returns sorted unique non-empty strings from parts.
func uniqueStrings(parts ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range parts {
		for _, s := range part {
			s = strings.TrimSpace(s)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
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
