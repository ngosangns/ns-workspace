package internalutil

import (
	"os"
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
	} else {
		if len(names) != 1 || names[0] != "gopls" {
			t.Errorf("ExecutableNames on unix = %v, want [gopls]", names)
		}
	}
}
