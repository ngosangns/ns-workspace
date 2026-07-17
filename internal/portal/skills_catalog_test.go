package portal

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandleSkillsSearch(t *testing.T) {
	prevURL := skillsCatalogSearchURL
	prevClient := skillsCatalogHTTPClient
	t.Cleanup(func() {
		skillsCatalogSearchURL = prevURL
		skillsCatalogHTTPClient = prevClient
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "commit" {
			t.Fatalf("unexpected q: %s", r.URL.Query().Get("q"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": "commit",
			"skills": []map[string]any{
				{
					"id":       "github/awesome-copilot/git-commit",
					"skillId":  "git-commit",
					"name":     "git-commit",
					"installs": 100,
					"source":   "github/awesome-copilot",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	skillsCatalogSearchURL = srv.URL + "/api/search"
	skillsCatalogHTTPClient = srv.Client()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\ndescription: run\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":      &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/portal/disabled.json":      &fstest.MapFile{Data: []byte(`{}`)},
		"presets/manifest.json":             &fstest.MapFile{Data: []byte(`{}`)},
	}
	ps, err := newPortalServer(presets, filepath.Join(home, ".agents"))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=commit", nil)
	rr := httptest.NewRecorder()
	ps.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var res CatalogSearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Query != "commit" || len(res.Skills) != 1 {
		t.Fatalf("unexpected response: %+v", res)
	}
	if res.Skills[0].SkillID != "git-commit" || res.Skills[0].URL == "" {
		t.Fatalf("skill hit: %+v", res.Skills[0])
	}

	// Short query rejected
	req = httptest.NewRequest(http.MethodGet, "/api/skills/search?q=x", nil)
	rr = httptest.NewRecorder()
	ps.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("short query status %d", rr.Code)
	}
}

func TestListSkillsInRegistryViaGitHubTree(t *testing.T) {
	prevBase := githubAPIBase
	prevClient := skillsCatalogHTTPClient
	t.Cleanup(func() {
		githubAPIBase = prevBase
		skillsCatalogHTTPClient = prevClient
		registryListCacheMu.Lock()
		registryListCache = map[string]registryListCacheEntry{}
		registryListCacheMu.Unlock()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/skills", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc("/repos/acme/skills/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []map[string]any{
				{"path": "skills/alpha/SKILL.md", "type": "blob"},
				{"path": "skills/beta/SKILL.md", "type": "blob"},
				{"path": "README.md", "type": "blob"},
				{"path": "skills/alpha/refs.md", "type": "blob"},
			},
			"truncated": false,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	githubAPIBase = srv.URL
	skillsCatalogHTTPClient = srv.Client()

	skills, err := listSkillsInRegistrySource("acme/skills")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("skills = %+v", skills)
	}
	if skills[0].SkillID != "alpha" || skills[1].SkillID != "beta" {
		t.Fatalf("order/ids: %+v", skills)
	}
	if skills[0].Source != "acme/skills" || skills[0].ID != "acme/skills/alpha" {
		t.Fatalf("skill0: %+v", skills[0])
	}
}

func TestSkillIDFromSkillMDPath(t *testing.T) {
	cases := []struct {
		path, repo, want string
	}{
		{"SKILL.md", "landing-page-design", "landing-page-design"},
		{"skills/SKILL.md", "landing-page-design", "landing-page-design"},
		{"skill/SKILL.md", "my-skill", "my-skill"},
		{"skills/git-commit/SKILL.md", "awesome-copilot", "git-commit"},
		{".github/skills/foo/SKILL.md", "repo", "foo"},
		{"packages/skills/bar/SKILL.md", "repo", "bar"},
		{"packages/skills/SKILL.md", "mono", "mono"},
	}
	for _, tc := range cases {
		got := skillIDFromSkillMDPath(tc.path, tc.repo)
		if got != tc.want {
			t.Fatalf("skillIDFromSkillMDPath(%q, %q) = %q, want %q", tc.path, tc.repo, got, tc.want)
		}
	}
}

func TestUninstallInstalledSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	agents := filepath.Join(home, ".agents")
	skillDir := filepath.Join(agents, "skills", "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":      &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/portal/disabled.json":      &fstest.MapFile{Data: []byte(`{}`)},
	}
	store, err := NewStore(presets, agentsyncOptions(agents))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertRegistrySkill(RegistrySkill{Name: "demo-skill", Source: "acme/pack", Skill: "demo-skill"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UninstallInstalledSkill("demo-skill"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("expected skill dir removed, stat err=%v", err)
	}
	reg, err := store.ReadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, sk := range reg.Skills {
		if sk.Name == "demo-skill" || sk.Skill == "demo-skill" {
			t.Fatalf("registry still has demo-skill: %+v", reg.Skills)
		}
	}
	if err := store.UninstallInstalledSkill("demo-skill"); err == nil {
		t.Fatal("expected error uninstalling missing skill")
	}
}

func TestPackageDirFromSkillPath(t *testing.T) {
	cases := map[string]string{
		"adversarial-review/SKILL.md": "adversarial-review",
		"skills/foo/SKILL.md":         "skills/foo",
		"ogilvy/SKILL.md":             "ogilvy",
		"skills/foo":                  "skills/foo",
		"":                            "",
	}
	for in, want := range cases {
		if got := packageDirFromSkillPath(in); got != want {
			t.Fatalf("packageDirFromSkillPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInstallSkillPackageFromGitHub(t *testing.T) {
	prevBase := githubAPIBase
	prevClient := skillsCatalogHTTPClient
	t.Cleanup(func() {
		githubAPIBase = prevBase
		skillsCatalogHTTPClient = prevClient
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/skills", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc("/repos/acme/skills/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []map[string]any{
				{"path": "broken-yaml/SKILL.md", "type": "blob"},
				{"path": "broken-yaml/extra.md", "type": "blob"},
				{"path": "other/SKILL.md", "type": "blob"},
			},
		})
	})
	mux.HandleFunc("/repos/acme/skills/contents/broken-yaml/SKILL.md", func(w http.ResponseWriter, r *http.Request) {
		// Invalid YAML description (unquoted colon) — same class as adversarial-review.
		content := "---\nname: broken-yaml\ndescription: review: assume broken\n---\n# body\n"
		_ = json.NewEncoder(w).Encode(map[string]any{"content": base64Encode(content), "encoding": "base64"})
	})
	mux.HandleFunc("/repos/acme/skills/contents/broken-yaml/extra.md", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"content": base64Encode("extra"), "encoding": "base64"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	githubAPIBase = srv.URL
	skillsCatalogHTTPClient = srv.Client()

	agents := t.TempDir()
	if err := installSkillPackageFromGitHub(agents, "acme/skills", "broken-yaml", "broken-yaml"); err != nil {
		t.Fatal(err)
	}
	skillMD := filepath.Join(agents, "skills", "broken-yaml", "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# body") {
		t.Fatalf("content: %s", data)
	}
	if _, err := os.Stat(filepath.Join(agents, "skills", "broken-yaml", "extra.md")); err != nil {
		t.Fatal(err)
	}
}

func TestListSkillsUsesFrontmatterNameForInstallID(t *testing.T) {
	// Folder ogilvy/ but YAML name: ogilvy-copywriting — npx skills matches the name.
	prevBase := githubAPIBase
	prevClient := skillsCatalogHTTPClient
	t.Cleanup(func() {
		githubAPIBase = prevBase
		skillsCatalogHTTPClient = prevClient
		registryListCacheMu.Lock()
		registryListCache = map[string]registryListCacheEntry{}
		registryListCacheMu.Unlock()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/boraoztunc/skills", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc("/repos/boraoztunc/skills/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []map[string]any{
				{"path": "ogilvy/SKILL.md", "type": "blob"},
				{"path": "adversarial-review/SKILL.md", "type": "blob"},
			},
		})
	})
	mux.HandleFunc("/repos/boraoztunc/skills/contents/ogilvy/SKILL.md", func(w http.ResponseWriter, r *http.Request) {
		content := "---\nname: ogilvy-copywriting\ndescription: ads\n---\n# body\n"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":  base64Encode(content),
			"encoding": "base64",
		})
	})
	mux.HandleFunc("/repos/boraoztunc/skills/contents/adversarial-review/SKILL.md", func(w http.ResponseWriter, r *http.Request) {
		content := "---\nname: adversarial-review\ndescription: review\n---\n# body\n"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":  base64Encode(content),
			"encoding": "base64",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	githubAPIBase = srv.URL
	skillsCatalogHTTPClient = srv.Client()

	skills, err := listSkillsInRegistrySource("boraoztunc/skills")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]CatalogSkill{}
	for _, sk := range skills {
		byID[sk.SkillID] = sk
	}
	if _, ok := byID["ogilvy"]; ok {
		t.Fatalf("should not use folder name ogilvy as skillId: %+v", skills)
	}
	og, ok := byID["ogilvy-copywriting"]
	if !ok {
		t.Fatalf("expected skillId ogilvy-copywriting, got %+v", skills)
	}
	if og.Path != "ogilvy/SKILL.md" {
		t.Fatalf("path: %+v", og)
	}
	if _, ok := byID["adversarial-review"]; !ok {
		t.Fatalf("expected adversarial-review: %+v", skills)
	}
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestListSkillsSingleSkillUnderSkillsFolder(t *testing.T) {
	// Repro: 2389-research/landing-page-design uses skills/SKILL.md
	prevBase := githubAPIBase
	prevClient := skillsCatalogHTTPClient
	t.Cleanup(func() {
		githubAPIBase = prevBase
		skillsCatalogHTTPClient = prevClient
		registryListCacheMu.Lock()
		registryListCache = map[string]registryListCacheEntry{}
		registryListCacheMu.Unlock()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/2389-research/landing-page-design", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc("/repos/2389-research/landing-page-design/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []map[string]any{
				{"path": "skills/SKILL.md", "type": "blob"},
				{"path": "README.md", "type": "blob"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	githubAPIBase = srv.URL
	skillsCatalogHTTPClient = srv.Client()

	skills, err := listSkillsInRegistrySource("2389-research/landing-page-design")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].SkillID != "landing-page-design" {
		t.Fatalf("expected skillId landing-page-design, got %+v", skills)
	}
}

func TestHandleSkillsRegistriesAndCatalog(t *testing.T) {
	prevBase := githubAPIBase
	prevClient := skillsCatalogHTTPClient
	t.Cleanup(func() {
		githubAPIBase = prevBase
		skillsCatalogHTTPClient = prevClient
		registryListCacheMu.Lock()
		registryListCache = map[string]registryListCacheEntry{}
		registryListCacheMu.Unlock()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/pack", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc("/repos/acme/pack/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []map[string]any{
				{"path": "foo/SKILL.md", "type": "blob"},
				{"path": "bar/SKILL.md", "type": "blob"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	githubAPIBase = srv.URL
	skillsCatalogHTTPClient = srv.Client()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json": &fstest.MapFile{Data: []byte(`{"skills":[
			{"name":"foo","source":"acme/pack","skill":"foo"},
			{"name":"gitbutler","skill":"but","installer":"but-skill"}
		]}`)},
		"presets/portal/disabled.json": &fstest.MapFile{Data: []byte(`{}`)},
	}
	ps, err := newPortalServer(presets, filepath.Join(home, ".agents"))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/registries", nil)
	rr := httptest.NewRecorder()
	ps.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("registries %d %s", rr.Code, rr.Body.String())
	}
	var regs RegistriesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &regs); err != nil {
		t.Fatal(err)
	}
	if len(regs.Registries) < 2 {
		t.Fatalf("registries: %+v", regs)
	}
	var acme *RegistrySource
	for i := range regs.Registries {
		if regs.Registries[i].Source == "acme/pack" {
			acme = &regs.Registries[i]
			break
		}
	}
	if acme == nil {
		t.Fatalf("acme/pack missing: %+v", regs.Registries)
	}
	// Registries endpoint stays cheap: skillCount is configured-entry bound
	// until /catalog is loaded (then UI updates count from catalog).
	if acme.EnabledEntries != 1 {
		t.Fatalf("acme enabledEntries = %d, want 1", acme.EnabledEntries)
	}
	if !acme.Listable {
		t.Fatal("acme/pack should be listable")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/skills/catalog?registry=acme/pack", nil)
	rr = httptest.NewRecorder()
	ps.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("catalog %d %s", rr.Code, rr.Body.String())
	}
	var cat CatalogListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Count != 2 {
		t.Fatalf("catalog: %+v", cat)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/skills/catalog?registry=acme/pack&q=foo", nil)
	rr = httptest.NewRecorder()
	ps.router().ServeHTTP(rr, req)
	if err := json.Unmarshal(rr.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Count != 1 || cat.Skills[0].SkillID != "foo" {
		t.Fatalf("filtered catalog: %+v", cat)
	}
}

func TestListSkillsIncludesInstalledHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	agents := filepath.Join(home, ".agents")
	skillDir := filepath.Join(agents, "skills", "from-registry")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: from-registry\ndescription: hello\n---\n# body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\ndescription: run\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":      &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/portal/disabled.json":      &fstest.MapFile{Data: []byte(`{}`)},
	}
	store, err := NewStore(presets, agentsyncOptions(agents))
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sk := range list {
		if sk.ID == "from-registry" {
			found = true
			if sk.Source != "installed" || sk.Description != "hello" {
				t.Fatalf("unexpected skill: %+v", sk)
			}
		}
	}
	if !found {
		t.Fatalf("installed skill missing from list: %+v", list)
	}

	read, err := store.ReadSkill("from-registry")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read.Content, "# body") || read.Source != "installed" {
		t.Fatalf("ReadSkill: %+v", read)
	}
}

func TestUpsertRegistrySkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":      &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/portal/disabled.json":      &fstest.MapFile{Data: []byte(`{}`)},
	}
	store, err := NewStore(presets, agentsyncOptions(filepath.Join(home, ".agents")))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertRegistrySkill(RegistrySkill{Name: "git-commit", Source: "github/awesome-copilot", Skill: "git-commit"}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.ReadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Skills) != 1 || reg.Skills[0].Source != "github/awesome-copilot" {
		t.Fatalf("registry: %+v", reg)
	}
}

func TestListSkillsRegistrySourceFromOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	agents := filepath.Join(home, ".agents")
	skillDir := filepath.Join(agents, "skills", "from-registry")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: from-registry\ndescription: hello\n---\n# body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	presets := fstest.MapFS{
		"presets/skills/execution/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: execution\ndescription: run\n---\n")},
		"presets/mcp/servers.json":          &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":      &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/portal/disabled.json":      &fstest.MapFile{Data: []byte(`{}`)},
	}
	store, err := NewStore(presets, agentsyncOptions(agents))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertRegistrySkill(RegistrySkill{
		Name:   "from-registry",
		Source: "acme/pack",
		Skill:  "from-registry",
	}); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	var found *Skill
	for i := range list {
		if list[i].ID == "from-registry" {
			found = &list[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("skill missing: %+v", list)
	}
	if found.RegistrySource != "acme/pack" {
		t.Fatalf("registrySource = %q, want acme/pack; skill=%+v", found.RegistrySource, found)
	}
}
