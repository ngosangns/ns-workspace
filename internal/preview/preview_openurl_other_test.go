//go:build !darwin && !windows

package preview

import (
	"os/exec"
	"testing"
)

// TestOpenURLDefault exercises the default (linux/bsd) branch of openURL.
// We override openURLCmdForTest with a stub so we don't need xdg-open.
func TestOpenURLDefault(t *testing.T) {
	orig := openURLCmdForTest
	defer func() { openURLCmdForTest = orig }()

	called := false
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		called = true
		if name != "xdg-open" {
			t.Errorf("expected command 'xdg-open', got %q", name)
		}
		if len(args) != 1 || args[0] != "http://example.com" {
			t.Errorf("unexpected args: %v", args)
		}
		return exec.Command("true")
	}

	if err := openURL("http://example.com"); err != nil {
		t.Errorf("openURL = %v", err)
	}
	if !called {
		t.Errorf("openURLCmdForTest was not called")
	}
}//go:build !darwin && !windows

package preview

import (
	"os/exec"
	"testing"
)

// TestOpenURLDefault exercises the default (linux/bsd) branch of openURL.
// We override openURLCmdForTest with a stub so we don't need xdg-open.
func TestOpenURLDefault(t *testing.T) {
	orig := openURLCmdForTest
	defer func() { openURLCmdForTest = orig }()

	called := false
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		called = true
		if name != "xdg-open" {
			t.Errorf("expected command 'xdg-open', got %q", name)
		}
		if len(args) != 1 || args[0] != "http://example.com" {
			t.Errorf("unexpected args: %v", args)
		}
		return exec.Command("true")
	}

	if err := openURL("http://example.com"); err != nil {
		t.Errorf("openURL = %v", err)
	}
	if !called {
		t.Errorf("openURLCmdForTest was not called")
	}
}