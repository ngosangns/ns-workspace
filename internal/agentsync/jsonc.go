package agentsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StripJSONC removes // line comments and /* */ block comments from JSONC
// input while preserving string literals. The result is standard JSON
// suitable for encoding/json.Unmarshal.
func StripJSONC(src []byte) []byte {
	if len(src) == 0 {
		return src
	}
	var out bytes.Buffer
	out.Grow(len(src))
	i := 0
	inString := false
	escaped := false
	for i < len(src) {
		c := src[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			i++
			continue
		}
		if c == '"' {
			inString = true
			out.WriteByte(c)
			i++
			continue
		}
		if c == '/' && i+1 < len(src) {
			next := src[i+1]
			if next == '/' {
				// Line comment: skip until newline (keep newline).
				i += 2
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			}
			if next == '*' {
				// Block comment: skip until */
				i += 2
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 < len(src) {
					i += 2
				}
				continue
			}
		}
		out.WriteByte(c)
		i++
	}
	return out.Bytes()
}

// UnmarshalJSONC unmarshals JSONC (JSON with comments) into v.
// Trailing commas left after comment removal are stripped.
func UnmarshalJSONC(data []byte, v any) error {
	return json.Unmarshal(stripTrailingCommas(StripJSONC(data)), v)
}

// CommentedMap holds active (enabled) object properties and disabled ones
// recovered from // comments in a JSONC object body.
type CommentedMap struct {
	Enabled  map[string]any
	Disabled map[string]any
	// Order lists enabled keys in file appearance order when available.
	Order []string
}

// ParseCommentedObjectMap parses a JSONC document whose top-level is an
// object, splitting active properties from properties that appear only as
// // comments (disabled entries preserved for re-enable).
//
// Example input:
//
//	{
//	  "a": 1,
//	  // "b": 2
//	}
//
// yields Enabled{"a":1} and Disabled{"b":2}.
func ParseCommentedObjectMap(data []byte) (CommentedMap, error) {
	var active map[string]any
	if err := UnmarshalJSONC(data, &active); err != nil {
		return CommentedMap{}, fmt.Errorf("parse jsonc object: %w", err)
	}
	if active == nil {
		active = map[string]any{}
	}
	disabled := extractCommentedProperties(string(data))
	// Active keys win if somehow present in both.
	for k := range active {
		delete(disabled, k)
	}
	order := objectKeyOrder(string(data), active)
	return CommentedMap{Enabled: active, Disabled: disabled, Order: order}, nil
}

// FormatCommentedObjectMap renders a JSONC object with enabled keys as live
// properties and disabled keys as // commented blocks. Keys are sorted
// unless order is provided for enabled keys.
func FormatCommentedObjectMap(enabled, disabled map[string]any, order []string) ([]byte, error) {
	if enabled == nil {
		enabled = map[string]any{}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	keys := orderedKeys(enabled, order)
	var b strings.Builder
	b.WriteString("{\n")
	for i, key := range keys {
		raw, err := json.MarshalIndent(enabled[key], "  ", "  ")
		if err != nil {
			return nil, err
		}
		// indent already includes 2 spaces on first line of multi-line values
		// but MarshalIndent with prefix "  " puts prefix on subsequent lines only.
		// For single-line: "  \"key\": value"
		b.WriteString("  ")
		b.WriteString(fmt.Sprintf("%q", key))
		b.WriteString(": ")
		// raw starts with value; multi-line values start with { or [ on same line
		// after ": ". For multi-line indent, re-marshal with prefix.
		raw = bytes.TrimSpace(raw)
		if bytes.Contains(raw, []byte("\n")) {
			// re-indent body under 2 spaces for key line
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, raw, "  ", "  "); err != nil {
				return nil, err
			}
			b.Write(pretty.Bytes())
		} else {
			b.Write(raw)
		}
		if i < len(keys)-1 || len(disabled) > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	if len(disabled) > 0 {
		if len(keys) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  // disabled by portal (re-enable from UI)\n")
		dkeys := make([]string, 0, len(disabled))
		for k := range disabled {
			dkeys = append(dkeys, k)
		}
		sort.Strings(dkeys)
		for i, key := range dkeys {
			raw, err := json.MarshalIndent(disabled[key], "", "  ")
			if err != nil {
				return nil, err
			}
			// Format as: // "key": <value with each line // prefixed>
			entry := fmt.Sprintf("%q: %s", key, string(raw))
			if i < len(dkeys)-1 {
				entry += ","
			}
			for _, line := range strings.Split(entry, "\n") {
				b.WriteString("  // ")
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

// MCPDisabledPath is the overlay path for portal-disabled MCP servers.
// Shape matches the enabled file: { "mcpServers": { "name": { ... } } }.
const MCPDisabledPath = "presets/mcp/servers.disabled.json"

// MCPEnabledPath is the enabled MCP servers preset path.
const MCPEnabledPath = "presets/mcp/servers.json"

// FormatMCPServersJSON writes pure JSON for the enabled MCP servers file.
func FormatMCPServersJSON(enabled map[string]any, order []string) ([]byte, error) {
	if enabled == nil {
		enabled = map[string]any{}
	}
	// Preserve key order when provided by building via ordered marshal.
	keys := orderedKeys(enabled, order)
	ordered := make(map[string]any, len(keys))
	for _, k := range keys {
		ordered[k] = enabled[k]
	}
	// Re-insert any keys missing from orderedKeys (should not happen).
	for k, v := range enabled {
		if _, ok := ordered[k]; !ok {
			ordered[k] = v
		}
	}
	return encodeJSONIndent(MCPManifest{MCPServers: ordered})
}

// FormatMCPDisabledJSON writes pure JSON for MCPDisabledPath.
func FormatMCPDisabledJSON(disabled map[string]any) ([]byte, error) {
	if disabled == nil {
		disabled = map[string]any{}
	}
	return encodeJSONIndent(MCPManifest{MCPServers: disabled})
}

// ParseMCPDisabledJSON parses servers.disabled.json.
func ParseMCPDisabledJSON(data []byte) (map[string]any, error) {
	var wrap struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := UnmarshalJSONC(data, &wrap); err != nil {
		return nil, fmt.Errorf("parse mcp disabled json: %w", err)
	}
	if wrap.MCPServers == nil {
		wrap.MCPServers = map[string]any{}
	}
	return wrap.MCPServers, nil
}

// FormatMCPServersJSONC writes the shared MCP servers manifest as JSONC with
// disabled servers preserved as // comments under mcpServers.
// Deprecated for new writes: prefer FormatMCPServersJSON + FormatMCPDisabledJSON.
// Still used when migrating legacy overlays that only exist as JSONC comments.
func FormatMCPServersJSONC(enabled, disabled map[string]any, order []string) ([]byte, error) {
	if enabled == nil {
		enabled = map[string]any{}
	}
	if disabled == nil {
		disabled = map[string]any{}
	}
	keys := orderedKeys(enabled, order)
	var b strings.Builder
	b.WriteString("{\n  \"mcpServers\": {\n")
	for i, key := range keys {
		raw, err := json.MarshalIndent(enabled[key], "    ", "  ")
		if err != nil {
			return nil, err
		}
		b.WriteString("    ")
		b.WriteString(fmt.Sprintf("%q", key))
		b.WriteString(": ")
		raw = bytes.TrimSpace(raw)
		if bytes.Contains(raw, []byte("\n")) {
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, raw, "    ", "  "); err != nil {
				return nil, err
			}
			b.Write(pretty.Bytes())
		} else {
			b.Write(raw)
		}
		if i < len(keys)-1 || len(disabled) > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	if len(disabled) > 0 {
		if len(keys) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("    // disabled by portal (re-enable from UI)\n")
		dkeys := make([]string, 0, len(disabled))
		for k := range disabled {
			dkeys = append(dkeys, k)
		}
		sort.Strings(dkeys)
		for i, key := range dkeys {
			raw, err := json.MarshalIndent(disabled[key], "", "  ")
			if err != nil {
				return nil, err
			}
			entry := fmt.Sprintf("%q: %s", key, string(raw))
			// Trailing comma between commented properties (except last) so
			// re-parse as JSONC object body is unambiguous.
			if i < len(dkeys)-1 {
				entry += ","
			}
			for _, line := range strings.Split(entry, "\n") {
				b.WriteString("    // ")
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("  }\n}\n")
	return []byte(b.String()), nil
}

// ParseMCPServersJSONC parses mcp/servers.json allowing // comments and
// recovering disabled servers from commented properties inside mcpServers.
func ParseMCPServersJSONC(data []byte) (enabled, disabled map[string]any, order []string, err error) {
	// First parse active shape.
	var wrap struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := UnmarshalJSONC(data, &wrap); err != nil {
		return nil, nil, nil, fmt.Errorf("parse mcp servers jsonc: %w", err)
	}
	if wrap.MCPServers == nil {
		wrap.MCPServers = map[string]any{}
	}
	// Extract commented properties from the mcpServers object body.
	body, ok := extractObjectBody(string(data), "mcpServers")
	if !ok {
		// Fallback: whole-file comment extract on a synthetic object.
		cm, err := ParseCommentedObjectMap(data)
		if err != nil {
			return wrap.MCPServers, map[string]any{}, orderedKeys(wrap.MCPServers, nil), nil
		}
		// If top-level was full servers map without wrapper.
		if _, has := cm.Enabled["mcpServers"]; !has && len(cm.Enabled) > 0 {
			for k := range wrap.MCPServers {
				delete(cm.Disabled, k)
			}
			return wrap.MCPServers, cm.Disabled, orderedKeys(wrap.MCPServers, nil), nil
		}
		return wrap.MCPServers, map[string]any{}, orderedKeys(wrap.MCPServers, nil), nil
	}
	disabled = extractCommentedProperties(body)
	for k := range wrap.MCPServers {
		delete(disabled, k)
	}
	order = objectKeyOrder(body, wrap.MCPServers)
	return wrap.MCPServers, disabled, order, nil
}

func orderedKeys(m map[string]any, order []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(m))
	for _, k := range order {
		if _, ok := m[k]; ok && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	rest := make([]string, 0, len(m))
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// objectKeyOrder returns keys of active in the order they appear as
// un-commented properties in src.
func objectKeyOrder(src string, active map[string]any) []string {
	if len(active) == 0 {
		return nil
	}
	order := make([]string, 0, len(active))
	seen := map[string]bool{}
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "//") {
			continue
		}
		key, ok := leadingJSONKey(trim)
		if !ok {
			continue
		}
		if _, exists := active[key]; exists && !seen[key] {
			order = append(order, key)
			seen[key] = true
		}
	}
	return order
}

func leadingJSONKey(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, `"`) {
		return "", false
	}
	// Parse JSON string key at start.
	var key string
	dec := json.NewDecoder(strings.NewReader(line))
	if err := dec.Decode(&key); err != nil {
		return "", false
	}
	rest := strings.TrimSpace(line[len(mustJSONString(key)):])
	// after quoted key we expect :
	if !strings.HasPrefix(rest, ":") {
		// mustJSONString length might not match if escapes differ; scan manually
		i := 1
		for i < len(line) {
			if line[i] == '\\' {
				i += 2
				continue
			}
			if line[i] == '"' {
				i++
				break
			}
			i++
		}
		rest = strings.TrimSpace(line[i:])
		if !strings.HasPrefix(rest, ":") {
			return "", false
		}
	}
	return key, true
}

func mustJSONString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// extractObjectBody returns the raw text inside the {...} value of the
// given top-level or nested property name (first match of `"name": {`).
func extractObjectBody(src, name string) (string, bool) {
	needle := fmt.Sprintf("%q", name)
	idx := strings.Index(src, needle)
	if idx < 0 {
		return "", false
	}
	rest := src[idx+len(needle):]
	// skip whitespace and :
	j := 0
	for j < len(rest) && unicode.IsSpace(rune(rest[j])) {
		j++
	}
	if j >= len(rest) || rest[j] != ':' {
		return "", false
	}
	j++
	for j < len(rest) && unicode.IsSpace(rune(rest[j])) {
		j++
	}
	if j >= len(rest) || rest[j] != '{' {
		return "", false
	}
	// find matching }
	depth := 0
	inString := false
	escaped := false
	start := j
	for j < len(rest) {
		c := rest[j]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			j++
			continue
		}
		if c == '"' {
			inString = true
			j++
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return rest[start : j+1], true
			}
		}
		j++
	}
	return "", false
}

// extractCommentedProperties reconstructs object properties that appear only
// as consecutive // comment lines. Comment lines are stripped of leading //
// and joined; commas are inserted between top-level properties (commented
// blocks are written without trailing commas between entries).
func extractCommentedProperties(src string) map[string]any {
	out := map[string]any{}
	var block []string
	flush := func() {
		if len(block) == 0 {
			return
		}
		joined := strings.Join(block, "\n")
		// Insert missing commas between top-level "key": value members so
		// multiple consecutive commented properties parse as one object.
		joined = insertCommasBetweenObjectProperties(joined)
		body := "{\n" + joined + "\n}"
		cleaned := stripTrailingCommas(StripJSONC([]byte(body)))
		var m map[string]any
		if err := json.Unmarshal(cleaned, &m); err == nil {
			for k, v := range m {
				out[k] = v
			}
		}
		block = block[:0]
	}
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "//") {
			content := strings.TrimSpace(strings.TrimPrefix(trim, "//"))
			// Skip pure marker comments
			if content == "" || strings.HasPrefix(content, "disabled by portal") {
				continue
			}
			block = append(block, content)
			continue
		}
		// Non-comment breaks the block
		if trim != "" {
			flush()
		}
	}
	flush()
	return out
}

// insertCommasBetweenObjectProperties inserts commas between adjacent
// top-level members of an object body that was reconstructed from //
// comments (where writers omit trailing commas after each property).
//
// Example input (missing commas):
//
//	"a": { "x": 1 }
//	"b": { "y": 2 }
//
// becomes:
//
//	"a": { "x": 1 },
//	"b": { "y": 2 }
func insertCommasBetweenObjectProperties(src string) string {
	var out strings.Builder
	out.Grow(len(src) + 8)
	depth := 0 // brace/bracket depth; top-level props sit at depth 0
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			// If we are at depth 0 and the previous non-space output was a
			// finished value (not ':' and not ',' and not start), insert comma.
			if depth == 0 && needsCommaBeforeKey(out.String()) {
				out.WriteByte(',')
			}
			inString = true
			out.WriteByte(c)
		case '{', '[':
			depth++
			out.WriteByte(c)
		case '}', ']':
			depth--
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// needsCommaBeforeKey reports whether the object-body text so far ends with a
// complete property value (so a following "key" needs a separating comma).
func needsCommaBeforeKey(soFar string) bool {
	// Trim trailing whitespace
	i := len(soFar) - 1
	for i >= 0 && (soFar[i] == ' ' || soFar[i] == '\t' || soFar[i] == '\n' || soFar[i] == '\r') {
		i--
	}
	if i < 0 {
		return false
	}
	c := soFar[i]
	// Already has comma, or expecting a value after ':', or empty.
	if c == ',' || c == ':' || c == '{' || c == '[' {
		return false
	}
	// End of object/array/string/number/keyword → next key needs comma.
	if c == '}' || c == ']' || c == '"' {
		return true
	}
	// true/false/null or number
	if c == 'e' || c == 'l' || (c >= '0' && c <= '9') {
		return true
	}
	return false
}

// stripTrailingCommas removes trailing commas before } or ] outside strings.
func stripTrailingCommas(src []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(src))
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out.WriteByte(c)
			continue
		}
		if c == ',' {
			// Look ahead for next non-space; if } or ], skip comma.
			j := i + 1
			for j < len(src) {
				r, size := utf8.DecodeRune(src[j:])
				if unicode.IsSpace(r) {
					j += size
					continue
				}
				break
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}
