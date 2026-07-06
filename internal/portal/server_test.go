package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func newTestServer(t *testing.T) *portalServer {
	t.Helper()
	tmp := t.TempDir()
	fsys := fstest.MapFS{
		"presets/skills/commit/SKILL.md": &fstest.MapFile{Data: []byte("# commit\n")},
		"presets/mcp/servers.json": &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json": &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
		"presets/settings/claude.json":   &fstest.MapFile{Data: []byte(`{"permissions":{"defaultMode":"bypassPermissions"},"env":{}}`)},
	}
	srv, err := newPortalServer(fsys, tmp)
	if err != nil {
		t.Fatalf("newPortalServer: %v", err)
	}
	return srv
}

func TestHandleStatus(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var summary StatusSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if summary.AgentsDir != srv.agentsDir {
		t.Fatalf("unexpected agents dir: %s", summary.AgentsDir)
	}
}

func TestHandleSkillsCRUD(t *testing.T) {
	srv := newTestServer(t)

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var skills []Skill
	if err := json.Unmarshal(rr.Body.Bytes(), &skills); err != nil {
		t.Fatalf("unmarshal skills: %v", err)
	}
	if len(skills) != 1 || skills[0].ID != "commit" {
		t.Fatalf("unexpected skills: %+v", skills)
	}

	// Update
	body := `{"content":"# updated\n"}`
	req = httptest.NewRequest(http.MethodPut, "/api/skills/commit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var skill Skill
	if err := json.Unmarshal(rr.Body.Bytes(), &skill); err != nil {
		t.Fatalf("unmarshal skill: %v", err)
	}
	if !skill.Overridden || skill.Content != "# updated\n" {
		t.Fatalf("unexpected skill after update: %+v", skill)
	}

	// Reset
	req = httptest.NewRequest(http.MethodDelete, "/api/skills/commit", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("reset: expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleMCPs(t *testing.T) {
	srv := newTestServer(t)
	body := `{"mcpServers":{"ctx":{"type":"http","url":"https://example.com/mcp"}}}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcps", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put mcps: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/mcps", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get mcps: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var manifest MCPManifest
	if err := json.Unmarshal(rr.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("unmarshal mcps: %v", err)
	}
	if manifest.Servers()["ctx"] == nil {
		t.Fatal("expected ctx server")
	}
	if !manifest.Overridden {
		t.Fatal("expected manifest overridden after put")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/mcps/preset", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get mcps preset: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var preset MCPServers
	if err := json.Unmarshal(rr.Body.Bytes(), &preset); err != nil {
		t.Fatalf("unmarshal mcps preset: %v", err)
	}
	if preset.MCPServers["ctx"] != nil {
		t.Fatal("preset should not contain override server")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/mcps", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete mcps: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	manifest = MCPManifest{}
	if err := json.Unmarshal(rr.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("unmarshal mcps after reset: %v", err)
	}
	if manifest.Overridden {
		t.Fatal("expected manifest not overridden after reset")
	}
	if manifest.Servers()["ctx"] != nil {
		t.Fatal("reset should remove override server")
	}
}

func TestHandleClaudeSettings(t *testing.T) {
	srv := newTestServer(t)
	body := `{"permissions":{"defaultMode":"bypassPermissions"},"env":{"ANTHROPIC_BASE_URL":"https://router.example.com/anthropic","ANTHROPIC_MODEL":"anthropic/claude-opus-4"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/claude", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put claude settings: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var settings ClaudeSettings
	if err := json.Unmarshal(rr.Body.Bytes(), &settings); err != nil {
		t.Fatalf("unmarshal claude settings: %v", err)
	}
	if settings.Env.AnthropicBaseURL != "https://router.example.com/anthropic" {
		t.Fatalf("unexpected base url: %q", settings.Env.AnthropicBaseURL)
	}
	if !settings.Overridden {
		t.Fatal("expected claude settings overridden after put")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/settings/claude/preset", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get claude preset: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var preset ClaudeSettings
	if err := json.Unmarshal(rr.Body.Bytes(), &preset); err != nil {
		t.Fatalf("unmarshal claude preset: %v", err)
	}
	if preset.Env.AnthropicBaseURL != "" {
		t.Fatal("preset should not contain overlay env values")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/settings/claude", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete claude settings: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	settings = ClaudeSettings{}
	if err := json.Unmarshal(rr.Body.Bytes(), &settings); err != nil {
		t.Fatalf("unmarshal claude settings after reset: %v", err)
	}
	if settings.Overridden {
		t.Fatal("expected claude settings not overridden after reset")
	}
	if settings.Env.AnthropicBaseURL != "" {
		t.Fatal("reset should remove overlay env values")
	}
}

func TestHandleRegistry(t *testing.T) {
	srv := newTestServer(t)
	body := `{"skills":[{"name":"new","source":"org/repo","skill":"new"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/registry", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put registry: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get registry: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var reg RegistrySkills
	if err := json.Unmarshal(rr.Body.Bytes(), &reg); err != nil {
		t.Fatalf("unmarshal registry: %v", err)
	}
	if len(reg.Skills) != 1 || reg.Skills[0].Name != "new" {
		t.Fatalf("unexpected registry: %+v", reg)
	}
}

func TestSyncReporter(t *testing.T) {
	_ = NewSyncRunner(fstest.MapFS{})
	job := &syncJob{ID: "test", Command: "test"}
	job.cond = sync.NewCond(&job.mu)
	rep := syncReporter{job: job}
	rep.Line("hello %s", "world")
	if len(job.Lines) != 1 || job.Lines[0] != "hello world" {
		t.Fatalf("unexpected lines: %v", job.Lines)
	}
}

func TestJsonEscape(t *testing.T) {
	got := jsonEscape(`hello "world"`)
	want := `"hello \"world\""`
	if got != want {
		t.Fatalf("jsonEscape: got %q, want %q", got, want)
	}
}
