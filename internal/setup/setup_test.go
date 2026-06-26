package setup

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// runSetupFromDir chạy Run() thật từ một working directory giả lập (thông qua
// override target) và trả về stdout output. Dùng để test end-to-end mà không
// cần phụ thuộc vào os.Getwd() thật.
func runSetupFromDir(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"--target", dir}, args...)
	var buf bytes.Buffer
	err := runWithWriter(full, &buf)
	return buf.String(), err
}

func TestRunWritesNewTaskfile(t *testing.T) {
	dir := t.TempDir()
	out, err := runSetupFromDir(t, dir)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// stdout phải có thông báo ghi file.
	if !strings.Contains(out, "wrote") || !strings.Contains(out, "Taskfile.yml") {
		t.Errorf("stdout should mention writing Taskfile.yml, got: %q", out)
	}

	// File phải tồn tại.
	path := filepath.Join(dir, "Taskfile.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Taskfile.yml was not created: %v", err)
	}

	// Phải chứa tất cả task mặc định.
	body := string(data)
	for _, name := range ScriptNames() {
		if !strings.Contains(body, name+":") {
			t.Errorf("Taskfile.yml is missing task %q", name)
		}
	}
}

func TestRunGeneratesValidYAML(t *testing.T) {
	dir := t.TempDir()
	if _, err := runSetupFromDir(t, dir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Taskfile.yml is not valid YAML: %v\n---\n%s", err, data)
	}

	// Kiểm tra version được set.
	if v, ok := parsed["version"].(string); !ok || v == "" {
		t.Errorf("expected version to be a non-empty string, got %v", parsed["version"])
	}

	// Kiểm tra tasks là map và có đủ số lượng.
	tasks, ok := parsed["tasks"].(map[string]any)
	if !ok {
		t.Fatalf("expected tasks to be a map, got %T", parsed["tasks"])
	}
	if got, want := len(tasks), len(defaultScripts); got != want {
		t.Errorf("expected %d tasks, got %d", want, got)
	}

	// Kiểm tra cấu trúc của một task cụ thể.
	nsStatus, ok := tasks["ns:status"].(map[string]any)
	if !ok {
		t.Fatalf("ns:status task should be a map, got %T", tasks["ns:status"])
	}
	desc, _ := nsStatus["desc"].(string)
	if desc == "" {
		t.Errorf("ns:status should have a desc, got %q", desc)
	}
	cmds, ok := nsStatus["cmds"].([]any)
	if !ok || len(cmds) == 0 {
		t.Fatalf("ns:status should have non-empty cmds, got %v", nsStatus["cmds"])
	}
	cmd0, _ := cmds[0].(string)
	if !strings.Contains(cmd0, "ns-workspace@latest") || !strings.Contains(cmd0, "status") {
		t.Errorf("ns:status cmd should invoke ns-workspace status, got %q", cmd0)
	}
}

func TestRunMergesAndRewritesExistingTasks(t *testing.T) {
	dir := t.TempDir()
	// File có sẵn: 1 custom task (giữ lại) + 1 task trùng tên (sẽ bị rewrite).
	existing := `version: "3"
vars:
  CUSTOM_VAR: hello
tasks:
  ns:status:
    desc: USER OVERRIDE - this should be replaced
    cmds:
      - echo user-custom-command
  my:custom:
    desc: A user-defined task
    cmds:
      - echo custom
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetupFromDir(t, dir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("re-parsing failed: %v", err)
	}

	tasks := parsed["tasks"].(map[string]any)

	// 1. Custom task phải còn nguyên.
	custom, ok := tasks["my:custom"].(map[string]any)
	if !ok {
		t.Fatalf("user-defined task my:custom was lost: %v", tasks)
	}
	if desc, _ := custom["desc"].(string); !strings.Contains(desc, "user-defined") {
		t.Errorf("my:custom desc was modified: %q", desc)
	}

	// 2. ns:status phải bị rewrite (không còn "USER OVERRIDE").
	nsStatus, ok := tasks["ns:status"].(map[string]any)
	if !ok {
		t.Fatalf("ns:status missing: %v", tasks)
	}
	if desc, _ := nsStatus["desc"].(string); strings.Contains(desc, "USER OVERRIDE") {
		t.Errorf("ns:status should have been rewritten, got desc=%q", desc)
	}
	if desc, _ := nsStatus["desc"].(string); !strings.Contains(desc, "ns-workspace") {
		t.Errorf("ns:status desc should be the preset description, got %q", desc)
	}

	// 3. Top-level var CUSTOM_VAR phải được giữ nguyên.
	vars, ok := parsed["vars"].(map[string]any)
	if !ok {
		t.Fatalf("vars was lost: %v", parsed)
	}
	if cv, _ := vars["CUSTOM_VAR"].(string); cv != "hello" {
		t.Errorf("CUSTOM_VAR was modified: %q", cv)
	}

	// 4. Tất cả default scripts phải có mặt.
	for _, name := range ScriptNames() {
		if _, ok := tasks[name]; !ok {
			t.Errorf("missing task %q after merge", name)
		}
	}
}

func TestRunForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	// File có sẵn với custom task mà --force sẽ xoá.
	existing := `version: "3"
tasks:
  my:custom:
    desc: User task that should disappear
    cmds:
      - echo bye
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetupFromDir(t, dir, "--force"); err != nil {
		t.Fatalf("setup --force failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, "my:custom") {
		t.Errorf("--force should have removed custom task, body:\n%s", body)
	}

	// Vẫn phải có toàn bộ default scripts.
	for _, name := range ScriptNames() {
		if !strings.Contains(body, name+":") {
			t.Errorf("missing default task %q after --force", name)
		}
	}
}

func TestRunDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	out, err := runSetupFromDir(t, dir, "--dry-run")
	if err != nil {
		t.Fatalf("setup --dry-run failed: %v", err)
	}

	// File không được tạo.
	path := filepath.Join(dir, "Taskfile.yml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write file, but %s exists (err=%v)", path, err)
	}

	// Output phải chứa YAML hợp lệ.
	if !strings.Contains(out, "tasks:") {
		t.Errorf("dry-run output should contain YAML body, got: %q", out)
	}
	if !strings.Contains(out, "ns:status:") {
		t.Errorf("dry-run output should list default tasks, got: %q", out)
	}
}

func TestRunRespectsTargetFlag(t *testing.T) {
	parent := t.TempDir()
	subdir := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetupFromDir(t, parent, "--target", subdir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// File phải ở subdir, không ở parent.
	if _, err := os.Stat(filepath.Join(subdir, "Taskfile.yml")); err != nil {
		t.Fatalf("Taskfile.yml not written to --target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "Taskfile.yml")); !os.IsNotExist(err) {
		t.Errorf("Taskfile.yml should NOT be at parent level")
	}
}

func TestRunContainsExpectedTasks(t *testing.T) {
	dir := t.TempDir()
	if _, err := runSetupFromDir(t, dir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)

	expectedTasks := []string{
		"ns:status", "ns:doctor", "ns:init", "ns:update",
		"ns:preview", "ns:search", "ns:export", "ns:graph", "ns:mcp",
		"ns:kb:validate", "ns:kb:index", "ns:lsp:list", "ns:lsp:install",
		"ns:harness:list", "ns:harness:run",
		"lint:docs", "lint:docs:fix", "lint:preview", "lint:preview:fix",
		"format:docs", "format:docs:check", "format:preview", "format:preview:check",
		"build:preview",
		"go:build", "go:test",
	}
	for _, name := range expectedTasks {
		if !strings.Contains(body, name+":") {
			t.Errorf("missing expected task %q in Taskfile.yml", name)
		}
	}

	// Đếm tổng số task phải đúng.
	if got, want := len(ScriptNames()), 26; got != want {
		t.Errorf("DefaultScripts length = %d, want %d", got, want)
	}
}

func TestScriptNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range defaultScripts {
		if seen[s.Name] {
			t.Errorf("duplicate task name: %q", s.Name)
		}
		seen[s.Name] = true
		if s.Name == "" {
			t.Error("script has empty Name")
		}
		if s.Description == "" {
			t.Errorf("script %q has empty Description", s.Name)
		}
		if len(s.Commands) == 0 {
			t.Errorf("script %q has no Commands", s.Name)
		}
	}
}

func TestScriptNamesAreSorted(t *testing.T) {
	// ScriptNames() phải trả về alphabet cho documentation.
	got := ScriptNames()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("ScriptNames() not sorted at index %d: %q >= %q", i, got[i-1], got[i])
		}
	}
}

func TestRunHandlesMissingValueFlag(t *testing.T) {
	dir := t.TempDir()
	_, err := runSetupFromDir(t, dir, "--target")
	if err == nil {
		t.Fatal("expected error for --target without value")
	}
	// Go flag trả về "flag needs an argument: -target" được wrap với "setup:"
	if !strings.Contains(err.Error(), "flag needs an argument") &&
		!strings.Contains(err.Error(), "setup:") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Tests targeting 100% coverage ---

func TestRunEntryPoint(t *testing.T) {
	// Gọi Run() thật (entry point) thay vì wrapper, để cover entry point.
	dir := t.TempDir()
	// Chuyển cwd sang dir để --target mặc định = dir.
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })

	if err := Run([]string{"--dry-run"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestRunHelp(t *testing.T) {
	// -h / --help / help phải in usage ra stdout và trả về nil error.
	var buf bytes.Buffer
	if err := runWithWriter([]string{"-h"}, &buf); err != nil {
		t.Fatalf("Run -h should not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Usage: setup") {
		t.Errorf("help output should contain usage, got: %q", out)
	}
}

func TestRunLongHelp(t *testing.T) {
	var buf bytes.Buffer
	if err := runWithWriter([]string{"--help"}, &buf); err != nil {
		t.Fatalf("Run --help should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage: setup") {
		t.Errorf("--help should print usage, got: %q", buf.String())
	}
}

func TestRunInvalidFlag(t *testing.T) {
	// Flag không hợp lệ phải trả về error.
	var buf bytes.Buffer
	err := runWithWriter([]string{"--unknown-flag"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if !strings.Contains(err.Error(), "setup:") {
		t.Errorf("error should be prefixed with 'setup:', got: %v", err)
	}
}

func TestWriteTaskfileInvalidTargetPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows path semantics differ")
	}
	// Trên Unix, /dev/null/foo không tạo được ⇒ MkdirAll fail.
	var buf bytes.Buffer
	err := writeTaskfile("/dev/null/foo", defaultScripts, false, false, &buf)
	if err == nil {
		t.Fatal("expected error when target cannot be created")
	}
	// Có thể fail ở mkdir (cannot create) hoặc ở read (cannot read existing).
	if !strings.Contains(err.Error(), "cannot create target directory") &&
		!strings.Contains(err.Error(), "cannot read existing Taskfile.yml") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteTaskfileInvalidYAMLExisting(t *testing.T) {
	// Tạo Taskfile.yml với nội dung YAML không hợp lệ để cover nhánh parse error.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte("::: invalid :::\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err := writeTaskfile(dir, defaultScripts, false, false, &buf)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "cannot parse existing Taskfile.yml") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteTaskfileExistingTasksNotMap(t *testing.T) {
	// `tasks:` tồn tại nhưng là null/list (không phải map) → phải reset.
	dir := t.TempDir()
	// tasks: null  ⇒ parse thành nil
	existing := "version: \"3\"\ntasks: null\n"
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeTaskfile(dir, defaultScripts, false, false, &buf); err != nil {
		t.Fatalf("setup should recover from tasks: null, got: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output invalid: %v", err)
	}
	tasks, ok := parsed["tasks"].(map[string]any)
	if !ok || len(tasks) != len(defaultScripts) {
		t.Errorf("tasks should be reset to map of size %d, got: %v", len(defaultScripts), parsed["tasks"])
	}
}

func TestWriteTaskfileExistingVersionKept(t *testing.T) {
	// Nếu file đã có version, không ghi đè.
	dir := t.TempDir()
	existing := "version: \"2\"\ntasks: {}\n"
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeTaskfile(dir, defaultScripts, false, false, &buf); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if v, _ := parsed["version"].(string); v != "2" {
		t.Errorf("version should be preserved as '2', got %q", v)
	}
}

func TestWriteTaskfileEmptyFile(t *testing.T) {
	// File trống → readExistingTaskfile trả về map rỗng → phải set version 3.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeTaskfile(dir, defaultScripts, false, false, &buf); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if v, _ := parsed["version"].(string); v != "3" {
		t.Errorf("version should be '3', got %q", v)
	}
}

func TestWriteTaskfileDryRunOutput(t *testing.T) {
	// Dry-run phải in header + YAML ra stdout.
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := writeTaskfile(dir, defaultScripts, true, false, &buf); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "# Generated by ns-workspace setup.") {
		t.Errorf("dry-run should start with header, got: %q", out[:min(80, len(out))])
	}
	if !strings.Contains(out, "ns:status:") {
		t.Errorf("dry-run should include default tasks, got: %q", out)
	}
}

func TestReadExistingTaskfileUnreadable(t *testing.T) {
	// File không đọc được. Trên Unix, chmod 000 sẽ ngăn read (nếu không phải owner root).
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod 000 ineffective on windows / as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "Taskfile.yml")
	if err := os.WriteFile(path, []byte("version: \"3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	_, err := readExistingTaskfile(path)
	if err == nil {
		t.Fatal("expected error when file unreadable")
	}
	if !strings.Contains(err.Error(), "cannot read existing Taskfile.yml") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadExistingTaskfileNilParsed(t *testing.T) {
	// YAML parse được nhưng trả về nil map (vd: file chỉ chứa "---" + EOF).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte("---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readExistingTaskfile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read should not error on nil parsed: %v", err)
	}
	if got == nil {
		t.Error("readExistingTaskfile should return non-nil empty map")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got: %v", got)
	}
}

func TestToStringSliceEmpty(t *testing.T) {
	got := toStringSlice(nil)
	if len(got) != 0 {
		t.Errorf("toStringSlice(nil) = %v, want empty", got)
	}
	got = toStringSlice([]string{})
	if len(got) != 0 {
		t.Errorf("toStringSlice([]) = %v, want empty", got)
	}
}

func TestDefaultScriptsReturnsCopy(t *testing.T) {
	// DefaultScripts phải trả về bản copy — sửa bản trả về không ảnh hưởng list gốc.
	got := DefaultScripts()
	if len(got) != len(defaultScripts) {
		t.Fatalf("DefaultScripts length = %d, want %d", len(got), len(defaultScripts))
	}
	// Đổi tên task đầu tiên.
	got[0].Name = "modified"
	// Đổi Commands của task đầu tiên.
	got[0].Commands[0] = "modified"

	// defaultScripts phải còn nguyên.
	if defaultScripts[0].Name == "modified" {
		t.Error("DefaultScripts returned a shared slice (Name mutation leaked)")
	}
	if defaultScripts[0].Commands[0] == "modified" {
		t.Error("DefaultScripts returned a shared Commands slice")
	}

	// Gọi lần nữa phải trả về bản copy tươi.
	got2 := DefaultScripts()
	if got2[0].Name == "modified" {
		t.Error("DefaultScripts is not idempotent")
	}
}

func TestRunParseErrorSurfaces(t *testing.T) {
	// Nhánh error path không phải ErrHelp trong runWithWriter: flag unknown.
	var buf bytes.Buffer
	err := runWithWriter([]string{"--bogus"}, &buf)
	if err == nil {
		t.Fatal("expected error for bogus flag")
	}
	// Confirm error không phải nil (cover nhánh return fmt.Errorf("setup: %w", err)).
	if !strings.Contains(err.Error(), "setup:") {
		t.Errorf("error should be prefixed with 'setup:', got: %v", err)
	}
}

func TestRunWithWriterGetwdError(t *testing.T) {
	// Inject mock getwdFunc để simulate Getwd fail.
	origGetwd := getwdFunc
	getwdFunc = func() (string, error) {
		return "", errors.New("simulated getwd failure")
	}
	t.Cleanup(func() { getwdFunc = origGetwd })

	var buf bytes.Buffer
	err := runWithWriter([]string{}, &buf)
	if err == nil {
		t.Fatal("expected error when Getwd fails")
	}
	if !strings.Contains(err.Error(), "cannot determine working directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteTaskfileAbsError(t *testing.T) {
	// Inject mock systemOps với abs luôn fail.
	ops := systemOps{
		abs: func(string) (string, error) {
			return "", errors.New("simulated abs failure")
		},
		mkdirAll:    defaultOps.mkdirAll,
		writeFile:   defaultOps.writeFile,
		yamlMarshal: defaultOps.yamlMarshal,
	}
	var buf bytes.Buffer
	err := writeTaskfileWithOps("/some/path", defaultScripts, false, false, &buf, ops)
	if err == nil {
		t.Fatal("expected error when Abs fails")
	}
	if !strings.Contains(err.Error(), "cannot resolve target") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteTaskfileMkdirError(t *testing.T) {
	ops := systemOps{
		abs: defaultOps.abs,
		mkdirAll: func(string, os.FileMode) error {
			return errors.New("simulated mkdir failure")
		},
		writeFile:   defaultOps.writeFile,
		yamlMarshal: defaultOps.yamlMarshal,
	}
	var buf bytes.Buffer
	err := writeTaskfileWithOps(t.TempDir(), defaultScripts, false, false, &buf, ops)
	if err == nil {
		t.Fatal("expected error when MkdirAll fails")
	}
	if !strings.Contains(err.Error(), "cannot create target directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteTaskfileWriteError(t *testing.T) {
	ops := systemOps{
		abs:         defaultOps.abs,
		mkdirAll:    defaultOps.mkdirAll,
		writeFile:   func(string, []byte, os.FileMode) error { return errors.New("simulated write failure") },
		yamlMarshal: defaultOps.yamlMarshal,
	}
	var buf bytes.Buffer
	err := writeTaskfileWithOps(t.TempDir(), defaultScripts, false, false, &buf, ops)
	if err == nil {
		t.Fatal("expected error when WriteFile fails")
	}
	if !strings.Contains(err.Error(), "cannot write") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteTaskfileMarshalError(t *testing.T) {
	// Inject yaml.Marshal mock để simulate unmarshalable type.
	ops := systemOps{
		abs:         defaultOps.abs,
		mkdirAll:    defaultOps.mkdirAll,
		writeFile:   defaultOps.writeFile,
		yamlMarshal: func(any) ([]byte, error) { return nil, errors.New("simulated marshal failure") },
	}
	var buf bytes.Buffer
	err := writeTaskfileWithOps(t.TempDir(), defaultScripts, false, false, &buf, ops)
	if err == nil {
		t.Fatal("expected error when yaml.Marshal fails")
	}
	if !strings.Contains(err.Error(), "cannot marshal") {
		t.Errorf("unexpected error: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
