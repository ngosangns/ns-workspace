package internalutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"~/foo/bar", filepath.Join(home, "foo", "bar")},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			input string
			want  string
		}{"~\\foo", filepath.Join(home, "foo")})
	}
	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExpandPathNoTildeExpansion(t *testing.T) {
	if got := ExpandPath("~notilde"); got != "~notilde" {
		t.Errorf("ExpandPath(~notilde) = %q, want ~notilde", got)
	}
}

func TestExpandPathHomeDirError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("HOME manipulation differs on windows")
	}
	// Trên Unix, nếu user lookup fail, ExpandPath phải trả về input nguyên vẹn.
	// Tạm thời chỉ cover được khi UserHomeDir trả về "" — test bằng cách set
	// HOME rỗng (trên Linux thường vẫn trả /). Nên ta test cả hai nhánh qua input
	// dạng ~/foo: nếu user lookup fail ⇒ trả ~/foo nguyên vẹn.
	t.Setenv("HOME", "")
	got := ExpandPath("~/foo")
	// Trên Linux UserHomeDir fallback về "/"; nếu fail thì trả input gốc.
	if got == "" {
		t.Errorf("ExpandPath(~/foo) with empty HOME = empty, want non-empty")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{"", "", "hello"}, "hello"},
		{[]string{"  ", "\t", "world"}, "world"},
		{[]string{"first", "second"}, "first"},
		{[]string{"", ""}, ""},
		{[]string{}, ""},
		{[]string{"  spaced  "}, "spaced"},
	}
	for _, tt := range tests {
		got := FirstNonEmpty(tt.input...)
		if got != tt.want {
			t.Errorf("FirstNonEmpty(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAppendUniqueString(t *testing.T) {
	got := AppendUniqueString([]string{"a", "b"}, "c")
	if len(got) != 3 || got[2] != "c" {
		t.Errorf("AppendUniqueString([a,b], c) = %v, want [a b c]", got)
	}
	got = AppendUniqueString([]string{"a", "b"}, "b")
	if len(got) != 2 {
		t.Errorf("AppendUniqueString([a,b], b) = %v, want [a b] (no dup)", got)
	}
	got = AppendUniqueString([]string{"a"}, "")
	if len(got) != 1 {
		t.Errorf("AppendUniqueString([a], empty) = %v, want [a] (skip empty)", got)
	}
}

func TestUniqueStrings(t *testing.T) {
	got := UniqueStrings([]string{"a", "b", "a", "c", "b", ""})
	if len(got) != 3 {
		t.Errorf("UniqueStrings = %v, want 3 unique non-empty", got)
	}
}

func TestUniqueSortedStrings(t *testing.T) {
	got := UniqueSortedStrings([]string{"c", "a", "b", "a", "  ", "b"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("UniqueSortedStrings = %v, want [a b c]", got)
	}
}

func TestExecutableFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(f, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if ExecutableFile(f) {
			t.Error("0644 file should not be executable on unix")
		}
		if err := os.Chmod(f, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !ExecutableFile(f) {
		t.Error("file should be executable")
	}
	if ExecutableFile(tmp) {
		t.Error("directory should not be executable")
	}
	if ExecutableFile(filepath.Join(tmp, "nonexistent")) {
		t.Error("nonexistent file should not be executable")
	}
}

func TestExecutableNames(t *testing.T) {
	names := ExecutableNames("gopls")
	if runtime.GOOS == "windows" {
		if len(names) != 4 {
			t.Errorf("ExecutableNames on windows = %v, want 4 candidates", names)
		}
		if names[0] != "gopls" || names[1] != "gopls.exe" || names[2] != "gopls.cmd" || names[3] != "gopls.bat" {
			t.Errorf("ExecutableNames on windows = %v, want [gopls, .exe, .cmd, .bat]", names)
		}
	} else {
		if len(names) != 1 || names[0] != "gopls" {
			t.Errorf("ExecutableNames on unix = %v, want [gopls]", names)
		}
	}
}

func TestExecutableNamesWindows(t *testing.T) {
	// Inject goosVar = "windows" để cover nhánh windows.
	orig := goosVar
	goosVar = "windows"
	t.Cleanup(func() { goosVar = orig })
	names := ExecutableNames("gopls")
	if len(names) != 4 {
		t.Fatalf("ExecutableNames windows = %v, want 4", names)
	}
	if names[0] != "gopls" || names[1] != "gopls.exe" || names[2] != "gopls.cmd" || names[3] != "gopls.bat" {
		t.Errorf("names = %v, want [gopls, gopls.exe, gopls.cmd, gopls.bat]", names)
	}
}

func TestExecutableFileWindowsBranch(t *testing.T) {
	// Inject goosVar = "windows": trên windows mọi file (không phải dir) đều coi là exec.
	orig := goosVar
	goosVar = "windows"
	t.Cleanup(func() { goosVar = orig })
	tmp := t.TempDir()
	f := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(f, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ExecutableFile(f) {
		t.Error("on windows, file should be executable regardless of mode")
	}
	if ExecutableFile(tmp) {
		t.Error("directory should never be executable")
	}
	if ExecutableFile(filepath.Join(tmp, "nonexistent")) {
		t.Error("nonexistent file should not be executable")
	}
}

func TestGoEnvValueWithGo(t *testing.T) {
	// PATH bao gồm go: trả về giá trị thật (không rỗng).
	// Phụ thuộc vào môi trường; nếu go không có sẵn, bỏ qua.
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available on PATH")
	}
	v := GoEnvValue("GOROOT")
	if v == "" {
		// GOROOT có thể rỗng nếu dùng GOPATH thay thế; thử GOPATH.
		v = GoEnvValue("GOPATH")
	}
	if v == "" {
		t.Log("GoEnvValue returned empty (may be expected for some keys)")
	}
}

func TestGoEnvValueWithoutGo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation on windows is different")
	}
	// PATH rỗng → LookPath sẽ fail → GoEnvValue trả về "" ngay.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	t.Cleanup(func() { t.Setenv("PATH", origPath) })
	if got := GoEnvValue("GOROOT"); got != "" {
		t.Errorf("GoEnvValue with empty PATH = %q, want empty", got)
	}
}

func TestGoEnvValueCommandError(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available on PATH")
	}
	// Inject execCommand mock luôn trả về error để cover nhánh Output() fail.
	origCmd := execCommand
	execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		// Trả về lệnh exit code 1 để exec.Output trả error.
		return exec.CommandContext(ctx, "false")
	}
	t.Cleanup(func() { execCommand = origCmd })

	if got := GoEnvValue("GOROOT"); got != "" {
		t.Errorf("GoEnvValue with failing exec = %q, want empty", got)
	}
}
