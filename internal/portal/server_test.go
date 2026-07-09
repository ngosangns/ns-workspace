package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func newTestServer(t *testing.T) *portalServer {
	t.Helper()
	tmp := t.TempDir()
	fsys := fstest.MapFS{
		"presets/skills/commit/SKILL.md": &fstest.MapFile{Data: []byte("# commit\n")},
		"presets/mcp/servers.json":       &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/registry/skills.json":   &fstest.MapFile{Data: []byte(`{"skills":[]}`)},
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

// TestSyncStatusJobRetainedForLateStream reproduces the portal bug where
// status/doctor finished (and deleted the job) before the SSE client
// connected, so Status/Doctor produced no terminal output.
func TestSyncStatusJobRetainedForLateStream(t *testing.T) {
	fsys := fstest.MapFS{
		"presets/mcp/servers.json":     &fstest.MapFile{Data: []byte(`{"mcpServers":{}}`)},
		"presets/settings/claude.json": &fstest.MapFile{Data: []byte(`{}`)},
		"presets/agents/AGENTS.md":     &fstest.MapFile{Data: []byte("# agents\n")},
	}
	runner := NewSyncRunner(fsys)
	agentsDir := t.TempDir()

	id, err := runner.Start("status", agentsDir, "all")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until the fast command finishes (job.Done), simulating a slow
	// browser connecting after the work is complete.
	deadline := time.Now().Add(3 * time.Second)
	var job *syncJob
	for time.Now().Before(deadline) {
		job = runner.Job(id)
		if job == nil {
			t.Fatal("job was deleted before the SSE client could attach")
		}
		job.mu.Lock()
		done := job.Done
		job.mu.Unlock()
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if job == nil {
		t.Fatal("job missing after wait")
	}
	job.mu.Lock()
	if !job.Done {
		job.mu.Unlock()
		t.Fatal("status job did not finish in time")
	}
	if len(job.Lines) == 0 {
		job.mu.Unlock()
		t.Fatal("status job produced no report lines")
	}
	job.mu.Unlock()

	var got []string
	job.Subscribe(func(line string) { got = append(got, line) })
	if len(got) == 0 {
		t.Fatal("Subscribe after Done returned no lines")
	}
}

func TestHandleSyncStatusStreamReplaysLines(t *testing.T) {
	srv := newTestServer(t)

	// Start status via HTTP so the real stream path is exercised after the
	// job is already complete.
	req := httptest.NewRequest(http.MethodPost, "/api/sync/status", strings.NewReader(`{"command":"status","tools":"all"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("start status: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var started SyncJob
	if err := json.Unmarshal(rr.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal job: %v", err)
	}
	if started.ID == "" {
		t.Fatal("missing job id")
	}

	// Poll until the job is done so we connect "late" like a slow EventSource.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job := srv.runner.Job(started.ID)
		if job == nil {
			t.Fatal("job deleted before stream attach")
		}
		job.mu.Lock()
		done := job.Done
		n := len(job.Lines)
		job.mu.Unlock()
		if done && n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	streamReq := httptest.NewRequest(http.MethodGet, "/api/sync/stream?jobId="+started.ID, nil)
	streamRR := httptest.NewRecorder()
	srv.router().ServeHTTP(streamRR, streamReq)
	if streamRR.Code != http.StatusOK {
		t.Fatalf("stream: expected 200, got %d: %s", streamRR.Code, streamRR.Body.String())
	}
	body := streamRR.Body.String()
	if !strings.Contains(body, "event: start") || !strings.Contains(body, "event: end") {
		t.Fatalf("stream missing start/end events: %s", body)
	}
	if !strings.Contains(body, "data:") {
		t.Fatalf("stream missing data lines: %s", body)
	}
}

func TestJsonEscape(t *testing.T) {
	got := jsonEscape(`hello "world"`)
	want := `"hello \"world\""`
	if got != want {
		t.Fatalf("jsonEscape: got %q, want %q", got, want)
	}
}
