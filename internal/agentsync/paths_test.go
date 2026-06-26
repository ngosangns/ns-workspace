package agentsync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAgentsDirWithHomeEnv(t *testing.T) {
	t.Setenv("AGENTS_HOME", "~/my-agents")
	got, err := DefaultAgentsDir()
	if err != nil {
		t.Fatalf("DefaultAgentsDir failed: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "my-agents")
	if got != want {
		t.Errorf("DefaultAgentsDir = %q, want %q", got, want)
	}
}

func TestDefaultAgentsDirHomeError(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	orig := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("simulated home error") }
	t.Cleanup(func() { userHomeDir = orig })

	_, err := DefaultAgentsDir()
	if err == nil {
		t.Fatal("expected error when UserHomeDir fails")
	}
}

func TestDefaultUserConfigPathWithEnv(t *testing.T) {
	t.Setenv("NS_WORKSPACE_CONFIG", "~/my-config.json")
	got, err := DefaultUserConfigPath()
	if err != nil {
		t.Fatalf("DefaultUserConfigPath failed: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "my-config.json")
	if got != want {
		t.Errorf("DefaultUserConfigPath = %q, want %q", got, want)
	}
}

func TestDefaultUserConfigPathError(t *testing.T) {
	t.Setenv("NS_WORKSPACE_CONFIG", "")
	orig := userConfigDir
	userConfigDir = func() (string, error) { return "", errors.New("simulated configdir error") }
	t.Cleanup(func() { userConfigDir = orig })

	_, err := DefaultUserConfigPath()
	if err == nil {
		t.Fatal("expected error when UserConfigDir fails")
	}
}
