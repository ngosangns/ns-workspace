package agentsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePiAgentDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", "")
	if got := resolvePiAgentDir(home, ""); got != filepath.Join(home, ".pi", "agent") {
		t.Fatalf("default = %s", got)
	}
	explicit := filepath.Join(home, "custom-pi")
	if got := resolvePiAgentDir(home, explicit); got != explicit {
		t.Fatalf("explicit = %s", got)
	}
	envHome := filepath.Join(home, "from-env")
	t.Setenv("PI_CODING_AGENT_DIR", envHome)
	if got := resolvePiAgentDir(home, ""); got != envHome {
		t.Fatalf("env = %s want %s", got, envHome)
	}
	if got := resolvePiAgentDir(home, explicit); got != explicit {
		t.Fatalf("explicit over env = %s", got)
	}
}

func TestPiAdapterLinksAgentsMD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")

	mgr := Manager{Presets: os.DirFS("../..")}
	opt := Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("pi"),
		Force:      true,
	}
	if err := mgr.Apply(opt, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	agents := filepath.Join(home, ".pi", "agent", "AGENTS.md")
	data, err := os.ReadFile(agents)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(data), "Workflow / dev") {
		snippet := string(data)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("AGENTS.md missing expected content: %s", snippet)
	}
	// Skills must not be mirrored under ~/.pi/agent/skills by this adapter.
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "skills", "harness", "SKILL.md")); err == nil {
		t.Fatal("pi adapter should not mirror skills into ~/.pi/agent/skills")
	}
	// Shared skills home still materializes.
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "harness", "SKILL.md")); err != nil {
		t.Fatalf("shared skills missing: %v", err)
	}
}

func TestPiAdapterRespectsPiCodingAgentDir(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(home, "alt-pi")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", custom)

	mgr := Manager{Presets: os.DirFS("../..")}
	if err := mgr.Apply(Options{
		Command:    "init",
		AgentsDir:  filepath.Join(home, ".agents"),
		NoRegistry: true,
		ToolFilter: ParseTools("pi"),
		Force:      true,
	}, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, err := os.Stat(filepath.Join(custom, "AGENTS.md")); err != nil {
		t.Fatalf("expected AGENTS.md under PI_CODING_AGENT_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "AGENTS.md")); err == nil {
		t.Fatal("should not write default ~/.pi/agent when PI_CODING_AGENT_DIR set")
	}
}

func TestPiAdapterAliases(t *testing.T) {
	reg := NewAdapterRegistry(RegistryOptions{Home: "/home"})
	for _, id := range []string{"pi", "pi-agent", "pi-coding-agent"} {
		if reg.Lookup(id) == nil {
			t.Fatalf("missing lookup for %s", id)
		}
	}
	a := reg.Lookup("pi")
	caps := a.Capabilities()
	if caps.Tier != TierStable {
		t.Fatalf("tier = %s", caps.Tier)
	}
	hasInstr := false
	for _, art := range caps.Artifacts {
		if art == ArtifactMCP {
			t.Fatal("pi should not claim MCP artifact")
		}
		if art == ArtifactInstructions {
			hasInstr = true
		}
		if art == ArtifactSkills {
			t.Fatal("pi should not mirror skills artifact")
		}
	}
	if !hasInstr {
		t.Fatal("pi should expose instructions artifact")
	}
}

func TestPiAdapterExplicitPiAgentDir(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(home, "explicit-pi")
	reg := NewAdapterRegistry(RegistryOptions{Home: home, PiAgentDir: custom})
	a := reg.Lookup("pi")
	if a == nil {
		t.Fatal("missing pi")
	}
	paths := a.StatusPaths(Context{
		Options: Options{AgentsDir: filepath.Join(home, ".agents")},
		Home:    home,
	})
	found := false
	for _, p := range paths {
		if p == filepath.Join(custom, "AGENTS.md") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("StatusPaths missing custom AGENTS.md, got %v", paths)
	}
}
