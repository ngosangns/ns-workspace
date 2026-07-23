package agentsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// readPresetFileHook is a seam for tests; production code uses
// readPresetFile below.
var readPresetFileHook = readPresetFile

// readPresetFile reads embeddedPath from the user-config overlay if
// present, otherwise from the embedded preset FS. Caches the result so
// repeated lookups during a single Apply pass do not re-read disk.
func readPresetFile(ctx Context, embeddedPath string) ([]byte, error) {
	if user, ok := ctx.UserConfig.Lookup(embeddedPath); ok {
		return os.ReadFile(user)
	}
	cacheKey := "preset:" + embeddedPath
	if cached, ok := ctx.manifestCache[cacheKey]; ok {
		return cached.([]byte), nil
	}
	data, err := fsReadFile(ctx.Presets, embeddedPath)
	if err != nil {
		return nil, fmt.Errorf("preset %s: %w", embeddedPath, err)
	}
	ctx.manifestCache[cacheKey] = data
	return data, nil
}

// readPresetFileFromUser reads embeddedPath only from the user-config
// overlay, returning fs.ErrNotExist when not present. Used by
// InstallPresetTree to enumerate user additions without falling back
// to embedded content.
func readPresetFileFromUser(ctx Context, embeddedPath string) ([]byte, error) {
	user, ok := ctx.UserConfig.Lookup(embeddedPath)
	if !ok {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(user)
}

// fsReadFile reads name from fsys using the stdlib fs.ReadFile helper.
// Wrapped here so callers don't have to import io/fs in their hot
// paths.
func fsReadFile(fsys fs.FS, name string) ([]byte, error) {
	return fs.ReadFile(fsys, name)
}

// readMCPManifestHook is a seam for tests; production code uses
// readMCPManifest below.
var readMCPManifestHook = readMCPManifest

// readMCPManifest returns the shared `presets/mcp/servers.json` content
// as a typed MCPManifest, honoring the user-config overlay.
func readMCPManifest(ctx Context) (MCPManifest, error) {
	const cacheKey = "mcp-manifest"
	if cached, ok := ctx.manifestCache[cacheKey]; ok {
		return cached.(MCPManifest), nil
	}
	var manifest MCPManifest
	// In init mode the materialized file on disk (~/.agents/mcp/servers.json)
	// is normally the source of truth so user edits outside the portal are
	// preserved. However, when the portal has written an overlay for the
	// enabled preset, that overlay must win so toggles made in the portal are
	// reflected immediately, even before the materialized file is rewritten.
	if !ctx.Update {
		if ctx.UserConfig.HasOverlay() {
			if _, hasMCPOoverlay := ctx.UserConfig.Lookup(MCPEnabledPath); hasMCPOoverlay {
				data, err := readPresetFile(ctx, MCPEnabledPath)
				if err != nil {
					return manifest, err
				}
				if err := UnmarshalJSONC(data, &manifest); err != nil {
					return manifest, err
				}
				if manifest.MCPServers == nil {
					manifest.MCPServers = map[string]any{}
				}
				ctx.manifestCache[cacheKey] = manifest
				return manifest, nil
			}
		}
		path := filepath.Join(ctx.Options.AgentsDir, "mcp", "servers.json")
		data, err := os.ReadFile(path)
		if err == nil {
			if err := UnmarshalJSONC(data, &manifest); err != nil {
				return manifest, err
			}
			if manifest.MCPServers == nil {
				manifest.MCPServers = map[string]any{}
			}
			ctx.manifestCache[cacheKey] = manifest
			return manifest, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return manifest, err
		}
	}
	data, err := readPresetFile(ctx, MCPEnabledPath)
	if err != nil {
		return manifest, err
	}
	// Enabled file may still be legacy JSONC with // commented disabled
	// servers; UnmarshalJSONC strips comments so only live keys remain.
	if err := UnmarshalJSONC(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.MCPServers == nil {
		manifest.MCPServers = map[string]any{}
	}
	ctx.manifestCache[cacheKey] = manifest
	return manifest, nil
}

// readSettingsManifestHook is a seam for tests; production code uses
// readSettingsManifest below.
var readSettingsManifestHook = readSettingsManifest

// readSettingsManifest returns the cross-cutting settings preset
// (hooks). Profile-based providers merge additional fields on top via
// AdapterSettingsProfile.
func readSettingsManifest(ctx Context) (SettingsManifest, error) {
	const cacheKey = "settings-manifest"
	if cached, ok := ctx.manifestCache[cacheKey]; ok {
		return cached.(SettingsManifest), nil
	}
	var manifest SettingsManifest
	path := filepath.Join(ctx.Options.AgentsDir, "settings.json")
	if !ctx.Update {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return manifest, err
			}
			if manifest.Hooks == nil {
				manifest.Hooks = map[string]any{}
			}
			ctx.manifestCache[cacheKey] = manifest
			return manifest, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return manifest, err
		}
	}
	data, err := readPresetFile(ctx, "presets/settings/default.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Hooks == nil {
		manifest.Hooks = map[string]any{}
	}
	ctx.manifestCache[cacheKey] = manifest
	return manifest, nil
}

// readRegistryManifest returns the registry skills manifest used to
// drive the `npx --yes skills add` installer.
func readRegistryManifest(ctx Context) (RegistryManifest, error) {
	const cacheKey = "registry-manifest"
	if cached, ok := ctx.manifestCache[cacheKey]; ok {
		return cached.(RegistryManifest), nil
	}
	var manifest RegistryManifest
	path := filepath.Join(ctx.Options.AgentsDir, "registry", "skills.json")
	if !ctx.Update {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return manifest, err
			}
			ctx.manifestCache[cacheKey] = manifest
			return manifest, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return manifest, err
		}
	}
	data, err := readPresetFile(ctx, "presets/registry/skills.json")
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	ctx.manifestCache[cacheKey] = manifest
	return manifest, nil
}

// readOpenCodeConfigValues returns the full opencode preset as a
// generic map so user-defined keys (timeout, provider, etc.) flow
// through to the native config alongside the canonical `mcp` and
// `permission` keys. `mcp` is intentionally stripped here because the
// opencode plugin layers the shared MCP manifest on top after this
// call.
func readOpenCodeConfigValues(ctx Context) (map[string]any, error) {
	const cacheKey = "opencode-config"
	if cached, ok := ctx.manifestCache[cacheKey]; ok {
		return cached.(map[string]any), nil
	}
	data, err := readPresetFile(ctx, "presets/opencode/opencode.json")
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	delete(values, "mcp")
	ctx.manifestCache[cacheKey] = values
	return values, nil
}

// readSharedMCPValues returns the shared MCP manifest as a generic map
// ready for inclusion in a profile merge. The per-server entries are
// not yet rewritten here; callers that need provider-shaped servers
// must run them through transformMCPServersForAdapter first.
func readSharedMCPValues(ctx Context) (map[string]any, error) {
	manifest, err := readMCPManifest(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"mcpServers": manifest.MCPServers}, nil
}

// readMCPDisabledNames returns server names from the portal disabled
// overlay (presets/mcp/servers.disabled.json). Missing/empty overlays
// yield nil. Used by AppendManagedBlock to purge orphan [mcp_servers.*]
// tables that remain after a portal disable.
func readMCPDisabledNames(ctx Context) []string {
	data, err := readPresetFile(ctx, MCPDisabledPath)
	if err != nil {
		return nil
	}
	disabled, err := ParseMCPDisabledJSON(data)
	if err != nil || len(disabled) == 0 {
		return nil
	}
	names := make([]string, 0, len(disabled))
	for name := range disabled {
		if strings.TrimSpace(name) == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// loadAdapterSettingsManifest reads the central adapter settings
// catalog and returns a map keyed by adapter id. Used by the plan
// phase (not the apply phase) to resolve which providers own a profile.
func loadAdapterSettingsManifest(ctx Context) (map[string]string, error) {
	data, err := readPresetFile(ctx, "presets/manifest.json")
	if err != nil {
		return nil, err
	}
	manifest := AdapterSettingsManifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("adapter settings manifest: invalid JSON: %w", err)
	}
	out := map[string]string{}
	for _, row := range manifest.Adapters {
		if row.SettingsProfile != "" {
			out[row.ID] = row.SettingsProfile
		}
	}
	return out, nil
}

// resolveAdapterSettingsTargetHook is a seam for tests; production code
// uses resolveAdapterSettingsTarget below.
var resolveAdapterSettingsTargetHook = resolveAdapterSettingsTarget

// resolveAdapterSettingsTarget reads the profile at profilePath and
// returns the resolved native config path (Target joined with homeDir).
func resolveAdapterSettingsTarget(ctx Context, profilePath, homeDir string) (string, error) {
	profile, err := readAdapterSettingsProfileHook(ctx, profilePath)
	if err != nil {
		return "", err
	}
	return resolveHomeRelative(homeDir, profile.Target)
}

// resolveHomeRelative returns target joined under homeDir after
// verifying target starts with "." or ".." (i.e. is relative).
func resolveHomeRelative(home, target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("empty target")
	}
	if !strings.HasPrefix(target, ".") {
		return "", fmt.Errorf("target %q must start with . or .. (relative to home)", target)
	}
	return filepath.Join(home, target), nil
}
