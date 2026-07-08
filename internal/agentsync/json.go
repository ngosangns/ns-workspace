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
