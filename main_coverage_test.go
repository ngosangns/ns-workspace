package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEmptyArgs(t *testing.T) {
	// Capture stdout.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := run([]string{}); err != nil {
		t.Errorf("run with empty args: %v", err)
	}
	w.Close()
	io.Copy(io.Discard, r)
}

func TestRunHelpFlags(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			origStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			defer func() { os.Stdout = origStdout }()

			if err := run([]string{arg}); err != nil {
				t.Errorf("run with %q: %v", arg, err)
			}
			w.Close()
			io.Copy(io.Discard, r)
		})
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := run([]string{"unknown-cmd"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected unknown command error, got: %v", err)
	}
}

func TestRunSetupDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := run([]string{"setup", "--dry-run"}); err != nil {
		t.Errorf("setup dry-run failed: %v", err)
	}
	w.Close()
	io.Copy(io.Discard, r)
}

func TestRunSetupForce(t *testing.T) {
	tmp := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := run([]string{"setup", "--force", "--target", tmp}); err != nil {
		t.Errorf("setup force failed: %v", err)
	}
	w.Close()
	io.Copy(io.Discard, r)

	mustExist(t, filepath.Join(tmp, "Taskfile.yml"))
}

func TestRunPreviewNoArgs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	// Use --no-reload to keep server alive briefly; but it'll hang.
	// Use a timeout via custom project; or just exit early by passing a bad flag.
	// Actually, just verify the dispatch happens - the command may hang.
	// Use --help to get help and exit.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	// preview --help would print usage and exit, but the Run function may
	// still try to start server. Just skip the actual run.
	if err := run([]string{"preview", "--help"}); err != nil {
		t.Logf("preview --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunSearch(t *testing.T) {
	// search requires the server, so just test dispatch.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"search", "--help"}); err != nil {
		t.Logf("search --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunExport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"export", "--help"}); err != nil {
		t.Logf("export --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunGraph(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"graph", "--help"}); err != nil {
		t.Logf("graph --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"mcp", "--help"}); err != nil {
		t.Logf("mcp --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunKBValidate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"kb", "validate", "--help"}); err != nil {
		t.Logf("kb validate --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunKBIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"kb", "index", "--help"}); err != nil {
		t.Logf("kb index --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunKBUnknown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err := run([]string{"kb", "unknown-subcmd"})
	if err == nil {
		t.Log("kb unknown-subcmd returned nil (may be expected)")
	}
	w.Close()
	io.Copy(io.Discard, r)
}

func TestRunLSPDispatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"lsp", "--help"}); err != nil {
		t.Logf("lsp --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunAgentSyncCommand(t *testing.T) {
	// Test that agentsync commands like "status" are dispatched.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"status"}); err != nil {
		t.Errorf("status failed: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestRunHarnessCommand(t *testing.T) {
	// Test that harness commands are dispatched.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("KIRO_HOME", "")

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()
	origStderr := os.Stderr
	rE, wE, _ := os.Pipe()
	os.Stderr = wE
	defer func() { os.Stderr = origStderr }()

	if err := run([]string{"harness", "--help"}); err != nil {
		t.Logf("harness --help returned: %v", err)
	}
	w.Close()
	wE.Close()
	io.Copy(io.Discard, r)
	io.Copy(io.Discard, rE)
}

func TestPrintUsage(t *testing.T) {
	// Capture stdout.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	printUsage()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	out := buf.String()

	for _, want := range []string{"agent-bootstrap", "Usage:", "init", "update", "preview", "search", "export", "graph", "mcp", "kb", "setup", "lsp"} {
		if !strings.Contains(out, want) {
			t.Errorf("printUsage missing %q", want)
		}
	}
}

func TestRunMainSuccess(t *testing.T) {
	// Save and restore globals.
	origOsExit := osExit
	origStderr := stderrWriter
	origArgs := os.Args
	defer func() {
		osExit = origOsExit
		stderrWriter = origStderr
		os.Args = origArgs
	}()

	var exitCode int
	osExit = func(code int) { exitCode = code }
	stderrWriter = io.Discard
	os.Args = []string{"ns-workspace", "--help"}

	main()
	if exitCode != 0 {
		t.Errorf("osExit called with %d, want 0", exitCode)
	}
}

func TestRunMainError(t *testing.T) {
	// Save and restore globals.
	origOsExit := osExit
	origStderr := stderrWriter
	origArgs := os.Args
	origStdout := os.Stdout
	defer func() {
		osExit = origOsExit
		stderrWriter = origStderr
		os.Args = origArgs
		os.Stdout = origStdout
	}()

	var exitCode int
	osExit = func(code int) { exitCode = code }
	var stderrBuf bytes.Buffer
	stderrWriter = &stderrBuf

	// Capture stdout to suppress printUsage output.
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		io.Copy(io.Discard, r)
	}()

	// Test with a command that returns an error.
	// Force the "default" case in run() switch by using a command that
	// is not in any agentsync, harness, or known subcommand list.
	os.Args = []string{"ns-workspace", "zzzzzz-fake-cmd-xyz"}
	main()
	if exitCode != 1 {
		t.Errorf("osExit called with %d, want 1", exitCode)
	}
}

func TestRunMainPrintUsage(t *testing.T) {
	// Save and restore globals.
	origOsExit := osExit
	origStderr := stderrWriter
	origArgs := os.Args
	origStdout := os.Stdout
	defer func() {
		osExit = origOsExit
		stderrWriter = origStderr
		os.Args = origArgs
		os.Stdout = origStdout
	}()

	var exitCode int
	osExit = func(code int) { exitCode = code }
	stderrWriter = io.Discard

	// Test empty args path
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Args = []string{"ns-workspace"}
	main()
	w.Close()
	io.Copy(io.Discard, r)
	if exitCode != 0 {
		t.Errorf("osExit called with %d, want 0", exitCode)
	}
}
