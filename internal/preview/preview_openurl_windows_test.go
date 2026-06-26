//go:build windows

package preview

import (
	"os/exec"
	"testing"
)

// TestOpenURLWindows exercises the windows branch of openURL. We override
// openURLCmdForTest with a stub so we don't spawn a real browser process.
func TestOpenURLWindows(t *testing.T) {
	orig := openURLCmdForTest
	defer func() { openURLCmdForTest = orig }()

	called := false
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		called = true
		if name != "rundll32" {
			t.Errorf("expected command 'rundll32', got %q", name)
		}
		if len(args) != 2 || args[0] != "url.dll,FileProtocolHandler" || args[1] != "http://example.com" {
			t.Errorf("unexpected args: %v", args)
		}
		return exec.Command("cmd", "/c", "exit", "0")
	}

	if err := openURL("http://example.com"); err != nil {
		t.Errorf("openURL = %v", err)
	}
	if !called {
		t.Errorf("openURLCmdForTest was not called")
	}
}