package agentsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UserConfig maps a preset path (relative to the project root, e.g.
// "presets/opencode/opencode.json") to an absolute filesystem path that
// holds the user-supplied replacement content.
//
// The JSON shape is a flat object:
//
//	{
//	  "presets/agents/AGENTS.md": "/home/me/.config/ns-workspace/AGENTS.md",
//	  "presets/opencode/opencode.json": "/home/me/.config/ns-workspace/opencode.json",
//	  "presets/skills/minimax-cli/SKILL.md": "/home/me/.config/ns-workspace/minimax-cli.md"
//	}
//
// Keys MUST start with "presets/" and use forward slashes. Values MUST be
// absolute paths to regular files. Paths that do not match an existing
// embedded preset are still allowed and behave as additions (e.g. a brand
// new skill under presets/skills/).
type UserConfig struct {
	// entries is the raw map loaded from disk. All keys are normalized to
	// forward-slash form and start with "presets/".
	entries map[string]string
	// origin is the file the config was loaded from. Empty when no user
	// config is active. Tests assert on this to keep loadUserConfig()
	// observable.
	origin string
}

// IsZero reports whether the user config carries no overlay entries.
// Callers can safely invoke methods on a zero UserConfig.
func (u UserConfig) IsZero() bool { return len(u.entries) == 0 }

// HasOverlay reports whether the user config is non-empty. Equivalent to
// !IsZero(); provided for readability at call sites.
func (u UserConfig) HasOverlay() bool { return !u.IsZero() }

// Origin returns the path the user config was loaded from. Empty when
// the user did not supply a config file.
func (u UserConfig) Origin() string { return u.origin }

// Lookup returns the user file path mapped to the given preset path and
// whether the entry exists. embeddedPath is normalized to forward slashes
// and must start with "presets/".
func (u UserConfig) Lookup(embeddedPath string) (string, bool) {
	if u.IsZero() {
		return "", false
	}
	key := normalizePresetKey(embeddedPath)
	value, ok := u.entries[key]
	return value, ok
}

// Entries returns a copy of the raw user entries. Callers MUST NOT mutate
// the returned map.
func (u UserConfig) Entries() map[string]string {
	if u.IsZero() {
		return nil
	}
	out := make(map[string]string, len(u.entries))
	for k, v := range u.entries {
		out[k] = v
	}
	return out
}

// EntriesUnder returns the subset of entries that live under a tree root
// such as "presets/skills" or "presets/subagents". Returned names are
// relative to the tree root and use forward slashes.
func (u UserConfig) EntriesUnder(treeRoot string) []string {
	if u.IsZero() {
		return nil
	}
	root := normalizePresetKey(treeRoot)
	rootWithSlash := root
	if !strings.HasSuffix(rootWithSlash, "/") {
		rootWithSlash += "/"
	}
	out := []string{}
	for key := range u.entries {
		if !strings.HasPrefix(key, rootWithSlash) {
			continue
		}
		out = append(out, strings.TrimPrefix(key, rootWithSlash))
	}
	sortStrings(out)
	return out
}

// userConfigDir is the package-internal seam for tests. External test
// packages should mutate UserConfigDirForTest (same value), so this
// variable is just an alias.
var userConfigDir = func() (string, error) { return UserConfigDirForTest() }

// UserConfigDirForTest exposes userConfigDir to external test packages
// (e.g. internal/cli) so they can simulate config-dir failures.
var UserConfigDirForTest = os.UserConfigDir

// DefaultUserConfigPath returns the conventional location for a user
// config file when no --config flag is supplied. It honors XDG_CONFIG_HOME
// and falls back to $HOME/.config on POSIX, %AppData% on Windows.
func DefaultUserConfigPath() (string, error) {
	if env := strings.TrimSpace(os.Getenv("NS_WORKSPACE_CONFIG")); env != "" {
		return ExpandPath(env), nil
	}
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ns-workspace", "config.json"), nil
}

// loadUserConfig resolves the effective user config and loads it. The
// resolution order is:
//  1. opt.ConfigPath if non-empty
//  2. DefaultUserConfigPath() if it exists
//
// Returns a zero UserConfig (no error) when no config file is present so
// callers can treat "no overlay" as the default state.
func loadUserConfig(opt Options) (UserConfig, error) {
	candidates := []string{}
	if p := strings.TrimSpace(opt.ConfigPath); p != "" {
		candidates = append(candidates, ExpandPath(p))
	}
	defaultPath, err := DefaultUserConfigPath()
	if err != nil {
		return UserConfig{}, err
	}
	candidates = append(candidates, defaultPath)

	for _, path := range candidates {
		if path == "" {
			continue
		}
		cfg, err := readUserConfigFile(path)
		if err != nil {
			return UserConfig{}, err
		}
		if !cfg.IsZero() {
			return *cfg, nil
		}
	}
	return UserConfig{}, nil
}

// readUserConfigFile returns a zero UserConfig when the file does not
// exist so callers can fall through to the next candidate.
func readUserConfigFile(path string) (*UserConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &UserConfig{}, nil
		}
		return nil, fmt.Errorf("read user config %s: %w", path, err)
	}
	raw := map[string]string{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse user config %s: %w", path, err)
	}
	entries := make(map[string]string, len(raw))
	for key, value := range raw {
		normalized := normalizePresetKey(key)
		if !strings.HasPrefix(normalized, "presets/") {
			return nil, fmt.Errorf("user config %s: key %q must start with \"presets/\"", path, key)
		}
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("user config %s: key %q has empty value", path, key)
		}
		abs := ExpandPath(value)
		if !filepath.IsAbs(abs) {
			return nil, fmt.Errorf("user config %s: key %q value %q must be an absolute path", path, key, value)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("user config %s: key %q source %s: %w", path, key, abs, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("user config %s: key %q source %s is a directory, expected a file", path, key, abs)
		}
		entries[normalized] = abs
	}
	return &UserConfig{entries: entries, origin: path}, nil
}

// readPresetFile returns the bytes for a preset path, preferring the user
// overlay and falling back to the embedded preset FS. Returns fs.ErrNotExist
// when neither source has the file.
func normalizePresetKey(key string) string {
	key = strings.TrimSpace(key)
	// Convert Windows-style backslashes to forward slashes before the
	// OS-native ToSlash pass. This makes the config format forgiving when
	// the user copies a path off a Windows machine.
	key = strings.ReplaceAll(key, "\\", "/")
	key = filepath.ToSlash(key)
	key = strings.TrimLeft(key, "/")
	return key
}

func sortStrings(values []string) {
	sort.Strings(values)
}
