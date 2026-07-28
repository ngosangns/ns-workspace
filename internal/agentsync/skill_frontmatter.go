package agentsync

import (
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SKILL.md carries a YAML frontmatter block that agent runtimes parse
// strictly. Kiro surfaces a user-visible "Invalid SKILL.md frontmatter"
// warning and drops the skill; other runtimes fail just as hard but more
// quietly.
//
// The most common authoring mistake is a plain (unquoted) scalar that
// contains ": ", which YAML reads as a nested mapping key:
//
//	description: Sửa bug. Trigger: fix lỗi, debug.
//
// normalizeSkillFrontmatter repairs that class of error on the way into
// the shared skills home so every provider mirror gets parseable YAML.
// It is deliberately conservative: content that already parses is
// returned byte-for-byte, and a repair attempt that still does not parse
// is discarded rather than written.

// skillFrontmatterKeyLine matches a top-level `key: value` line inside a
// frontmatter block (no leading indentation, plain identifier key).
var skillFrontmatterKeyLine = regexp.MustCompile(`^([A-Za-z0-9_.-]+):[ \t]*(.*)$`)

// isSkillDoc reports whether relSlash (a forward-slash path relative to
// the skills tree root) is a skill's frontmatter-bearing entry point.
func isSkillDoc(relSlash string) bool {
	return path.Base(relSlash) == "SKILL.md"
}

// normalizeSkillFrontmatter returns data with an unparseable frontmatter
// block repaired by quoting offending plain scalars. The second result
// reports whether anything changed.
//
// Returns (data, false) when there is no frontmatter, when the existing
// frontmatter already parses, or when the repair does not produce valid
// YAML — in every one of those cases the caller must write the original
// bytes untouched.
func normalizeSkillFrontmatter(data []byte) ([]byte, bool) {
	start, body, end, ok := splitFrontmatter(string(data))
	if !ok {
		return data, false
	}
	if yamlParsesAsMapping(body) {
		return data, false
	}
	repaired, changed := quotePlainScalars(body)
	if !changed || !yamlParsesAsMapping(repaired) {
		return data, false
	}
	text := string(data)
	return []byte(text[:start] + repaired + text[end:]), true
}

// splitFrontmatter locates the YAML block delimited by the leading `---`
// line and the next `---` line. It returns the byte offsets of the block
// body plus the body itself. ok is false when the document does not open
// with a frontmatter fence or the closing fence is missing.
func splitFrontmatter(text string) (start int, body string, end int, ok bool) {
	const fence = "---"
	if !strings.HasPrefix(text, fence+"\n") && !strings.HasPrefix(text, fence+"\r\n") {
		return 0, "", 0, false
	}
	openEnd := strings.Index(text, "\n") + 1
	rest := text[openEnd:]
	// Closing fence must be a line of its own.
	offset := 0
	for {
		idx := strings.Index(rest[offset:], fence)
		if idx < 0 {
			return 0, "", 0, false
		}
		abs := offset + idx
		atLineStart := abs == 0 || rest[abs-1] == '\n'
		after := abs + len(fence)
		atLineEnd := after == len(rest) || rest[after] == '\n' || rest[after] == '\r'
		if atLineStart && atLineEnd {
			return openEnd, rest[:abs], openEnd + abs, true
		}
		offset = abs + len(fence)
	}
}

// yamlParsesAsMapping reports whether body decodes into a YAML mapping.
// A frontmatter block that decodes to a scalar or sequence is treated as
// invalid because every consumer expects keys.
func yamlParsesAsMapping(body string) bool {
	var out map[string]any
	if err := yaml.Unmarshal([]byte(body), &out); err != nil {
		return false
	}
	return out != nil
}

// quotePlainScalars double-quotes top-level plain scalar values so that
// characters with YAML meaning (notably ": ") stop being interpreted.
// Values that are already quoted, or that open a nested/flow/block node,
// are left alone.
func quotePlainScalars(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	changed := false
	for i, line := range lines {
		carriage := strings.HasSuffix(line, "\r")
		trimmed := strings.TrimSuffix(line, "\r")
		m := skillFrontmatterKeyLine.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		key, value := m[1], m[2]
		if !needsQuoting(value) {
			continue
		}
		next := key + ": " + quoteYAMLDouble(value)
		if carriage {
			next += "\r"
		}
		lines[i] = next
		changed = true
	}
	if !changed {
		return body, false
	}
	return strings.Join(lines, "\n"), true
}

// needsQuoting reports whether a plain scalar value must be quoted to
// survive YAML parsing. Empty values (nested mapping follows), already
// quoted values and non-scalar openers are excluded.
func needsQuoting(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	switch value[0] {
	case '"', '\'', '[', '{', '|', '>', '&', '*', '!':
		return false
	}
	// A plain scalar breaks on ": " (mapping value) and " #" (comment).
	return strings.Contains(value, ": ") || strings.HasSuffix(value, ":") || strings.Contains(value, " #")
}

// quoteYAMLDouble wraps value in a YAML double-quoted scalar, escaping
// the only two characters that are special inside one.
func quoteYAMLDouble(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
