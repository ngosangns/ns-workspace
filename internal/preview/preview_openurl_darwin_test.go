//go:build darwin

package preview

import (
	"os/exec"
	"testing"
)

// TestOpenURLDarwin exercises the darwin branch of openURL. We override
// openURLCmdForTest with a stub so we don't spawn a real browser process.
func TestOpenURLDarwin(t *testing.T) {
	orig := openURLCmdForTest
	defer func() { openURLCmdForTest = orig }()

	called := false
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		called = true
		if name != "open" {
			t.Errorf("expected command 'open', got %q", name)
		}
		if len(args) != 1 || args[0] != "http://example.com" {
			t.Errorf("unexpected args: %v", args)
		}
		// Return a command that does nothing.
		return exec.Command("true")
	}

	if err := openURL("http://example.com"); err != nil {
		t.Errorf("openURL = %v", err)
	}
	if !called {
		t.Errorf("openURLCmdForTest was not called")
	}
}