package agentsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

// stubRegistryInstaller counts invocations and writes SKILL.md so the
// real presence + fingerprint path advances state without network.
func stubRegistryInstaller(calls *atomic.Int32, installed *[]string) func(Context, RegistrySkill, []string, string) error {
	return func(ctx Context, skill RegistrySkill, _ []string, _ string) error {
		calls.Add(1)
		if installed != nil {
			*installed = append(*installed, skillInstallID(skill))
		}
		dir := registrySkillDir(ctx.Options.AgentsDir, skill)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+skillInstallID(skill)+"\n"), 0o644)
	}
}

func testRegistryCtx(t *testing.T, agentsDir string, skills []RegistrySkill, refresh bool) Context {
	t.Helper()
	// Embed only what installRegistrySkills needs via UserConfig overlay path
	// for the registry manifest — inject through materialized registry file
	// and force Update mode so readRegistryManifest uses preset path; easier
	// to write agentsDir/registry/skills.json and use init mode (!Update).
	body, err := json.Marshal(RegistryManifest{Skills: skills})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(agentsDir, "registry"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "registry", "skills.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	// Minimal preset FS so other reads do not panic if touched.
	presets := fstest.MapFS{
		"presets/registry/skills.json": &fstest.MapFile{Data: body},
	}
	return Context{
		Options: Options{
			Command:       "update",
			AgentsDir:     agentsDir,
			RefreshSkills: refresh,
		},
		Presets:       presets,
		Report:        stdoutReporter{},
		Update:        false, // prefer on-disk registry/skills.json written above
		manifestCache: map[string]any{},
		seenDirs:      map[string]bool{},
	}
}

func TestInstallRegistrySkillsSkipsWhenUnchanged(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{
		{Name: "alpha", Source: "acme/alpha-skills", Skill: "alpha"},
		{Name: "beta", Source: "acme/beta-skills", Skill: "beta"},
	}

	var calls atomic.Int32
	var installed []string
	orig := installOneRegistrySkill
	installOneRegistrySkill = stubRegistryInstaller(&calls, &installed)
	defer func() { installOneRegistrySkill = orig }()

	// Put a fake npx on PATH so needNpx LookPath succeeds (stub never runs it).
	bin := t.TempDir()
	npx := filepath.Join(bin, "npx")
	if err := os.WriteFile(npx, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx := testRegistryCtx(t, agentsDir, skills, false)
	if err := installRegistrySkills(ctx); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("first install calls = %d, want 2 (got skills %v)", got, installed)
	}
	// Clear manifest cache between runs (fresh Context).
	ctx2 := testRegistryCtx(t, agentsDir, skills, false)
	calls.Store(0)
	installed = nil
	if err := installRegistrySkills(ctx2); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("second install must skip all; calls = %d, installed=%v", got, installed)
	}
	st := loadRegistrySyncState(agentsDir)
	if st.Fingerprint == "" {
		t.Fatal("expected catalog fingerprint after successful install")
	}
	if st.Fingerprint != catalogFingerprint(skills) {
		t.Fatalf("fingerprint mismatch: state=%s catalog=%s", st.Fingerprint, catalogFingerprint(skills))
	}
}

func TestInstallRegistrySkillsInstallsOnlyMissing(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{
		{Name: "alpha", Source: "acme/alpha-skills", Skill: "alpha"},
		{Name: "beta", Source: "acme/beta-skills", Skill: "beta"},
	}

	var calls atomic.Int32
	var installed []string
	orig := installOneRegistrySkill
	installOneRegistrySkill = stubRegistryInstaller(&calls, &installed)
	defer func() { installOneRegistrySkill = orig }()

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "npx"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx := testRegistryCtx(t, agentsDir, skills, false)
	if err := installRegistrySkills(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Remove one skill dir.
	if err := os.RemoveAll(filepath.Join(agentsDir, "skills", "beta")); err != nil {
		t.Fatal(err)
	}
	calls.Store(0)
	installed = nil
	ctx2 := testRegistryCtx(t, agentsDir, skills, false)
	if err := installRegistrySkills(ctx2); err != nil {
		t.Fatalf("reinstall missing: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 for missing beta only; installed=%v", calls.Load(), installed)
	}
	if len(installed) != 1 || installed[0] != "beta" {
		t.Fatalf("installed = %v, want [beta]", installed)
	}
}

func TestInstallRegistrySkillsInstallsWhenSourceChanges(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{
		{Name: "alpha", Source: "acme/alpha-skills", Skill: "alpha"},
	}

	var calls atomic.Int32
	var installed []string
	orig := installOneRegistrySkill
	installOneRegistrySkill = stubRegistryInstaller(&calls, &installed)
	defer func() { installOneRegistrySkill = orig }()

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "npx"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := installRegistrySkills(testRegistryCtx(t, agentsDir, skills, false)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	changed := []RegistrySkill{
		{Name: "alpha", Source: "acme/alpha-skills-v2", Skill: "alpha"},
	}
	calls.Store(0)
	installed = nil
	if err := installRegistrySkills(testRegistryCtx(t, agentsDir, changed, false)); err != nil {
		t.Fatalf("source change: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 after source change", calls.Load())
	}
}

func TestInstallRegistrySkillsRefreshSkillsForcesReinstall(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{
		{Name: "alpha", Source: "acme/alpha-skills", Skill: "alpha"},
		{Name: "beta", Source: "acme/beta-skills", Skill: "beta"},
	}

	var calls atomic.Int32
	orig := installOneRegistrySkill
	installOneRegistrySkill = stubRegistryInstaller(&calls, nil)
	defer func() { installOneRegistrySkill = orig }()

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "npx"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := installRegistrySkills(testRegistryCtx(t, agentsDir, skills, false)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	calls.Store(0)
	if err := installRegistrySkills(testRegistryCtx(t, agentsDir, skills, true)); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("refresh-skills calls = %d, want 2", calls.Load())
	}
}

func TestPlanRegistryInstallsPure(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{
		{Name: "a", Source: "acme/a", Skill: "a"},
		{Name: "b", Source: "acme/b", Skill: "b"},
	}
	// No state, nothing on disk → install all.
	plan := planRegistryInstalls(agentsDir, skills, registrySyncState{}, false)
	if plan.SkipAll || len(plan.ToInstall) != 2 {
		t.Fatalf("empty state plan = %+v", plan)
	}
	// Present + matching per-skill stamps → skip all.
	for _, s := range skills {
		dir := registrySkillDir(agentsDir, s)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := buildRegistrySyncState(skills, map[string]bool{"a": true, "b": true})
	plan = planRegistryInstalls(agentsDir, skills, st, false)
	if !plan.SkipAll || len(plan.ToInstall) != 0 {
		t.Fatalf("unchanged plan = %+v", plan)
	}
	// Force.
	plan = planRegistryInstalls(agentsDir, skills, st, true)
	if plan.SkipAll || len(plan.ToInstall) != 2 {
		t.Fatalf("force plan = %+v", plan)
	}
	// Catalog shrink: only remaining skill stamped → skip that one only.
	plan = planRegistryInstalls(agentsDir, skills[:1], st, false)
	if !plan.SkipAll || len(plan.ToInstall) != 0 {
		t.Fatalf("catalog shrink with stamps = %+v", plan)
	}
}

func TestCatalogFingerprintStableOrder(t *testing.T) {
	a := []RegistrySkill{
		{Name: "z", Source: "acme/z", Skill: "z"},
		{Name: "a", Source: "acme/a", Skill: "a"},
	}
	b := []RegistrySkill{
		{Name: "a", Source: "acme/a", Skill: "a"},
		{Name: "z", Source: "acme/z", Skill: "z"},
	}
	if catalogFingerprint(a) != catalogFingerprint(b) {
		t.Fatal("fingerprint must be order-independent")
	}
	if catalogFingerprint(a) == catalogFingerprint(a[:1]) {
		t.Fatal("different catalogs must differ")
	}
}

// Ensure skip message path does not require npx when fully unchanged.
func TestInstallRegistrySkillsSkipDoesNotNeedNpx(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{
		{Name: "alpha", Source: "acme/alpha-skills", Skill: "alpha"},
	}
	// Seed state + skill without calling installer loop with npx.
	dir := filepath.Join(agentsDir, "skills", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := buildRegistrySyncState(skills, map[string]bool{"alpha": true})
	if err := saveRegistrySyncState(agentsDir, st); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	orig := installOneRegistrySkill
	installOneRegistrySkill = stubRegistryInstaller(&calls, nil)
	defer func() { installOneRegistrySkill = orig }()

	// PATH without npx.
	t.Setenv("PATH", t.TempDir())

	if err := installRegistrySkills(testRegistryCtx(t, agentsDir, skills, false)); err != nil {
		t.Fatalf("skip without npx: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected 0 installer calls, got %d", calls.Load())
	}
}

func TestInstallRegistrySkillsReportsUnchanged(t *testing.T) {
	agentsDir := t.TempDir()
	skills := []RegistrySkill{{Name: "x", Source: "acme/x-skills", Skill: "x"}}
	dir := filepath.Join(agentsDir, "skills", "x")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x\n"), 0o644)
	_ = saveRegistrySyncState(agentsDir, buildRegistrySyncState(skills, map[string]bool{"x": true}))

	plan := planRegistryInstalls(agentsDir, skills, loadRegistrySyncState(agentsDir), false)
	if !plan.SkipAll {
		t.Fatalf("expected SkipAll, plan=%+v", plan)
	}
	if plan.CatalogFP == "" {
		t.Fatal("empty catalog fp")
	}
}

// Portal one-shot install must stamp sync state so a later bulk plan skips.
func TestInstallRegistrySkillStampsSyncState(t *testing.T) {
	agentsDir := t.TempDir()
	skill := RegistrySkill{Name: "portal-skill", Source: "acme/portal", Skill: "portal-skill"}

	orig := installOneRegistrySkill
	installOneRegistrySkill = stubRegistryInstaller(new(atomic.Int32), nil)
	defer func() { installOneRegistrySkill = orig }()

	if err := InstallRegistrySkill(agentsDir, skill); err != nil {
		t.Fatalf("InstallRegistrySkill: %v", err)
	}
	if !registrySkillPresent(agentsDir, skill) {
		t.Fatal("skill not on disk after portal install")
	}
	st := loadRegistrySyncState(agentsDir)
	id := skillInstallID(skill)
	if st.Skills[id].EntryFP != entryFingerprint(skill) {
		t.Fatalf("stamp missing/wrong: %+v", st.Skills[id])
	}
	plan := planRegistryInstalls(agentsDir, []RegistrySkill{skill}, st, false)
	if !plan.SkipAll {
		t.Fatalf("after portal stamp, plan should SkipAll: %+v", plan)
	}
}

// Core InstallPresetTree must not delete registry skills, otherwise the
// catalog-fingerprint skip path never observes present SKILL.md files.
func TestInstallPresetTreePreservesRegistrySkills(t *testing.T) {
	ctx, home := newTestContext(t)
	dstRoot := filepath.Join(home, ".agents", "skills")
	// Seed a foreign registry skill alongside presets.
	regDir := filepath.Join(dstRoot, "from-registry")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	regSkill := filepath.Join(regDir, "SKILL.md")
	if err := os.WriteFile(regSkill, []byte("# from-registry\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	op := InstallPresetTree{SrcRoot: "presets/skills", DstRoot: dstRoot, Replace: true}
	if err := op.Apply(ctx); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(regSkill); err != nil {
		t.Fatalf("registry skill removed by core tree: %v", err)
	}
	// Preset skill still materializes.
	if _, err := os.Stat(filepath.Join(dstRoot, "execution", "SKILL.md")); err != nil {
		t.Fatalf("preset skill missing: %v", err)
	}
}

func TestInstallPresetTreeRemovesDisabledSkill(t *testing.T) {
	ctx, home := newTestContext(t)
	dstRoot := filepath.Join(home, ".agents", "skills")
	// Materialize once.
	op := InstallPresetTree{SrcRoot: "presets/skills", DstRoot: dstRoot, Replace: true}
	if err := op.Apply(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	execSkill := filepath.Join(dstRoot, "execution", "SKILL.md")
	if _, err := os.Stat(execSkill); err != nil {
		t.Fatalf("expected execution skill: %v", err)
	}
	// Portal-disable "execution" and re-apply.
	ctx.DisabledSkills = map[string]bool{"execution": true}
	if err := op.Apply(ctx); err != nil {
		t.Fatalf("disable apply: %v", err)
	}
	if _, err := os.Stat(execSkill); !os.IsNotExist(err) {
		t.Fatalf("disabled skill should be removed, err=%v", err)
	}
}

func TestLinkSkillDirsSelectiveReplace(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	// Two skills in shared home.
	for _, name := range []string{"keep", "drop"} {
		dir := filepath.Join(src, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := Context{
		Options:  Options{AgentsDir: t.TempDir()},
		Report:   stdoutReporter{},
		seenDirs: map[string]bool{},
	}
	// First pass: both linked/copied.
	if err := (LinkSkillDirs{SrcRoot: src, DstRoot: dst, Replace: true}).Apply(ctx); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Remove "drop" from src; keep "keep". Add "new".
	if err := os.RemoveAll(filepath.Join(src, "drop")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "new", "SKILL.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (LinkSkillDirs{SrcRoot: src, DstRoot: dst, Replace: true}).Apply(ctx); err != nil {
		t.Fatalf("second: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "keep", "SKILL.md")); err != nil {
		t.Fatalf("keep missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "new", "SKILL.md")); err != nil {
		t.Fatalf("new missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "drop")); !os.IsNotExist(err) {
		t.Fatalf("drop should be removed from mirror")
	}
}
