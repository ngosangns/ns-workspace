package agentsync

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// userHomeDir is the package-internal seam for tests. External test
// packages should mutate UserHomeDirForTest (same value), so this
// variable is just an alias.
var userHomeDir = func() (string, error) { return UserHomeDirForTest() }

// UserHomeDirForTest exposes userHomeDir to external test packages
// (e.g. internal/cli) so they can simulate home-resolution failures
// without modifying private state.
var UserHomeDirForTest = os.UserHomeDir

// expandPathImpl is the seam test used by ExpandPath so tests can
// simulate unusual home states (e.g. userHomeDir returning ("", nil)).
var expandPathImpl = func(path string) string {
	if path == "~" {
		home, _ := userHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := userHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// DefaultAgentsDir resolves the shared ~/.agents/ directory, honoring
// AGENTS_HOME if set. Used by CLI bootstrap to fill Options.AgentsDir.
func DefaultAgentsDir() (string, error) {
	if env := os.Getenv("AGENTS_HOME"); env != "" {
		return ExpandPath(env), nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents"), nil
}

// ExpandPath converts a leading "~" or "~/" to the user home directory.
// Other paths are returned untouched.
func ExpandPath(path string) string {
	return expandPathImpl(path)
}

// ParseTools parses the comma-separated --tools value into a lookup
// map keyed by lowercase adapter id or alias.
func ParseTools(value string) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" {
			out[item] = true
		}
	}
	return out
}

// shellWord quotes a value for safe inclusion in shell scripts. Strings
// that consist solely of safe characters are returned verbatim; the
// empty string and anything else is wrapped in single quotes with the
// classic '\'' escape.
func shellWord(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
	}) == -1 {
		return value
	}
	return "'" + shellSingleQuotePayload(value) + "'"
}

// shellSingleQuotePayload escapes a value for embedding inside single
// quotes using the POSIX shell idiom: end the quote, insert a literal
// single quote, then reopen the quote.
func shellSingleQuotePayload(value string) string {
	return strings.ReplaceAll(value, "'", `'"'"'`)
}

// sameLink reports whether path is an existing symlink whose target is
// want. Used by linkOrCopy to skip redundant work.
func sameLink(path, want string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	return target == want
}

// compact removes empty and duplicate entries from values and returns
// the result sorted. Status paths from multiple sources often repeat
// the same native file (instruction + subagents sharing ~/.claude/...);
// this dedupes them before printing.
func compact(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
