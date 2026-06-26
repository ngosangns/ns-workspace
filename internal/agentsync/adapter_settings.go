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

// AdapterSettingsProfile mô tả cách materialize settings cho một provider.
//
// Mỗi profile thuộc về một provider id, trỏ tới target path tương đối so với
// home (vd `.claude/settings.json`), preset mặc định cross-cutting và preset
// riêng của provider. Bảng `merge` mô tả merge strategy cho từng field
// (deep-merge, shallow-merge, hoặc replace) và field đó lấy từ preset mặc
// định, preset provider, hoặc shared manifest (vd `mcpServers`).
type AdapterSettingsProfile struct {
	ID            string                            `json:"id"`
	Target        string                            `json:"target"`
	DefaultPreset string                            `json:"defaultPreset,omitempty"`
	Preset        string                            `json:"preset,omitempty"`
	Raw           bool                              `json:"raw,omitempty"`
	Merge         map[string]AdapterSettingsMergeOp `json:"merge,omitempty"`
	Notes         string                            `json:"notes,omitempty"`
}

// AdapterSettingsMergeOp mô tả cách merge một field trong target settings.
//
// Strategy hợp lệ: `merge-deep`, `merge-shallow`, `replace`.
//
// `from` chỉ ra nguồn: `default` (preset mặc định cross-cutting), `preset`
// (preset riêng provider) hoặc `shared` (manifest shared như `mcpServers`).
type AdapterSettingsMergeOp struct {
	Strategy string `json:"strategy"`
	From     string `json:"from"`
}

// AdapterSettingsManifest chứa catalog trung tâm cho adapters cần settings
// profile. Đây là source of truth cho việc áp dụng settings per-provider;
// Go code chỉ đọc manifest thay vì hard-code target path, key path, hay
// merge strategy.
type AdapterSettingsManifest struct {
	Version  int                          `json:"version"`
	Adapters []AdapterSettingsManifestRow `json:"adapters"`
}

type AdapterSettingsManifestRow struct {
	ID              string   `json:"id"`
	Tier            string   `json:"tier"`
	Executable      string   `json:"executable,omitempty"`
	Docs            []string `json:"docs,omitempty"`
	SettingsProfile string   `json:"settingsProfile"`
}

// ApplyAdapterSettings là operation materialize settings cho một provider từ
// profile + preset tương ứng. Operation này thay thế `LinkOrCopy` thẳng cho
// provider có profile; provider chưa migrate vẫn dùng path cũ.
type ApplyAdapterSettings struct {
	ProfilePath string
	TargetPath  string
	HomeDir     string
	Replace     bool
}

func (op ApplyAdapterSettings) Apply(ctx Context) error {
	profile, err := readAdapterSettingsProfileHook(ctx, op.ProfilePath)
	if err != nil {
		return err
	}
	dst, err := resolveHomeRelative(op.HomeDir, profile.Target)
	if err != nil {
		return err
	}
	if profile.Raw {
		return applyAdapterSettingsRaw(ctx, profile, dst, op.Replace)
	}
	values, err := buildAdapterSettings(ctx, profile)
	if err != nil {
		return err
	}
	return writeAdapterSettingsJSON(ctx, dst, values, op.Replace)
}

func (op ApplyAdapterSettings) Describe(ctx Context) {
	ctx.Report.Line("adapter settings: %s", op.ProfilePath)
}
func (op ApplyAdapterSettings) Path() string {
	return op.TargetPath
}

// readAdapterSettingsProfileHook is a seam for tests; production code
// uses readAdapterSettingsProfile below.
var readAdapterSettingsProfileHook = readAdapterSettingsProfile

// readAdapterSettingsProfile reads and validates the JSON profile at
// profilePath.
func readAdapterSettingsProfile(ctx Context, profilePath string) (*AdapterSettingsProfile, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("adapter settings: empty profile path")
	}
	data, err := readPresetFileHook(ctx, profilePath)
	if err != nil {
		return nil, err
	}
	profile := &AdapterSettingsProfile{}
	if err := json.Unmarshal(data, profile); err != nil {
		return nil, fmt.Errorf("adapter settings: invalid profile %s: %w", profilePath, err)
	}
	if profile.ID == "" {
		return nil, fmt.Errorf("adapter settings: missing id in %s", profilePath)
	}
	if profile.Target == "" {
		return nil, fmt.Errorf("adapter settings: missing target in %s", profilePath)
	}
	return profile, nil
}

func buildAdapterSettings(ctx Context, profile *AdapterSettingsProfile) (map[string]any, error) {
	values := map[string]any{}
	defaultValues, err := readAdapterSettingsPreset(ctx, profile.DefaultPreset)
	if err != nil {
		return nil, err
	}
	providerValues, err := readAdapterSettingsPreset(ctx, profile.Preset)
	if err != nil {
		return nil, err
	}
	sharedMCP, err := readSharedMCPValues(ctx)
	if err != nil {
		return nil, err
	}
	mergeKeys := make([]string, 0, len(profile.Merge))
	for key := range profile.Merge {
		mergeKeys = append(mergeKeys, key)
	}
	sort.Strings(mergeKeys)
	for _, key := range mergeKeys {
		op := profile.Merge[key]
		var source map[string]any
		switch op.From {
		case "default":
			source = defaultValues
		case "preset":
			source = providerValues
		case "shared":
			source = sharedMCP
		default:
			return nil, fmt.Errorf("adapter settings %s: field %q has unknown source %q", profile.ID, key, op.From)
		}
		chunk, ok := source[key]
		if !ok || chunk == nil {
			continue
		}
		chunkMap, ok := chunk.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("adapter settings %s: field %q is not a JSON object", profile.ID, key)
		}
		switch op.Strategy {
		case "merge-deep":
			values[key] = mergeDeep(asMap(values[key]), chunkMap)
		case "merge-shallow":
			values[key] = mergeShallow(asMap(values[key]), chunkMap)
		case "replace":
			values[key] = chunkMap
		default:
			return nil, fmt.Errorf("adapter settings %s: field %q has unknown strategy %q", profile.ID, key, op.Strategy)
		}
	}
	return values, nil
}

func readAdapterSettingsPreset(ctx Context, presetPath string) (map[string]any, error) {
	out := map[string]any{}
	if presetPath == "" {
		return out, nil
	}
	data, err := readPresetFile(ctx, presetPath)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("adapter settings: invalid preset %s: %w", presetPath, err)
	}
	return out, nil
}

func applyAdapterSettingsRaw(ctx Context, profile *AdapterSettingsProfile, dst string, replace bool) error {
	if profile.Preset == "" {
		return fmt.Errorf("adapter settings %s: raw profile requires preset", profile.ID)
	}
	data, err := readPresetFile(ctx, profile.Preset)
	if err != nil {
		return err
	}
	if !json.Valid(data) {
		return fmt.Errorf("adapter settings %s: raw preset is not valid JSON: %s", profile.ID, profile.Preset)
	}
	if !replace {
		if existing, err := os.ReadFile(dst); err == nil && len(strings.TrimSpace(string(existing))) > 0 {
			if string(existing) == strings.TrimRight(string(data), "\n")+"\n" {
				ctx.Report.Line("ok: %s", dst)
				return nil
			}
		}
	}
	if err := ensureDir(ctx, filepath.Dir(dst)); err != nil {
		return err
	}
	return writeFileManaged(ctx, dst, data, true)
}

func writeAdapterSettingsJSON(ctx Context, dst string, values map[string]any, replace bool) error {
	if !replace {
		existing, err := os.ReadFile(dst)
		if err == nil && len(strings.TrimSpace(string(existing))) > 0 {
			current := map[string]any{}
			if jerr := json.Unmarshal(existing, &current); jerr == nil {
				if mapsEqualAdapterSettings(current, values) {
					ctx.Report.Line("ok: %s", dst)
					return nil
				}
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := ensureDir(ctx, filepath.Dir(dst)); err != nil {
		return err
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileManaged(ctx, dst, data, true)
}

func mapsEqualAdapterSettings(a, b map[string]any) bool {
	abytes, aerr := json.Marshal(a)
	bbytes, berr := json.Marshal(b)
	if aerr != nil || berr != nil {
		return false
	}
	return string(abytes) == string(bbytes)
}

