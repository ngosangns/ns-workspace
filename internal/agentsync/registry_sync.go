package agentsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// registrySyncStateVersion is the on-disk schema version for
// ~/.agents/registry/.sync-state.json.
const registrySyncStateVersion = 1

// registrySyncStateFile is the relative path under AgentsDir for the
// last successful registry install stamps.
const registrySyncStateFile = "registry/.sync-state.json"

// registrySyncState records per-skill entry fingerprints from the last
// successful installs so default update can skip network installers when
// a skill's catalog identity is unchanged and SKILL.md is present.
// Fingerprint is a whole-catalog hash written after a fully successful
// bulk pass (debug / humans); planning uses only per-skill stamps.
type registrySyncState struct {
	Version     int                           `json:"version"`
	Fingerprint string                        `json:"fingerprint,omitempty"`
	Skills      map[string]registrySkillStamp `json:"skills"`
}

// registrySkillStamp is one skill's applied catalog identity.
type registrySkillStamp struct {
	Source    string `json:"source,omitempty"`
	Skill     string `json:"skill"`
	Installer string `json:"installer,omitempty"`
	EntryFP   string `json:"entryFingerprint"`
}

// skillInstallID is the on-disk directory name under skills/ for a
// registry entry (the skills CLI skill id, or "but" for but-skill).
func skillInstallID(skill RegistrySkill) string {
	id := strings.TrimSpace(skill.Skill)
	if id == "" {
		id = strings.TrimSpace(skill.Name)
	}
	if id == "" {
		return "but"
	}
	return id
}

// registrySkillDir is the shared skills home path for one registry entry.
func registrySkillDir(agentsDir string, skill RegistrySkill) string {
	return filepath.Join(agentsDir, "skills", skillInstallID(skill))
}

// registrySkillPresent reports whether the skill appears installed under
// the shared agents home (SKILL.md exists).
func registrySkillPresent(agentsDir string, skill RegistrySkill) bool {
	path := filepath.Join(registrySkillDir(agentsDir, skill), "SKILL.md")
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// entryFingerprint is a stable content hash of the install identity for
// one registry row (installer, source, skill id, name).
func entryFingerprint(skill RegistrySkill) string {
	parts := []string{
		skill.installerKind(),
		strings.TrimSpace(skill.Source),
		skillInstallID(skill),
		strings.TrimSpace(skill.Name),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// catalogFingerprint hashes the full sanitized skill list so a catalog
// that is byte-identical (order-independent) yields the same value.
func catalogFingerprint(skills []RegistrySkill) string {
	fps := make([]string, 0, len(skills))
	for _, s := range skills {
		fps = append(fps, entryFingerprint(s))
	}
	sort.Strings(fps)
	h := sha256.New()
	for _, fp := range fps {
		_, _ = h.Write([]byte(fp))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// loadRegistrySyncState reads the last-applied state, or a zero state.
func loadRegistrySyncState(agentsDir string) registrySyncState {
	path := filepath.Join(agentsDir, filepath.FromSlash(registrySyncStateFile))
	data, err := os.ReadFile(path)
	if err != nil {
		return registrySyncState{Version: registrySyncStateVersion, Skills: map[string]registrySkillStamp{}}
	}
	var st registrySyncState
	if err := json.Unmarshal(data, &st); err != nil {
		return registrySyncState{Version: registrySyncStateVersion, Skills: map[string]registrySkillStamp{}}
	}
	if st.Skills == nil {
		st.Skills = map[string]registrySkillStamp{}
	}
	if st.Version == 0 {
		st.Version = registrySyncStateVersion
	}
	return st
}

// saveRegistrySyncState writes state under agentsDir/registry/.
func saveRegistrySyncState(agentsDir string, st registrySyncState) error {
	st.Version = registrySyncStateVersion
	if st.Skills == nil {
		st.Skills = map[string]registrySkillStamp{}
	}
	path := filepath.Join(agentsDir, filepath.FromSlash(registrySyncStateFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// registryInstallPlan is the result of decide which registry skills need
// a network/local installer run.
type registryInstallPlan struct {
	// CatalogFP is the fingerprint of the full enabled catalog (metadata).
	CatalogFP string
	// ToInstall is the subset that must run installers.
	ToInstall []RegistrySkill
	// SkipAll is true when every skill is present with a matching stamp
	// (or the catalog is empty) — zero installer invocations.
	SkipAll bool
	// Skipped lists skill install ids skipped because present + stamp matches.
	Skipped []string
}

// planRegistryInstalls decides which skills need installers using only
// per-skill stamps + presence. force=true installs every entry.
func planRegistryInstalls(agentsDir string, skills []RegistrySkill, state registrySyncState, force bool) registryInstallPlan {
	plan := registryInstallPlan{
		CatalogFP: catalogFingerprint(skills),
	}
	if len(skills) == 0 {
		plan.SkipAll = true
		return plan
	}
	for _, s := range skills {
		id := skillInstallID(s)
		fp := entryFingerprint(s)
		present := registrySkillPresent(agentsDir, s)
		stamp, hasStamp := state.Skills[id]
		unchanged := !force && hasStamp && stamp.EntryFP == fp && present
		if unchanged {
			plan.Skipped = append(plan.Skipped, id)
			continue
		}
		plan.ToInstall = append(plan.ToInstall, s)
	}
	sort.Strings(plan.Skipped)
	plan.SkipAll = len(plan.ToInstall) == 0
	return plan
}

// buildRegistrySyncState constructs state after a successful install pass.
// installedOK maps skill install id → whether installer succeeded (or was
// not needed). Only skills that are present and OK are stamped; the catalog
// fingerprint is set only when every catalog skill is stamped.
func buildRegistrySyncState(skills []RegistrySkill, installedOK map[string]bool) registrySyncState {
	st := registrySyncState{
		Version: registrySyncStateVersion,
		Skills:  map[string]registrySkillStamp{},
	}
	all := true
	for _, s := range skills {
		id := skillInstallID(s)
		if !installedOK[id] {
			all = false
			continue
		}
		st.Skills[id] = registrySkillStamp{
			Source:    strings.TrimSpace(s.Source),
			Skill:     skillInstallID(s),
			Installer: s.installerKind(),
			EntryFP:   entryFingerprint(s),
		}
	}
	if all {
		st.Fingerprint = catalogFingerprint(skills)
	}
	return st
}

// stampOneRegistrySkill merges a single successful install into sync state
// so a later bulk update can skip that skill. Does not require the full
// catalog fingerprint.
func stampOneRegistrySkill(agentsDir string, skill RegistrySkill) error {
	if !registrySkillPresent(agentsDir, skill) {
		return fmt.Errorf("skill %s not present after install", skillInstallID(skill))
	}
	st := loadRegistrySyncState(agentsDir)
	if st.Skills == nil {
		st.Skills = map[string]registrySkillStamp{}
	}
	id := skillInstallID(skill)
	st.Skills[id] = registrySkillStamp{
		Source:    strings.TrimSpace(skill.Source),
		Skill:     id,
		Installer: skill.installerKind(),
		EntryFP:   entryFingerprint(skill),
	}
	// Single-skill stamp invalidates whole-catalog fingerprint.
	st.Fingerprint = ""
	return saveRegistrySyncState(agentsDir, st)
}
