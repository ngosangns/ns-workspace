package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnableDisableSkillAndMCPAndAdapter(t *testing.T) {
	srv := newTestServer(t)

	// Disable skill
	req := httptest.NewRequest(http.MethodPost, "/api/skills/commit/enabled", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable skill: %d %s", rr.Code, rr.Body.String())
	}
	var skill Skill
	if err := json.Unmarshal(rr.Body.Bytes(), &skill); err != nil {
		t.Fatal(err)
	}
	if skill.Enabled {
		t.Fatal("expected skill disabled")
	}

	// List skills
	req = httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	var skills []Skill
	_ = json.Unmarshal(rr.Body.Bytes(), &skills)
	found := false
	for _, s := range skills {
		if s.ID == "commit" {
			found = true
			if s.Enabled {
				t.Fatal("list should show disabled")
			}
		}
	}
	if !found {
		t.Fatal("commit not listed")
	}

	// Seed an MCP then disable it
	req = httptest.NewRequest(http.MethodPut, "/api/mcps", strings.NewReader(`{"mcpServers":{"demo":{"command":"echo"}}}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put mcp: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/mcps/demo/enabled", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable mcp: %d %s", rr.Code, rr.Body.String())
	}
	var manifest MCPManifest
	if err := json.Unmarshal(rr.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest.MCPServers.MCPServers["demo"]; ok {
		t.Fatal("demo should not be in active map")
	}
	if _, ok := manifest.DisabledServers["demo"]; !ok {
		t.Fatalf("demo should be in disabledServers: %#v", manifest.DisabledServers)
	}
	// Disabled must still appear in items; content is the full catalog
	// (mcpServers + disabled[]) so demo remains editable in one document.
	foundItem := false
	for _, it := range manifest.Items {
		if it.Name == "demo" {
			foundItem = true
			if it.Enabled {
				t.Fatal("demo item should be disabled")
			}
		}
	}
	if !foundItem {
		t.Fatal("demo must remain in items list when disabled")
	}
	if !strings.Contains(manifest.Content, "demo") {
		t.Fatalf("unified content should include disabled demo:\n%s", manifest.Content)
	}
	if !strings.Contains(manifest.Content, `"disabled"`) {
		t.Fatalf("unified content should list disabled names:\n%s", manifest.Content)
	}
	if strings.Contains(manifest.Content, "//") {
		t.Fatalf("content should be pure JSON without comments:\n%s", manifest.Content)
	}

	// Re-enable MCP
	req = httptest.NewRequest(http.MethodPost, "/api/mcps/demo/enabled", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enable mcp: %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &manifest)
	if _, ok := manifest.MCPServers.MCPServers["demo"]; !ok {
		t.Fatal("demo should be active again")
	}

	// Disable a provider (may or may not be in registry depending on env)
	req = httptest.NewRequest(http.MethodPost, "/api/adapters/gemini/enabled", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable adapter: %d %s", rr.Code, rr.Body.String())
	}

	// Disable a registry skill → skills.disabled.json (not delete).
	req = httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get registry: %d %s", rr.Code, rr.Body.String())
	}
	var reg RegistrySkills
	if err := json.Unmarshal(rr.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}
	if len(reg.Skills) == 0 {
		t.Fatal("expected at least one registry skill in test fixtures")
	}
	regName := reg.Skills[0].Name
	req = httptest.NewRequest(http.MethodPost, "/api/registry/"+regName+"/enabled", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable registry skill: %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &reg)
	for _, sk := range reg.Skills {
		if sk.Name == regName {
			t.Fatalf("registry skill %q should not remain in enabled list", regName)
		}
	}
	foundDis := false
	for _, sk := range reg.DisabledSkills {
		if sk.Name == regName {
			foundDis = true
		}
	}
	if !foundDis {
		t.Fatalf("registry skill %q should be in disabledSkills: %#v", regName, reg.DisabledSkills)
	}
}
