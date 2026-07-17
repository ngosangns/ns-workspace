package portal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

// skillsCatalogSearchURL is the public skills.sh search endpoint.
// Overridable in tests.
var skillsCatalogSearchURL = "https://skills.sh/api/search"

// githubAPIBase is the GitHub REST API root. Overridable in tests.
var githubAPIBase = "https://api.github.com"

// skillsCatalogHTTPClient is used for outbound catalog HTTP. Overridable in tests.
var skillsCatalogHTTPClient = &http.Client{Timeout: 30 * time.Second}

// CatalogSkill is one skill package available from a registry source.
type CatalogSkill struct {
	ID          string `json:"id"`
	SkillID     string `json:"skillId"` // installable id for `npx skills add --skill`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Installs    int    `json:"installs,omitempty"`
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	Path        string `json:"path,omitempty"`
	Installed   bool   `json:"installed"`
}

// CatalogSearchResponse is returned by GET /api/skills/search.
type CatalogSearchResponse struct {
	Query  string         `json:"query"`
	Skills []CatalogSkill `json:"skills"`
}

// RegistrySource is one GitHub owner/repo used as a skills registry.
type RegistrySource struct {
	Source     string `json:"source"`
	// EnabledEntries / DisabledEntries count configured rows in
	// skills.json / skills.disabled.json that reference this source.
	EnabledEntries  int `json:"enabledEntries"`
	DisabledEntries int `json:"disabledEntries"`
	// SkillCount is the number of installable skills in the package (GitHub
	// SKILL.md tree). The cheap /registries endpoint always returns 0; the
	// client fills this after GET /skills/catalog for that source.
	SkillCount int    `json:"skillCount"`
	Configured bool   `json:"configured"`
	Listable   bool   `json:"listable"`
	Note       string `json:"note,omitempty"`
}

// RegistriesResponse is returned by GET /api/skills/registries.
type RegistriesResponse struct {
	Registries []RegistrySource `json:"registries"`
}

// CatalogListResponse is returned by GET /api/skills/catalog.
type CatalogListResponse struct {
	Registry string         `json:"registry,omitempty"`
	Query    string         `json:"query,omitempty"`
	Skills   []CatalogSkill `json:"skills"`
	Count    int            `json:"count"`
}

// SkillInstallRequest is the body for POST /api/skills/install.
type SkillInstallRequest struct {
	Source string `json:"source"`
	Skill  string `json:"skill"`
	Name   string `json:"name,omitempty"`
	// Path is the repo-relative path to SKILL.md (or its package dir) from
	// catalog listing. Used for direct GitHub install when npx skills cannot
	// discover the package (e.g. invalid YAML frontmatter).
	Path string `json:"path,omitempty"`
}

// SkillUninstallRequest is the body for POST /api/skills/uninstall.
type SkillUninstallRequest struct {
	// Skill is the install package id under ~/.agents/skills/<id>.
	Skill string `json:"skill"`
}

// SkillInstallResponse is returned after a successful install.
type SkillInstallResponse struct {
	Skill    Skill         `json:"skill"`
	Registry RegistrySkill `json:"registry"`
}

// SkillInstallBatchRequest installs multiple skills.
type SkillInstallBatchRequest struct {
	Skills []SkillInstallRequest `json:"skills"`
}

// SkillInstallBatchResponse reports per-skill results.
type SkillInstallBatchResponse struct {
	Installed []SkillInstallResponse `json:"installed"`
	Failed    []SkillInstallFailure  `json:"failed,omitempty"`
}

// SkillInstallFailure is one failed install in a batch.
type SkillInstallFailure struct {
	Source string `json:"source"`
	Skill  string `json:"skill"`
	Error  string `json:"error"`
}

// registry skill list cache (process-local).
var (
	registryListCacheMu sync.Mutex
	registryListCache   = map[string]registryListCacheEntry{}
)

type registryListCacheEntry struct {
	skills    []CatalogSkill
	expiresAt time.Time
}

const registryListCacheTTL = 10 * time.Minute

// handleSkillCatalogOrSkill routes catalog endpoints before skill id paths.
func (s *portalServer) handleSkillCatalogOrSkill(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/skills/")
	path = strings.Trim(path, "/")
	switch {
	case path == "search":
		s.handleSkillsSearch(w, r)
	case path == "install":
		s.handleSkillsInstall(w, r)
	case path == "install-batch":
		s.handleSkillsInstallBatch(w, r)
	case path == "uninstall":
		s.handleSkillsUninstall(w, r)
	case path == "registries":
		s.handleSkillsRegistries(w, r)
	case path == "catalog":
		s.handleSkillsCatalog(w, r)
	default:
		s.handleSkill(w, r)
	}
}

func (s *portalServer) handleSkillsRegistries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	writeJSON(w, RegistriesResponse{Registries: s.listConfiguredRegistries()})
}

func (s *portalServer) handleSkillsCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	registry := strings.TrimSpace(r.URL.Query().Get("registry"))
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	refresh := strings.EqualFold(r.URL.Query().Get("refresh"), "1") ||
		strings.EqualFold(r.URL.Query().Get("refresh"), "true")

	var sources []string
	if registry != "" && !strings.EqualFold(registry, "all") {
		if !isGitHubSource(registry) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("registry %q is not a listable GitHub owner/repo", registry))
			return
		}
		sources = []string{registry}
	} else {
		// "all" is expensive (many GitHub trees). Prefer the first listable
		// source when the client forgot to pick one; still honor explicit all.
		for _, reg := range s.listConfiguredRegistries() {
			if reg.Listable {
				sources = append(sources, reg.Source)
			}
		}
		if strings.EqualFold(registry, "all") && len(sources) > 3 {
			// Cap concurrent cost: client should filter by registry for full browse.
			// Still return merged results but stop after first failure streak.
		}
	}
	if len(sources) == 0 {
		writeJSON(w, CatalogListResponse{Registry: registry, Query: q, Skills: []CatalogSkill{}, Count: 0})
		return
	}

	if refresh {
		invalidateRegistryListCache(sources...)
	}

	installed := s.installedSkillIDs()
	var all []CatalogSkill
	var errs []string
	for _, src := range sources {
		skills, err := listSkillsInRegistrySource(src)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", src, err))
			continue
		}
		for i := range skills {
			skills[i].Installed = installed[skills[i].SkillID] || installed[skills[i].Name]
			if skills[i].URL == "" {
				skills[i].URL = "https://skills.sh/" + skills[i].ID
			}
		}
		all = append(all, skills...)
	}
	if q != "" {
		ql := strings.ToLower(q)
		filtered := all[:0]
		for _, sk := range all {
			hay := strings.ToLower(sk.Name + " " + sk.SkillID + " " + sk.Source + " " + sk.Path)
			if strings.Contains(hay, ql) {
				filtered = append(filtered, sk)
			}
		}
		all = filtered
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Source != all[j].Source {
			return all[i].Source < all[j].Source
		}
		return all[i].SkillID < all[j].SkillID
	})
	if len(errs) > 0 && len(all) == 0 {
		writeError(w, http.StatusBadGateway, fmt.Errorf("failed to list registries: %s", strings.Join(errs, "; ")))
		return
	}
	writeJSON(w, CatalogListResponse{
		Registry: registry,
		Query:    q,
		Skills:   all,
		Count:    len(all),
	})
}

func (s *portalServer) handleSkillsSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	registry := strings.TrimSpace(r.URL.Query().Get("registry"))
	if len(q) < 2 && registry == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("query must be at least 2 characters (or pass registry=)"))
		return
	}
	// Prefer full registry listing when filtering by a concrete registry.
	if registry != "" && !strings.EqualFold(registry, "all") {
		s.handleSkillsCatalog(w, r)
		return
	}
	if len(q) < 2 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("query must be at least 2 characters"))
		return
	}
	results, err := searchSkillsCatalog(q, registry)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	installed := s.installedSkillIDs()
	for i := range results {
		results[i].Installed = installed[results[i].SkillID] || installed[results[i].Name]
		if results[i].URL == "" && results[i].ID != "" {
			results[i].URL = "https://skills.sh/" + results[i].ID
		}
	}
	writeJSON(w, CatalogSearchResponse{Query: q, Skills: results})
}

func (s *portalServer) handleSkillsInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req SkillInstallRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.installOneSkill(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, res)
}

func (s *portalServer) handleSkillsUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req SkillUninstallRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	req.Skill = strings.TrimSpace(req.Skill)
	if req.Skill == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("skill is required"))
		return
	}
	if err := s.store.UninstallInstalledSkill(req.Skill); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "skill": req.Skill})
}

func (s *portalServer) handleSkillsInstallBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req SkillInstallBatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Skills) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("skills array is required"))
		return
	}
	if len(req.Skills) > 50 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("batch install limited to 50 skills per request"))
		return
	}
	out := SkillInstallBatchResponse{}
	for _, item := range req.Skills {
		res, err := s.installOneSkill(item)
		if err != nil {
			out.Failed = append(out.Failed, SkillInstallFailure{
				Source: item.Source,
				Skill:  item.Skill,
				Error:  err.Error(),
			})
			continue
		}
		out.Installed = append(out.Installed, *res)
	}
	writeJSON(w, out)
}

func (s *portalServer) installOneSkill(req SkillInstallRequest) (*SkillInstallResponse, error) {
	req.Source = strings.TrimSpace(req.Source)
	req.Skill = strings.TrimSpace(req.Skill)
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if req.Skill == "" {
		return nil, fmt.Errorf("skill is required")
	}
	if req.Name == "" {
		req.Name = req.Skill
	}
	if req.Source == "" {
		return nil, fmt.Errorf("source is required (owner/repo)")
	}
	if agentsync.IsPlaceholderRegistrySource(req.Source) {
		return nil, fmt.Errorf("placeholder source %q is not allowed (e.g. org/repo is docs-only)", req.Source)
	}
	if err := agentsync.ValidateRegistrySource(req.Source); err != nil {
		return nil, err
	}
	entry := agentsync.RegistrySkill{
		Name:   req.Name,
		Source: req.Source,
		Skill:  req.Skill,
	}

	// Prefer npx skills (copies into agent skill dirs). Fall back to a direct
	// GitHub package copy when discovery fails — common when SKILL.md YAML is
	// invalid (unquoted ":" in description) so the CLI skips the package.
	npxErr := agentsync.InstallRegistrySkill(s.agentsDir, entry)
	if npxErr != nil {
		pkgPath := packageDirFromSkillPath(req.Path)
		if pkgPath == "" {
			// Last resort: treat skill id as package folder name.
			pkgPath = req.Skill
		}
		if derr := installSkillPackageFromGitHub(s.agentsDir, req.Source, pkgPath, req.Skill); derr != nil {
			return nil, fmt.Errorf("%w (direct install also failed: %v)", npxErr, derr)
		}
	}

	portalEntry := RegistrySkill{Name: entry.Name, Source: entry.Source, Skill: entry.Skill}
	if err := s.store.UpsertRegistrySkill(portalEntry); err != nil {
		return nil, fmt.Errorf("installed but failed to update registry overlay: %w", err)
	}
	skill, err := s.store.ReadSkill(req.Skill)
	if err != nil {
		skill = &Skill{ID: req.Skill, Name: req.Name, Source: "installed", Enabled: true, RegistrySource: req.Source}
	}
	return &SkillInstallResponse{Skill: *skill, Registry: portalEntry}, nil
}

// packageDirFromSkillPath returns the package directory for a catalog path
// like "ogilvy/SKILL.md" or "skills/foo/SKILL.md" → "ogilvy" / "skills/foo".
func packageDirFromSkillPath(skillMDPath string) string {
	p := strings.Trim(strings.ReplaceAll(skillMDPath, "\\", "/"), "/")
	if p == "" {
		return ""
	}
	if strings.EqualFold(path.Base(p), "SKILL.md") {
		dir := path.Dir(p)
		if dir == "." || dir == "/" || dir == "" {
			return ""
		}
		return dir
	}
	return p
}

// installSkillPackageFromGitHub copies every file under packageDir in the
// GitHub repo into agentsDir/skills/installName/. Does not use npx discovery.
func installSkillPackageFromGitHub(agentsDir, source, packageDir, installName string) error {
	if strings.TrimSpace(agentsDir) == "" {
		return fmt.Errorf("agents home is required")
	}
	installName = strings.TrimSpace(installName)
	if installName == "" {
		return fmt.Errorf("install name is required")
	}
	packageDir = strings.Trim(strings.ReplaceAll(packageDir, "\\", "/"), "/")
	if packageDir == "" || packageDir == "." {
		return fmt.Errorf("package path is required for direct install")
	}
	owner, repo, ok := parseGitHubSource(source)
	if !ok {
		return fmt.Errorf("invalid github source %q", source)
	}
	token := githubToken()
	branch, err := githubDefaultBranch(owner, repo, token)
	if err != nil {
		return err
	}
	// List recursive tree and keep blobs under packageDir/.
	treeURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", githubAPIBase, owner, repo, url.PathEscape(branch))
	req, err := http.NewRequest(http.MethodGet, treeURL, nil)
	if err != nil {
		return err
	}
	applyGitHubHeaders(req, token)
	resp, err := skillsCatalogHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github tree %s/%s: HTTP %d", owner, repo, resp.StatusCode)
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return err
	}
	prefix := packageDir + "/"
	var files []string
	for _, node := range tree.Tree {
		if node.Type != "blob" {
			continue
		}
		p := node.Path
		if strings.HasPrefix(p, prefix) {
			files = append(files, p)
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("no files under %q in %s/%s", packageDir, owner, repo)
	}
	// Require SKILL.md somewhere in the package.
	hasSkillMD := false
	for _, f := range files {
		if strings.EqualFold(path.Base(f), "SKILL.md") {
			hasSkillMD = true
			break
		}
	}
	if !hasSkillMD {
		return fmt.Errorf("package %q has no SKILL.md", packageDir)
	}

	destRoot := filepath.Join(agentsDir, "skills", installName)
	if err := os.RemoveAll(destRoot); err != nil {
		return fmt.Errorf("clear install dir: %w", err)
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rel := strings.TrimPrefix(f, prefix)
		if rel == "" || rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
			continue
		}
		content, err := fetchGitHubFileContent(owner, repo, branch, f, token)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", f, err)
		}
		outPath := filepath.Join(destRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *portalServer) installedSkillIDs() map[string]bool {
	installed := map[string]bool{}
	if skills, err := s.store.ListSkills(); err == nil {
		for _, sk := range skills {
			installed[sk.ID] = true
			installed[sk.Name] = true
		}
	}
	return installed
}

func (s *portalServer) listConfiguredRegistries() []RegistrySource {
	reg, err := s.store.ReadRegistry()
	if err != nil {
		reg = &RegistrySkills{}
	}
	type counts struct {
		enabled, disabled int
		installer         string
		fromPreset        bool
	}
	bySource := map[string]*counts{}
	add := func(sk RegistrySkill, enabled bool, fromPreset bool) {
		src := strings.TrimSpace(sk.Source)
		if src == "" {
			// but-skill and similar have no GitHub source.
			src = "(local/" + sk.Installer + ")"
			if sk.Installer == "" {
				src = "(unknown)"
			}
		}
		c := bySource[src]
		if c == nil {
			c = &counts{installer: sk.Installer, fromPreset: fromPreset}
			bySource[src] = c
		}
		if !fromPreset {
			c.fromPreset = false
		}
		if enabled {
			c.enabled++
		} else {
			c.disabled++
		}
	}
	for _, sk := range reg.Skills {
		add(sk, true, false)
	}
	for _, sk := range reg.DisabledSkills {
		add(sk, false, false)
	}

	// If the user overlay only has non-listable rows (e.g. placeholder
	// org/repo, but-skill), seed sources from the embedded registry preset
	// so Discover can still browse real GitHub packages.
	hasListable := false
	for src := range bySource {
		if isGitHubSource(src) {
			hasListable = true
			break
		}
	}
	if !hasListable {
		if preset, err := s.store.ReadRegistryPresetSources(); err == nil {
			for _, sk := range preset {
				src := strings.TrimSpace(sk.Source)
				if src == "" || !isGitHubSource(src) {
					continue
				}
				if _, exists := bySource[src]; exists {
					continue
				}
				// Seed for browse only — zero configured entry counts.
				bySource[src] = &counts{fromPreset: true}
			}
		}
	}

	// Keep this endpoint cheap: never hit GitHub here.
	// SkillCount stays 0 until the client loads /api/skills/catalog (package
	// SKILL.md count). Do not put configured row counts into SkillCount —
	// that confuses the UI (1 configured entry ≠ 1 skill in the package).
	out := make([]RegistrySource, 0, len(bySource))
	for src, c := range bySource {
		listable := isGitHubSource(src)
		note := ""
		if !listable {
			note = "not a GitHub owner/repo; browse unavailable"
		} else if c.fromPreset && c.enabled == 0 && c.disabled == 0 {
			note = "from embedded preset (not in your enabled list)"
		}
		out = append(out, RegistrySource{
			Source:          src,
			EnabledEntries:  c.enabled,
			DisabledEntries: c.disabled,
			SkillCount:      0,
			Configured:      !c.fromPreset || c.enabled+c.disabled > 0,
			Listable:        listable,
			Note:            note,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

func isGitHubSource(source string) bool {
	owner, repo, ok := parseGitHubSource(source)
	return ok && owner != "" && repo != "" && !strings.HasPrefix(source, "(")
}

func parseGitHubSource(source string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(source)
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.TrimPrefix(s, "http://github.com/")
	s = strings.TrimPrefix(s, "git@github.com:")
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	owner, repo = parts[0], parts[1]
	if owner == "" || repo == "" {
		return "", "", false
	}
	// Reject placeholders
	if err := agentsync.ValidateRegistrySource(owner + "/" + repo); err != nil {
		return "", "", false
	}
	return owner, repo, true
}

func searchSkillsCatalog(query, registry string) ([]CatalogSkill, error) {
	u, err := url.Parse(skillsCatalogSearchURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", "100")
	if registry != "" {
		if owner, _, ok := parseGitHubSource(registry); ok {
			q.Set("owner", owner)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ns-workspace-portal")

	resp, err := skillsCatalogHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skills catalog search failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Error != "" {
			return nil, fmt.Errorf("skills catalog: %s", apiErr.Error)
		}
		return nil, fmt.Errorf("skills catalog HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Skills []CatalogSkill `json:"skills"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid catalog response: %w", err)
	}
	if parsed.Skills == nil {
		parsed.Skills = []CatalogSkill{}
	}
	if registry != "" {
		want := normalizeSourceKey(registry)
		filtered := parsed.Skills[:0]
		for _, sk := range parsed.Skills {
			if normalizeSourceKey(sk.Source) == want {
				filtered = append(filtered, sk)
			}
		}
		parsed.Skills = filtered
	}
	return parsed.Skills, nil
}

func normalizeSourceKey(source string) string {
	owner, repo, ok := parseGitHubSource(source)
	if !ok {
		return strings.ToLower(strings.TrimSpace(source))
	}
	return strings.ToLower(owner + "/" + repo)
}

// listSkillsInRegistrySource returns every skill package under a GitHub source
// (owner/repo) by walking the default branch tree for SKILL.md files.
func listSkillsInRegistrySource(source string) ([]CatalogSkill, error) {
	owner, repo, ok := parseGitHubSource(source)
	if !ok {
		return nil, fmt.Errorf("not a listable GitHub source: %s", source)
	}
	key := owner + "/" + repo

	registryListCacheMu.Lock()
	if ent, hit := registryListCache[key]; hit && time.Now().Before(ent.expiresAt) {
		out := append([]CatalogSkill(nil), ent.skills...)
		registryListCacheMu.Unlock()
		return out, nil
	}
	registryListCacheMu.Unlock()

	skills, err := listSkillsViaGitHubTree(owner, repo)
	if err != nil {
		return nil, err
	}
	registryListCacheMu.Lock()
	registryListCache[key] = registryListCacheEntry{skills: skills, expiresAt: time.Now().Add(registryListCacheTTL)}
	registryListCacheMu.Unlock()
	return append([]CatalogSkill(nil), skills...), nil
}

func invalidateRegistryListCache(sources ...string) {
	registryListCacheMu.Lock()
	defer registryListCacheMu.Unlock()
	if len(sources) == 0 {
		registryListCache = map[string]registryListCacheEntry{}
		return
	}
	for _, src := range sources {
		owner, repo, ok := parseGitHubSource(src)
		if !ok {
			continue
		}
		delete(registryListCache, owner+"/"+repo)
	}
}

func listSkillsViaGitHubTree(owner, repo string) ([]CatalogSkill, error) {
	token := githubToken()
	branch, err := githubDefaultBranch(owner, repo, token)
	if err != nil {
		return nil, err
	}
	treeURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", githubAPIBase, owner, repo, url.PathEscape(branch))
	req, err := http.NewRequest(http.MethodGet, treeURL, nil)
	if err != nil {
		return nil, err
	}
	applyGitHubHeaders(req, token)
	resp, err := skillsCatalogHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github tree %s/%s: HTTP %d: %s", owner, repo, resp.StatusCode, truncate(string(body), 200))
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, err
	}
	source := owner + "/" + repo
	type pending struct {
		path     string
		folderID string
	}
	var pendingSkills []pending
	for _, node := range tree.Tree {
		if node.Type != "blob" {
			continue
		}
		p := node.Path
		if !strings.EqualFold(path.Base(p), "SKILL.md") {
			continue
		}
		folderID := skillIDFromSkillMDPath(p, repo)
		if folderID == "" {
			continue
		}
		pendingSkills = append(pendingSkills, pending{path: p, folderID: folderID})
	}

	// Resolve installable skill id from frontmatter `name` when present.
	// `npx skills add --skill X` matches the YAML name field, which can differ
	// from the package folder (e.g. folder ogilvy → name ogilvy-copywriting).
	type resolved struct {
		path, folderID, skillID, name, description string
	}
	resolvedList := make([]resolved, len(pendingSkills))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, p := range pendingSkills {
		wg.Add(1)
		go func(i int, p pending) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			skillID := p.folderID
			name := p.folderID
			desc := ""
			if content, err := fetchGitHubFileContent(owner, repo, branch, p.path, token); err == nil {
				if n, d := parseSkillFrontmatter(content); n != "" {
					skillID = n
					name = n
					desc = d
				}
			}
			resolvedList[i] = resolved{
				path: p.path, folderID: p.folderID,
				skillID: skillID, name: name, description: desc,
			}
		}(i, p)
	}
	wg.Wait()

	seen := map[string]bool{}
	var skills []CatalogSkill
	for _, r := range resolvedList {
		if r.skillID == "" || seen[r.skillID] {
			continue
		}
		seen[r.skillID] = true
		id := source + "/" + r.skillID
		skills = append(skills, CatalogSkill{
			ID:          id,
			SkillID:     r.skillID,
			Name:        r.name,
			Description: r.description,
			Source:      source,
			Path:        r.path,
			URL:         "https://skills.sh/" + id,
		})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].SkillID < skills[j].SkillID })
	return skills, nil
}

// fetchGitHubFileContent loads a text file from the GitHub Contents API.
func fetchGitHubFileContent(owner, repo, branch, filePath, token string) ([]byte, error) {
	parts := strings.Split(filePath, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		githubAPIBase, owner, repo, strings.Join(parts, "/"), url.QueryEscape(branch))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	applyGitHubHeaders(req, token)
	resp, err := skillsCatalogHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github contents %s: HTTP %d", filePath, resp.StatusCode)
	}
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Encoding != "" && payload.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported content encoding %q", payload.Encoding)
	}
	// GitHub base64 is wrapped with newlines.
	raw := strings.ReplaceAll(payload.Content, "\n", "")
	return base64.StdEncoding.DecodeString(raw)
}

// skillIDFromSkillMDPath derives the installable skill name from the path of
// a SKILL.md blob and the repository name.
//
// Layouts:
//
//	SKILL.md                          → repo name (single-skill root package)
//	skills/SKILL.md                   → repo name (single skill under skills/)
//	skills/<id>/SKILL.md              → <id>
//	.github/skills/<id>/SKILL.md      → <id>
//
// The bare folder name "skills" must not be used as the skill id — npx skills
// add --skill skills fails when the real skill is named after the repo
// (e.g. 2389-research/landing-page-design → landing-page-design).
func skillIDFromSkillMDPath(skillMDPath, repo string) string {
	p := strings.Trim(strings.ReplaceAll(skillMDPath, "\\", "/"), "/")
	dir := path.Dir(p)
	if dir == "." || dir == "/" || dir == "" {
		return repo
	}
	base := path.Base(dir)
	if base == "." || base == "/" || base == "" {
		return repo
	}
	// Generic container directories that hold a single SKILL.md (not a package id).
	switch strings.ToLower(base) {
	case "skills", "skill", "src", "packages", "lib", "content":
		parent := path.Dir(dir)
		if parent == "." || parent == "/" || parent == "" {
			return repo
		}
		// Nested under another package folder: packages/skills/foo/SKILL.md
		// already handled when base is "foo". When base is still a container
		// (e.g. packages/skills/SKILL.md), fall back to repo.
		return repo
	}
	return base
}

func githubDefaultBranch(owner, repo, token string) (string, error) {
	u := fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, repo)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	applyGitHubHeaders(req, token)
	resp, err := skillsCatalogHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github repo %s/%s: HTTP %d: %s", owner, repo, resp.StatusCode, truncate(string(body), 200))
	}
	var meta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", err
	}
	if meta.DefaultBranch == "" {
		return "main", nil
	}
	return meta.DefaultBranch, nil
}

func applyGitHubHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "ns-workspace-portal")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func githubToken() string {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t
	}
	if t := strings.TrimSpace(os.Getenv("GH_TOKEN")); t != "" {
		return t
	}
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
